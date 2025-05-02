package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type SpotlightAnalytic struct {
	ActivatedAt       string `json:"activated_at"` // ISO 8601 format
	ExpiresAt         string `json:"expires_at"`   // ISO 8601 format
	ImpressionsDuring int64  `json:"impressions_during"`
}

type SpotlightAnalyticsResponse struct {
	Success    bool                `json:"success"`
	Spotlights []SpotlightAnalytic `json:"spotlights"`
	Message    string              `json:"message,omitempty"`
}

func GetSpotlightAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
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

	startDate := parseDateToTimestamptz(startDateStr, false) // Use helper from summary handler
	endDate := parseDateToTimestamptz(endDateStr, true)      // Use helper from summary handler

	// Validate date range if both are provided
	if startDate.Valid && endDate.Valid && endDate.Time.Before(startDate.Time) {
		utils.RespondWithError(w, http.StatusBadRequest, "end_date cannot be before start_date")
		return
	}

	log.Printf("GetSpotlightAnalytics: UserID=%d, StartDate='%s' (Valid: %t), EndDate='%s' (Valid: %t)",
		userID, startDateStr, startDate.Valid, endDateStr, endDate.Valid)

	// 1. Fetch potential spotlight periods for the user relevant to the date range
	spotlightPeriods, err := queries.GetUserSpotlightActivationTimes(ctx, migrations.GetUserSpotlightActivationTimesParams{
		UserID:    userID,
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		log.Printf("Error fetching spotlight activation times for user %d: %v", userID, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve spotlight data")
		return
	}

	responseSpotlights := make([]SpotlightAnalytic, 0, len(spotlightPeriods))

	// 2. For each period, fetch the impressions specifically during that activation
	for _, period := range spotlightPeriods {
		if !period.ExpiresAt.Valid || !period.PotentiallyActivatedAt.Valid {
			log.Printf("WARN: Skipping spotlight period for user %d due to invalid activation/expiry.", userID)
			continue // Skip if essential timestamps are missing
		}

		// IMPORTANT: Query impressions using the *actual* activation/expiry times of this specific period,
		// NOT the potentially wider query range parameters (startDate, endDate).
		impressionCount, errImp := queries.CountImpressionsDuringSpotlight(ctx, migrations.CountImpressionsDuringSpotlightParams{
			ShownUserID: userID,
			StartDate:   period.PotentiallyActivatedAt, // Use the period's start
			EndDate:     period.ExpiresAt,              // Use the period's end
		})

		if errImp != nil {
			log.Printf("Error counting spotlight impressions for user %d during %v to %v: %v",
				userID, period.PotentiallyActivatedAt.Time, period.ExpiresAt.Time, errImp)
			// Optionally skip this period or return 0 impressions
			impressionCount = 0 // Default to 0 if count fails for this specific period
		}

		analytic := SpotlightAnalytic{
			ActivatedAt:       period.PotentiallyActivatedAt.Time.UTC().Format(time.RFC3339),
			ExpiresAt:         period.ExpiresAt.Time.UTC().Format(time.RFC3339),
			ImpressionsDuring: impressionCount,
		}
		responseSpotlights = append(responseSpotlights, analytic)
	}

	utils.RespondWithJSON(w, http.StatusOK, SpotlightAnalyticsResponse{
		Success:    true,
		Spotlights: responseSpotlights,
	})
}
