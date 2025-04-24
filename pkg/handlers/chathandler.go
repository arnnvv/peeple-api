package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now, tighten in production
		return true
	},
}

type Client struct {
	conn   *websocket.Conn
	userID int32
}

var (
	clients   = make(map[int32]*Client)
	clientsMu sync.RWMutex
)

type ChatWsMessage struct {
	Type             string `json:"type"` // "text", "image", "video", "audio", "error", "info"
	RecipientUserID  int32  `json:"recipient_user_id,omitempty"`
	Text             string `json:"text,omitempty"`
	MediaURL         string `json:"media_url,omitempty"`
	MediaType        string `json:"media_type,omitempty"`
	SenderUserID     int32  `json:"sender_user_id,omitempty"`
	Content          string `json:"content,omitempty"`
	ReplyToMessageID *int64 `json:"reply_to_message_id,omitempty"`
}

func ChatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: ChatHandler: Database connection not available.")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		log.Println("ERROR: ChatHandler: Authentication claims missing or invalid.")
		http.Error(w, "Unauthorized", http.StatusUnauthorized) // Try to send error before upgrade
		return
	}
	userID := int32(claims.UserID)
	log.Printf("INFO: ChatHandler: User %d attempting WebSocket upgrade.", userID)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERROR: ChatHandler: WebSocket upgrade failed for user %d: %v", userID, err)
		return
	}
	defer conn.Close()

	client := &Client{conn: conn, userID: userID}
	clientsMu.Lock()
	if oldClient, exists := clients[userID]; exists {
		log.Printf("WARN: ChatHandler: Closing existing connection for user %d.", userID)
		oldClient.conn.Close()
	}
	clients[userID] = client
	clientsMu.Unlock()
	log.Printf("INFO: ChatHandler: User %d connected via WebSocket.", userID)

	// Cleanup on exit
	defer func() {
		clientsMu.Lock()
		if currentClient, exists := clients[userID]; exists && currentClient.conn == conn {
			delete(clients, userID)
			log.Printf("INFO: ChatHandler: User %d disconnected.", userID)
		} // else connection was already replaced or removed
		clientsMu.Unlock()
	}()

	// Send connection confirmation
	_ = conn.WriteJSON(ChatWsMessage{Type: "info", Content: fmt.Sprintf("Connected as user %d", userID)})

	for {
		var msg ChatWsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("ERROR: ChatHandler: Read error for user %d: %v", userID, err)
			} else {
				log.Printf("INFO: ChatHandler: WebSocket closed for user %d (Reason: %v)", userID, err)
			}
			break
		}

		log.Printf("DEBUG: ChatHandler: Message received from %d: Type=%s, To=%d, ReplyTo=%v", userID, msg.Type, msg.RecipientUserID, msg.ReplyToMessageID)

		if msg.RecipientUserID <= 0 || msg.RecipientUserID == userID {
			log.Printf("WARN: ChatHandler: Invalid recipient_user_id %d from user %d.", msg.RecipientUserID, userID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Invalid recipient_user_id."})
			continue
		}

		createParams := migrations.CreateChatMessageParams{
			SenderUserID:     userID,
			RecipientUserID:  msg.RecipientUserID,
			MessageText:      pgtype.Text{Valid: false},
			MediaUrl:         pgtype.Text{Valid: false},
			MediaType:        pgtype.Text{Valid: false},
			ReplyToMessageID: pgtype.Int8{Valid: false},
		}

		switch msg.Type {
		case "text":
			trimmedText := strings.TrimSpace(msg.Text)
			if trimmedText == "" || len(msg.Text) > 500 {
				log.Printf("WARN: ChatHandler: Invalid text message from user %d (empty or too long).", userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Invalid text message content or length."})
				continue
			}
			createParams.MessageText = pgtype.Text{String: msg.Text, Valid: true}
		case "image", "video", "audio":
			trimmedURL := strings.TrimSpace(msg.MediaURL)
			trimmedType := strings.TrimSpace(msg.MediaType)
			_, urlErr := url.ParseRequestURI(trimmedURL)
			if trimmedURL == "" || trimmedType == "" || urlErr != nil || !strings.Contains(trimmedType, "/") {
				log.Printf("WARN: ChatHandler: Invalid media message from user %d (URL: '%s', Type: '%s').", userID, msg.MediaURL, msg.MediaType)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Invalid media_url or media_type format."})
				continue
			}
			createParams.MediaUrl = pgtype.Text{String: trimmedURL, Valid: true}
			createParams.MediaType = pgtype.Text{String: trimmedType, Valid: true}
		default:
			log.Printf("WARN: ChatHandler: Unknown message type '%s' from user %d.", msg.Type, userID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: fmt.Sprintf("Unknown message type: %s", msg.Type)})
			continue
		}

		// Check Mutual Like (Required before sending any message)
		mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: userID, LikedUserID: msg.RecipientUserID}
		mutualLike, errLikeCheck := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
		if errLikeCheck != nil {
			log.Printf("ERROR: ChatHandler: Failed to check mutual like between %d and %d: %v", userID, msg.RecipientUserID, errLikeCheck)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to check chat permission"})
			continue
		}
		if !mutualLike.Valid || !mutualLike.Bool {
			log.Printf("INFO: ChatHandler: Blocked message from %d to %d (no mutual like on send attempt)", userID, msg.RecipientUserID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "You can only message users you have matched with."})
			continue
		}
		log.Printf("DEBUG: ChatHandler: Mutual like confirmed between %d and %d before saving message.", userID, msg.RecipientUserID)

		if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
			replyToID := *msg.ReplyToMessageID
			log.Printf("DEBUG: ChatHandler: Validating reply_to_message_id %d for user %d", replyToID, userID)

			originalMsg, errVal := queries.GetMessageSenderRecipient(ctx, replyToID)
			if errVal != nil {
				errMsgContent := "Error validating reply."
				if errors.Is(errVal, pgx.ErrNoRows) || errors.Is(errVal, sql.ErrNoRows) {
					log.Printf("WARN: ChatHandler: User %d tried to reply to non-existent message ID %d", userID, replyToID)
					errMsgContent = "Message you tried to reply to does not exist."
				} else {
					log.Printf("ERROR: ChatHandler: Failed to fetch original message %d for reply validation: %v", replyToID, errVal)
				}
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: errMsgContent})
				continue
			}

			isValidConversation := (originalMsg.SenderUserID == userID && originalMsg.RecipientUserID == msg.RecipientUserID) ||
				(originalMsg.SenderUserID == msg.RecipientUserID && originalMsg.RecipientUserID == userID)

			if !isValidConversation {
				log.Printf("WARN: ChatHandler: User %d tried to reply to message %d which is not part of the conversation with user %d.", userID, replyToID, msg.RecipientUserID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Cannot reply to a message from a different conversation."})
				continue
			}

			createParams.ReplyToMessageID = pgtype.Int8{Int64: replyToID, Valid: true}
			log.Printf("DEBUG: ChatHandler: Reply validation passed for message %d.", replyToID)
		}

		savedMsg, dbErr := queries.CreateChatMessage(ctx, createParams)
		if dbErr != nil {
			log.Printf("ERROR: ChatHandler: Failed to save chat message from %d to %d: %v", userID, msg.RecipientUserID, dbErr)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to save message to database"})
			continue
		}
		log.Printf("INFO: ChatHandler: Message saved to DB: ID=%d, %d -> %d (Type: %s, ReplyTo: %v)", savedMsg.ID, userID, msg.RecipientUserID, msg.Type, savedMsg.ReplyToMessageID)

		clientsMu.RLock()
		recipientClient, recipientOnline := clients[msg.RecipientUserID]
		clientsMu.RUnlock()

		if !recipientOnline {
			log.Printf("INFO: ChatHandler: Recipient %d not currently connected. Message ID %d saved.", msg.RecipientUserID, savedMsg.ID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "info", Content: fmt.Sprintf("User %d is not online. Message saved.", msg.RecipientUserID)})
			continue
		}

		forwardMsg := ChatWsMessage{
			Type:             msg.Type,
			SenderUserID:     userID,
			RecipientUserID:  msg.RecipientUserID,
			Text:             savedMsg.MessageText.String,
			MediaURL:         savedMsg.MediaUrl.String,
			MediaType:        savedMsg.MediaType.String,
			ReplyToMessageID: nil,
		}
		if savedMsg.ReplyToMessageID.Valid {
			replyID := savedMsg.ReplyToMessageID.Int64
			forwardMsg.ReplyToMessageID = &replyID
		}

		errForward := recipientClient.conn.WriteJSON(forwardMsg)
		if errForward != nil {
			log.Printf("ERROR: ChatHandler: Failed to forward message ID %d from %d to %d: %v", savedMsg.ID, userID, msg.RecipientUserID, errForward)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to deliver message to recipient (they may have disconnected)"})

			clientsMu.Lock()
			if currentRecipient, stillExists := clients[msg.RecipientUserID]; stillExists && currentRecipient.conn == recipientClient.conn {
				log.Printf("INFO: ChatHandler: Removing disconnected recipient client %d after write failure.", msg.RecipientUserID)
				delete(clients, msg.RecipientUserID)
				recipientClient.conn.Close()
			}
			clientsMu.Unlock()
		} else {
			log.Printf("INFO: ChatHandler: Message ID %d forwarded successfully: %d -> %d (Type: %s)", savedMsg.ID, userID, msg.RecipientUserID, forwardMsg.Type)
		}
	}
}

// Maybe needed
//func getMessageSenderRecipient(ctx context.Context, queries *migrations.Queries, messageID int64) (migrations.GetMessageSenderRecipientRow, error) {
//	return queries.GetMessageSenderRecipient(ctx, messageID)
//}
