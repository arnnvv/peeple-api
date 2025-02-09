// middleware.go
package token

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var (
	errInvalidHeader = []byte("Invalid Authorization header format")
	errInvalidToken  = []byte("Invalid token")
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	parser := jwt.NewParser(
		jwt.WithoutClaimsValidation(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	secret := getSecret()

	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		prefix, tokenString, ok := strings.Cut(authHeader, " ")
		if !ok || !strings.EqualFold(prefix, "Bearer") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(errInvalidHeader)
			return
		}

		claims := &Claims{}
		token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			return secret, nil
		})

		if err != nil || !token.Valid {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write(errInvalidToken)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
