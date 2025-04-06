package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
)

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type SetAdminRequest struct {
	PhoneNumber string `json:"phone_number"`
	IsAdmin     bool   `json:"is_admin"`
}

type SetAdminResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	UserID  int32               `json:"user_id"`
	Role    migrations.UserRole `json:"role"`
}

func SetAdminHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	// --- Decode Body ---
	var req SetAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("SetAdminHandler: Error decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close() // closinbg the body

	if req.PhoneNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Success: false, Message: "Phone number is required"})
		return
	}

	queries := db.GetDB()
	user, err := queries.GetUserByPhone(r.Context(), req.PhoneNumber)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "User with the provided phone number not found",
			})
		} else {
			log.Printf("SetAdminHandler: Error fetching user by phone %s: %v\n", req.PhoneNumber, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Success: false, Message: "Database error retrieving user"})
		}
		return
	}

	var targetRole migrations.UserRole
	if req.IsAdmin {
		targetRole = migrations.UserRoleAdmin
	} else {
		targetRole = migrations.UserRoleUser
	}

	// 3. Update the user's role
	if user.Role == targetRole {
		// Role is already set, maybe return success or a specific message
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SetAdminResponse{
			Success: true,
			Message: "User role is already set to the desired value",
			UserID:  user.ID,
			Role:    user.Role,
		})
		return
	}

	updateParams := migrations.UpdateUserRoleParams{
		Role: targetRole,
		ID:   user.ID,
	}

	updatedUser, err := queries.UpdateUserRole(r.Context(), updateParams)
	if err != nil {
		log.Printf("SetAdminHandler: Error updating role for user %d: %v\n", user.ID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Success: false, Message: "Failed to update user role in database"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SetAdminResponse{
		Success: true,
		Message: "User role updated successfully",
		UserID:  updatedUser.ID,
		Role:    updatedUser.Role,
	})
}
