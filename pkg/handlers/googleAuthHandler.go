// FILE: pkg/handlers/googleAuthHandler.go
// (MODIFIED TO GRANT DEFAULT ROSE ON NEW USER CREATION)
package handlers

import (
	"context" // Import context package
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type GoogleAuthRequest struct {
	AccessToken string `json:"accessToken"`
}

// See: https://developers.google.com/identity/protocols/oauth2/reference#tokeninfo-response
type googleTokenInfoResponse struct {
	Audience      string `json:"aud"`
	UserID        string `json:"sub"`
	Scope         string `json:"scope"`
	ExpiresIn     string `json:"expires_in"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	AccessType    string `json:"access_type"`
	Error         string `json:"error"`
	ErrorDesc     string `json:"error_description"`
	IssuedTo      string `json:"issued_to"`
	Exp           string `json:"exp"`
}

type GoogleAuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

const googleTokenInfoURL = "https://www.googleapis.com/oauth2/v3/tokeninfo"

var expectedClientID string

func init() {
	expectedClientID = os.Getenv("GOOGLE_CLIENT_ID_ANDROID")
	if expectedClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID_ANDROID environment variable not set!")
	} else {
		log.Printf("Google Auth: Expected Client ID (Audience): %s", expectedClientID)
	}
}

func GoogleAuthHandler(w http.ResponseWriter, r *http.Request) {
	if expectedClientID == "" {
		log.Println("ERROR: GoogleAuthHandler: GOOGLE_CLIENT_ID_ANDROID not configured on server.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Server configuration error [Google Client ID]",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	if queries == nil {
		log.Println("ERROR: GoogleAuthHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, GoogleAuthResponse{
			Success: false, Message: "Method Not Allowed: Use POST",
		})
		return
	}

	var req GoogleAuthRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, GoogleAuthResponse{
			Success: false, Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}
	defer r.Body.Close()

	if req.AccessToken == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, GoogleAuthResponse{
			Success: false, Message: "Missing accessToken in request body",
		})
		return
	}

	googleVerifyURL := fmt.Sprintf("%s?access_token=%s", googleTokenInfoURL, url.QueryEscape(req.AccessToken))
	googleResp, err := http.Get(googleVerifyURL)
	if err != nil {
		log.Printf("Error calling Google tokeninfo endpoint: %v", err)
		utils.RespondWithJSON(w, http.StatusServiceUnavailable, GoogleAuthResponse{
			Success: false, Message: "Failed to contact Google verification service",
		})
		return
	}
	defer googleResp.Body.Close()

	bodyBytes, err := io.ReadAll(googleResp.Body)
	if err != nil {
		log.Printf("Error reading Google tokeninfo response body: %v", err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to read Google's response",
		})
		return
	}

	var tokenInfo googleTokenInfoResponse
	err = json.Unmarshal(bodyBytes, &tokenInfo)
	if err != nil {
		log.Printf("Error unmarshalling Google tokeninfo response: %v. Body: %s", err, string(bodyBytes))
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to parse Google's response",
		})
		return
	}

	if googleResp.StatusCode != http.StatusOK || tokenInfo.Error != "" {
		errorMsg := tokenInfo.ErrorDesc
		if errorMsg == "" {
			errorMsg = tokenInfo.Error
		}
		if errorMsg == "" {
			errorMsg = fmt.Sprintf("Google verification failed with status %d", googleResp.StatusCode)
		}
		log.Printf("Google token verification failed: %s (Status: %d, Body: %s)", errorMsg, googleResp.StatusCode, string(bodyBytes))
		utils.RespondWithJSON(w, http.StatusUnauthorized, GoogleAuthResponse{
			Success: false, Message: "Invalid or expired Google token",
		})
		return
	}

	// --- 4. CRITICAL: Validate the Audience (Client ID) ---
	if tokenInfo.Audience != expectedClientID {
		log.Printf("Token audience mismatch. Expected: %s, Got: %s", expectedClientID, tokenInfo.Audience)
		utils.RespondWithJSON(w, http.StatusUnauthorized, GoogleAuthResponse{
			Success: false, Message: "Token is not intended for this application",
		})
		return
	}

	if tokenInfo.Email == "" {
		log.Printf("Google token verified, but email is missing. UserID: %s", tokenInfo.UserID)
		utils.RespondWithJSON(w, http.StatusBadRequest, GoogleAuthResponse{
			Success: false, Message: "Email scope missing or email not available from Google",
		})
		return
	}
	if tokenInfo.EmailVerified != "true" {
		log.Printf("Google token verified, but email '%s' is not verified by Google. UserID: %s", tokenInfo.Email, tokenInfo.UserID)
		utils.RespondWithJSON(w, http.StatusForbidden, GoogleAuthResponse{
			Success: false, Message: "Google account email must be verified",
		})
		return
	}

	expiresInSeconds := int64(0)
	if tokenInfo.ExpiresIn != "" {
		expiresInVal, convErr := strconv.ParseInt(tokenInfo.ExpiresIn, 10, 64)
		if convErr == nil && expiresInVal > 0 {
			expiresInSeconds = expiresInVal
		} else {
			log.Printf("Warning: Could not parse expires_in value '%s' from Google response.", tokenInfo.ExpiresIn)
		}
	}
	if expiresInSeconds <= 0 {
		log.Printf("Token may be expired or has very short life. expires_in=%s", tokenInfo.ExpiresIn)
	}

	var appUser migrations.User
	appUser, err = queries.GetUserByEmail(ctx, tokenInfo.Email)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			log.Printf("User with email '%s' not found. Creating new user.", tokenInfo.Email)
			// --- Create the User ---
			newUser, createErr := queries.CreateUserWithEmail(ctx, tokenInfo.Email)
			if createErr != nil {
				log.Printf("CRITICAL: Failed to create user for email '%s': %v", tokenInfo.Email, createErr)
				var pgErr *pgconn.PgError
				if errors.As(createErr, &pgErr) && pgErr.Code == "23505" { // Check for unique constraint violation
					utils.RespondWithJSON(w, http.StatusConflict, GoogleAuthResponse{Success: false, Message: "Email already exists (concurrent registration?)"})
				} else {
					utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{Success: false, Message: "Failed to create user account"})
				}
				return
			}
			log.Printf("New user created successfully: ID=%d, Email=%s", newUser.ID, newUser.Email)
			appUser = newUser

			// --- Grant Default Rose ---
			// ADDED: Grant 1 Rose to the newly created user
			log.Printf("Granting 1 default Rose to new user ID: %d", appUser.ID)
			roseParams := migrations.UpsertUserConsumableParams{
				UserID:         appUser.ID,
				ConsumableType: migrations.PremiumFeatureTypeRose,
				Quantity:       1, // Grant exactly 1 rose
			}
			_, roseErr := queries.UpsertUserConsumable(context.Background(), roseParams) // Use Background context or ctx
			if roseErr != nil {
				// Log the error, but don't fail the login process just because the rose grant failed
				log.Printf("WARNING: Failed to grant default rose to user ID %d: %v", appUser.ID, roseErr)
			} else {
				log.Printf("Successfully granted 1 default Rose to user ID %d", appUser.ID)
			}
			// --- End Grant Default Rose ---

		} else {
			log.Printf("Database error looking up user by email '%s': %v", tokenInfo.Email, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{Success: false, Message: "Database error checking user"})
			return
		}
	} else {
		log.Printf("Existing user found: ID=%d, Email=%s", appUser.ID, appUser.Email)
	}

	// --- Generate App Token ---
	appToken, err := token.GenerateToken(appUser.ID)
	if err != nil {
		log.Printf("Failed to generate application token for user ID %d: %v", appUser.ID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to generate session token",
		})
		return
	}

	log.Printf("Google token verified and app token generated successfully for user ID: %d, Email: %s", appUser.ID, tokenInfo.Email)
	utils.RespondWithJSON(w, http.StatusOK, GoogleAuthResponse{
		Success: true,
		Message: "Authentication successful",
		Token:   appToken,
	})
}
