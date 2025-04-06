package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/debug"
	"time"
	"unicode"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type SendOTPRequest struct {
	PhoneNumber string `json:"phoneNumber"`
}

type SendOTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func SendOTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	log.Printf("[DEBUG] Handler started: method=%s, path=%s", r.Method, r.URL.Path)
	defer func() {
		log.Printf("[DEBUG] Handler completed in %v", time.Since(startTime))
	}()

	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()

	log.Printf("[DEBUG] Method check: received method=%s", r.Method)
	if r.Method != http.MethodPost {
		log.Printf("[WARN] Invalid method attempted: method=%s", r.Method)
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, SendOTPResponse{
			Success: false,
			Message: "Only POST method allowed",
		})
		return
	}

	var req SendOTPRequest
	log.Printf("[DEBUG] Decoding request body for phone number")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] JSON decode failed: error=%v, stack=%s", err, debug.Stack())
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}
	log.Printf("[DEBUG] Request decoded successfully: phone=%s", req.PhoneNumber)

	log.Printf("[DEBUG] Validating phone number: %s", req.PhoneNumber)
	if err := validatePhoneNumber(req.PhoneNumber); err != nil {
		log.Printf("[VALIDATION ERROR] Phone validation failed: phone=%s, error=%v", req.PhoneNumber, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, SendOTPResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}
	log.Printf("[DEBUG] Phone number validation passed")

	log.Printf("[DEBUG] Starting user lookup: phone=%s", req.PhoneNumber)
	user, err := queries.GetUserByPhone(ctx, req.PhoneNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[INFO] User not found, creating new user: phone=%s", req.PhoneNumber)
			newUser, err := queries.AddPhoneNumberInUsers(ctx, req.PhoneNumber)

			if err != nil {
				log.Printf("[CRITICAL] User creation failed: phone=%s, error=%v, stack=%s",
					req.PhoneNumber, err, debug.Stack())

				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) {
					log.Printf("[DB ERROR] PostgreSQL error: code=%s, message=%s, detail=%s",
						pgErr.Code, pgErr.Message, pgErr.Detail)
				}

				utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
					Success: false,
					Message: "Failed to create user account",
				})
				return
			}

			log.Printf("[INFO] New user created: user_id=%d, phone=%s", newUser.ID, newUser.PhoneNumber)
			user = newUser
		} else {
			log.Printf("[DATABASE ERROR] User lookup failed: phone=%s, error_type=%T, error=%v, stack=%s",
				req.PhoneNumber, err, err, debug.Stack())

			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				log.Printf("[DB DETAIL] PostgreSQL error: code=%s, message=%s, query=%s, hint=%s",
					pgErr.Code, pgErr.Message, pgErr.Where, pgErr.Hint)
			}

			utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
				Success: false,
				Message: "Database error checking user",
			})
			return
		}
	} else {
		log.Printf("[DEBUG] Existing user found: user_id=%d", user.ID)
	}

	log.Printf("[DEBUG] Deleting existing OTPs for user: user_id=%d", user.ID)
	if err := queries.DeleteOTPByUser(ctx, user.ID); err != nil {
		log.Printf("[WARN] OTP cleanup failed: user_id=%d, error=%v", user.ID, err)
	}

	otpCode := generateOTP()
	log.Printf("[DEBUG] Generated OTP: code=%s (THIS SHOULD BE REMOVED IN PRODUCTION)", otpCode)

	log.Printf("[DEBUG] Storing new OTP: user_id=%d", user.ID)
	otpParams := migrations.CreateOTPParams{
		UserID:  user.ID,
		OtpCode: otpCode,
	}

	_, err = queries.CreateOTP(ctx, otpParams)
	if err != nil {
		log.Printf("[CRITICAL] OTP creation failed: user_id=%d, error=%v, stack=%s",
			user.ID, err, debug.Stack())

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			log.Printf("[DB ERROR] PostgreSQL error during OTP creation: code=%s, detail=%s",
				pgErr.Code, pgErr.Detail)
		}

		utils.RespondWithJSON(w, http.StatusInternalServerError, SendOTPResponse{
			Success: false,
			Message: "Failed to store OTP",
		})
		return
	}
	log.Printf("[INFO] OTP stored successfully: user_id=%d", user.ID)

	log.Printf("[DEBUG] Initiating SMS delivery: phone=%s", req.PhoneNumber)
	sendOTPViaSMS(req.PhoneNumber, otpCode)
	log.Printf("[INFO] SMS delivery simulated: phone=%s", req.PhoneNumber)

	log.Printf("[SUCCESS] OTP process completed: phone=%s, user_id=%d", req.PhoneNumber, user.ID)
	utils.RespondWithJSON(w, http.StatusOK, SendOTPResponse{
		Success: true,
		Message: "OTP sent successfully",
	})
}

func validatePhoneNumber(phoneNumber string) error {
	log.Printf("[DEBUG] Validating phone number structure: %s", phoneNumber)

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
