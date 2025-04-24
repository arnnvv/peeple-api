package ws

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 4
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking for production
		return true // Allow all for now
	},
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	Send   chan []byte
	UserID int32
}

func (c *Client) readPump() {
	defer func() {
		log.Printf("Client ReadPump: Unregistering and closing connection for user %d", c.UserID)
		c.hub.unregister <- c
		c.conn.Close()
		log.Printf("Client ReadPump: Finished cleanup for user %d", c.UserID)
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Printf("Client ReadPump FATAL: Cannot get DB queries for user %d: %v", c.UserID, errDb)
		return
	}

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				log.Printf("Client ReadPump: Unexpected close error for user %d: %v", c.UserID, err)
			} else {
				log.Printf("Client ReadPump: Websocket closed for user %d (Normal or EOF): %v", c.UserID, err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.ReplaceAll(message, newline, space))

		var msg WsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Client ReadPump: Error unmarshalling message from user %d: %v. Message: %s", c.UserID, err, string(message))
			errMsg := WsMessage{Type: "error", Content: Ptr("Invalid message format")}
			errBytes, _ := json.Marshal(errMsg)
			select {
			case c.Send <- errBytes:
			default:
				log.Printf("Client ReadPump: Send channel closed for user %d while sending unmarshal error.", c.UserID)
			}
			continue
		}

		if msg.Type == "chat_message" {
			if msg.RecipientUserID == nil || *msg.RecipientUserID <= 0 || *msg.RecipientUserID == c.UserID {
				log.Printf("Client ReadPump: Invalid recipient %v from user %d", msg.RecipientUserID, c.UserID)
				errMsg := WsMessage{Type: "error", Content: Ptr("Invalid recipient user ID")}
				errBytes, _ := json.Marshal(errMsg)
				select {
				case c.Send <- errBytes:
				default:
				}
				continue
			}
			recipientID := *msg.RecipientUserID

			createParams := migrations.CreateChatMessageParams{
				SenderUserID:     c.UserID,
				RecipientUserID:  recipientID,
				MessageText:      pgtype.Text{Valid: false},
				MediaUrl:         pgtype.Text{Valid: false},
				MediaType:        pgtype.Text{Valid: false},
				ReplyToMessageID: pgtype.Int8{Valid: false},
			}

			isMedia := false
			if msg.Text != nil && *msg.Text != "" {
				if len(*msg.Text) > 500 {
					log.Printf("Client ReadPump: Text message too long from user %d", c.UserID)
					errMsg := WsMessage{Type: "error", Content: Ptr("Message text too long")}
					errBytes, _ := json.Marshal(errMsg)
					select {
					case c.Send <- errBytes:
					default:
					}
					continue
				}
				createParams.MessageText = pgtype.Text{String: *msg.Text, Valid: true}
			} else if msg.MediaURL != nil && *msg.MediaURL != "" && msg.MediaType != nil && *msg.MediaType != "" {
				createParams.MediaUrl = pgtype.Text{String: *msg.MediaURL, Valid: true}
				createParams.MediaType = pgtype.Text{String: *msg.MediaType, Valid: true}
				isMedia = true
			} else {
				log.Printf("Client ReadPump: Invalid chat message content from user %d (missing text or media)", c.UserID)
				errMsg := WsMessage{Type: "error", Content: Ptr("Invalid message content (text or media required)")}
				errBytes, _ := json.Marshal(errMsg)
				select {
				case c.Send <- errBytes:
				default:
				}
				continue
			}

			if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
				replyToID := *msg.ReplyToMessageID

				originalMsg, errVal := queries.GetMessageSenderRecipient(context.Background(), replyToID)
				if errVal != nil {
					errMsgContent := "Error validating reply message."
					if errors.Is(errVal, pgx.ErrNoRows) || errors.Is(errVal, sql.ErrNoRows) {
						errMsgContent = "Message you tried to reply to does not exist."
					} else {
						log.Printf("Client ReadPump ERROR: Failed to fetch original message %d for reply validation: %v", replyToID, errVal)
					}
					errMsg := WsMessage{Type: "error", Content: &errMsgContent}
					errBytes, _ := json.Marshal(errMsg)
					select {
					case c.Send <- errBytes:
					default:
					}
					continue
				}

				isValidConversation := (originalMsg.SenderUserID == c.UserID && originalMsg.RecipientUserID == recipientID) ||
					(originalMsg.SenderUserID == recipientID && originalMsg.RecipientUserID == c.UserID)

				if !isValidConversation {
					errMsgContent := "Cannot reply to a message from a different conversation."
					errMsg := WsMessage{Type: "error", Content: &errMsgContent}
					errBytes, _ := json.Marshal(errMsg)
					select {
					case c.Send <- errBytes:
					default:
					}
					continue
				}

				createParams.ReplyToMessageID = pgtype.Int8{Int64: replyToID, Valid: true}
			}

			mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: c.UserID, LikedUserID: recipientID}
			mutualLike, errLikeCheck := queries.CheckMutualLikeExists(context.Background(), mutualLikeParams)
			if errLikeCheck != nil {
				log.Printf("Client ReadPump ERROR: Failed to check mutual like between %d and %d: %v", c.UserID, recipientID, errLikeCheck)
				errMsg := WsMessage{Type: "error", Content: Ptr("Failed to check send permission")}
				errBytes, _ := json.Marshal(errMsg)
				select {
				case c.Send <- errBytes:
				default:
				}
				continue
			}
			if !mutualLike.Valid || !mutualLike.Bool {
				errMsg := WsMessage{Type: "error", Content: Ptr("You can only message matched users")}
				errBytes, _ := json.Marshal(errMsg)
				select {
				case c.Send <- errBytes:
				default:
				}
				continue
			}

			savedMsg, dbErr := queries.CreateChatMessage(context.Background(), createParams)
			if dbErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to save chat message from %d to %d: %v", c.UserID, recipientID, dbErr)
				errMsg := WsMessage{Type: "error", Content: Ptr("Failed to save message")}
				errBytes, _ := json.Marshal(errMsg)
				select {
				case c.Send <- errBytes:
				default:
				}
				continue
			}
			log.Printf("Client ReadPump INFO: Message saved: ID=%d, %d -> %d (Media: %t)", savedMsg.ID, c.UserID, recipientID, isMedia)

			wsMsgToSend := WsMessage{
				Type:            "chat_message",
				ID:              &savedMsg.ID,
				SenderUserID:    &savedMsg.SenderUserID,
				RecipientUserID: &savedMsg.RecipientUserID,
				Text:            nil,
				MediaURL:        nil,
				MediaType:       nil,
				SentAt:          Ptr(savedMsg.SentAt.Time.UTC().Format(time.RFC3339Nano)),
			}
			if savedMsg.MessageText.Valid {
				wsMsgToSend.Text = &savedMsg.MessageText.String
			}
			if savedMsg.MediaUrl.Valid {
				wsMsgToSend.MediaURL = &savedMsg.MediaUrl.String
			}
			if savedMsg.MediaType.Valid {
				wsMsgToSend.MediaType = &savedMsg.MediaType.String
			}
			if savedMsg.ReplyToMessageID.Valid {
				wsMsgToSend.ReplyToMessageID = &savedMsg.ReplyToMessageID.Int64
			}

			msgBytes, marshalErr := json.Marshal(wsMsgToSend)
			if marshalErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to marshal outgoing message ID %d: %v", savedMsg.ID, marshalErr)
			} else {
				if !c.hub.SendToUser(recipientID, msgBytes) {
					ackMsg := WsMessage{Type: "message_ack", Content: Ptr("Message sent, recipient offline."), ID: &savedMsg.ID}
					ackBytes, _ := json.Marshal(ackMsg)
					select {
					case c.Send <- ackBytes:
					default:
					}
				} else {
					ackMsg := WsMessage{Type: "message_ack", Content: Ptr("Message delivered."), ID: &savedMsg.ID}
					ackBytes, _ := json.Marshal(ackMsg)
					select {
					case c.Send <- ackBytes:
					default:
					}
				}
			}

		} else {
			log.Printf("Client ReadPump: Received unhandled message type '%s' from user %d", msg.Type, c.UserID)
			errMsg := WsMessage{Type: "error", Content: Ptr("Unknown message type")}
			errBytes, _ := json.Marshal(errMsg)
			select {
			case c.Send <- errBytes:
			default:
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		log.Printf("Client WritePump: Closing connection for user %d", c.UserID)
		c.conn.Close()
		log.Printf("Client WritePump: Finished cleanup for user %d", c.UserID)
	}()
	for {
		select {
		case message, ok := <-c.Send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Printf("Client WritePump: Hub closed channel for user %d", c.UserID)
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Client WritePump: Error getting next writer for user %d: %v", c.UserID, err)
				return
			}
			_, err = w.Write(message)
			if err != nil {
				log.Printf("Client WritePump: Error writing message for user %d: %v", c.UserID, err)
			}

			if err := w.Close(); err != nil {
				log.Printf("Client WritePump: Error closing writer for user %d: %v", c.UserID, err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Client WritePump: Error sending ping to user %d: %v", c.UserID, err)
				return
			}
		}
	}
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID int32) {
	log.Printf("ServeWs: Attempting to upgrade connection for user %d", userID)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ServeWs: Failed to upgrade connection for user %d: %v", userID, err)
		return
	}
	log.Printf("ServeWs: Connection upgraded successfully for user %d", userID)

	client := &Client{hub: hub, conn: conn, Send: make(chan []byte, 256), UserID: userID}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()

	log.Printf("ServeWs: Read and Write pumps started for user %d", userID)

	// Send welcome/info message
	infoMsg := WsMessage{Type: "info", Content: Ptr("Connected successfully.")}
	infoBytes, _ := json.Marshal(infoMsg)
	// Use a select to avoid blocking if the Send channel is somehow closed immediately
	select {
	case client.Send <- infoBytes:
	default:
		log.Printf("ServeWs: Failed to send initial info message to user %d, channel might be closed.", userID)
	}
}
