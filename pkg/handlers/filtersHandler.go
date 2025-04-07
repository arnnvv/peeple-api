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
	"github.com/jackc/pgx/v5/pgtype"
)

// ApplyFiltersRequest defines the expected structure for the filter request body.
type ApplyFiltersRequest struct {
	WhoYouWantToSee string `json:"whoYouWantToSee"` // Expect "man" or "woman"
	RadiusKm        int    `json:"radius"`          // In kilometers
	ActiveToday     *bool  `json:"activeToday"`     // Optional, defaults to false
	AgeMin          int    `json:"ageMin"`
	AgeMax          int    `json:"ageMax"`
}

// ApplyFiltersResponse defines the structure for the response.
type ApplyFiltersResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Filters *migrations.Filter `json:"filters,omitempty"` // Return the saved filters
}

// ApplyFiltersHandler handles POST requests to save/update user feed filters.
func ApplyFiltersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, ApplyFiltersResponse{
			Success: false, Message: "Method Not Allowed: Use POST",
		})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, ApplyFiltersResponse{
			Success: false, Message: "Authentication required",
		})
		return
	}
	userID := int32(claims.UserID)

	var req ApplyFiltersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ApplyFiltersHandler: Error decoding request body for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: fmt.Sprintf("Invalid request body format: %v", err),
		})
		return
	}
	defer r.Body.Close()

	// --- Validation ---
	var whoSee migrations.NullGenderEnum
	if req.WhoYouWantToSee == string(migrations.GenderEnumMan) || req.WhoYouWantToSee == string(migrations.GenderEnumWoman) {
		whoSee = migrations.NullGenderEnum{
			GenderEnum: migrations.GenderEnum(req.WhoYouWantToSee),
			Valid:      true,
		}
	} else if req.WhoYouWantToSee == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'whoYouWantToSee' must be 'man' or 'woman'",
		})
		return
	} else {
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'whoYouWantToSee' must be 'man' or 'woman'",
		})
		return
	}

	if req.RadiusKm <= 0 || req.RadiusKm > 500 { // Match DB constraint
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'radius' must be between 1 and 500 km",
		})
		return
	}

	if req.AgeMin < 18 {
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'ageMin' must be 18 or greater",
		})
		return
	}

	if req.AgeMax < 18 {
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'ageMax' must be 18 or greater",
		})
		return
	}

	if req.AgeMax < req.AgeMin {
		utils.RespondWithJSON(w, http.StatusBadRequest, ApplyFiltersResponse{
			Success: false, Message: "Validation Error: 'ageMax' cannot be less than 'ageMin'",
		})
		return
	}

	activeTodayValue := false // Default value
	if req.ActiveToday != nil {
		activeTodayValue = *req.ActiveToday
	}
	// No need for: activeTodayParam := pgtype.Bool{Bool: activeTodayValue, Valid: true}

	// --- Prepare DB Parameters ---
	params := migrations.UpsertUserFiltersParams{
		UserID:          userID,
		WhoYouWantToSee: whoSee, // Use the validated NullGenderEnum
		RadiusKm:        pgtype.Int4{Int32: int32(req.RadiusKm), Valid: true},
		ActiveToday:     activeTodayValue, // <-- CORRECTED: Assign the bool value directly
		AgeMin:          pgtype.Int4{Int32: int32(req.AgeMin), Valid: true},
		AgeMax:          pgtype.Int4{Int32: int32(req.AgeMax), Valid: true},
	}

	// --- Execute Upsert Query ---
	log.Printf("ApplyFiltersHandler: Upserting filters for user %d", userID)
	updatedFilters, err := queries.UpsertUserFilters(ctx, params)
	if err != nil {
		log.Printf("ApplyFiltersHandler: Error upserting filters for user %d: %v", userID, err)
		// Consider more specific error handling for constraint violations if needed
		utils.RespondWithJSON(w, http.StatusInternalServerError, ApplyFiltersResponse{
			Success: false, Message: "Database error saving filters",
		})
		return
	}
	log.Printf("ApplyFiltersHandler: Filters successfully saved/updated for user %d", userID)

	// --- Respond ---
	utils.RespondWithJSON(w, http.StatusOK, ApplyFiltersResponse{
		Success: true,
		Message: "Filters applied successfully",
		Filters: &updatedFilters,
	})
}
