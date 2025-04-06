package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"unicode"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
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
	ctx := r.Context()
	queries := db.GetDB()

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

	user, err := queries.GetUserByPhone(ctx, req.PhoneNumber)
	if err != nil {
		if err == pgx.ErrNoRows {
			utils.RespondWithJSON(w, http.StatusNotFound, SendOTPResponse{
				Success: false,
				Message: "User with this phone number not found",
			})
		} else {
			log.Printf("Error finding user by phone %s: %v", req.PhoneNumber, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
				Success: false,
				Message: "Database error checking user",
			})
		}
		return
	}

	// preventing accumulation if the user requests OTP multiple times.
	queries.DeleteOTPByUser(ctx, user.ID)

	otpCode := generateOTP()

	otpParams := migrations.CreateOTPParams{
		UserID:  user.ID,
		OtpCode: otpCode,
	}

	_, err = queries.CreateOTP(ctx, otpParams)
	if err != nil {
		log.Printf("Error creating OTP for user %d: %v", user.ID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
			Success: false,
			Message: "Failed to store OTP",
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
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func sendOTPViaSMS(phoneNumber, otpCode string) {
	fmt.Printf("Sending OTP %s to %s\n", otpCode, phoneNumber)
}
