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
	Status  string `json:"status"`
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

	var user migrations.User
	var err error

	queries := db.GetDB()
	user, err = queries.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[%s] User not found for ID: %d", "handlers.CheckAuthStatus", userID)
			respondAuthStatus(w, http.StatusUnauthorized, AuthStatusResponse{
				Success: false,
				Status:  "login",
				Message: "User account not found",
			})
		} else {
			log.Printf("[%s] Database error fetching user ID %d: %v", "handlers.CheckAuthStatus", userID, err)
			respondAuthStatus(w, http.StatusInternalServerError, AuthStatusResponse{
				Success: false,
				Status:  "login", // generic error status will be btr
				Message: "Error checking user status",
			})
		}
		return
	}

	if !user.Name.Valid || user.Name.String == "" {
		respondAuthStatus(w, http.StatusOK, AuthStatusResponse{
			Success: true,
			Status:  "onboarding",
			Message: "User profile requires completion",
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
