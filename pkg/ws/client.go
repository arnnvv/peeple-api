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
	"sync"
	"time"
	"unicode/utf8"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/go-redis/redis_rate/v10"
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
	hub               *Hub
	conn              *websocket.Conn
	Send              chan []byte
	UserID            int32
	typingToUserID    int32
	typingMu          sync.Mutex
	recordingToUserID int32
	recordingMu       sync.Mutex
}

func (c *Client) sendStopTypingStatus(recipientID int32) {
	if recipientID == 0 {
		return
	}
	stopTypingMsg := WsMessage{
		Type:         "typing_status",
		TypingUserID: &c.UserID,
		IsTyping:     PtrBool(false),
	}
	statusBytes, marshalErr := json.Marshal(stopTypingMsg)
	if marshalErr != nil {
		log.Printf("Client Helper ERROR: Failed marshal stop typing status %d -> %d: %v", c.UserID, recipientID, marshalErr)
		return
	}
	_ = c.hub.SendToUser(recipientID, statusBytes)
}

func (c *Client) sendStopRecordingStatus(recipientID int32) {
	if recipientID == 0 {
		return
	}
	stopRecordingMsg := WsMessage{
		Type:            "recording_status",
		RecordingUserID: &c.UserID,
		IsRecording:     PtrBool(false),
	}
	statusBytes, marshalErr := json.Marshal(stopRecordingMsg)
	if marshalErr != nil {
		log.Printf("Client Helper ERROR: Failed marshal stop recording status %d -> %d: %v", c.UserID, recipientID, marshalErr)
		return
	}
	_ = c.hub.SendToUser(recipientID, statusBytes)
}

func (c *Client) readPump() {
	defer func() {
		log.Printf("Client ReadPump: Unregistering and closing connection for user %d", c.UserID)
		c.typingMu.Lock()
		currentTypingTo := c.typingToUserID
		c.typingToUserID = 0
		c.typingMu.Unlock()
		if currentTypingTo != 0 {
			c.sendStopTypingStatus(currentTypingTo)
		}
		c.recordingMu.Lock()
		currentRecordingTo := c.recordingToUserID
		c.recordingToUserID = 0
		c.recordingMu.Unlock()
		if currentRecordingTo != 0 {
			c.sendStopRecordingStatus(currentRecordingTo)
		}
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
	pool, errPool := db.GetPool()
	if errDb != nil || queries == nil || errPool != nil || pool == nil {
		log.Printf("Client ReadPump FATAL: Cannot get DB queries/pool for user %d: DB Err: %v, Pool Err: %v", c.UserID, errDb, errPool)
		return
	}

	likeLimit := redis_rate.Limit{Rate: 1, Period: 2 * time.Second, Burst: 3}
	interactLimit := redis_rate.Limit{Rate: 1, Period: 5 * time.Second, Burst: 2}

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

		switch msg.Type {
		case "chat_message":
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
			mutualLikeParams := migrations.CheckMutualLikeExistsParams{
				LikerUserID: c.UserID,
				LikedUserID: recipientID,
			}
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
				continue
			}
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

		case "react_to_message":
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
			existingReaction, err := queries.GetSingleReactionByUser(ctx,
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
				addParams := migrations.UpsertMessageReactionParams{
					MessageID: targetMessageID,
					UserID:    reactorUserID,
					Emoji:     reactionEmoji,
				}
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
					deleteParams := migrations.DeleteMessageReactionByUserParams{
						MessageID: targetMessageID,
						UserID:    reactorUserID,
					}
					_, dbErr = queries.DeleteMessageReactionByUser(ctx, deleteParams)
					if dbErr == nil {
						finalEmoji = ""
						wasRemoved = true
						ackContent = "Reaction removed."
					} else {
						log.Printf("Client ReadPump ERROR: Failed to delete reaction for user %d: %v", reactorUserID, dbErr)
					}
				} else {
					updateParams := migrations.UpsertMessageReactionParams{
						MessageID: targetMessageID,
						UserID:    reactorUserID,
						Emoji:     reactionEmoji,
					}
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

		case "mark_read":
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
					} else {
					}
				}
			}

		case "typing_event":
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
			mutualLikeParams := migrations.CheckMutualLikeExistsParams{
				LikerUserID: senderUserID,
				LikedUserID: recipientID,
			}
			mutualLike, errLikeCheck := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
			if errLikeCheck != nil {
				log.Printf("Client ReadPump ERROR: Failed to check mutual like for typing indicator (%d -> %d): %v", senderUserID, recipientID, errLikeCheck)
				c.sendWsError("Failed to check match status")
				continue
			}
			if !mutualLike.Valid || !mutualLike.Bool {
				c.sendWsError("Can only send typing status to matched users")
				continue
			}
			c.typingMu.Lock()
			currentTypingTo := c.typingToUserID
			var oldRecipientID int32 = 0
			if isTypingState {
				if currentTypingTo != 0 && currentTypingTo != recipientID {
					oldRecipientID = currentTypingTo
				}
				c.typingToUserID = recipientID
			} else {
				if currentTypingTo == recipientID {
					c.typingToUserID = 0
				} else {
					c.typingMu.Unlock()
					continue
				}
			}
			c.typingMu.Unlock()
			if oldRecipientID != 0 {
				c.sendStopTypingStatus(oldRecipientID)
			}
			typingStatusMsg := WsMessage{
				Type:         "typing_status",
				TypingUserID: &senderUserID,
				IsTyping:     &isTypingState,
			}
			statusBytes, marshalErr := json.Marshal(typingStatusMsg)
			if marshalErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to marshal typing status message for user %d -> %d: %v", senderUserID, recipientID, marshalErr)
				continue
			}
			if !c.hub.SendToUser(recipientID, statusBytes) { /* offline */
			}

		case "recording_event":
			senderUserID := c.UserID
			if msg.RecipientUserID == nil || *msg.RecipientUserID <= 0 {
				c.sendWsError("RecipientUserID is required for recording event")
				continue
			}
			if msg.IsRecording == nil {
				c.sendWsError("IsRecording boolean is required for recording event")
				continue
			}
			recipientID := *msg.RecipientUserID
			isRecordingState := *msg.IsRecording
			if recipientID == senderUserID {
				c.sendWsError("Cannot send recording indicator to yourself")
				continue
			}
			mutualLikeParams := migrations.CheckMutualLikeExistsParams{
				LikerUserID: senderUserID,
				LikedUserID: recipientID,
			}
			mutualLike, errLikeCheck := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
			if errLikeCheck != nil {
				log.Printf("Client ReadPump ERROR: Failed to check mutual like for recording indicator (%d -> %d): %v", senderUserID, recipientID, errLikeCheck)
				c.sendWsError("Failed to check match status")
				continue
			}
			if !mutualLike.Valid || !mutualLike.Bool {
				c.sendWsError("Can only send recording status to matched users")
				continue
			}
			c.recordingMu.Lock()
			currentRecordingTo := c.recordingToUserID
			var oldRecipientID int32 = 0
			if isRecordingState {
				if currentRecordingTo != 0 && currentRecordingTo != recipientID {
					oldRecipientID = currentRecordingTo
				}
				c.recordingToUserID = recipientID
			} else {
				if currentRecordingTo == recipientID {
					c.recordingToUserID = 0
				} else {
					c.recordingMu.Unlock()
					continue
				}
			}
			c.recordingMu.Unlock()
			if oldRecipientID != 0 {
				c.sendStopRecordingStatus(oldRecipientID)
			}
			recordingStatusMsg := WsMessage{
				Type:            "recording_status",
				RecordingUserID: &senderUserID,
				IsRecording:     &isRecordingState,
			}
			statusBytes, marshalErr := json.Marshal(recordingStatusMsg)
			if marshalErr != nil {
				log.Printf("Client ReadPump ERROR: Failed to marshal recording status message for user %d -> %d: %v", senderUserID, recipientID, marshalErr)
				continue
			}
			if !c.hub.SendToUser(recipientID, statusBytes) { /* offline */
			}

		case "send_like":
			if msg.LikePayload == nil {
				c.sendWsError("Missing like_payload")
				continue
			}
			key := fmt.Sprintf("ws_action:like:%d", c.UserID)
			res, err := c.hub.rateLimiter.Allow(ctx, key, likeLimit)
			if err != nil {
				log.Printf("ERROR: readPump: Rate limiter check failed for send_like user %d: %v", c.UserID, err)
				c.sendWsError("Internal error checking rate limit.")
				continue
			}
			if res.Allowed == 0 {
				log.Printf("WARN: Rate limit exceeded for send_like user %d", c.UserID)
				c.sendWsError("Like rate limit exceeded. Please wait.")
				continue
			}
			err = ProcessLike(ctx, queries, pool, c.hub, c.UserID, *msg.LikePayload)
			if err != nil {
				log.Printf("Client ReadPump ERROR: Processing Like failed for user %d: %v", c.UserID, err)
				c.sendWsError(err.Error())
			} else {
				ackMsg := WsMessage{
					Type:    "like_ack",
					Content: Ptr("Like processed successfully."),
				}
				ackBytes, _ := json.Marshal(ackMsg)
				select {
				case c.Send <- ackBytes:
				default:
				}
			}

		case "send_dislike":
			if msg.DislikePayload == nil {
				c.sendWsError("Missing dislike_payload")
				continue
			}
			key := fmt.Sprintf("ws_action:dislike:%d", c.UserID)
			res, err := c.hub.rateLimiter.Allow(ctx, key, interactLimit)
			if err != nil {
				log.Printf("ERROR: readPump: Rate limiter check failed for send_dislike user %d: %v", c.UserID, err)
				c.sendWsError("Internal error checking rate limit.")
				continue
			}
			if res.Allowed == 0 {
				log.Printf("WARN: Rate limit exceeded for send_dislike user %d", c.UserID)
				c.sendWsError("Action rate limit exceeded. Please wait.")
				continue
			}
			err = ProcessDislike(ctx, queries, c.hub, c.UserID, *msg.DislikePayload)
			if err != nil {
				log.Printf("Client ReadPump ERROR: Processing Dislike failed for user %d: %v", c.UserID, err)
				c.sendWsError(err.Error())
			} else {
				ackMsg := WsMessage{
					Type:    "dislike_ack",
					Content: Ptr("Dislike processed successfully."),
				}
				ackBytes, _ := json.Marshal(ackMsg)
				select {
				case c.Send <- ackBytes:
				default:
				}
			}

		case "send_unmatch":
			if msg.UnmatchPayload == nil {
				c.sendWsError("Missing unmatch_payload")
				continue
			}
			key := fmt.Sprintf("ws_action:unmatch:%d", c.UserID)
			res, err := c.hub.rateLimiter.Allow(ctx, key, interactLimit)
			if err != nil {
				log.Printf("ERROR: readPump: Rate limiter check failed for send_unmatch user %d: %v", c.UserID, err)
				c.sendWsError("Internal error checking rate limit.")
				continue
			}
			if res.Allowed == 0 {
				log.Printf("WARN: Rate limit exceeded for send_unmatch user %d", c.UserID)
				c.sendWsError("Action rate limit exceeded. Please wait.")
				continue
			}
			err = ProcessUnmatch(ctx, queries, pool, c.hub, c.UserID, *msg.UnmatchPayload)
			if err != nil {
				log.Printf("Client ReadPump ERROR: Processing Unmatch failed for user %d: %v", c.UserID, err)
				c.sendWsError(err.Error())
			} else {
				ackMsg := WsMessage{
					Type:    "unmatch_ack",
					Content: Ptr("Unmatch processed successfully."),
				}
				ackBytes, _ := json.Marshal(ackMsg)
				select {
				case c.Send <- ackBytes:
				default:
				}
			}

		default:
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
	infoMsg := WsMessage{
		Type:    "info",
		Content: Ptr("Connected successfully."),
	}
	infoBytes, _ := json.Marshal(infoMsg)
	select {
	case client.Send <- infoBytes:
	default:
		log.Printf("ServeWs: Failed send initial info msg user %d.", userID)
	}
}

func (c *Client) sendWsError(errorMessage string) {
	errMsg := WsMessage{
		Type:    "error",
		Content: Ptr(errorMessage),
	}
	errBytes, err := json.Marshal(errMsg)
	if err != nil {
		log.Printf("Client ReadPump ERROR: Failed marshal error msg: %v", err)
		return
	}
	select {
	case c.Send <- errBytes:
	default:
		log.Printf("Client ReadPump: Send channel closed user %d sending error: %s", c.UserID, errorMessage)
	}
}

func PtrBool(b bool) *bool {
	return &b
}
