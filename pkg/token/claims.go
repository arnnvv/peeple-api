package token

import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	PhoneNumber string `json:"phoneNumber"`
	jwt.RegisteredClaims
}

func (c *Claims) Valid() error {
	// Add any custom validation logic here
	// Since we want no expiration, we'll completely ignore time checks
	return nil
}
