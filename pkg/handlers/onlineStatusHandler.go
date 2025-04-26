package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type FetchLastOnlineRequest struct {
	UserID int32 `json:"user_id"`
}

type FetchLastOnlineResponse struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message,omitempty"`
	LastOnline *time.Time `json:"last_online"`
	IsOnline   bool       `json:"is_online"`
}

func pgTimestampToTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func FetchLastOnlineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, FetchLastOnlineResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, FetchLastOnlineResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, FetchLastOnlineResponse{Success: false, Message: "Authentication required"})
		return
	}
	requesterUserID := int32(claims.UserID)

	var req FetchLastOnlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, FetchLastOnlineResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	targetUserID := req.UserID

	if targetUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, FetchLastOnlineResponse{Success: false, Message: "Valid user_id is required in request body"})
		return
	}
	if targetUserID == requesterUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, FetchLastOnlineResponse{Success: false, Message: "Cannot fetch your own online status this way"})
		return
	}

	log.Printf("INFO: FetchLastOnlineHandler: User %d requesting status for user %d", requesterUserID, targetUserID)

	mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: requesterUserID, LikedUserID: targetUserID}
	mutualLikeResult, checkErr := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
	if checkErr != nil {
		log.Printf("ERROR: FetchLastOnlineHandler: Failed to check mutual like between %d and %d: %v", requesterUserID, targetUserID, checkErr)
		utils.RespondWithJSON(w, http.StatusInternalServerError, FetchLastOnlineResponse{Success: false, Message: "Error checking match status"})
		return
	}
	if !mutualLikeResult.Valid || !mutualLikeResult.Bool {
		utils.RespondWithJSON(w, http.StatusForbidden, FetchLastOnlineResponse{Success: false, Message: "You can only see the online status of users you have matched with."})
		return
	}
	log.Printf("INFO: FetchLastOnlineHandler: Mutual match confirmed between %d and %d.", requesterUserID, targetUserID)

	targetUser, err := queries.GetUserByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithJSON(w, http.StatusNotFound, FetchLastOnlineResponse{Success: false, Message: "Target user not found"})
		} else {
			log.Printf("ERROR: FetchLastOnlineHandler: Failed to fetch user data for user %d: %v", targetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, FetchLastOnlineResponse{Success: false, Message: "Failed to retrieve user status"})
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, FetchLastOnlineResponse{
		Success:    true,
		LastOnline: pgTimestampToTimePtr(targetUser.LastOnline),
		IsOnline:   targetUser.IsOnline,
	})
}
