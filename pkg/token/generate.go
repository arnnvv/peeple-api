package token

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

type TokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// ErrorResponse struct (can be shared or redefined if needed)
// type ErrorResponse struct {
// 	Success bool   `json:"success"`
// 	Message string `json:"message"`
// }

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

func getSecret() []byte {
	jwtSecretOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			log.Fatal("FATAL: JWT_SECRET environment variable not set!")
		}
		jwtSecret = []byte(secret)
	})
	return jwtSecret
}

func GenerateToken(userID int32) (string, error) {
	claims := &Claims{
		UserID: uint(userID),
		// Add RegisteredClaims if needed (e.g., expiry)
		// RegisteredClaims: jwt.RegisteredClaims{
		//  ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24 * 30)), // e.g., 30 days
		// },
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := getSecret()
	if len(secret) == 0 {
		return "", errors.New("JWT secret is not configured")
	}
	return token.SignedString(secret)
}

// GenerateTokenHandler is primarily for testing/debugging.
// It generates a token for an existing user based on their email.
// WARNING: In production, this endpoint should be removed or heavily secured.
func GenerateTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Message: "Only GET method allowed",
		})
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Message: "Email address query parameter is required",
		})
		return
	}

	if !strings.Contains(email, "@") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Message: "Invalid email format provided",
		})
		return
	}

	queries, err2 := db.GetDB()
	if err2 != nil {
		log.Printf("AdminAuthMiddleware: Failed to get database connection: %v", err2)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errInternalServer)
		return
	}
	if queries == nil {
		log.Printf("GenerateTokenHandler: Database connection not available")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Message: "Internal server error (DB connection)",
		})
		return
	}

	var user migrations.User
	var err error

	user, err = queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "User with the provided email not found",
			})
		} else {
			log.Printf("GenerateTokenHandler: Error fetching user by email %s: %v\n", email, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "Internal server error while retrieving user data",
			})
		}
		return
	}

	tokenString, err := GenerateToken(user.ID)
	if err != nil {
		log.Printf("GenerateTokenHandler: Error generating token for user %d (%s): %v\n", user.ID, email, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Success: false,
			Message: "Failed to generate authentication token",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TokenResponse{
		Success: true,
		Message: "Token generated successfully (for testing/debug)",
		Token:   tokenString,
	})
}
