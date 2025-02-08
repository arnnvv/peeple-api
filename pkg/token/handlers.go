package token

import (
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Extract phone number from query parameters
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	// Retrieve JWT secret from environment
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Create token with phone number claim
	claims := &Claims{
		PhoneNumber: phone,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Return the token as plain text
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(tokenString))
}
