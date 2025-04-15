package handlers

import (
	"errors"
	"log"
	"net/http"
	// "strconv" // No longer needed for page parsing
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// const defaultPageSize = 15 // Keep this for the LIMIT
const feedBatchSize = 15 // Renamed constant for clarity
const defaultAgeRange = 4

// HomeFeedResponse defines the structure for the home feed response (no pagination).
type HomeFeedResponse struct {
	Success  bool                        `json:"success"`
	Message  string                      `json:"message,omitempty"`
	Profiles []migrations.GetHomeFeedRow `json:"profiles,omitempty"`
	// Page     int                         `json:"page,omitempty"` // REMOVED
	HasMore bool `json:"has_more"` // Indicates if more un-actioned profiles might exist
}

// GetHomeFeedHandler handles GET requests to retrieve the user's home feed.
// Fetches a batch of profiles excluding previously liked/disliked ones. No pagination params.
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
				RadiusKm:        pgtype.Int4{Valid: false},
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
			// Handle other potential errors fetching filters
			log.Printf("GetHomeFeedHandler: Error fetching filters for user %d: %v", requestingUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
				Success: false, Message: "Error retrieving user filters.",
			})
			return
		}
	}
	// Filters now exist (either pre-existing or default)

	// --- REMOVED Pagination Logic ---
	// pageStr := r.URL.Query().Get("page") ...
	// offset := (page - 1) * feedBatchSize

	// --- Execute Feed Query ---
	log.Printf("GetHomeFeedHandler: Fetching feed batch (limit %d) for user %d", feedBatchSize, requestingUserID)
	// The GetHomeFeedParams struct generated by sqlc should now only have ID and Limit
	feedParams := migrations.GetHomeFeedParams{
		ID:    requestingUserID,
		Limit: int32(feedBatchSize), // Use the batch size constant
		// Offset field removed from params struct by sqlc regenerate
	}

	profiles, err := queries.GetHomeFeed(ctx, feedParams)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("GetHomeFeedHandler: Database error fetching feed for user %d: %v", requestingUserID, err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			log.Printf("DB Error Details: Code=%s, Message=%s", pgErr.Code, pgErr.Message)
			if pgErr.Code == "42883" {
				utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
					Success: false, Message: "Server configuration error (missing distance function).",
				})
				return
			}
		}
		utils.RespondWithJSON(w, http.StatusInternalServerError, HomeFeedResponse{
			Success: false, Message: "Error retrieving home feed.",
		})
		return
	}

	if profiles == nil {
		profiles = []migrations.GetHomeFeedRow{}
	}
	log.Printf("GetHomeFeedHandler: Found %d profiles for user %d", len(profiles), requestingUserID)

	// --- Determine if more pages exist (still relevant) ---
	// If we got exactly the batch size, assume there might be more un-actioned profiles available.
	hasMore := len(profiles) == feedBatchSize

	// --- Respond ---
	utils.RespondWithJSON(w, http.StatusOK, HomeFeedResponse{
		Success:  true,
		Profiles: profiles,
		// Page:     page, // REMOVED
		HasMore: hasMore,
	})
}
