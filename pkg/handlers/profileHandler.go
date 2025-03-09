package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"gorm.io/gorm"
)

type ProfileResponse struct {
	Success bool          `json:"success"`
	User    *db.UserModel `json:"user,omitempty"`
	Error   string        `json:"error,omitempty"`
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := ProfileResponse{Success: true}

	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var user db.UserModel
	err := db.DB.
		Preload("Prompts").
		Preload("AudioPrompt").
		First(&user, claims.UserID).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			resp.Success = false
			resp.Error = "User not found"
			w.WriteHeader(http.StatusNotFound)
		} else {
			resp.Success = false
			resp.Error = "Database error"
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp.User = &user
	json.NewEncoder(w).Encode(resp)
}
