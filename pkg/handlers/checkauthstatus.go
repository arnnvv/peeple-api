package handlers

import (
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
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

	var user db.UserModel
	if err := db.DB.First(&user, claims.UserID).Error; err != nil {
		utils.RespondWithJSON(w, http.StatusUnauthorized, AuthStatusResponse{
			Success: false,
			Status:  "login",
			Message: "User not found",
		})
		return
	}

	if user.Name == nil || *user.Name == "" {
		utils.RespondWithJSON(w, http.StatusOK, AuthStatusResponse{
			Success: true,
			Status:  "onboarding",
			Message: "User profile not completed",
		})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, AuthStatusResponse{
		Success: true,
		Status:  "home",
		Message: "User authenticated with complete profile",
	})
}
