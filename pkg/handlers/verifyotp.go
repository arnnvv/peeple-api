package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"gorm.io/gorm"
)

// VerifyOTPRequest represents the request to verify an OTP
type VerifyOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTPCode     string `json:"otpCode"`
}

// VerifyOTPResponse represents the response from verifying an OTP
type VerifyOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// VerifyOTP handles the request to verify an OTP
func VerifyOTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, VerifyOTPResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	// Parse request body
	var req VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Validate OTP code
	if req.OTPCode == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: "OTP code is required",
		})
		return
	}

	// Verify OTP
	isValid, err := db.VerifyOTP(req.PhoneNumber, req.OTPCode)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
			Success: false,
			Message: "Failed to verify OTP",
		})
		return
	}

	if !isValid {
		utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyOTPResponse{
			Success: false,
			Message: "Invalid or expired OTP",
		})
		return
	}

	// Create or get user
	userID, err := createOrGetUser(req.PhoneNumber)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
			Success: false,
			Message: "Failed to create user",
		})
		return
	}

	// Generate JWT token
	tokenString, err := generateToken(userID)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	// Return success response with token
	utils.RespondWithJSON(w, http.StatusOK, VerifyOTPResponse{
		Success: true,
		Message: "OTP verified successfully",
		Token:   tokenString,
	})
}

// createOrGetUser creates a new user if it doesn't exist, or returns the existing user ID
func createOrGetUser(phoneNumber string) (uint, error) {
	var user db.UserModel
	result := db.DB.Where("phone_number = ?", phoneNumber).First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new user
			newUser := db.UserModel{
				PhoneNumber: &phoneNumber,
			}

			if err := db.DB.Create(&newUser).Error; err != nil {
				return 0, err
			}

			return newUser.ID, nil
		}
		return 0, result.Error
	}

	return user.ID, nil
}

// generateToken generates a JWT token for the user
func generateToken(userID uint) (string, error) {
	// Create and sign token with only UserID
	tokenString, err := token.GenerateToken(userID)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
