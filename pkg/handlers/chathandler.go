package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/gorilla/websocket"
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

type Message struct {
	RecipientUserID int32  `json:"recipient_user_id"`
	Text            string `json:"text"`
	SenderUserID    int32  `json:"sender_user_id,omitempty"`
	Type            string `json:"type,omitempty"`
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

	// --- WebSocket Upgrade ---
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
			log.Printf("INFO: ChatHandler: User %d already disconnected or replaced.", userID)
		}
		clientsMu.Unlock()
	}()

	_ = conn.WriteJSON(Message{Type: "info", Content: fmt.Sprintf("Connected as user %d", userID)})

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("ERROR: ChatHandler: Read error for user %d: %v", userID, err)
			} else {
				log.Printf("INFO: ChatHandler: WebSocket closed for user %d.", userID)
			}
			break
		}

		log.Printf("DEBUG: ChatHandler: Message received from %d: To=%d, Text='%s'", userID, msg.RecipientUserID, msg.Text)

		if msg.RecipientUserID <= 0 || msg.Text == "" {
			log.Printf("WARN: ChatHandler: Invalid message from user %d: Missing recipient or text.", userID)
			_ = conn.WriteJSON(Message{Type: "error", Content: "recipient_user_id and text fields are required"})
			continue
		}
		if msg.RecipientUserID == userID {
			log.Printf("WARN: ChatHandler: User %d attempting to send message to themselves.", userID)
			_ = conn.WriteJSON(Message{Type: "error", Content: "Cannot send messages to yourself"})
			continue
		}

		mutualLikeParams := migrations.CheckMutualLikeExistsParams{
			LikerUserID: userID,
			LikedUserID: msg.RecipientUserID,
		}
		mutualLike, err := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
		if err != nil {
			log.Printf("ERROR: ChatHandler: Failed to check mutual like between %d and %d: %v", userID, msg.RecipientUserID, err)
			_ = conn.WriteJSON(Message{Type: "error", Content: "Failed to check chat permission"})
			continue
		}

		if !mutualLike.Bool {
			log.Printf("INFO: ChatHandler: Blocked message from %d to %d (no mutual like)", userID, msg.RecipientUserID)
			_ = conn.WriteJSON(Message{Type: "error", Content: "You can only message users you have matched with."})
			continue
		}

		log.Printf("INFO: ChatHandler: Mutual like confirmed between %d and %d. Proceeding with message.", userID, msg.RecipientUserID)

		createParams := migrations.CreateChatMessageParams{
			SenderUserID:    userID,
			RecipientUserID: msg.RecipientUserID,
			MessageText:     msg.Text,
		}
		_, dbErr := queries.CreateChatMessage(ctx, createParams)
		if dbErr != nil {
			log.Printf("ERROR: ChatHandler: Failed to save chat message from %d to %d: %v", userID, msg.RecipientUserID, dbErr)
			_ = conn.WriteJSON(Message{Type: "error", Content: "Failed to save message to database"})
			continue
		}
		log.Printf("INFO: ChatHandler: Message saved to DB: %d -> %d", userID, msg.RecipientUserID)

		clientsMu.RLock()
		recipientClient, exists := clients[msg.RecipientUserID]
		clientsMu.RUnlock()

		if !exists {
			log.Printf("INFO: ChatHandler: Recipient %d not currently connected. Message saved.", msg.RecipientUserID)
			_ = conn.WriteJSON(Message{Type: "info", Content: fmt.Sprintf("User %d is not online. Message saved.", msg.RecipientUserID)})
			continue
		}

		err = recipientClient.conn.WriteJSON(Message{
			RecipientUserID: msg.RecipientUserID,
			Text:            msg.Text,
			SenderUserID:    userID,
			Type:            "message",
		})

		if err != nil {
			log.Printf("ERROR: ChatHandler: Failed to forward message from %d to %d: %v", userID, msg.RecipientUserID, err)
			_ = conn.WriteJSON(Message{Type: "error", Content: "Failed to deliver message to recipient"})

			clientsMu.Lock()
			if currentRecipient, stillExists := clients[msg.RecipientUserID]; stillExists && currentRecipient.conn == recipientClient.conn {
				delete(clients, msg.RecipientUserID)
				log.Printf("INFO: ChatHandler: Removed disconnected recipient client %d after write failure.", msg.RecipientUserID)
			}
			clientsMu.Unlock()
		} else {
			log.Printf("INFO: ChatHandler: Message forwarded successfully: %d -> %d", userID, msg.RecipientUserID)
		}
	}
}
