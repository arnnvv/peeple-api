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

type DislikeRequest struct {
	DislikedUserID int32 `json:"disliked_user_id"`
}

type DislikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func DislikeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, DislikeResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, DislikeResponse{Success: false, Message: "Authentication required"})
		return
	}
	dislikerUserID := int32(claims.UserID)

	var req DislikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.DislikedUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Valid disliked_user_id is required"})
		return
	}

	if req.DislikedUserID == dislikerUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Cannot dislike yourself"})
		return
	}

	log.Printf("Dislike attempt: User %d -> User %d", dislikerUserID, req.DislikedUserID)

	err := queries.AddDislike(ctx, migrations.AddDislikeParams{
		DislikerUserID: dislikerUserID,
		DislikedUserID: req.DislikedUserID,
	})

	if err != nil {
		log.Printf("Error adding dislike for user %d -> %d: %v", dislikerUserID, req.DislikedUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, DislikeResponse{Success: false, Message: "Failed to process dislike"})
		return
	}

	log.Printf("Dislike processed successfully: User %d -> User %d", dislikerUserID, req.DislikedUserID)
	utils.RespondWithJSON(w, http.StatusOK, DislikeResponse{Success: true, Message: "Disliked successfully"})
}
