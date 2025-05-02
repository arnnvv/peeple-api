package handlers

import (
	"context" // <-- ADDED: Import context
	"encoding/json"
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

// --- DTO for profile items in the response (No Changes) ---
type ProfileFeedItem struct {
	ID                   int32                                `json:"ID"`
	CreatedAt            pgtype.Timestamptz                   `json:"CreatedAt"`
	Name                 pgtype.Text                          `json:"Name"`
	LastName             pgtype.Text                          `json:"LastName"`
	Email                string                               `json:"Email"`
	DateOfBirth          pgtype.Date                          `json:"DateOfBirth"`
	Latitude             pgtype.Float8                        `json:"Latitude"`
	Longitude            pgtype.Float8                        `json:"Longitude"`
	Gender               migrations.NullGenderEnum            `json:"Gender"`
	DatingIntention      migrations.NullDatingIntention       `json:"DatingIntention"`
	Height               pgtype.Float8                        `json:"Height"`
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
	Prompts              json.RawMessage                      `json:"prompts"`
	DistanceKm           float64                              `json:"distance_km"`
}

// --- HomeFeedResponse (No Changes) ---
type HomeFeedResponse struct {
	Success  bool              `json:"success"`
	Message  string            `json:"message,omitempty"`
	Profiles []ProfileFeedItem `json:"profiles,omitempty"`
	HasMore  bool              `json:"has_more"`
}

// GetHomeFeedHandler handles GET requests to retrieve the user's home feed.
func GetHomeFeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
	if queries == nil { // <-- ADDED: Check if queries is nil
		log.Println("ERROR: GetHomeFeedHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
			Success: false, Message: "Database connection error.",
		})
		return
	}

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

	// --- Get Requesting User's necessary info ---
	requestingUser, err := queries.GetUserByID(ctx, requestingUserID)
	if err != nil {
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

	// --- Location Check ---
	if !requestingUser.Latitude.Valid || !requestingUser.Longitude.Valid {
		log.Printf("GetHomeFeedHandler: Requesting user %d missing location data.", requestingUserID)
		utils.RespondWithJSON(w, http.StatusBadRequest, HomeFeedResponse{
			Success: false, Message: "Please set your location in your profile to use the feed.",
		})
		return
	}

	// --- Default Filters Logic (No Changes Needed Here) ---
	_, err = queries.GetUserFilters(ctx, requestingUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("GetHomeFeedHandler: No filters found for user %d. Creating defaults.", requestingUserID)
			// (Code to calculate and upsert default filters - remains the same)
			// ... [existing default filter creation logic] ...
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
			// *** IMPORTANT: Ensure defaultAgeMin/Max are set if DOB is invalid ***
			// Use fixed defaults if calculated ones aren't valid
			if !defaultAgeMin.Valid {
				defaultAgeMin = pgtype.Int4{Int32: fixedDefaultFilterMinAge, Valid: true}
			}
			if !defaultAgeMax.Valid {
				defaultAgeMax = pgtype.Int4{Int32: fixedDefaultFilterMaxAge, Valid: true}
			}
			// *** END FIX ***

			defaultParams := migrations.UpsertUserFiltersParams{
				UserID:          requestingUserID,
				WhoYouWantToSee: defaultWhoSee,
				AgeMin:          defaultAgeMin,
				AgeMax:          defaultAgeMax,
				ActiveToday:     false,
				RadiusKm:        pgtype.Int4{Int32: defaultFilterRadiusKm, Valid: true}, // Use constant
			}

			// Insert the defaults
			_, upsertErr := queries.UpsertUserFilters(ctx, defaultParams)
			if upsertErr != nil {
				log.Printf("GetHomeFeedHandler: Failed to insert default filters for user %d: %v", requestingUserID, upsertErr)
				utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
					Success: false, Message: "Failed to initialize user filters.",
				})
				return
			}
			log.Printf("GetHomeFeedHandler: Default filters created successfully for user %d.", requestingUserID)

		} else {
			log.Printf("GetHomeFeedHandler: Error fetching filters for user %d: %v", requestingUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
				Success: false, Message: "Error retrieving user filters.",
			})
			return
		}
	}

	// --- Execute Feed Query ---
	log.Printf("GetHomeFeedHandler: Fetching feed batch (limit %d) for user %d", feedBatchSize, requestingUserID)
	feedParams := migrations.GetHomeFeedParams{
		ID:    requestingUserID,
		Limit: int32(feedBatchSize),
	}
	dbProfiles, err := queries.GetHomeFeed(ctx, feedParams)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("GetHomeFeedHandler: Database error fetching feed for user %d: %v", requestingUserID, err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			log.Printf("DB Error Details: Code=%s, Message=%s", pgErr.Code, pgErr.Message)
			if pgErr.Code == "42883" {
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

	// --- Prepare Response (Mapping DTO) ---
	responseProfiles := make([]ProfileFeedItem, 0, len(dbProfiles))
	if dbProfiles != nil {
		for _, dbProfile := range dbProfiles {
			responseItem := ProfileFeedItem{
				// (Mapping code remains the same)
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
				Height:               dbProfile.Height,
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
				Prompts:              dbProfile.Prompts,
				DistanceKm:           dbProfile.DistanceKm,
			}
			responseProfiles = append(responseProfiles, responseItem)
		}
	}
	log.Printf("GetHomeFeedHandler: Found %d profiles for user %d", len(responseProfiles), requestingUserID)
	hasMore := len(responseProfiles) == feedBatchSize

	// --- Respond ---
	utils.RespondWithJSON(w, http.StatusOK, HomeFeedResponse{
		Success:  true,
		Profiles: responseProfiles,
		HasMore:  hasMore,
	})

	// --- ADDED: Log Impressions Asynchronously ---
	if len(dbProfiles) > 0 {
		go func(viewerID int32, profiles []migrations.GetHomeFeedRow) {
			// Create a background context for the goroutine
			bgCtx := context.Background()
			queriesBG, dbErr := db.GetDB() // Get DB instance again for the goroutine
			if dbErr != nil {
				log.Printf("GetHomeFeedHandler [Goroutine]: Failed to get DB for impression logging: %v", dbErr)
				return
			}

			impressionCount := 0
			for _, profile := range profiles {
				// Determine source based on spotlight status
				source := "homefeed"
				// Check if spotlight is active *at the time of impression*
				if profile.SpotlightActiveUntil.Valid && profile.SpotlightActiveUntil.Time.After(time.Now()) {
					source = "spotlight"
				}

				logParams := migrations.LogUserProfileImpressionParams{
					ViewerUserID: viewerID,
					ShownUserID:  profile.ID,
					Source:       source,
				}
				err := queriesBG.LogUserProfileImpression(bgCtx, logParams)
				if err != nil {
					// Log error but don't stop processing other impressions
					log.Printf("GetHomeFeedHandler [Goroutine]: Failed to log impression for viewer %d -> shown %d: %v", viewerID, profile.ID, err)
				} else {
					impressionCount++
				}
			}
			log.Printf("GetHomeFeedHandler [Goroutine]: Logged %d impressions for viewer %d.", impressionCount, viewerID)
		}(requestingUserID, dbProfiles) // Pass necessary data to the goroutine
	}
	// --- END Added Impression Logging ---
}
