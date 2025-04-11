// FILE: pkg/utils/utils.go
package utils

import (
	"encoding/json"
	"net/http"
)

// RespondWithJSON sends a JSON response with a given status code and payload.
func RespondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

// ErrorResponse defines a standard structure for JSON error responses.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// RespondWithError sends a JSON error response.
func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, ErrorResponse{Success: false, Message: message})
}
