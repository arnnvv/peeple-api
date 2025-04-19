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
	MatchedUserID      int32      `json:"matched_user_id"`
	Name               string     `json:"name"`
	FirstProfilePicURL string     `json:"first_profile_pic_url"`
	LastMessage        *string    `json:"last_message,omitempty"`
	LastMessageSentAt  *time.Time `json:"last_message_sent_at,omitempty"`
}

type GetMatchesResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Matches []MatchInfo `json:"matches"`
}

func GetMatchesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
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
			MatchedUserID:      dbMatch.MatchedUserID,
			Name:               buildFullName(dbMatch.MatchedUserName, dbMatch.MatchedUserLastName),
			FirstProfilePicURL: getFirstMediaURL(dbMatch.MatchedUserMediaUrls),
			LastMessage:        nil,
			LastMessageSentAt:  nil,
		}

		if dbMatch.LastMessageSentAt.Valid {
			messageText := dbMatch.LastMessageText
			match.LastMessage = &messageText

			validTime := dbMatch.LastMessageSentAt.Time
			match.LastMessageSentAt = &validTime

			// Optionally handle sender ID if needed and generated correctly
			// if dbMatch.LastMessageSenderID.Valid {
			//     senderID := dbMatch.LastMessageSenderID.Int32
			//     match.LastMessageSenderID = &senderID
			// }
		}

		responseMatches = append(responseMatches, match)
	}

	log.Printf("INFO: GetMatchesHandler: Found %d matches for user %d", len(responseMatches), requestingUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetMatchesResponse{
		Success: true,
		Matches: responseMatches,
	})
}
