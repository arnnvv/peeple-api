// FILE: pkg/handlers/analyticsSummaryHandler.go
package handlers

import (
	"context" // Import context
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---- MODIFIED: AnalyticsSummaryFields ----
type AnalyticsSummaryFields struct {
	ProfileVisibilityCount        int64   `json:"profile_visibility_count"`
	AverageProfileViewTimeSeconds float64 `json:"average_profile_view_time_seconds"`
	DislikesSentCount             int64   `json:"dislikes_sent_count"`
	DislikesReceivedCount         int64   `json:"dislikes_received_count"`
	ProfilesOpenedFromLike        int64   `json:"profiles_opened_from_like"`
	// --- ADDED ---
	PhotoAvgViewTimeMs map[int]float64 `json:"photo_avg_view_time_ms"` // Map[photo_index]average_duration_ms
}

// ---- UNCHANGED: AnalyticsSummaryResponse ----
type AnalyticsSummaryResponse struct {
	Success   bool                    `json:"success"`
	Analytics *AnalyticsSummaryFields `json:"analytics,omitempty"`
	Message   string                  `json:"message,omitempty"`
}

// ---- UNCHANGED: parseDateToTimestamptz ----
// Helper function to parse date string and return Timestamptz for query
// Returns Valid=false if dateStr is empty or invalid format.
// Adjusts end date to be end of the day.
func parseDateToTimestamptz(dateStr string, isEndDate bool) pgtype.Timestamptz {
	if dateStr == "" {
		return pgtype.Timestamptz{Valid: false} // Null timestamp
	}
	layout := "2006-01-02"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return pgtype.Timestamptz{Valid: false} // Invalid format, treat as null
	}
	if isEndDate {
		// Set time to the very end of the specified day
		t = t.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	// For start date, it implicitly starts at 00:00:00 of that day.
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ---- MODIFIED: GetAnalyticsSummaryHandler ----
func GetAnalyticsSummaryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection error")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	userID := int32(claims.UserID)

	// Parse query parameters for date range
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	startDate := parseDateToTimestamptz(startDateStr, false)
	endDate := parseDateToTimestamptz(endDateStr, true)

	// Validate date range if both are provided
	if startDate.Valid && endDate.Valid && endDate.Time.Before(startDate.Time) {
		utils.RespondWithError(w, http.StatusBadRequest, "end_date cannot be before start_date")
		return
	}

	log.Printf("GetAnalyticsSummary: UserID=%d, StartDate='%s' (Valid: %t), EndDate='%s' (Valid: %t)",
		userID, startDateStr, startDate.Valid, endDateStr, endDate.Valid)

	// --- MODIFIED: Declare variables to hold results from ALL goroutines ---
	var visibilityCount, dislikesSent, dislikesReceived, profilesOpened int64
	var avgViewTime float64
	var avgPhotoDurations []migrations.GetPhotoAverageViewDurationsRow // Holds result of new query
	var firstError error                                               // Variable to capture the first error

	// Use WaitGroupWithError for concurrent fetching
	wg := utils.NewWaitGroupWithError(ctx) // Create a group tied to the request context

	// --- MODIFIED: Add goroutines for ALL metrics (now 6) ---

	// 1. Profile Visibility Count
	wg.Add(func(innerCtx context.Context) error {
		var err error
		visibilityCount, err = queries.CountProfileImpressions(innerCtx, migrations.CountProfileImpressionsParams{
			ShownUserID: userID,
			StartDate:   startDate,
			EndDate:     endDate,
		})
		return err
	})

	// 2. Average Profile View Time
	wg.Add(func(innerCtx context.Context) error {
		var err error
		avgViewTime, err = queries.GetApproximateProfileViewTimeSeconds(innerCtx, migrations.GetApproximateProfileViewTimeSecondsParams{
			TargetUserID: userID,
			StartDate:    startDate,
			EndDate:      endDate,
		})
		return err
	})

	// 3. Dislikes Sent
	wg.Add(func(innerCtx context.Context) error {
		var err error
		dislikesSent, err = queries.CountDislikesSent(innerCtx, migrations.CountDislikesSentParams{
			DislikerUserID: userID,
			StartDate:      startDate,
			EndDate:        endDate,
		})
		return err
	})

	// 4. Dislikes Received
	wg.Add(func(innerCtx context.Context) error {
		var err error
		dislikesReceived, err = queries.CountDislikesReceived(innerCtx, migrations.CountDislikesReceivedParams{
			DislikedUserID: userID,
			StartDate:      startDate,
			EndDate:        endDate,
		})
		return err
	})

	// 5. Profiles Opened From Like Screen
	wg.Add(func(innerCtx context.Context) error {
		var err error
		profilesOpened, err = queries.CountProfilesOpenedFromLike(innerCtx, migrations.CountProfilesOpenedFromLikeParams{
			LikerUserID: userID,
			StartDate:   startDate,
			EndDate:     endDate,
		})
		return err
	})

	// 6. --- NEW: Average Photo View Durations ---
	wg.Add(func(innerCtx context.Context) error {
		var err error
		log.Printf("GetAnalyticsSummary [Goroutine]: Fetching avg photo durations for user %d", userID)
		avgPhotoDurations, err = queries.GetPhotoAverageViewDurations(innerCtx, migrations.GetPhotoAverageViewDurationsParams{
			ViewedUserID: userID, // User requesting stats about *their* photos being viewed
			StartDate:    startDate,
			EndDate:      endDate,
		})
		if err != nil {
			log.Printf("GetAnalyticsSummary [Goroutine]: Error fetching avg photo durations for user %d: %v", userID, err)
		} else {
			log.Printf("GetAnalyticsSummary [Goroutine]: Fetched %d photo duration averages for user %d", len(avgPhotoDurations), userID)
		}
		return err // Return error to the WaitGroup
	})

	// --- MODIFIED: Wait for all goroutines and check for the first error ---
	firstError = wg.Wait(ctx) // Wait blocks until all Go routines return or one returns an error

	if firstError != nil {
		// Log the actual first error encountered by any goroutine
		log.Printf("Error fetching analytics summary component for user %d: %v", userID, firstError)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve one or more analytics data components")
		return
	}

	// --- NEW: Process photo durations after waiting and error check ---
	photoAvgViewTimeMs := make(map[int]float64)
	// Check avgPhotoDurations is not nil which could happen if the goroutine panicked
	// although errgroup handles this. It's safe defensive coding.
	if avgPhotoDurations != nil {
		for _, rowData := range avgPhotoDurations {
			// Use int for map key as JSON keys are strings anyway, but Go maps work better with simple types
			photoAvgViewTimeMs[int(rowData.PhotoIndex)] = rowData.AverageDurationMs
		}
	}
	log.Printf("GetAnalyticsSummary: Processed photo durations into map for user %d: %v", userID, photoAvgViewTimeMs)

	// Construct the response
	summary := AnalyticsSummaryFields{
		ProfileVisibilityCount:        visibilityCount,
		AverageProfileViewTimeSeconds: avgViewTime,
		DislikesSentCount:             dislikesSent,
		DislikesReceivedCount:         dislikesReceived,
		ProfilesOpenedFromLike:        profilesOpened,
		// --- ADDED ---
		PhotoAvgViewTimeMs: photoAvgViewTimeMs, // Assign the processed map
	}

	utils.RespondWithJSON(w, http.StatusOK, AnalyticsSummaryResponse{
		Success:   true,
		Analytics: &summary,
	})
}
