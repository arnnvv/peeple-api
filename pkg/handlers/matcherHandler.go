package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type MatchInfo struct {
	MatchedUserID       int32      `json:"matched_user_id"`
	Name                string     `json:"name"`
	FirstProfilePicURL  string     `json:"first_profile_pic_url"`
	LastMessage         *string    `json:"last_message,omitempty"`
	LastMessageType     *string    `json:"last_message_type,omitempty"`
	LastMessageMediaURL *string    `json:"last_message_media_url,omitempty"`
	LastMessageSentAt   *time.Time `json:"last_message_sent_at,omitempty"`
	UnreadMessageCount  int64      `json:"unread_message_count"`
}

type GetMatchesResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Matches []MatchInfo `json:"matches"`
}

func GetMatchesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: GetMatchesHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetMatchesResponse{Success: false, Message: "Database connection error", Matches: []MatchInfo{}})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, GetMatchesResponse{Success: false, Message: "Method Not Allowed: Use GET", Matches: []MatchInfo{}})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, GetMatchesResponse{Success: false, Message: "Authentication required", Matches: []MatchInfo{}})
		return
	}
	requestingUserID := int32(claims.UserID)

	log.Printf("INFO: GetMatchesHandler: Fetching matches for user %d", requestingUserID)

	dbMatches, err := queries.GetMatchesWithLastMessage(ctx, requestingUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: GetMatchesHandler: Failed to fetch matches for user %d: %v", requestingUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetMatchesResponse{Success: false, Message: "Error retrieving matches", Matches: []MatchInfo{}})
		return
	}

	responseMatches := make([]MatchInfo, 0, len(dbMatches))

	for _, dbMatch := range dbMatches {
		match := MatchInfo{
			MatchedUserID:       dbMatch.MatchedUserID,
			Name:                buildFullName(dbMatch.MatchedUserName, dbMatch.MatchedUserLastName),
			FirstProfilePicURL:  getFirstMediaURL(dbMatch.MatchedUserMediaUrls),
			UnreadMessageCount:  dbMatch.UnreadMessageCount, // Assign the count directly
			LastMessage:         nil,
			LastMessageType:     nil,
			LastMessageMediaURL: nil,
			LastMessageSentAt:   nil,
		}

		if dbMatch.LastMessageText != "" { // If text is not empty, it was likely a text message
			messageText := dbMatch.LastMessageText
			match.LastMessage = &messageText
			messageType := "text" // Assume text if text content exists
			match.LastMessageType = &messageType
		} else if dbMatch.LastMessageMediaUrl.Valid { // If no text, check if media URL is valid
			mediaUrl := dbMatch.LastMessageMediaUrl.String
			match.LastMessageMediaURL = &mediaUrl
			if dbMatch.LastMessageMediaType.Valid { // If media URL is valid, use the media type
				mediaType := dbMatch.LastMessageMediaType.String
				match.LastMessageType = &mediaType // e.g., "image/jpeg", "video/mp4"
			} else {
				// Fallback if URL exists but type doesn't (shouldn't happen with constraints)
				unknownType := "media"
				match.LastMessageType = &unknownType
			}
		}
		// If both text and media are absent, LastMessageType remains nil

		if dbMatch.LastMessageSentAt.Valid {
			validTime := dbMatch.LastMessageSentAt.Time
			match.LastMessageSentAt = &validTime
		}

		// if dbMatch.LastMessageSenderID != 0 {
		//     senderID := dbMatch.LastMessageSenderID
		//     match.LastMessageSenderID = &senderID
		// }

		responseMatches = append(responseMatches, match)
	}

	log.Printf("INFO: GetMatchesHandler: Found %d matches for user %d, processed with last message details and unread counts.", len(responseMatches), requestingUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetMatchesResponse{
		Success: true,
		Matches: responseMatches,
	})
}
