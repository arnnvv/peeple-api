package handlers

import (
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

// AppOpenResponse defines the structure for the response.
type AppOpenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// LogAppOpenHandler handles POST requests to log an app open event for the authenticated user.
func LogAppOpenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	// Although POST, we don't expect a body, but POST is suitable for creating a resource (log entry)
	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, AppOpenResponse{
			Success: false, Message: "Method Not Allowed: Use POST",
		})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, AppOpenResponse{
			Success: false, Message: "Authentication required",
		})
		return
	}
	userID := int32(claims.UserID)

	// Log the event in the database
	log.Printf("LogAppOpenHandler: Logging app open for user %d", userID)
	err := queries.LogAppOpen(ctx, userID)
	if err != nil {
		log.Printf("LogAppOpenHandler: Error logging app open for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, AppOpenResponse{
			Success: false, Message: "Database error logging event",
		})
		return
	}
	log.Printf("LogAppOpenHandler: App open successfully logged for user %d", userID)

	// Respond with success
	utils.RespondWithJSON(w, http.StatusOK, AppOpenResponse{
		Success: true,
		Message: "App open event logged successfully",
	})
}
