// FILE: pkg/handlers/locationGenderHandler.go
// (NEW FILE)
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

type UpdateLocationGenderRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Gender    *string  `json:"gender"` // Expect "man" or "woman"
}

type UpdateLocationGenderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// UpdateLocationGenderHandler handles updating only the user's location and gender.
func UpdateLocationGenderHandler(w http.ResponseWriter, r *http.Request) {
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
	userID := int32(claims.UserID)

	if r.Method != http.MethodPost {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	var req UpdateLocationGenderRequest
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
	genderStr := *req.Gender

	if lat < -90 || lat > 90 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid latitude: must be between -90 and 90")
		return
	}
	if lon < -180 || lon > 180 {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid longitude: must be between -180 and 180")
		return
	}

	var genderEnum migrations.GenderEnum
	isValidGender := false
	if genderStr == string(migrations.GenderEnumMan) {
		genderEnum = migrations.GenderEnumMan
		isValidGender = true
	} else if genderStr == string(migrations.GenderEnumWoman) {
		genderEnum = migrations.GenderEnumWoman
		isValidGender = true
	}

	if !isValidGender {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid gender: must be 'man' or 'woman'")
		return
	}

	// --- Database Update ---
	params := migrations.UpdateUserLocationGenderParams{
		Latitude:  pgtype.Float8{Float64: lat, Valid: true},
		Longitude: pgtype.Float8{Float64: lon, Valid: true},
		Gender:    migrations.NullGenderEnum{GenderEnum: genderEnum, Valid: true},
		ID:        userID,
	}

	_, err := queries.UpdateUserLocationGender(ctx, params)
	if err != nil {
		log.Printf("Error updating location/gender for user %d: %v", userID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update location and gender")
		return
	}

	log.Printf("Successfully updated location and gender for user %d", userID)
	utils.RespondWithJSON(w, http.StatusOK, UpdateLocationGenderResponse{
		Success: true,
		Message: "Location and gender updated successfully",
	})
}
