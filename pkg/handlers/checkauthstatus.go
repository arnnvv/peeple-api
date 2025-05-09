package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type AuthStatusResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"` // "login", "onboarding1", "onboarding2", "home"
	Message string `json:"message,omitempty"`
}

func CheckAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		utils.RespondWithJSON(w, http.StatusUnauthorized, AuthStatusResponse{
			Success: false,
			Status:  "login",
			Message: "Invalid or missing token",
		})
		return
	}

	userID := int32(claims.UserID)
	queries, _ := db.GetDB()
	user, err := queries.GetUserByID(r.Context(), userID)

	if err != nil {
		log.Printf("[%s] Error fetching user ID %d: %v", "handlers.CheckAuthStatus", userID, err)
		status := http.StatusInternalServerError
		resp := AuthStatusResponse{
			Success: false,
			Status:  "login", // Default to login on error
			Message: "Error checking user status",
		}
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusUnauthorized // Treat non-existent user same as invalid token
			resp.Message = "User account not found"
		}
		respondAuthStatus(w, status, resp) // Use helper
		return
	}

	if !user.Gender.Valid || (user.Gender.GenderEnum != migrations.GenderEnumMan && user.Gender.GenderEnum != migrations.GenderEnumWoman) {
		respondAuthStatus(w, http.StatusOK, AuthStatusResponse{
			Success: true,
			Status:  "onboarding1", // Gender/Location step
			Message: "User requires gender and location setup",
		})
		return
	}

	if !user.Name.Valid || user.Name.String == "" {
		respondAuthStatus(w, http.StatusOK, AuthStatusResponse{
			Success: true,
			Status:  "onboarding2", // Main profile details step
			Message: "User requires profile details completion",
		})
		return
	}

	respondAuthStatus(w, http.StatusOK, AuthStatusResponse{
		Success: true,
		Status:  "home",
		Message: "User authenticated",
	})
}

func respondAuthStatus(w http.ResponseWriter, code int, payload AuthStatusResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Error encoding auth status response: %v", err)
	}
}
