// FILE: pkg/handlers/locationGenderHandler.go
// (MODIFIED)
package handlers

import (
	"encoding/json"
	"errors" // Import errors package
	"fmt"
	"log"
	"net/http"
	"time" // Import time package

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5" // Import pgx
	"github.com/jackc/pgx/v5/pgtype"
)

// Constants for default filter settings
const (
	defaultFilterRadiusKm    = 500
	defaultFilterAgeRange    = 4 // +/- 4 years from user's age
	minFilterAge             = 18
	fixedDefaultFilterMinAge = 18 // <-- ADDED Fixed Default Min Age
	fixedDefaultFilterMaxAge = 55 // <-- ADDED Fixed Default Max Age
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
// ADDED: Also sets default filters upon successful update.
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

	// --- ADDED: Fetch user data for default filter calculation ---
	userData, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("UpdateLocationGenderHandler: User %d not found while fetching for filters", userID)
			utils.RespondWithError(w, http.StatusNotFound, "User not found")
		} else {
			log.Printf("UpdateLocationGenderHandler: Error fetching user %d for filters: %v", userID, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Error retrieving user data")
		}
		return
	}

	// --- Database Update (Location/Gender) ---
	params := migrations.UpdateUserLocationGenderParams{
		Latitude:  pgtype.Float8{Float64: lat, Valid: true},
		Longitude: pgtype.Float8{Float64: lon, Valid: true},
		Gender:    migrations.NullGenderEnum{GenderEnum: genderEnum, Valid: true},
		ID:        userID,
	}

	_, err = queries.UpdateUserLocationGender(ctx, params)
	if err != nil {
		log.Printf("Error updating location/gender for user %d: %v", userID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update location and gender")
		return
	}

	log.Printf("Successfully updated location and gender for user %d", userID)

	// --- MODIFIED: Set Default Filters ---
	log.Printf("UpdateLocationGenderHandler: Setting default filters for user %d", userID)

	// Calculate Default Filters
	defaultWhoSee := migrations.NullGenderEnum{Valid: false}
	if genderEnum == migrations.GenderEnumMan {
		defaultWhoSee = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumWoman, Valid: true}
	} else if genderEnum == migrations.GenderEnumWoman {
		defaultWhoSee = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumMan, Valid: true}
	}

	defaultAgeMin := pgtype.Int4{Valid: false}
	defaultAgeMax := pgtype.Int4{Valid: false}

	// Try to calculate based on DOB if available
	if userData.DateOfBirth.Valid && !userData.DateOfBirth.Time.IsZero() {
		age := int(time.Since(userData.DateOfBirth.Time).Hours() / 24 / 365.25)
		if age >= minFilterAge {
			calcAgeMin := age - defaultFilterAgeRange
			if calcAgeMin < minFilterAge {
				calcAgeMin = minFilterAge
			}
			defaultAgeMin = pgtype.Int4{Int32: int32(calcAgeMin), Valid: true} // Set Valid to true

			calcAgeMax := age + defaultFilterAgeRange
			if calcAgeMax < calcAgeMin {
				calcAgeMax = calcAgeMin
			}
			if calcAgeMax < minFilterAge {
				calcAgeMax = minFilterAge
			}
			defaultAgeMax = pgtype.Int4{Int32: int32(calcAgeMax), Valid: true} // Set Valid to true
			log.Printf("UpdateLocationGenderHandler: User %d age %d, calculated default filter age range: %d-%d", userID, age, calcAgeMin, calcAgeMax)
		} else {
			log.Printf("UpdateLocationGenderHandler: User %d age %d is less than min filter age %d. Using fixed defaults.", userID, age, minFilterAge)
			// Fallback to fixed defaults if calculated age is too low
			defaultAgeMin = pgtype.Int4{Int32: fixedDefaultFilterMinAge, Valid: true}
			defaultAgeMax = pgtype.Int4{Int32: fixedDefaultFilterMaxAge, Valid: true}
		}
	} else {
		// *** ADDED ELSE BLOCK: Use fixed defaults if DOB is not valid ***
		log.Printf("UpdateLocationGenderHandler: User %d has no valid DOB. Using fixed default age range: %d-%d.", userID, fixedDefaultFilterMinAge, fixedDefaultFilterMaxAge)
		defaultAgeMin = pgtype.Int4{Int32: fixedDefaultFilterMinAge, Valid: true}
		defaultAgeMax = pgtype.Int4{Int32: fixedDefaultFilterMaxAge, Valid: true}
	}

	// Prepare filter upsert parameters
	defaultFilterParams := migrations.UpsertUserFiltersParams{
		UserID:          userID,
		WhoYouWantToSee: defaultWhoSee,
		RadiusKm:        pgtype.Int4{Int32: defaultFilterRadiusKm, Valid: true},
		ActiveToday:     false,         // Explicitly set to false as requested
		AgeMin:          defaultAgeMin, // Will now have a valid value
		AgeMax:          defaultAgeMax, // Will now have a valid value
	}

	// Upsert the default filters
	_, filterErr := queries.UpsertUserFilters(ctx, defaultFilterParams)
	if filterErr != nil {
		// Log the error but don't fail the overall request, as location/gender succeeded
		log.Printf("WARN: UpdateLocationGenderHandler: Failed to upsert default filters for user %d after location/gender update: %v", userID, filterErr)
	} else {
		log.Printf("UpdateLocationGenderHandler: Successfully upserted default filters for user %d", userID)
	}
	// --- END MODIFIED Filter Logic ---

	// Return success for the primary operation (location/gender update)
	utils.RespondWithJSON(w, http.StatusOK, UpdateLocationGenderResponse{
		Success: true,
		Message: "Location and gender updated successfully", // Message reflects the main goal
	})
}
