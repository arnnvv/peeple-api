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
	MatchedUserID      int32  `json:"matched_user_id"`
	Name               string `json:"name"`
	FirstProfilePicURL string `json:"first_profile_pic_url"`
	UnreadMessageCount int64  `json:"unread_message_count"`

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

	log.Printf("INFO: GetMatchesHandler: Fetching matches with last event for user %d", requestingUserID)

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
			UnreadMessageCount: dbMatch.UnreadMessageCount,
			LastEventTimestamp: nil,
			LastEventUserID:    nil,
			LastEventType:      nil,
			LastEventContent:   nil,
			LastEventMediaURL:  nil,
		}

		// *** FIX: Check nullability based on Timestamp being valid ***
		// If the LEFT JOIN didn't find an event, the timestamp (and likely others) will be NULL/Invalid.
		// Assume LastEventTimestamp is pgtype.Timestamptz as it's less likely sqlc got *that* wrong.
		// If it IS ALSO a basic time.Time, check !dbMatch.LastEventTimestamp.IsZero()
		if dbMatch.LastEventTimestamp.Valid {

			ts := dbMatch.LastEventTimestamp.Time
			match.LastEventTimestamp = &ts

			// Assign basic types directly IF they are not zero/empty (use pointers)
			// We assume if Timestamp is valid, the other fields from the LATERAL JOIN row are also valid,
			// even if they are basic types now.
			eventType := dbMatch.LastEventType // Access directly as string
			if eventType != "" {               // Check if not empty string
				match.LastEventType = &eventType
			}

			eventUserID := dbMatch.LastEventUserID // Access directly as int32
			if eventUserID != 0 {                  // Check if not zero (assuming 0 is not a valid user ID here)
				match.LastEventUserID = &eventUserID
			}

			if dbMatch.LastEventContent.Valid {
				content := dbMatch.LastEventContent.String
				match.LastEventContent = &content
			}
			if eventType == "media" && dbMatch.LastEventExtra.Valid {
				mediaUrl := dbMatch.LastEventExtra.String
				match.LastEventMediaURL = &mediaUrl
			}
		}

		responseMatches = append(responseMatches, match)
	}

	log.Printf("INFO: GetMatchesHandler: Found %d matches for user %d, processed with last event details.", len(responseMatches), requestingUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetMatchesResponse{
		Success: true,
		Matches: responseMatches,
	})
}
