package token

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Validate Bearer token format
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		// Retrieve JWT secret
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			http.Error(w, "Server configuration error", http.StatusInternalServerError)
			return
		}

		// Parse and validate token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
