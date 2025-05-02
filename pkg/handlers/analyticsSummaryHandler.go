package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type AnalyticsSummaryResponse struct {
	Success   bool                    `json:"success"`
	Analytics *AnalyticsSummaryFields `json:"analytics,omitempty"`
	Message   string                  `json:"message,omitempty"`
}

type AnalyticsSummaryFields struct {
	ProfileVisibilityCount        int64   `json:"profile_visibility_count"`
	AverageProfileViewTimeSeconds float64 `json:"average_profile_view_time_seconds"`
	DislikesSentCount             int64   `json:"dislikes_sent_count"`
	DislikesReceivedCount         int64   `json:"dislikes_received_count"`
	ProfilesOpenedFromLike        int64   `json:"profiles_opened_from_like"`
}

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

	// Fetch individual metrics
	var visibilityCount, dislikesSent, dislikesReceived, profilesOpened int64
	var avgViewTime float64
	var errVis, errDisSent, errDisRec, errProfOpen, errAvgView error

	// Run queries concurrently for potential performance improvement
	errChan := make(chan error, 5) // Channel to collect errors from goroutines

	go func() {
		visibilityCount, errVis = queries.CountProfileImpressions(ctx, migrations.CountProfileImpressionsParams{
			ShownUserID: userID,
			StartDate:   startDate,
			EndDate:     endDate,
		})
		errChan <- errVis
	}()

	// Inside GetAnalyticsSummaryHandler, in the goroutine for avgViewTime:
	go func() {
		avgViewTime, errAvgView = queries.GetApproximateProfileViewTimeSeconds(ctx, migrations.GetApproximateProfileViewTimeSecondsParams{
			TargetUserID: userID, // <-- CORRECTED Field Name
			StartDate:    startDate,
			EndDate:      endDate,
		})
		errChan <- errAvgView
	}()

	go func() {
		dislikesSent, errDisSent = queries.CountDislikesSent(ctx, migrations.CountDislikesSentParams{
			DislikerUserID: userID,
			StartDate:      startDate,
			EndDate:        endDate,
		})
		errChan <- errDisSent
	}()

	go func() {
		dislikesReceived, errDisRec = queries.CountDislikesReceived(ctx, migrations.CountDislikesReceivedParams{
			DislikedUserID: userID,
			StartDate:      startDate,
			EndDate:        endDate,
		})
		errChan <- errDisRec
	}()

	go func() {
		profilesOpened, errProfOpen = queries.CountProfilesOpenedFromLike(ctx, migrations.CountProfilesOpenedFromLikeParams{
			LikerUserID: userID,
			StartDate:   startDate,
			EndDate:     endDate,
		})
		errChan <- errProfOpen
	}()

	// Wait for all goroutines to finish and check errors
	var firstError error
	for i := 0; i < 5; i++ {
		err := <-errChan
		if err != nil && firstError == nil {
			firstError = err // Capture the first error encountered
		}
	}
	close(errChan)

	if firstError != nil {
		log.Printf("Error fetching analytics summary for user %d: %v", userID, firstError)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve analytics data")
		return
	}

	// Construct the response
	summary := AnalyticsSummaryFields{
		ProfileVisibilityCount:        visibilityCount,
		AverageProfileViewTimeSeconds: avgViewTime,
		DislikesSentCount:             dislikesSent,
		DislikesReceivedCount:         dislikesReceived,
		ProfilesOpenedFromLike:        profilesOpened,
	}

	utils.RespondWithJSON(w, http.StatusOK, AnalyticsSummaryResponse{
		Success:   true,
		Analytics: &summary,
	})
}
