// FILE: pkg/handlers/quickFeedHandler.go
// (MODIFIED: Removed request body, fetches user data from DB based on token)
package handlers

import (
	"errors"
	// "fmt" // No longer needed for request body error formatting
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

// QuickFeedRequest removed as it's no longer needed.

// Use GetQuickFeedRow as confirmed from queries.sql.go
type QuickFeedResponse struct {
	Success  bool                         `json:"success"`
	Message  string                       `json:"message,omitempty"`
	Profiles []migrations.GetQuickFeedRow `json:"profiles,omitempty"` // Use GetQuickFeedRow
}

const quickFeedLimit = 2

// GetQuickFeedHandler serves the simple two-profile feed.
// MODIFIED: Now uses GET and retrieves user's location/gender from DB.
func GetQuickFeedHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection not available")
		return
	}

	// --- Get User ID from Token ---
	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}
	requestingUserID := int32(claims.UserID)

	// --- Changed Method Check to GET ---
	if r.Method != http.MethodGet {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use GET")
		return
	}

	// --- REMOVED Request Body Parsing ---
	// var req QuickFeedRequest ... json.NewDecoder ...

	// --- Fetch Requesting User's Data ---
	requestingUser, err := queries.GetUserByID(ctx, requestingUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("GetQuickFeedHandler: Requesting user %d not found", requestingUserID)
			utils.RespondWithJSON(w, http.StatusNotFound, QuickFeedResponse{
				Success: false, Message: "Requesting user account not found.",
			})
		} else {
			log.Printf("GetQuickFeedHandler: Error fetching requesting user %d: %v", requestingUserID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve user data")
		}
		return
	}

	// --- Validation of Fetched User Data ---
	if !requestingUser.Latitude.Valid || !requestingUser.Longitude.Valid {
		log.Printf("GetQuickFeedHandler: Requesting user %d missing required location data.", requestingUserID)
		utils.RespondWithError(w, http.StatusBadRequest, "Your location is not set. Please update your profile.")
		return
	}
	if !requestingUser.Gender.Valid {
		log.Printf("GetQuickFeedHandler: Requesting user %d missing required gender data.", requestingUserID)
		utils.RespondWithError(w, http.StatusBadRequest, "Your gender is not set. Please update your profile.")
		return
	}

	// --- Determine Opposite Gender based on Fetched Data ---
	var oppositeGender migrations.NullGenderEnum
	requestingGender := requestingUser.Gender.GenderEnum // For logging

	if requestingUser.Gender.GenderEnum == migrations.GenderEnumMan {
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumWoman, Valid: true}
	} else if requestingUser.Gender.GenderEnum == migrations.GenderEnumWoman {
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumMan, Valid: true}
	} else {
		// Should be caught by !requestingUser.Gender.Valid check above, but good to be safe
		log.Printf("GetQuickFeedHandler: Invalid gender '%v' found in database for user %d.", requestingUser.Gender.GenderEnum, requestingUserID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Invalid gender data found for user")
		return
	}

	// --- Use Fetched Data for Query ---
	lat := requestingUser.Latitude.Float64
	lon := requestingUser.Longitude.Float64

	log.Printf("Fetching quick feed for user %d (gender: %s) using DB location (lat: %f, lon: %f), showing %s",
		requestingUserID, requestingGender, lat, lon, oppositeGender.GenderEnum)

	// --- Prepare DB Parameters ---
	params := migrations.GetQuickFeedParams{
		Lat1:   lat,              // Fetched latitude
		Lon1:   lon,              // Fetched longitude
		ID:     requestingUserID, // User ID from token
		Gender: oppositeGender,   // Calculated opposite gender
		Limit:  quickFeedLimit,
	}

	// --- Execute Database Query ---
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
