package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
)

type SetAdminRequest struct {
	PhoneNumber string `json:"phone_number"`
	IsAdmin     bool   `json:"is_admin"`
}

func SetAdminHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SetAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PhoneNumber == "" {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	var user db.UserModel
	if result := db.DB.Where("phone_number = ?", req.PhoneNumber).First(&user); result.Error != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var role enums.UserRole
	if req.IsAdmin {
		role = enums.UserRoleAdmin
	} else {
		role = enums.UserRoleUser
	}

	user.Role = role.Ptr()

	if err := db.DB.Save(&user).Error; err != nil {
		http.Error(w, "Failed to update user role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "User role updated successfully",
		"user_id": user.ID,
		"role":    role,
	})
}
