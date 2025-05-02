// FILE: pkg/handlers/analyticsLogPhotoViewsHandler.go
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

// PhotoViewEvent represents a single photo view duration event
type PhotoViewEvent struct {
	ViewedUserID int32 `json:"viewed_user_id"`
	PhotoIndex   int   `json:"photo_index"` // Use int for easier JSON decoding
	DurationMs   int   `json:"duration_ms"`
}

// LogPhotoViewsRequest is the expected structure of the request body
type LogPhotoViewsRequest struct {
	Views []PhotoViewEvent `json:"views"`
}

// LogPhotoViewsResponse is the standard success/failure response
type LogPhotoViewsResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// LogPhotoViewsHandler handles POST requests to log photo view durations
func LogPhotoViewsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: LogPhotoViewsHandler: Database connection not available.")
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection error")
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	viewerUserID := int32(claims.UserID) // The user doing the viewing

	var req LogPhotoViewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: LogPhotoViewsHandler: Invalid request body for user %d: %v", viewerUserID, err)
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body format")
		return
	}
	defer r.Body.Close()

	if len(req.Views) == 0 {
		// It's debatable whether this is an error or just nothing to log.
		// Let's treat it as success with no action.
		log.Printf("INFO: LogPhotoViewsHandler: Received empty 'views' array from user %d. No action taken.", viewerUserID)
		utils.RespondWithJSON(w, http.StatusOK, LogPhotoViewsResponse{
			Success: true,
			Message: "No view data provided.",
		})
		return
	}

	log.Printf("INFO: LogPhotoViewsHandler: Processing %d photo view events from user %d.", len(req.Views), viewerUserID)

	loggedCount := 0
	var firstValidationError error

	// --- Validation Loop ---
	for i, view := range req.Views {
		if view.ViewedUserID <= 0 {
			firstValidationError = fmt.Errorf("view %d: invalid viewed_user_id (%d)", i, view.ViewedUserID)
			break
		}
		if view.ViewedUserID == viewerUserID {
			firstValidationError = fmt.Errorf("view %d: viewer_user_id cannot be the same as viewed_user_id (%d)", i, view.ViewedUserID)
			break
		}
		if view.PhotoIndex < 0 || view.PhotoIndex > 5 { // Ensure index is within 0-5 range
			firstValidationError = fmt.Errorf("view %d: invalid photo_index (%d), must be between 0 and 5", i, view.PhotoIndex)
			break
		}
		if view.DurationMs <= 0 {
			firstValidationError = fmt.Errorf("view %d: invalid duration_ms (%d), must be positive", i, view.DurationMs)
			break
		}
		// Optional: Could add a check here to ensure viewed_user_id exists in the DB, but might slow down logging.
	}

	if firstValidationError != nil {
		log.Printf("ERROR: LogPhotoViewsHandler: Validation failed for user %d: %v", viewerUserID, firstValidationError)
		utils.RespondWithError(w, http.StatusBadRequest, firstValidationError.Error())
		return
	}

	// --- Logging Loop ---
	// Consider if transaction is needed. For analytics, usually not critical.
	// Logging errors individually might be better than failing the whole batch.
	var firstDbError error
	for i, view := range req.Views {
		logParams := migrations.LogPhotoViewDurationParams{
			ViewerUserID: viewerUserID,
			ViewedUserID: view.ViewedUserID,
			PhotoIndex:   int16(view.PhotoIndex), // Convert int to int16 for DB query
			DurationMs:   int32(view.DurationMs), // Convert int to int32 for DB query
		}

		err := queries.LogPhotoViewDuration(ctx, logParams)
		if err != nil {
			// Log the specific error and continue, or break and report the first error
			log.Printf("ERROR: LogPhotoViewsHandler: Failed to log view event %d for user %d (Viewer: %d, Viewed: %d, Index: %d, Duration: %dms): %v",
				i, viewerUserID, viewerUserID, view.ViewedUserID, view.PhotoIndex, view.DurationMs, err)
			if firstDbError == nil { // Capture the first DB error encountered
				firstDbError = fmt.Errorf("failed to log view event for user %d, photo index %d", view.ViewedUserID, view.PhotoIndex)
			}
			// Decide whether to continue or break. Let's break on first DB error for now.
			break
		} else {
			loggedCount++
		}
	}

	if firstDbError != nil {
		// Respond with an internal server error if any DB logging failed
		utils.RespondWithError(w, http.StatusInternalServerError, firstDbError.Error())
		return
	}

	log.Printf("INFO: LogPhotoViewsHandler: Successfully logged %d out of %d photo view events for user %d.", loggedCount, len(req.Views), viewerUserID)

	utils.RespondWithJSON(w, http.StatusOK, LogPhotoViewsResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully logged %d view events.", loggedCount),
	})
}
