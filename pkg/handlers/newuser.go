package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"unicode"

	"github.com/arnnvv/peeple-api/pkg/dbpackage"
)

type createUserRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

type CreateUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func CreateNewUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Validate HTTP method
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	// Decode request body
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	if req.PhoneNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number is required",
		})
		return
	}

	// Check length exactly 10 digits
	if len(req.PhoneNumber) != 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number must be exactly 10 digits",
		})
		return
	}

	// Check all characters are digits
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
	q := dbpackage.GetDB()

	_, err := q.CreateUser(context.Background(), req.PhoneNumber)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError) // 500 Internal Server Error
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Failed to create user in database",
		})
		log.Printf("Error creating user: %v", err) // Log the error for debugging
		return
	}

	// Return success response with user ID and phone number
	json.NewEncoder(w).Encode(CreateUserResponse{
		Success: true,
		Message: "User created successfully",
	})
}
