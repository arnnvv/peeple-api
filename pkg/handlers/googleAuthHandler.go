// FILE: pkg/handlers/googleAuthHandler.go
// (NEW FILE)
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv" // Changed from fmt to strconv for ExpiresIn conversion

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5" // Import pgx/v5
	"github.com/jackc/pgx/v5/pgconn"
)

// Expected request body structure for Google auth
type GoogleAuthRequest struct {
	AccessToken string `json:"accessToken"`
}

// Structure for the response from Google's tokeninfo endpoint
// See: https://developers.google.com/identity/protocols/oauth2/reference#tokeninfo-response
type googleTokenInfoResponse struct {
	Audience      string `json:"aud"` // IMPORTANT: The Client ID the token was issued to
	UserID        string `json:"sub"` // User's unique Google ID
	Scope         string `json:"scope"`
	ExpiresIn     string `json:"expires_in"` // Remaining lifetime in seconds (string!)
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"` // "true" or "false" (string!)
	AccessType    string `json:"access_type"`
	Error         string `json:"error"`             // Populated on error from Google
	ErrorDesc     string `json:"error_description"` // Populated on error from Google
	IssuedTo      string `json:"issued_to"`         // Often same as Audience for access tokens
	Exp           string `json:"exp"`               // Absolute expiry timestamp (string!)
}

// Structure for our API's successful authentication response
type GoogleAuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"` // Our application's JWT
}

const googleTokenInfoURL = "https://www.googleapis.com/oauth2/v3/tokeninfo"

var expectedClientID string // Loaded from env var

func init() {
	// Load the expected Client ID once at startup
	expectedClientID = os.Getenv("GOOGLE_CLIENT_ID_ANDROID")
	if expectedClientID == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID_ANDROID environment variable not set!")
		// Allow startup but log warning, requests will fail validation later
	} else {
		log.Printf("Google Auth: Expected Client ID (Audience): %s", expectedClientID)
	}
}

// GoogleAuthHandler verifies a Google Access Token and issues an application JWT.
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

	// 1. Decode the incoming request
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

	// 2. Call Google's tokeninfo endpoint
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

	// Read Google's response body
	bodyBytes, err := io.ReadAll(googleResp.Body)
	if err != nil {
		log.Printf("Error reading Google tokeninfo response body: %v", err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to read Google's response",
		})
		return
	}

	// 3. Check Google's response status and decode
	var tokenInfo googleTokenInfoResponse
	err = json.Unmarshal(bodyBytes, &tokenInfo)
	if err != nil {
		log.Printf("Error unmarshalling Google tokeninfo response: %v. Body: %s", err, string(bodyBytes))
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to parse Google's response",
		})
		return
	}

	// If Google returned an error in the JSON body or a non-200 status
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

	// --- 5. Check Email and Verification Status ---
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

	// --- 6. Optional: Check Expiry (Google likely already did) ---
	expiresInSeconds := int64(0) // Default to 0 if not present
	if tokenInfo.ExpiresIn != "" {
		expiresInVal, convErr := strconv.ParseInt(tokenInfo.ExpiresIn, 10, 64)
		if convErr == nil && expiresInVal > 0 {
			expiresInSeconds = expiresInVal
		} else {
			log.Printf("Warning: Could not parse expires_in value '%s' from Google response.", tokenInfo.ExpiresIn)
		}
	}
	if expiresInSeconds <= 0 {
		// Potentially check 'exp' as a fallback if needed, similar to the example provided
		// But often tokeninfo endpoint guarantees non-expired if status is 200 OK.
		log.Printf("Token may be expired or has very short life. expires_in=%s", tokenInfo.ExpiresIn)
		// Decide if you want to reject tokens with 0 expiry here. For now, we allow.
	}

	// --- 7. Find or Create User in DB ---
	var appUser migrations.User
	appUser, err = queries.GetUserByEmail(ctx, tokenInfo.Email)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			// User does not exist, create them
			log.Printf("User with email '%s' not found. Creating new user.", tokenInfo.Email)
			newUser, createErr := queries.CreateUserWithEmail(ctx, tokenInfo.Email)
			if createErr != nil {
				log.Printf("CRITICAL: Failed to create user for email '%s': %v", tokenInfo.Email, createErr)
				var pgErr *pgconn.PgError
				if errors.As(createErr, &pgErr) && pgErr.Code == "23505" { // unique_violation
					utils.RespondWithJSON(w, http.StatusConflict, GoogleAuthResponse{Success: false, Message: "Email already exists (concurrent registration?)"})
				} else {
					utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{Success: false, Message: "Failed to create user account"})
				}
				return
			}
			log.Printf("New user created successfully: ID=%d, Email=%s", newUser.ID, newUser.Email)
			appUser = newUser // Use the newly created user data
		} else {
			// Other database error during lookup
			log.Printf("Database error looking up user by email '%s': %v", tokenInfo.Email, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{Success: false, Message: "Database error checking user"})
			return
		}
	} else {
		// User found
		log.Printf("Existing user found: ID=%d, Email=%s", appUser.ID, appUser.Email)
	}

	// --- 8. Generate Application JWT ---
	appToken, err := token.GenerateToken(appUser.ID)
	if err != nil {
		log.Printf("Failed to generate application token for user ID %d: %v", appUser.ID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GoogleAuthResponse{
			Success: false, Message: "Failed to generate session token",
		})
		return
	}

	// --- 9. Return Success with App JWT ---
	log.Printf("Google token verified and app token generated successfully for user ID: %d, Email: %s", appUser.ID, tokenInfo.Email)
	utils.RespondWithJSON(w, http.StatusOK, GoogleAuthResponse{
		Success: true,
		Message: "Authentication successful",
		Token:   appToken,
	})
}
