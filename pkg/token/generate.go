package token

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"
	"unicode"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/golang-jwt/jwt/v5"
)

type TokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var (
	jwtSecret     []byte
	jwtSecretOnce sync.Once
)

func getSecret() []byte {
	jwtSecretOnce.Do(func() {
		jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	})
	return jwtSecret
}

func GenerateToken(userID int32) (string, error) {
	// may add expiration time to claims when needed
	claims := &Claims{
		UserID: uint(userID),
		// RegisteredClaims: jwt.RegisteredClaims{
		//  ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // Example: 24h expiry
		//  IssuedAt:  jwt.NewNumericDate(time.Now()),
		//  NotBefore: jwt.NewNumericDate(time.Now()),
		// },
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(getSecret())
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func GenerateTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(TokenResponse{
			Success: false,
			Message: "Only GET method allowed",
		})
		return
	}

	phone := r.URL.Query().Get("phone")
	if phone == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(TokenResponse{
			Success: false,
			Message: "Phone number is required",
		})
		return
	}

	if len(phone) != 10 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(TokenResponse{
			Success: false,
			Message: "Phone number must be exactly 10 digits",
		})
		return
	}

	for _, c := range phone {
		if !unicode.IsDigit(c) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(TokenResponse{
				Success: false,
				Message: "Phone number must contain only digits",
			})
			return
		}
	}

	queries := db.GetDB()

	var user migrations.User
	var err error

	user, err = queries.GetUserByPhone(r.Context(), phone)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// User not found for the given phone number
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "User with the provided phone number not found",
			})
		} else {
			log.Printf("GenerateTokenHandler: Error fetching user by phone %s: %v\n", phone, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "Internal server error while retrieving user data",
			})
		}
		return
	}

	token, err := GenerateToken(user.ID)

	if err != nil {
		log.Printf("GenerateTokenHandler: Error generating token for user %d: %v\n", user.ID, err)
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
		Message: token,
	})
}
