package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
	"unicode"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type SendOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

type SendOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func SendOTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, SendOTPResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	var req SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	otpCode := generateOTP()

	if err := db.CreateOTP(req.PhoneNumber, otpCode, 3*time.Minute); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
			Success: false,
			Message: "Failed to create OTP",
		})
		return
	}

	sendOTPViaSMS(req.PhoneNumber, otpCode)

	utils.RespondWithJSON(w, http.StatusOK, SendOTPResponse{
		Success: true,
		Message: "OTP sent successfully",
	})
}

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

func generateOTP() string {
	rand.Seed(time.Now().UnixNano())

	otp := rand.Intn(900000) + 100000

	return fmt.Sprintf("%06d", otp)
}

func sendOTPViaSMS(phoneNumber, otpCode string) {
	fmt.Printf("Sending OTP %s to %s\n", otpCode, phoneNumber)
}
