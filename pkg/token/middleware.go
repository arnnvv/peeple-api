package token

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"

    "github.com/golang-jwt/jwt/v5"
)

type ErrorResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

var (
    errInvalidHeader = ErrorResponse{
        Success: false,
        Message: "Invalid Authorization header format",
    }
    errInvalidToken = ErrorResponse{
        Success: false,
        Message: "Invalid token",
    }
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
    parser := jwt.NewParser(
        jwt.WithoutClaimsValidation(),
        jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
    )
    secret := getSecret()

    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        
        authHeader := r.Header.Get("Authorization")
        prefix, tokenString, ok := strings.Cut(authHeader, " ")
        if !ok || !strings.EqualFold(prefix, "Bearer") {
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(errInvalidHeader)
            return
        }

        claims := &Claims{}
        token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
            return secret, nil
        })

        if err != nil || !token.Valid {
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(errInvalidToken)
            return
        }

        if claims.UserID == 0 {
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(errInvalidToken)
            return
        }

        ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}
