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
	IsOnline           bool       `json:"is_online"`
	LastOnline         *time.Time `json:"last_online,omitempty"`
	UnreadMessageCount int64      `json:"unread_message_count"`

	LastEventTimestamp *time.Time `json:"last_event_timestamp,omitempty"`
	LastEventUserID    *int32     `json:"last_event_user_id,omitempty"`
	LastEventType      *string    `json:"last_event_type,omitempty"`
	LastEventContent   *string    `json:"last_event_content,omitempty"`
	LastEventMediaURL  *string    `json:"last_event_media_url,omitempty"`
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

	log.Printf("INFO: GetMatchesHandler: Fetching matches with details for user %d", requestingUserID)

	dbMatches, err := queries.GetMatchesWithLastEvent(ctx, requestingUserID)
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
			IsOnline:           dbMatch.MatchedUserIsOnline,
			LastOnline:         pgTimestampToTimePtr(dbMatch.MatchedUserLastOnline),
			UnreadMessageCount: dbMatch.UnreadMessageCount,
			LastEventTimestamp: nil,
			LastEventUserID:    nil,
			LastEventType:      nil,
			LastEventContent:   nil,
			LastEventMediaURL:  nil,
		}

		if dbMatch.LastEventUserID != 0 {
			uid := dbMatch.LastEventUserID
			match.LastEventUserID = &uid

			if dbMatch.LastEventTimestamp.Valid {
				ts := dbMatch.LastEventTimestamp.Time
				match.LastEventTimestamp = &ts
			}
			if dbMatch.LastEventType != "" {
				et := dbMatch.LastEventType
				match.LastEventType = &et
			}
			if dbMatch.LastEventContent != "" {
				ec := dbMatch.LastEventContent
				match.LastEventContent = &ec
			}
			if dbMatch.LastEventType == "media" && dbMatch.LastEventExtra != "" {
				ee := dbMatch.LastEventExtra
				match.LastEventMediaURL = &ee
			}
		}

		responseMatches = append(responseMatches, match)
	}

	log.Printf("INFO: GetMatchesHandler: Found %d matches for user %d.", len(responseMatches), requestingUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetMatchesResponse{
		Success: true,
		Matches: responseMatches,
	})
}
