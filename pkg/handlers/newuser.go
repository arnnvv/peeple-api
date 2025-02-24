package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"unicode"

	"github.com/arnnvv/peeple-api/db"
	"gorm.io/gorm"
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

	// Check for existing phone number using First()
	var existingUser db.UserModel
	result := db.DB.Where("phone_number = ?", req.PhoneNumber).First(&existingUser)
	if result.Error == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Phone number already exists",
		})
		return
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Error checking user existence",
		})
		return
	}

	newUser := db.UserModel{
		PhoneNumber: req.PhoneNumber,
	}

	createResult := db.DB.Create(&newUser)
	if createResult.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CreateUserResponse{
			Success: false,
			Message: "Error creating user",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateUserResponse{
		Success: true,
		Message: "User created successfully",
	})
}
