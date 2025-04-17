package handlers

import (
	"encoding/json" // Import json package
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const feedBatchSize = 15
const defaultAgeRange = 4

// --- NEW: DTO for profile items in the response ---
// This includes all fields from GetHomeFeedRow PLUS the correctly typed Prompts
type ProfileFeedItem struct {
	// Embed all fields from migrations.User directly for simplicity
	// (or list them explicitly if you prefer more control)
	ID                   int32                                `json:"ID"` // Use uppercase ID as in desired JSON
	CreatedAt            pgtype.Timestamptz                   `json:"CreatedAt"`
	Name                 pgtype.Text                          `json:"Name"`
	LastName             pgtype.Text                          `json:"LastName"`
	Email                string                               `json:"Email"`
	DateOfBirth          pgtype.Date                          `json:"DateOfBirth"`
	Latitude             pgtype.Float8                        `json:"Latitude"`
	Longitude            pgtype.Float8                        `json:"Longitude"`
	Gender               migrations.NullGenderEnum            `json:"Gender"`
	DatingIntention      migrations.NullDatingIntention       `json:"DatingIntention"`
	Height               pgtype.Float8                        `json:"Height"` // Keep raw height if needed, or format here
	Hometown             pgtype.Text                          `json:"Hometown"`
	JobTitle             pgtype.Text                          `json:"JobTitle"`
	Education            pgtype.Text                          `json:"Education"`
	ReligiousBeliefs     migrations.NullReligion              `json:"ReligiousBeliefs"`
	DrinkingHabit        migrations.NullDrinkingSmokingHabits `json:"DrinkingHabit"`
	SmokingHabit         migrations.NullDrinkingSmokingHabits `json:"SmokingHabit"`
	MediaUrls            []string                             `json:"MediaUrls"`
	VerificationStatus   migrations.VerificationStatus        `json:"VerificationStatus"`
	VerificationPic      pgtype.Text                          `json:"VerificationPic"`
	Role                 migrations.UserRole                  `json:"Role"`
	AudioPromptQuestion  migrations.NullAudioPrompt           `json:"AudioPromptQuestion"`
	AudioPromptAnswer    pgtype.Text                          `json:"AudioPromptAnswer"`
	SpotlightActiveUntil pgtype.Timestamptz                   `json:"SpotlightActiveUntil"`

	// --- Added fields from GetHomeFeedRow (beyond User) ---
	Prompts    json.RawMessage `json:"prompts"`     // Use json.RawMessage for JSONB data
	DistanceKm float64         `json:"distance_km"` // Include distance
}

// Updated HomeFeedResponse to use the new DTO slice
type HomeFeedResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message,omitempty"`
	Profiles []ProfileFeedItem `json:"profiles,omitempty"` // Changed type here
	HasMore  bool              `json:"has_more"`
}

// GetHomeFeedHandler handles GET requests to retrieve the user's home feed.
func GetHomeFeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, HomeFeedResponse{
			Success: false, Message: "Method Not Allowed: Use GET",
		})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, HomeFeedResponse{
			Success: false, Message: "Authentication required",
		})
		return
	}
	requestingUserID := int32(claims.UserID)

	// --- Get Requesting User's necessary info (Location, Gender, DOB) ---
	requestingUser, err := queries.GetUserByID(ctx, requestingUserID)
	if err != nil {
		// ... (error handling as before) ...
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("GetHomeFeedHandler: Requesting user %d not found", requestingUserID)
			utils.RespondWithJSON(w, http.StatusNotFound, HomeFeedResponse{
				Success: false, Message: "Requesting user not found.",
			})
			return
		}
		log.Printf("GetHomeFeedHandler: Error fetching requesting user %d: %v", requestingUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
			Success: false, Message: "Error retrieving user data.",
		})
		return
	}

	// Check if user has location set (required for distance calc)
	if !requestingUser.Latitude.Valid || !requestingUser.Longitude.Valid {
		// ... (error handling as before) ...
		log.Printf("GetHomeFeedHandler: Requesting user %d missing location data.", requestingUserID)
		utils.RespondWithJSON(w, http.StatusBadRequest, HomeFeedResponse{
			Success: false, Message: "Please set your location in your profile to use the feed.",
		})
		return
	}

	// --- Check if user has filters set, create defaults if not ---
	_, err = queries.GetUserFilters(ctx, requestingUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Filters don't exist, create defaults
			log.Printf("GetHomeFeedHandler: No filters found for user %d. Creating defaults.", requestingUserID)

			// Calculate Default Filters (Logic remains the same)
			defaultWhoSee := migrations.NullGenderEnum{Valid: false}
			if requestingUser.Gender.Valid {
				if requestingUser.Gender.GenderEnum == migrations.GenderEnumMan {
					defaultWhoSee = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumWoman, Valid: true}
				} else if requestingUser.Gender.GenderEnum == migrations.GenderEnumWoman {
					defaultWhoSee = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumMan, Valid: true}
				}
			}
			defaultAgeMin := pgtype.Int4{Valid: false}
			defaultAgeMax := pgtype.Int4{Valid: false}
			if requestingUser.DateOfBirth.Valid && !requestingUser.DateOfBirth.Time.IsZero() {
				age := int(time.Since(requestingUser.DateOfBirth.Time).Hours() / 24 / 365.25)
				if age >= minFilterAge {
					calcAgeMin := age - defaultAgeRange
					if calcAgeMin < minFilterAge {
						calcAgeMin = minFilterAge
					}
					defaultAgeMin = pgtype.Int4{Int32: int32(calcAgeMin), Valid: true}

					calcAgeMax := age + defaultAgeRange
					if calcAgeMax < calcAgeMin {
						calcAgeMax = calcAgeMin
					}
					if calcAgeMax < minFilterAge {
						calcAgeMax = minFilterAge
					}
					defaultAgeMax = pgtype.Int4{Int32: int32(calcAgeMax), Valid: true}
				}
			}
			defaultParams := migrations.UpsertUserFiltersParams{
				UserID:          requestingUserID,
				WhoYouWantToSee: defaultWhoSee,
				AgeMin:          defaultAgeMin,
				AgeMax:          defaultAgeMax,
				ActiveToday:     false,
				RadiusKm:        pgtype.Int4{Valid: false}, // Let DB handle default or keep null
			}

			// Insert the defaults
			_, upsertErr := queries.UpsertUserFilters(ctx, defaultParams)
			if upsertErr != nil {
				// ... (error handling as before) ...
				log.Printf("GetHomeFeedHandler: Failed to insert default filters for user %d: %v", requestingUserID, upsertErr)
				utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
					Success: false, Message: "Failed to initialize user filters.",
				})
				return
			}
			log.Printf("GetHomeFeedHandler: Default filters created successfully for user %d.", requestingUserID)

		} else {
			// ... (error handling as before) ...
			log.Printf("GetHomeFeedHandler: Error fetching filters for user %d: %v", requestingUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
				Success: false, Message: "Error retrieving user filters.",
			})
			return
		}
	}
	// Filters now exist (either pre-existing or default)

	// --- Execute Feed Query ---
	log.Printf("GetHomeFeedHandler: Fetching feed batch (limit %d) for user %d", feedBatchSize, requestingUserID)
	// The GetHomeFeedParams struct generated by sqlc should now only have ID and Limit
	feedParams := migrations.GetHomeFeedParams{
		ID:    requestingUserID,
		Limit: int32(feedBatchSize),
	}

	// Use the updated Row struct from the generated code
	dbProfiles, err := queries.GetHomeFeed(ctx, feedParams) // dbProfiles is []migrations.GetHomeFeedRow
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// ... (error handling as before) ...
		log.Printf("GetHomeFeedHandler: Database error fetching feed for user %d: %v", requestingUserID, err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			log.Printf("DB Error Details: Code=%s, Message=%s", pgErr.Code, pgErr.Message)
			if pgErr.Code == "42883" { // Example check
				utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
					Success: false, Message: "Server configuration error (e.g., missing distance function).",
				})
				return
			}
		}
		utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
			Success: false, Message: "Error retrieving home feed.",
		})
		return
	}

	// --- Map DB results to the Response DTO ---
	responseProfiles := make([]ProfileFeedItem, 0, len(dbProfiles))
	if dbProfiles != nil {
		for _, dbProfile := range dbProfiles {
			responseItem := ProfileFeedItem{
				// Map fields explicitly - this ensures correct structure
				ID:                   dbProfile.ID,
				CreatedAt:            dbProfile.CreatedAt,
				Name:                 dbProfile.Name,
				LastName:             dbProfile.LastName,
				Email:                dbProfile.Email,
				DateOfBirth:          dbProfile.DateOfBirth,
				Latitude:             dbProfile.Latitude,
				Longitude:            dbProfile.Longitude,
				Gender:               dbProfile.Gender,
				DatingIntention:      dbProfile.DatingIntention,
				Height:               dbProfile.Height, // Keep raw, formatting done on frontend if needed
				Hometown:             dbProfile.Hometown,
				JobTitle:             dbProfile.JobTitle,
				Education:            dbProfile.Education,
				ReligiousBeliefs:     dbProfile.ReligiousBeliefs,
				DrinkingHabit:        dbProfile.DrinkingHabit,
				SmokingHabit:         dbProfile.SmokingHabit,
				MediaUrls:            dbProfile.MediaUrls,
				VerificationStatus:   dbProfile.VerificationStatus,
				VerificationPic:      dbProfile.VerificationPic,
				Role:                 dbProfile.Role,
				AudioPromptQuestion:  dbProfile.AudioPromptQuestion,
				AudioPromptAnswer:    dbProfile.AudioPromptAnswer,
				SpotlightActiveUntil: dbProfile.SpotlightActiveUntil,
				Prompts:              dbProfile.Prompts, // Assign the []byte directly to json.RawMessage
				DistanceKm:           dbProfile.DistanceKm,
			}
			responseProfiles = append(responseProfiles, responseItem)
		}
	}

	log.Printf("GetHomeFeedHandler: Found %d profiles for user %d", len(responseProfiles), requestingUserID)

	// Determine if more pages exist
	hasMore := len(responseProfiles) == feedBatchSize

	// --- Respond ---
	utils.RespondWithJSON(w, http.StatusOK, HomeFeedResponse{
		Success:  true,
		Profiles: responseProfiles, // Use the mapped slice
		HasMore:  hasMore,
	})
}
