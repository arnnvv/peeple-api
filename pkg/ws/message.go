package ws

import (
	"time"

	"github.com/arnnvv/peeple-api/migrations" // Keep this
	"github.com/jackc/pgx/v5/pgtype"          // Import pgtype for Timestamptz
)

// --- NEW: Payload Struct for New Like Notifications ---
// Contains basic info about the user who liked someone.
type WsBasicLikerInfo struct {
	LikerUserID        int32              `json:"liker_user_id"`
	Name               string             `json:"name"` // Combined Name + LastName
	FirstProfilePicURL string             `json:"first_profile_pic_url,omitempty"`
	IsRose             bool               `json:"is_rose"`
	LikeComment        *string            `json:"like_comment,omitempty"`
	LikedAt            pgtype.Timestamptz `json:"liked_at"` // Use pgtype for consistency
}

// --- NEW: Payload Struct for New Match Notifications ---
// Contains info about the new match and the like that triggered it.
type WsMatchInfo struct {
	MatchedUserID      int32      `json:"matched_user_id"`
	Name               string     `json:"name"` // Combined Name + LastName
	FirstProfilePicURL string     `json:"first_profile_pic_url,omitempty"`
	IsOnline           bool       `json:"is_online"`
	LastOnline         *time.Time `json:"last_online,omitempty"` // Use *time.Time for JSON marshal
	// ** Crucial addition: ID of the user whose like should be removed from the recipient's 'Likes You' list **
	InitiatingLikerUserID int32 `json:"initiating_liker_user_id"`
}

// --- NEW: Payload Struct for Like Removal Notifications ---
// Contains the ID of the user whose like should be removed.
type WsLikeRemovalInfo struct {
	LikerUserID int32 `json:"liker_user_id"` // ID of the person whose like is to be removed
}

// --- MODIFIED: WsMessage struct ---
// Added fields to carry payloads for new message types.
type WsMessage struct {
	Type string `json:"type"`         // e.g., "chat_message", "status_update", "new_like_received", "new_match", "like_removed"
	ID   *int64 `json:"id,omitempty"` // Usually for chat messages

	// --- Chat Message Fields (Keep as is) ---
	SenderUserID     *int32  `json:"sender_user_id,omitempty"`
	RecipientUserID  *int32  `json:"recipient_user_id,omitempty"`
	Text             *string `json:"text,omitempty"`
	MediaURL         *string `json:"media_url,omitempty"`
	MediaType        *string `json:"media_type,omitempty"`
	SentAt           *string `json:"sent_at,omitempty"` // ISO 8601 format string (outgoing)
	ReplyToMessageID *int64  `json:"reply_to_message_id,omitempty"`
	MessageID        *int64  `json:"message_id,omitempty"` // Used for Reactions, MarkRead

	// --- Reaction Fields (Keep as is) ---
	Emoji         *string `json:"emoji,omitempty"`
	ReactorUserID *int32  `json:"reactor_user_id,omitempty"`
	IsRemoved     *bool   `json:"is_removed,omitempty"`

	// --- Status Update Fields (Keep as is) ---
	UserID *int32  `json:"user_id,omitempty"` // User whose status changed
	Status *string `json:"status,omitempty"`  // "online" or "offline"

	// --- Mark Read Fields (Keep as is) ---
	OtherUserID  *int32 `json:"other_user_id,omitempty"`  // User whose messages were marked read
	ReaderUserID *int32 `json:"reader_user_id,omitempty"` // User who read the messages

	// --- Typing/Recording Fields (Keep as is) ---
	IsTyping        *bool  `json:"is_typing,omitempty"`
	TypingUserID    *int32 `json:"typing_user_id,omitempty"`
	IsRecording     *bool  `json:"is_recording,omitempty"`
	RecordingUserID *int32 `json:"recording_user_id,omitempty"`

	// --- General Info/Error/Ack Fields (Keep as is) ---
	Content *string `json:"content,omitempty"`
	Count   *int64  `json:"count,omitempty"`

	// --- NEW: Payload Fields for Like/Match Notifications ---
	LikerInfo   *WsBasicLikerInfo  `json:"liker_info,omitempty"`   // Payload for "new_like_received"
	MatchInfo   *WsMatchInfo       `json:"match_info,omitempty"`   // Payload for "new_match"
	RemovalInfo *WsLikeRemovalInfo `json:"removal_info,omitempty"` // Payload for "like_removed"
}

// --- Existing functions (ChatMessageToWsMessage, Ptr, PtrInt64) ---
// No changes needed for these existing functions.

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
	// Note: Reactions and CurrentUserReaction are not included here as they are handled separately
	// or added during the conversation retrieval process before sending.
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
