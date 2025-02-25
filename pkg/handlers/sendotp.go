package handlers

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
	"unicode"
	"encoding/json"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

// SendOTPRequest represents the request to send an OTP
type SendOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

// SendOTPResponse represents the response from sending an OTP
type SendOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendOTP handles the request to send an OTP
func SendOTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, SendOTPResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	// Parse request body
	var req SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate phone number
	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	// Generate a random 6-digit OTP
	otpCode := generateOTP()

	// Store OTP in database with 3-minute TTL
	if err := db.CreateOTP(req.PhoneNumber, otpCode, 3*time.Minute); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
			Success: false,
			Message: "Failed to create OTP",
		})
		return
	}

	// Send OTP via SMS (placeholder function)
	sendOTPViaSMS(req.PhoneNumber, otpCode)

	// Return success response
	utils.RespondWithJSON(w, http.StatusOK, SendOTPResponse{
		Success: true,
		Message: "OTP sent successfully",
	})
}

// validatePhoneNumber validates the phone number format
func validatePhoneNumber(phoneNumber string) error {
	if phoneNumber == "" {
		return fmt.Errorf("phone number is required")
	}

	if len(phoneNumber) != 10 {
		return fmt.Errorf("phone number must be exactly 10 digits")
	}

	for _, c := range phoneNumber {
		if !unicode.IsDigit(c) {
			return fmt.Errorf("phone number must contain only digits")
		}
	}

	return nil
}

// generateOTP generates a random 6-digit OTP
func generateOTP() string {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
	
	// Generate a random 6-digit number
	otp := rand.Intn(900000) + 100000
	
	return fmt.Sprintf("%06d", otp)
}

// sendOTPViaSMS is a placeholder function for sending OTP via SMS
func sendOTPViaSMS(phoneNumber, otpCode string) {
	// This is a placeholder function
	// Implementation will be added later
	fmt.Printf("Sending OTP %s to %s\n", otpCode, phoneNumber)
}
