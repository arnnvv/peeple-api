package handlers

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	conn *websocket.Conn
	user string
}

var (
	clients   = make(map[string]*Client)
	clientsMu sync.RWMutex
)

type Message struct {
	To      string `json:"to"`
	Text    string `json:"text"`
	Type    string `json:"type,omitempty"`
	Content string `json:"content,omitempty"`
}

func ChatHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	user := r.URL.Query().Get("user")
	if user == "" {
		log.Println("Missing user parameter")
		conn.WriteJSON(Message{Type: "error", Content: "Missing user parameter"})
		return
	}

	clientsMu.Lock()
	if oldClient, exists := clients[user]; exists {
		log.Printf("Closing existing connection for user %s", user)
		oldClient.conn.Close()
	}
	clients[user] = &Client{conn: conn, user: user}
	clientsMu.Unlock()
	log.Printf("User %s connected", user)

	defer func() {
		clientsMu.Lock()
		delete(clients, user)
		clientsMu.Unlock()
		log.Printf("User %s disconnected", user)
	}()

	conn.WriteJSON(Message{Type: "info", Content: "Connected successfully"})

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("Read error for %s: %v", user, err)
			}
			break
		}

		log.Printf("Message from %s to %s: %s", user, msg.To, msg.Text)

		if msg.To == "" || msg.Text == "" {
			conn.WriteJSON(Message{Type: "error", Content: "Both 'to' and 'text' fields are required"})
			continue
		}

		clientsMu.RLock()
		recipient, exists := clients[msg.To]
		clientsMu.RUnlock()

		if !exists {
			conn.WriteJSON(Message{Type: "error", Content: "Recipient not found"})
			continue
		}

		err = recipient.conn.WriteJSON(Message{
			To:      msg.To,
			Text:    msg.Text,
			Content: msg.Text,
			Type:    "message",
		})

		if err != nil {
			log.Printf("Failed to send message to %s: %v", msg.To, err)
			conn.WriteJSON(Message{Type: "error", Content: "Failed to deliver message"})

			clientsMu.Lock()
			delete(clients, msg.To)
			clientsMu.Unlock()
		}
	}
}
