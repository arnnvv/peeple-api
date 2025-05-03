package ws

import (
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5/pgtype"
)

type WsBasicLikerInfo struct {
	LikerUserID        int32              `json:"liker_user_id"`
	Name               string             `json:"name"`
	FirstProfilePicURL string             `json:"first_profile_pic_url,omitempty"`
	IsRose             bool               `json:"is_rose"`
	LikeComment        *string            `json:"like_comment,omitempty"`
	LikedAt            pgtype.Timestamptz `json:"liked_at"`
}

type WsMatchInfo struct {
	MatchedUserID         int32      `json:"matched_user_id"`
	Name                  string     `json:"name"`
	FirstProfilePicURL    string     `json:"first_profile_pic_url,omitempty"`
	IsOnline              bool       `json:"is_online"`
	LastOnline            *time.Time `json:"last_online,omitempty"`
	InitiatingLikerUserID int32      `json:"initiating_liker_user_id"`
}

type WsLikeRemovalInfo struct {
	LikerUserID int32 `json:"liker_user_id"`
}

type WsMessage struct {
	Type string `json:"type"`
	ID   *int64 `json:"id,omitempty"`

	SenderUserID     *int32  `json:"sender_user_id,omitempty"`
	RecipientUserID  *int32  `json:"recipient_user_id,omitempty"`
	Text             *string `json:"text,omitempty"`
	MediaURL         *string `json:"media_url,omitempty"`
	MediaType        *string `json:"media_type,omitempty"`
	SentAt           *string `json:"sent_at,omitempty"`
	ReplyToMessageID *int64  `json:"reply_to_message_id,omitempty"`
	MessageID        *int64  `json:"message_id,omitempty"`

	Emoji         *string `json:"emoji,omitempty"`
	ReactorUserID *int32  `json:"reactor_user_id,omitempty"`
	IsRemoved     *bool   `json:"is_removed,omitempty"`

	UserID *int32  `json:"user_id,omitempty"`
	Status *string `json:"status,omitempty"`

	OtherUserID  *int32 `json:"other_user_id,omitempty"`
	ReaderUserID *int32 `json:"reader_user_id,omitempty"`

	IsTyping        *bool  `json:"is_typing,omitempty"`
	TypingUserID    *int32 `json:"typing_user_id,omitempty"`
	IsRecording     *bool  `json:"is_recording,omitempty"`
	RecordingUserID *int32 `json:"recording_user_id,omitempty"`

	Content *string `json:"content,omitempty"`
	Count   *int64  `json:"count,omitempty"`

	LikerInfo   *WsBasicLikerInfo  `json:"liker_info,omitempty"`
	MatchInfo   *WsMatchInfo       `json:"match_info,omitempty"`
	RemovalInfo *WsLikeRemovalInfo `json:"removal_info,omitempty"`
}

const (
	RedisMsgTypeDirect           = "direct"
	RedisMsgTypeBroadcastMatches = "broadcast_matches"
	// broadcast_all
)

type RedisWsMessage struct {
	Type            string  `json:"type"`
	TargetUserID    *int32  `json:"target_user_id"`
	TargetUserIDs   []int32 `json:"target_user_ids"`
	SenderUserID    int32   `json:"sender_user_id"`
	OriginalPayload []byte  `json:"original_payload"`
}

const RedisChannelName = "peeple-websocket-messages"

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

func PtrInt32(i int32) *int32 {
	return &i
}
