package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"unicode"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/jackc/pgx/v5"
)

type createUserRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	Gender      string `json:"gender"` // Expect "man", "woman", "gay", "lesbian", "bisexual"
}

type CreateUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func isValidGender(gender string) bool {
	switch migrations.GenderEnum(gender) {
	case migrations.GenderEnumMan,
		migrations.GenderEnumWoman,
		migrations.GenderEnumGay,
		migrations.GenderEnumLesbian,
		migrations.GenderEnumBisexual:
		return true
	default:
		return false
	}
}

func CreateNewUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.PhoneNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number is required",
		})
		return
	}

	if len(req.PhoneNumber) != 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number must be exactly 10 digits",
		})
		return
	}

	for _, c := range req.PhoneNumber {
		if !unicode.IsDigit(c) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(CreateUserResponse{
				Success: false,
				Message: "Phone number must contain only digits",
			})
			return
		}
	}

	if req.Gender == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Gender is required",
		})
		return
	}

	if !isValidGender(req.Gender) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid gender provided. Allowed values: %s, %s, %s, %s, %s",
				migrations.GenderEnumMan, migrations.GenderEnumWoman, migrations.GenderEnumGay, migrations.GenderEnumLesbian, migrations.GenderEnumBisexual),
		})
		return
	}

	_, err := queries.GetUserByPhone(ctx, req.PhoneNumber)
	if err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number already exists",
		})
		return
	} else if err != pgx.ErrNoRows {
		// An unexpected database error occurred during lookup
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Database error checking user existence",
		})
		return
	}
	// If err is pgx.ErrNoRows, proceed to create user

	params := migrations.CreateUserMinimalParams{
		PhoneNumber: req.PhoneNumber,
		Gender:      migrations.NullGenderEnum{GenderEnum: migrations.GenderEnum(req.Gender), Valid: true},
	}

	_, createErr := queries.CreateUserMinimal(ctx, params)
	if createErr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Error creating user in database",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateUserResponse{
		Success: true,
		Message: "User created successfully",
	})
}
