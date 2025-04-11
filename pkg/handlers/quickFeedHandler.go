// FILE: pkg/handlers/quickFeedHandler.go
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations" // Make sure this import path is correct
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type QuickFeedRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Gender    *string  `json:"gender"` // Requesting user's gender ("man" or "woman")
}

// Use GetQuickFeedRow as confirmed from queries.sql.go
type QuickFeedResponse struct {
	Success  bool                         `json:"success"`
	Message  string                       `json:"message,omitempty"`
	Profiles []migrations.GetQuickFeedRow `json:"profiles,omitempty"` // Use GetQuickFeedRow
}

const quickFeedLimit = 2

// GetQuickFeedHandler serves the simple two-profile feed.
func GetQuickFeedHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection not available")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}
	requestingUserID := int32(claims.UserID)

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	var req QuickFeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	defer r.Body.Close()

	// --- Validation ---
	if req.Latitude == nil || req.Longitude == nil || req.Gender == nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields: latitude, longitude, and gender")
		return
	}

	lat := *req.Latitude
	lon := *req.Longitude
	requestingGenderStr := *req.Gender

	if lat < -90 || lat > 90 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid latitude")
		return
	}
	if lon < -180 || lon > 180 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid longitude")
		return
	}

	var requestingGender migrations.GenderEnum   // Keep this for logging if needed
	var oppositeGender migrations.NullGenderEnum // Use NullGenderEnum for the parameter struct
	isValidGender := false

	if requestingGenderStr == string(migrations.GenderEnumMan) {
		requestingGender = migrations.GenderEnumMan
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumWoman, Valid: true}
		isValidGender = true
	} else if requestingGenderStr == string(migrations.GenderEnumWoman) {
		requestingGender = migrations.GenderEnumWoman
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumMan, Valid: true}
		isValidGender = true
	}

	if !isValidGender {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid gender specified in request: must be 'man' or 'woman'")
		return
	}

	// --- Database Query ---
	log.Printf("Fetching quick feed for user %d (lat: %f, lon: %f, gender: %s), showing %s",
		requestingUserID, lat, lon, requestingGender, oppositeGender.GenderEnum) // Log the enum value

	// --- CORRECTED PARAMETER NAMES ---
	// Use the names from the generated GetQuickFeedParams struct
	params := migrations.GetQuickFeedParams{
		Lat1:   lat,              // Corresponds to $1
		Lon1:   lon,              // Corresponds to $2
		ID:     requestingUserID, // Corresponds to $3
		Gender: oppositeGender,   // Corresponds to $4 (needs NullGenderEnum)
		Limit:  quickFeedLimit,   // Corresponds to $5
	}

	// Use GetQuickFeedRow as the expected return type
	profiles, err := queries.GetQuickFeed(ctx, params)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Error fetching quick feed for user %d: %v", requestingUserID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve quick feed")
		return
	}

	if errors.Is(err, pgx.ErrNoRows) || len(profiles) == 0 {
		log.Printf("No profiles found for quick feed for user %d", requestingUserID)
		utils.RespondWithJSON(w, http.StatusOK, QuickFeedResponse{
			Success:  true,
			Profiles: []migrations.GetQuickFeedRow{}, // Use correct empty slice type
		})
		return
	}

	log.Printf("Found %d profiles for quick feed for user %d", len(profiles), requestingUserID)
	// Assign the result directly as its type matches the response struct field
	utils.RespondWithJSON(w, http.StatusOK, QuickFeedResponse{
		Success:  true,
		Profiles: profiles, // Type is []migrations.GetQuickFeedRow
	})
}
