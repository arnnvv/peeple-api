package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type VerifyOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
	OTPCode     string `json:"otpCode"`
}

type VerifyOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

func VerifyOTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, VerifyOTPResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	var req VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	if req.OTPCode == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, VerifyOTPResponse{
			Success: false,
			Message: "OTP code is required",
		})
		return
	}

	user, err := queries.GetUserByPhone(ctx, req.PhoneNumber)
	if err != nil {
		if err == pgx.ErrNoRows {
			utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyOTPResponse{
				Success: false,
				Message: "User not found or invalid phone number.",
			})
		} else {
			log.Printf("Error finding user by phone %s during OTP verify: %v", req.PhoneNumber, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
				Success: false,
				Message: "Database error verifying user",
			})
		}
		return
	}
	userID := user.ID // user's ID (int32)

	storedOtp, err := queries.GetOTPByUser(ctx, userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyOTPResponse{
				Success: false,
				Message: "Invalid or expired OTP.",
			})
		} else {
			log.Printf("Error fetching OTP for user %d: %v", userID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
				Success: false,
				Message: "Database error fetching OTP",
			})
		}
		return
	}

	if storedOtp.OtpCode != req.OTPCode {
		utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyOTPResponse{
			Success: false,
			Message: "Invalid or expired OTP.",
		})
		return
	}

	if !storedOtp.ExpiresAt.Valid || time.Now().After(storedOtp.ExpiresAt.Time) {
		_ = queries.DeleteOTPByID(ctx, storedOtp.ID)
		utils.RespondWithJSON(w, http.StatusUnauthorized, VerifyOTPResponse{
			Success: false,
			Message: "Invalid or expired OTP.",
		})
		return
	}

	err = queries.DeleteOTPByID(ctx, storedOtp.ID)
	if err != nil {
		log.Printf("Warning: Failed to delete OTP ID %d for user %d after successful verification: %v", storedOtp.ID, userID, err)
	}

	tokenString, err := token.GenerateToken(userID)
	if err != nil {
		log.Printf("Failed to generate token for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, VerifyOTPResponse{
			Success: false,
			Message: "Failed to generate authentication token",
		})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, VerifyOTPResponse{
		Success: true,
		Message: "OTP verified successfully",
		Token:   tokenString,
	})
}
