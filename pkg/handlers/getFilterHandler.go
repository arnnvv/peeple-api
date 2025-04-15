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

type GetFiltersResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message,omitempty"`
	Filters *migrations.Filter `json:"filters,omitempty"`
}

func GetFiltersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
		log.Println("ERROR: GetFiltersHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetFiltersResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, GetFiltersResponse{
			Success: false, Message: "Method Not Allowed: Use GET",
		})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, GetFiltersResponse{
			Success: false, Message: "Authentication required",
		})
		return
	}
	userID := int32(claims.UserID)

	log.Printf("GetFiltersHandler: Fetching filters for user %d", userID)

	filters, err := queries.GetUserFilters(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("GetFiltersHandler: No filters found for user %d. Responding with success=true, filters=null.", userID)
			utils.RespondWithJSON(w, http.StatusOK, GetFiltersResponse{
				Success: true,
				Message: "Filters not set by the user yet.",
				Filters: nil,
			})
			return
		}

		log.Printf("GetFiltersHandler: Error fetching filters for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetFiltersResponse{
			Success: false, Message: "Database error retrieving filters",
		})
		return
	}

	log.Printf("GetFiltersHandler: Filters successfully retrieved for user %d", userID)

	utils.RespondWithJSON(w, http.StatusOK, GetFiltersResponse{
		Success: true,
		Filters: &filters,
	})
}
