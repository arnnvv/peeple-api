package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type UnmatchRequest struct {
	TargetUserID int32 `json:"target_user_id"`
}

type UnmatchResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func UnmatchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
	pool, _ := db.GetPool()

	if queries == nil || pool == nil {
		log.Println("ERROR: UnmatchHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, UnmatchResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, UnmatchResponse{Success: false, Message: "Authentication required"})
		return
	}
	requesterUserID := int32(claims.UserID)

	var req UnmatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: UnmatchHandler: Invalid request body for user %d: %v", requesterUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.TargetUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Valid target_user_id is required"})
		return
	}
	if req.TargetUserID == requesterUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Cannot unmatch yourself"})
		return
	}

	log.Printf("INFO: Unmatch attempt: User %d -> User %d", requesterUserID, req.TargetUserID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("ERROR: UnmatchHandler: Failed to begin transaction for user %d: %v", requesterUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database transaction error"})
		return
	}
	defer func() {
		if err != nil {
			log.Printf("WARN: UnmatchHandler: Rolling back transaction due to error: %v", err)
			rbErr := tx.Rollback(ctx)
			if rbErr != nil {
				log.Printf("ERROR: UnmatchHandler: Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	qtx := queries.WithTx(tx)

	log.Printf("DEBUG: UnmatchHandler: Deleting likes between %d and %d", requesterUserID, req.TargetUserID)
	err = qtx.DeleteLikesBetweenUsers(ctx, migrations.DeleteLikesBetweenUsersParams{
		LikerUserID: requesterUserID,
		LikedUserID: req.TargetUserID,
	})
	if err != nil {
		log.Printf("ERROR: UnmatchHandler: Failed to delete likes between %d and %d: %v", requesterUserID, req.TargetUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to remove existing connection"})
		return
	}

	log.Printf("DEBUG: UnmatchHandler: Adding dislike from %d to %d", requesterUserID, req.TargetUserID)
	err = qtx.AddDislike(ctx, migrations.AddDislikeParams{
		DislikerUserID: requesterUserID,
		DislikedUserID: req.TargetUserID,
	})
	if err != nil {
		log.Printf("ERROR: UnmatchHandler: Failed to add dislike from %d to %d: %v", requesterUserID, req.TargetUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to record dislike"})
		return
	}

	log.Printf("DEBUG: UnmatchHandler: Marking chat messages as read for recipient %d from sender %d", requesterUserID, req.TargetUserID)
	markReadParams := migrations.MarkChatAsReadOnUnmatchParams{
		RecipientUserID: requesterUserID,
		SenderUserID:    req.TargetUserID,
	}
	cmdTag, err := qtx.MarkChatAsReadOnUnmatch(ctx, markReadParams)
	if err != nil {
		log.Printf("ERROR: UnmatchHandler: Failed to mark chat messages as read between %d and %d: %v", requesterUserID, req.TargetUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to update chat read status"})
		return
	}
	log.Printf("DEBUG: UnmatchHandler: Marked %d messages as read.", cmdTag.RowsAffected())

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("ERROR: UnmatchHandler: Failed to commit transaction for user %d unmatching %d: %v", requesterUserID, req.TargetUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database commit error"})
		return
	}

	log.Printf("INFO: Unmatch processed successfully: User %d -> User %d", requesterUserID, req.TargetUserID)
	utils.RespondWithJSON(w, http.StatusOK, UnmatchResponse{Success: true, Message: "Unmatched successfully"})
}
