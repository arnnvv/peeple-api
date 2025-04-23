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

type UpdateOnlineResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func UpdateOnlineStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: UpdateOnlineStatusHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, UpdateOnlineResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, UpdateOnlineResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, UpdateOnlineResponse{Success: false, Message: "Authentication required"})
		return
	}
	userID := int32(claims.UserID)

	log.Printf("INFO: UpdateOnlineStatusHandler: Explicitly updating last_online for user %d", userID)

	err := queries.UpdateUserLastOnline(ctx, userID)
	if err != nil {
		log.Printf("ERROR: UpdateOnlineStatusHandler: Explicit update failed for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UpdateOnlineResponse{Success: false, Message: "Failed to update online status"})
		return
	}

	log.Printf("INFO: UpdateOnlineStatusHandler: last_online explicitly updated for user %d.", userID)
	utils.RespondWithJSON(w, http.StatusOK, UpdateOnlineResponse{Success: true, Message: "Online status updated"})
}

type FetchLastOnlineRequest struct {
	UserID int32 `json:"user_id"`
}

type FetchLastOnlineResponse struct {
	Success    bool       `json:"success"`
	Message    string     `json:"message,omitempty"`
	LastOnline *time.Time `json:"last_online"`
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
		log.Println("ERROR: FetchLastOnlineHandler: Database connection not available.")
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
		log.Printf("ERROR: FetchLastOnlineHandler: Invalid request body for user %d: %v", requesterUserID, err)
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

	log.Printf("INFO: FetchLastOnlineHandler: User %d requesting last_online for user %d", requesterUserID, targetUserID)

	mutualLikeParams := migrations.CheckMutualLikeExistsParams{
		LikerUserID: requesterUserID,
		LikedUserID: targetUserID,
	}
	mutualLikeResult, checkErr := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
	if checkErr != nil {
		log.Printf("ERROR: FetchLastOnlineHandler: Failed to check mutual like between %d and %d: %v", requesterUserID, targetUserID, checkErr)
		utils.RespondWithJSON(w, http.StatusInternalServerError, FetchLastOnlineResponse{Success: false, Message: "Error checking match status"})
		return
	}

	if !mutualLikeResult.Valid || !mutualLikeResult.Bool {
		log.Printf("WARN: FetchLastOnlineHandler: No mutual match between %d and %d. Access denied.", requesterUserID, targetUserID)
		utils.RespondWithJSON(w, http.StatusForbidden, FetchLastOnlineResponse{
			Success: false,
			Message: "You can only see the online status of users you have matched with.",
		})
		return
	}
	log.Printf("INFO: FetchLastOnlineHandler: Mutual match confirmed between %d and %d.", requesterUserID, targetUserID)

	lastOnlineTimestamp, err := queries.GetUserLastOnline(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: FetchLastOnlineHandler: Target user %d not found, despite match check passing.", targetUserID)
			utils.RespondWithJSON(w, http.StatusNotFound, FetchLastOnlineResponse{Success: false, Message: "Target user not found"})
		} else {
			log.Printf("ERROR: FetchLastOnlineHandler: Failed to fetch last_online for user %d: %v", targetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, FetchLastOnlineResponse{Success: false, Message: "Failed to retrieve online status"})
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, FetchLastOnlineResponse{
		Success:    true,
		LastOnline: pgTimestampToTimePtr(lastOnlineTimestamp),
	})
}
