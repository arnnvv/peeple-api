package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type SetAdminRequest struct {
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
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
		utils.RespondWithError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
		return
	}

	var req SetAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("SetAdminHandler: Error decoding request body: %v", err)
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body format")
		return
	}
	defer r.Body.Close()

	if req.Email == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Email address is required")
		return
	}

	queries, _ := db.GetDB()
	user, err := queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithError(w, http.StatusNotFound, "User with the provided email not found")
		} else {
			log.Printf("SetAdminHandler: Error fetching user by email %s: %v\n", req.Email, err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Database error retrieving user")
		}
		return
	}

	var targetRole migrations.UserRole
	if req.IsAdmin {
		targetRole = migrations.UserRoleAdmin
	} else {
		targetRole = migrations.UserRoleUser
	}

	if user.Role == targetRole {
		utils.RespondWithJSON(w, http.StatusOK, SetAdminResponse{
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
		log.Printf("SetAdminHandler: Error updating role for user %d (%s): %v\n", user.ID, req.Email, err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update user role in database")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, SetAdminResponse{
		Success: true,
		Message: "User role updated successfully",
		UserID:  updatedUser.ID,
		Role:    updatedUser.Role,
	})
}
