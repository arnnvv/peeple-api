package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
)

type VerificationActionRequest struct {
	UserID  uint `json:"user_id"`
	Approve bool `json:"approve"`
}

func UpdateVerificationStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerificationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	var user db.UserModel
	if result := db.DB.Where("id = ?", req.UserID).First(&user); result.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if user.VerificationStatus == nil || *user.VerificationStatus != enums.VerificationStatusPending {
		http.Error(w, "User does not have pending verification", http.StatusBadRequest)
		return
	}

	var newStatus enums.VerificationStatus
	if req.Approve {
		newStatus = enums.VerificationStatusTrue
	} else {
		newStatus = enums.VerificationStatusFalse
	}

	user.VerificationStatus = &newStatus

	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Failed to update verification status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Verification status updated successfully",
		"user_id": user.ID,
		"status":  string(newStatus),
	})
}
