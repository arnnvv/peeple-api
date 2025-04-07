package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/jackc/pgx/v5"
)

type VerificationActionRequest struct {
	UserID  int32 `json:"user_id"`
	Approve bool  `json:"approve"`
}

func UpdateVerificationStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	// claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	// if !ok || claims == nil || claims.Role != string(migrations.UserRoleAdmin) {
	// 	http.Error(w, "Forbidden: Admin privileges required", http.StatusForbidden)
	// 	return
	// }

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerificationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 {
		http.Error(w, "valid user_id is required", http.StatusBadRequest)
		return
	}

	user, err := queries.GetUserByID(ctx, req.UserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, fmt.Sprintf("User not found with ID: %d", req.UserID), http.StatusNotFound)
		} else {
			http.Error(w, "Database error fetching user", http.StatusInternalServerError)
		}
		return
	}

	if user.VerificationStatus != migrations.VerificationStatusPending {
		http.Error(w, "User does not have a pending verification request", http.StatusBadRequest)
		return
	}

	var newStatus migrations.VerificationStatus
	if req.Approve {
		newStatus = migrations.VerificationStatusTrue
	} else {
		newStatus = migrations.VerificationStatusFalse
	}

	updateParams := migrations.UpdateUserVerificationStatusParams{
		ID:                 req.UserID,
		VerificationStatus: newStatus,
	}

	_, err = queries.UpdateUserVerificationStatus(ctx, updateParams)
	if err != nil {
		http.Error(w, "Failed to update verification status in database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Verification status updated successfully",
		"user_id": req.UserID,
		"status":  string(newStatus),
	})
}
