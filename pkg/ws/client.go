package ws

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

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
		return true
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
			c.sendWsError("Invalid message format")
			continue
		}

		ctx := context.Background()

		if msg.Type == "chat_message" {
			if msg.RecipientUserID == nil || *msg.RecipientUserID <= 0 || *msg.RecipientUserID == c.UserID {
				c.sendWsError("Invalid recipient user ID")
				continue
			}
			recipientID := *msg.RecipientUserID
			createParams := migrations.CreateChatMessageParams{
				SenderUserID:    c.UserID,
				RecipientUserID: recipientID,
			}
			isMedia := false
			if msg.Text != nil && *msg.Text != "" {
				if len(*msg.Text) > 500 {
					c.sendWsError("Message text too long")
					continue
				}
				createParams.MessageText = pgtype.Text{
					String: *msg.Text,
					Valid:  true,
				}
			} else if msg.MediaURL != nil && *msg.MediaURL != "" && msg.MediaType != nil && *msg.MediaType != "" {
				createParams.MediaUrl = pgtype.Text{
					String: *msg.MediaURL,
					Valid:  true,
				}
				createParams.MediaType = pgtype.Text{
					String: *msg.MediaType,
					Valid:  true,
				}
				isMedia = true
			} else {
				c.sendWsError("Invalid message content (text or media required)")
				continue
			}
			if msg.ReplyToMessageID != nil && *msg.ReplyToMessageID > 0 {
				replyToID := *msg.ReplyToMessageID
				originalMsg, errVal := queries.GetMessageSenderRecipient(ctx, replyToID)
				if errVal != nil {
					errMsgContent := "Error validating reply message."
					if errors.Is(errVal, pgx.ErrNoRows) || errors.Is(errVal, sql.ErrNoRows) {
						errMsgContent = "Message you tried to reply to does not exist."
					} else {
						log.Printf("Client ReadPump ERROR: Failed to fetch original message %d for reply validation: %v", replyToID, errVal)
					}
					c.sendWsError(errMsgContent)
					continue
				}
				isValidConversation := (originalMsg.SenderUserID == c.UserID && originalMsg.RecipientUserID == recipientID) || (originalMsg.SenderUserID == recipientID && originalMsg.RecipientUserID == c.UserID)
				if !isValidConversation {
					c.sendWsError("Cannot reply to a message from a different conversation.")
					continue
				}
				createParams.ReplyToMessageID = pgtype.Int8{
					Int64: replyToID,
					Valid: true,
				}
			}
			mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: c.UserID, LikedUserID: recipientID}
			mutualLike, errLikeCheck := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
			if errLikeCheck != nil {
				log.Printf("Client ReadPump ERROR: Failed to check mutual like between %d and %d: %v", c.UserID, recipientID, errLikeCheck)
				c.sendWsError("Failed to check send permission")
				continue
			}
			if !mutualLike.Valid || !mutualLike.Bool {
				c.sendWsError("You can only message matched users")
				continue
			}
			savedMsg, dbErr := queries.CreateChatMessage(ctx, createParams)
			if dbErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to save chat message from %d to %d: %v", c.UserID, recipientID, dbErr)
				c.sendWsError("Failed to save message")
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
					ackMsg := WsMessage{
						Type:    "message_ack",
						Content: Ptr("Message sent, recipient offline."),
						ID:      &savedMsg.ID,
					}
					ackBytes, _ := json.Marshal(ackMsg)
					select {
					case c.Send <- ackBytes:
					default:
					}
				} else {
					ackMsg := WsMessage{
						Type:    "message_ack",
						Content: Ptr("Message delivered."),
						ID:      &savedMsg.ID,
					}
					ackBytes, _ := json.Marshal(ackMsg)
					select {
					case c.Send <- ackBytes:
					default:
					}
				}
			}

		} else if msg.Type == "react_to_message" {
			reactorUserID := c.UserID
			if msg.MessageID == nil || *msg.MessageID <= 0 {
				c.sendWsError("Valid message_id is required for reaction")
				continue
			}
			if msg.Emoji == nil || *msg.Emoji == "" || utf8.RuneCountInString(*msg.Emoji) > 10 {
				c.sendWsError("Valid emoji is required (1-10 characters) for reaction")
				continue
			}
			targetMessageID := *msg.MessageID
			reactionEmoji := *msg.Emoji
			msgParticipants, err := queries.GetMessageSenderRecipient(ctx, targetMessageID)
			if err != nil {
				errMsg := "Error validating reaction message"
				if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
					errMsg = fmt.Sprintf("Message with ID %d not found.", targetMessageID)
				} else {
					log.Printf("Client ReadPump ERROR: Failed to fetch message participants for reaction (MsgID: %d): %v", targetMessageID, err)
				}
				c.sendWsError(errMsg)
				continue
			}
			if msgParticipants.SenderUserID != reactorUserID && msgParticipants.RecipientUserID != reactorUserID {
				c.sendWsError("You can only react to messages in your conversations.")
				continue
			}
			existingReaction, err := queries.GetSingleReactionByUser(
				ctx,
				migrations.GetSingleReactionByUserParams{
					MessageID: targetMessageID,
					UserID:    reactorUserID,
				})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, sql.ErrNoRows) {
				log.Printf("Client ReadPump ERROR: Failed to check existing reaction for user %d on message %d: %v", reactorUserID, targetMessageID, err)
				c.sendWsError("Failed to check existing reaction")
				continue
			}
			reactionExists := !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, sql.ErrNoRows)
			var finalEmoji string
			var wasRemoved bool
			var dbErr error
			ackContent := ""
			if !reactionExists {
				addParams := migrations.UpsertMessageReactionParams{MessageID: targetMessageID, UserID: reactorUserID, Emoji: reactionEmoji}
				_, dbErr = queries.UpsertMessageReaction(ctx, addParams)
				if dbErr == nil {
					finalEmoji = reactionEmoji
					wasRemoved = false
					ackContent = "Reaction added."
				} else {
					log.Printf("Client ReadPump ERROR: Failed to add reaction for user %d: %v", reactorUserID, dbErr)
				}
			} else {
				if existingReaction.Emoji == reactionEmoji {
					deleteParams := migrations.DeleteMessageReactionByUserParams{MessageID: targetMessageID, UserID: reactorUserID}
					_, dbErr = queries.DeleteMessageReactionByUser(ctx, deleteParams)
					if dbErr == nil {
						finalEmoji = ""
						wasRemoved = true
						ackContent = "Reaction removed."
					} else {
						log.Printf("Client ReadPump ERROR: Failed to delete reaction for user %d: %v", reactorUserID, dbErr)
					}
				} else {
					updateParams := migrations.UpsertMessageReactionParams{MessageID: targetMessageID, UserID: reactorUserID, Emoji: reactionEmoji}
					_, dbErr = queries.UpsertMessageReaction(ctx, updateParams)
					if dbErr == nil {
						finalEmoji = reactionEmoji
						wasRemoved = false
						ackContent = "Reaction updated."
					} else {
						log.Printf("Client ReadPump ERROR: Failed to update reaction for user %d: %v", reactorUserID, dbErr)
					}
				}
			}
			if dbErr != nil {
				c.sendWsError("Failed to process reaction.")
				continue
			}
			participants := []int32{msgParticipants.SenderUserID, msgParticipants.RecipientUserID}
			c.hub.BroadcastReaction(targetMessageID, reactorUserID, finalEmoji, wasRemoved, participants)
			ackMsg := WsMessage{
				Type:      "reaction_ack",
				Content:   Ptr(ackContent),
				MessageID: &targetMessageID,
			}
			ackBytes, _ := json.Marshal(ackMsg)
			select {
			case c.Send <- ackBytes:
			default:
			}
			log.Printf("Client ReadPump INFO: Reaction processed for user %d on msg %d. Action: %s", reactorUserID, targetMessageID, ackContent)

		} else if msg.Type == "mark_read" {
			recipientUserID := c.UserID
			if msg.OtherUserID == nil || *msg.OtherUserID <= 0 {
				c.sendWsError("Valid other_user_id is required for mark_read")
				continue
			}
			if msg.MessageID == nil || *msg.MessageID <= 0 {
				c.sendWsError("Valid message_id (last read) is required for mark_read")
				continue
			}
			senderUserID := *msg.OtherUserID
			lastMessageID := *msg.MessageID
			if senderUserID == recipientUserID {
				c.sendWsError("Cannot mark messages from yourself as read this way")
				continue
			}
			_, errCheck := queries.GetMessageSenderRecipient(ctx, lastMessageID)
			if errCheck != nil {
				errMsg := "Error validating message ID for mark read"
				if errors.Is(errCheck, pgx.ErrNoRows) || errors.Is(errCheck, sql.ErrNoRows) {
					errMsg = fmt.Sprintf("Invalid message ID (%d) provided for mark read.", lastMessageID)
					log.Printf("Client ReadPump WARN: %s (User: %d)", errMsg, c.UserID)
				} else {
					log.Printf("Client ReadPump ERROR: Failed to check existence for mark_read message ID %d: %v", lastMessageID, errCheck)
				}
				c.sendWsError(errMsg)
				continue
			}
			log.Printf("Client ReadPump INFO: User %d marking messages from user %d as read up to message ID %d", recipientUserID, senderUserID, lastMessageID)
			params := migrations.MarkMessagesAsReadUntilParams{
				RecipientUserID: recipientUserID,
				SenderUserID:    senderUserID,
				ID:              lastMessageID,
			}
			cmdTag, err := queries.MarkMessagesAsReadUntil(ctx, params)
			if err != nil {
				log.Printf("Client ReadPump ERROR: Failed to update messages read status for user %d from user %d: %v", recipientUserID, senderUserID, err)
				c.sendWsError("Failed to update message status")
				continue
			}
			rowsAffected := cmdTag.RowsAffected()
			log.Printf("Client ReadPump INFO: Successfully marked %d messages as read for user %d from user %d (up to ID %d)", rowsAffected, recipientUserID, senderUserID, lastMessageID)
			ackContent := fmt.Sprintf("Marked %d messages from user %d as read.", rowsAffected, senderUserID)
			ackMsg := WsMessage{
				Type:        "mark_read_ack",
				Content:     &ackContent,
				OtherUserID: &senderUserID,
				MessageID:   &lastMessageID,
				Count:       PtrInt64(rowsAffected),
			}
			ackBytes, _ := json.Marshal(ackMsg)
			select {
			case c.Send <- ackBytes:
			default:
			}
			if rowsAffected > 0 {
				readUpdateMsg := WsMessage{
					Type:         "messages_read_update",
					ReaderUserID: &recipientUserID,
					MessageID:    &lastMessageID,
				}
				readUpdateBytes, errMarshal := json.Marshal(readUpdateMsg)
				if errMarshal != nil {
					log.Printf("Client ReadPump ERROR: Failed marshal messages_read_update: %v", errMarshal)
				} else {
					if !c.hub.SendToUser(senderUserID, readUpdateBytes) {
						log.Printf("Client ReadPump DEBUG: User %d (sender) is offline, cannot send read receipt.", senderUserID)
					} else {
						log.Printf("Client ReadPump INFO: Sent read receipt to user %d (up to msg %d)", senderUserID, lastMessageID)
					}
				}
			}

		} else if msg.Type == "typing_event" {
			senderUserID := c.UserID

			if msg.RecipientUserID == nil || *msg.RecipientUserID <= 0 {
				c.sendWsError("RecipientUserID is required for typing event")
				continue
			}
			if msg.IsTyping == nil {
				c.sendWsError("IsTyping boolean is required for typing event")
				continue
			}
			recipientID := *msg.RecipientUserID
			isTypingState := *msg.IsTyping

			if recipientID == senderUserID {
				c.sendWsError("Cannot send typing indicator to yourself")
				continue
			}

			mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: senderUserID, LikedUserID: recipientID}
			mutualLike, errLikeCheck := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
			if errLikeCheck != nil {
				log.Printf("Client ReadPump ERROR: Failed to check mutual like for typing indicator (%d -> %d): %v", senderUserID, recipientID, errLikeCheck)
				c.sendWsError("Failed to check match status")
				continue
			}
			if !mutualLike.Valid || !mutualLike.Bool {
				log.Printf("Client ReadPump WARN: Blocked typing indicator from %d to %d (no mutual like)", senderUserID, recipientID)
				c.sendWsError("Can only send typing status to matched users")
				continue
			}

			typingStatusMsg := WsMessage{
				Type:         "typing_status",
				TypingUserID: &senderUserID,  // Who is typing
				IsTyping:     &isTypingState, // Are they starting or stopping?
			}
			statusBytes, marshalErr := json.Marshal(typingStatusMsg)
			if marshalErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to marshal typing status message for user %d -> %d: %v", senderUserID, recipientID, marshalErr)
				// Don't send error back to sender for this internal issue
				continue
			}

			// Send the status ONLY to the recipient
			if !c.hub.SendToUser(recipientID, statusBytes) {
				// Recipient is offline, no need to do anything else or send error
				log.Printf("Client ReadPump DEBUG: Typing indicator not sent (%d -> %d), recipient offline.", senderUserID, recipientID)
			} else {
				// log.Printf("Client ReadPump DEBUG: Sent typing status (%t) for user %d to user %d.", isTypingState, senderUserID, recipientID) // Can be noisy
			}

		} else {
			log.Printf("Client ReadPump: Received unhandled message type '%s' from user %d", msg.Type, c.UserID)
			c.sendWsError("Unknown message type")
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ServeWs: Failed to upgrade connection for user %d: %v", userID, err)
		return
	}
	log.Printf("ServeWs: Connection upgraded successfully for user %d", userID)
	client := &Client{hub: hub, conn: conn, Send: make(chan []byte, 256), UserID: userID}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
	log.Printf("ServeWs: Read and Write pumps started for user %d", userID)
	infoMsg := WsMessage{Type: "info", Content: Ptr("Connected successfully.")}
	infoBytes, _ := json.Marshal(infoMsg)
	select {
	case client.Send <- infoBytes:
	default:
		log.Printf("ServeWs: Failed to send initial info message to user %d, channel might be closed.", userID)
	}
}

func (c *Client) sendWsError(errorMessage string) {
	errMsg := WsMessage{Type: "error", Content: Ptr(errorMessage)}
	errBytes, err := json.Marshal(errMsg)
	if err != nil {
		log.Printf("Client ReadPump ERROR: Failed to marshal error message: %v", err)
		return
	}
	select {
	case c.Send <- errBytes:
	default:
		log.Printf("Client ReadPump: Send channel closed for user %d while sending error: %s", c.UserID, errorMessage)
	}
}
