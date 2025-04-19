package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type QuickFeedResponse struct {
	Success  bool                         `json:"success"`
	Message  string                       `json:"message,omitempty"`
	Profiles []migrations.GetQuickFeedRow `json:"profiles,omitempty"`
}

const quickFeedLimit = 2

func GetQuickFeedHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queries, er := db.GetDB()
	if er != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database connection not available")
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token claims")
		return
	}
	requestingUserID := int32(claims.UserID)

	if r.Method != http.MethodGet {
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use GET")
		return
	}

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

	var oppositeGender migrations.NullGenderEnum
	switch requestingUser.Gender.GenderEnum {
	case migrations.GenderEnumMan:
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumWoman, Valid: true}
	case migrations.GenderEnumWoman:
		oppositeGender = migrations.NullGenderEnum{GenderEnum: migrations.GenderEnumMan, Valid: true}
	default:
		log.Printf("GetQuickFeedHandler: Invalid gender '%v' found in database for user %d.", requestingUser.Gender.GenderEnum, requestingUserID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Invalid gender data found for user")
		return
	}

	lat := requestingUser.Latitude.Float64
	lon := requestingUser.Longitude.Float64
	log.Printf("Fetching quick feed for user %d (gender: %s) using DB location (lat: %f, lon: %f), showing %s",
		requestingUserID, requestingUser.Gender.GenderEnum, lat, lon, oppositeGender.GenderEnum)

	params := migrations.GetQuickFeedParams{
		Lat1:   lat,
		Lon1:   lon,
		ID:     requestingUserID,
		Gender: oppositeGender,
		Limit:  quickFeedLimit,
	}

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
			Profiles: []migrations.GetQuickFeedRow{},
		})
		return
	}

	log.Printf("Found %d profiles for quick feed for user %d", len(profiles), requestingUserID)
	utils.RespondWithJSON(w, http.StatusOK, QuickFeedResponse{
		Success:  true,
		Profiles: profiles,
	})
}
