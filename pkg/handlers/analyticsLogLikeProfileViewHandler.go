package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type LogLikeProfileViewRequest struct {
	LikerUserID int32 `json:"liker_user_id"` // The user who sent the like (whose profile is being viewed)
	LikeID      int32 `json:"like_id"`       // The ID of the specific like entry
}

type LogLikeProfileViewResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func LogLikeProfileViewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection error")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	// This is the user who RECEIVED the like and is VIEWING the liker's profile
	viewerUserID := int32(claims.UserID)

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	var req LogLikeProfileViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	defer r.Body.Close()

	if req.LikerUserID <= 0 || req.LikeID <= 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields: liker_user_id and like_id")
		return
	}

	// --- Verification Step ---
	// Ensure the provided like_id actually corresponds to a like received by the viewer from the specified liker.
	checkParams := migrations.CheckLikeExistsForRecipientParams{
		ID:          req.LikeID,
		LikedUserID: viewerUserID, // The viewer must be the recipient of the like
	}
	likeExists, errCheck := queries.CheckLikeExistsForRecipient(ctx, checkParams)
	if errCheck != nil {
		log.Printf("Error verifying like existence for log: viewer=%d, like_id=%d, error=%v", viewerUserID, req.LikeID, errCheck)
		utils.RespondWithError(w, http.StatusInternalServerError, "Error verifying like details")
		return
	}
	if !likeExists {
		log.Printf("WARN: Attempt to log view for non-existent or incorrect like: viewer=%d, like_id=%d, claimed_liker=%d", viewerUserID, req.LikeID, req.LikerUserID)
		utils.RespondWithError(w, http.StatusNotFound, "The specified like was not found for this user.")
		return
	}
	// Optional further check: Fetch the like details to confirm the liker_user_id matches req.LikerUserID if CheckLikeExistsForRecipient isn't sufficient.

	// --- Log the View ---
	logParams := migrations.LogLikeProfileViewParams{
		ViewerUserID: viewerUserID,
		LikerUserID:  req.LikerUserID,
		LikeID:       req.LikeID,
	}

	err := queries.LogLikeProfileView(ctx, logParams)
	if err != nil {
		// Handle potential duplicate logging if necessary (e.g., unique constraint on viewer/liker/like_id?)
		// For now, just log the generic error.
		log.Printf("Error logging like profile view: viewer=%d, liker=%d, like_id=%d, error=%v",
			viewerUserID, req.LikerUserID, req.LikeID, err)
		// Avoid sending 500 if it's a likely duplicate view log attempt, maybe return 200 or 409?
		// Let's return 500 for now to indicate a failure to log.
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to log profile view event")
		return
	}

	log.Printf("Successfully logged like profile view: viewer=%d, liker=%d, like_id=%d",
		viewerUserID, req.LikerUserID, req.LikeID)

	utils.RespondWithJSON(w, http.StatusOK, LogLikeProfileViewResponse{
		Success: true,
		Message: "View logged successfully.",
	})
}
