package handlers

import (
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
	"github.com/jackc/pgx/v5/pgtype"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Add more restrictive origin checks in production if needed
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
	Type            string `json:"type"`
	RecipientUserID int32  `json:"recipient_user_id,omitempty"`
	Text            string `json:"text,omitempty"`
	MediaURL        string `json:"media_url,omitempty"`
	MediaType       string `json:"media_type,omitempty"`
	SenderUserID    int32  `json:"sender_user_id,omitempty"`
	Content         string `json:"content,omitempty"`
}

func ChatHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries, _ := db.GetDB()
	if queries == nil {
		log.Println("ERROR: ChatHandler: Database connection not available.")
		// Cannot easily write HTTP error as upgrade might have started
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		log.Println("ERROR: ChatHandler: Authentication claims missing or invalid after middleware.")
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
	log.Printf("INFO: ChatHandler: User %d connected.", userID)

	defer func() {
		clientsMu.Lock()
		if currentClient, exists := clients[userID]; exists && currentClient.conn == conn {
			delete(clients, userID)
			log.Printf("INFO: ChatHandler: User %d disconnected.", userID)
		} else {
			log.Printf("INFO: ChatHandler: User %d's connection was already closed or replaced.", userID)
		}
		clientsMu.Unlock()
	}()

	_ = conn.WriteJSON(ChatWsMessage{Type: "info", Content: fmt.Sprintf("Connected as user %d", userID)})

	for {
		var msg ChatWsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ERROR: ChatHandler: Read error for user %d: %v", userID, err)
			} else {
				log.Printf("INFO: ChatHandler: WebSocket closed for user %d.", userID)
			}
			break
		}

		log.Printf("DEBUG: ChatHandler: Message received from %d: Type=%s, To=%d", userID, msg.Type, msg.RecipientUserID)

		if msg.RecipientUserID <= 0 {
			log.Printf("WARN: ChatHandler: Invalid message from user %d: Missing recipient_user_id.", userID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "recipient_user_id is required"})
			continue
		}
		if msg.RecipientUserID == userID {
			log.Printf("WARN: ChatHandler: User %d attempting to send message to themselves.", userID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Cannot send messages to yourself"})
			continue
		}

		var createParams migrations.CreateChatMessageParams
		createParams.SenderUserID = userID
		createParams.RecipientUserID = msg.RecipientUserID

		switch msg.Type {
		case "text":
			if strings.TrimSpace(msg.Text) == "" {
				log.Printf("WARN: ChatHandler: Empty text message from user %d.", userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Text message cannot be empty"})
				continue
			}
			if len(msg.Text) > 5000 {
				log.Printf("WARN: ChatHandler: Text message too long from user %d.", userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Text message exceeds length limit"})
				continue
			}
			createParams.MessageText = pgtype.Text{String: msg.Text, Valid: true}
			createParams.MediaUrl = pgtype.Text{Valid: false}
			createParams.MediaType = pgtype.Text{Valid: false}

		case "image", "video", "audio":
			if strings.TrimSpace(msg.MediaURL) == "" {
				log.Printf("WARN: ChatHandler: Empty media_url for %s message from user %d.", msg.Type, userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "media_url is required for media messages"})
				continue
			}
			if strings.TrimSpace(msg.MediaType) == "" {
				log.Printf("WARN: ChatHandler: Empty media_type for %s message from user %d.", msg.Type, userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "media_type is required for media messages"})
				continue
			}
			_, urlErr := url.ParseRequestURI(msg.MediaURL)
			if urlErr != nil {
				log.Printf("WARN: ChatHandler: Invalid media_url format '%s' from user %d.", msg.MediaURL, userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Invalid media_url format"})
				continue
			}
			if !strings.Contains(msg.MediaType, "/") {
				log.Printf("WARN: ChatHandler: Invalid media_type format '%s' from user %d.", msg.MediaType, userID)
				_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Invalid media_type format"})
				continue
			}

			createParams.MediaUrl = pgtype.Text{String: msg.MediaURL, Valid: true}
			createParams.MediaType = pgtype.Text{String: msg.MediaType, Valid: true}
			createParams.MessageText = pgtype.Text{Valid: false}

		default:
			log.Printf("WARN: ChatHandler: Unknown message type '%s' from user %d.", msg.Type, userID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: fmt.Sprintf("Unknown message type: %s", msg.Type)})
			continue
		}

		mutualLikeParams := migrations.CheckMutualLikeExistsParams{
			LikerUserID: userID,
			LikedUserID: msg.RecipientUserID,
		}
		mutualLike, err := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
		if err != nil {
			log.Printf("ERROR: ChatHandler: Failed to check mutual like between %d and %d: %v", userID, msg.RecipientUserID, err)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to check chat permission"})
			continue
		}

		if !mutualLike.Valid || !mutualLike.Bool {
			log.Printf("INFO: ChatHandler: Blocked message from %d to %d (no mutual like)", userID, msg.RecipientUserID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "You can only message users you have matched with."})
			continue
		}
		log.Printf("INFO: ChatHandler: Mutual like confirmed between %d and %d. Proceeding with message type '%s'.", userID, msg.RecipientUserID, msg.Type)

		_, dbErr := queries.CreateChatMessage(ctx, createParams)
		if dbErr != nil {
			log.Printf("ERROR: ChatHandler: Failed to save chat message from %d to %d: %v", userID, msg.RecipientUserID, dbErr)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to save message to database"})
			continue
		}
		log.Printf("INFO: ChatHandler: Message saved to DB: %d -> %d (Type: %s)", userID, msg.RecipientUserID, msg.Type)

		clientsMu.RLock()
		recipientClient, exists := clients[msg.RecipientUserID]
		clientsMu.RUnlock()

		if !exists {
			log.Printf("INFO: ChatHandler: Recipient %d not currently connected. Message saved.", msg.RecipientUserID)
			_ = conn.WriteJSON(ChatWsMessage{Type: "info", Content: fmt.Sprintf("User %d is not online. Message saved.", msg.RecipientUserID)})
			continue
		}

		forwardMsg := ChatWsMessage{
			Type:            msg.Type,
			SenderUserID:    userID,
			RecipientUserID: msg.RecipientUserID,
			Text:            msg.Text,
			MediaURL:        msg.MediaURL,
			MediaType:       msg.MediaType,
		}

		err = recipientClient.conn.WriteJSON(forwardMsg)
		if err != nil {
			log.Printf("ERROR: ChatHandler: Failed to forward message from %d to %d: %v", userID, msg.RecipientUserID, err)
			_ = conn.WriteJSON(ChatWsMessage{Type: "error", Content: "Failed to deliver message to recipient (they may have disconnected)"})

			clientsMu.Lock()
			if currentRecipient, stillExists := clients[msg.RecipientUserID]; stillExists && currentRecipient.conn == recipientClient.conn {
				log.Printf("INFO: ChatHandler: Removing disconnected recipient client %d after write failure.", msg.RecipientUserID)
				delete(clients, msg.RecipientUserID)
				recipientClient.conn.Close()
			}
			clientsMu.Unlock()
		} else {
			log.Printf("INFO: ChatHandler: Message forwarded successfully: %d -> %d (Type: %s)", userID, msg.RecipientUserID, msg.Type)
		}
	}
}
