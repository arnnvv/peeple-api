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

type MarkLikesSeenUntilRequest struct {
	LikeID int32 `json:"like_id"`
}

type MarkLikesSeenUntilResponse struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	LikesMarkedAsSeen int64  `json:"likes_marked_as_seen"`
}

func MarkLikesSeenUntilHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: MarkLikesSeenUntilHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, MarkLikesSeenUntilResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, MarkLikesSeenUntilResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, MarkLikesSeenUntilResponse{Success: false, Message: "Authentication required"})
		return
	}
	recipientUserID := int32(claims.UserID)

	var req MarkLikesSeenUntilRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: MarkLikesSeenUntilHandler: Invalid request body for user %d: %v", recipientUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkLikesSeenUntilResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.LikeID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkLikesSeenUntilResponse{Success: false, Message: "Valid like_id is required"})
		return
	}

	log.Printf("INFO: MarkLikesSeenUntilHandler: User %d attempting to mark likes as seen up to like ID %d", recipientUserID, req.LikeID)

	log.Printf("DEBUG: MarkLikesSeenUntilHandler: Validating boundary like ID %d for recipient %d", req.LikeID, recipientUserID)
	checkParams := migrations.CheckLikeExistsForRecipientParams{
		ID:          req.LikeID,
		LikedUserID: recipientUserID,
	}
	likeExists, errCheck := queries.CheckLikeExistsForRecipient(ctx, checkParams)
	if errCheck != nil {
		log.Printf("ERROR: MarkLikesSeenUntilHandler: Failed to check existence of like ID %d for user %d: %v", req.LikeID, recipientUserID, errCheck)
		utils.RespondWithJSON(w, http.StatusInternalServerError, MarkLikesSeenUntilResponse{Success: false, Message: "Error validating like ID"})
		return
	}

	if !likeExists {
		log.Printf("WARN: MarkLikesSeenUntilHandler: Boundary like ID %d does not exist or does not belong to user %d.", req.LikeID, recipientUserID)
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkLikesSeenUntilResponse{Success: false, Message: "Invalid boundary like ID provided. Ensure it's a like you received."})
		return
	}
	log.Printf("DEBUG: MarkLikesSeenUntilHandler: Boundary like ID %d validation passed for user %d.", req.LikeID, recipientUserID)

	updateParams := migrations.MarkLikesAsSeenUntilParams{
		LikedUserID: recipientUserID,
		ID:          req.LikeID,
	}

	cmdTag, err := queries.MarkLikesAsSeenUntil(ctx, updateParams)
	if err != nil {
		log.Printf("ERROR: MarkLikesSeenUntilHandler: Failed to update likes for user %d up to ID %d: %v", recipientUserID, req.LikeID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, MarkLikesSeenUntilResponse{Success: false, Message: "Failed to update like status"})
		return
	}

	rowsAffected := cmdTag.RowsAffected()
	log.Printf("INFO: MarkLikesSeenUntilHandler: Successfully marked %d likes as seen for user %d (up to ID %d)",
		rowsAffected, recipientUserID, req.LikeID)

	utils.RespondWithJSON(w, http.StatusOK, MarkLikesSeenUntilResponse{
		Success:           true,
		Message:           "Likes marked as seen successfully.",
		LikesMarkedAsSeen: rowsAffected,
	})
}
