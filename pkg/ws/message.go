package ws

import (
	"time"

	"github.com/arnnvv/peeple-api/migrations"
)

type WsMessage struct {
	Type string `json:"type"`
	ID   *int64 `json:"id,omitempty"`

	SenderUserID     *int32  `json:"sender_user_id,omitempty"`
	RecipientUserID  *int32  `json:"recipient_user_id,omitempty"`
	Text             *string `json:"text,omitempty"`
	MediaURL         *string `json:"media_url,omitempty"`
	MediaType        *string `json:"media_type,omitempty"`
	SentAt           *string `json:"sent_at,omitempty"` // ISO 8601 format string (outgoing)
	ReplyToMessageID *int64  `json:"reply_to_message_id,omitempty"`

	MessageID *int64 `json:"message_id,omitempty"`

	Emoji *string `json:"emoji,omitempty"`

	ReactorUserID *int32 `json:"reactor_user_id,omitempty"`
	IsRemoved     *bool  `json:"is_removed,omitempty"`

	UserID *int32  `json:"user_id,omitempty"`
	Status *string `json:"status,omitempty"`

	OtherUserID *int32 `json:"other_user_id,omitempty"`

	ReaderUserID *int32 `json:"reader_user_id,omitempty"`

	Content *string `json:"content,omitempty"`
	Count   *int64  `json:"count,omitempty"`
}

func ChatMessageToWsMessage(dbMsg migrations.GetConversationMessagesRow) WsMessage {
	sentAtStr := dbMsg.SentAt.Time.UTC().Format(time.RFC3339Nano)
	text := dbMsg.MessageText.String
	mediaUrl := dbMsg.MediaUrl.String
	mediaType := dbMsg.MediaType.String

	wsMsg := WsMessage{
		Type:            "chat_message",
		ID:              &dbMsg.ID,
		SenderUserID:    &dbMsg.SenderUserID,
		RecipientUserID: &dbMsg.RecipientUserID,
		Text:            nil,
		MediaURL:        nil,
		MediaType:       nil,
		SentAt:          &sentAtStr,
	}

	if dbMsg.MessageText.Valid {
		wsMsg.Text = &text
	}
	if dbMsg.MediaUrl.Valid {
		wsMsg.MediaURL = &mediaUrl
	}
	if dbMsg.MediaType.Valid {
		wsMsg.MediaType = &mediaType
	}
	if dbMsg.ReplyToMessageID.Valid {
		wsMsg.ReplyToMessageID = &dbMsg.ReplyToMessageID.Int64
	}

	return wsMsg
}

func Ptr(s string) *string {
	return &s
}

func PtrInt64(i int64) *int64 {
	return &i
}
