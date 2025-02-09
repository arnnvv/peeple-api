package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/token"
)

func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		// Respond with JSON error message
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to retrieve token claims",
		})
		return
	}

	// Build the JSON response with the phone number
	response := map[string]string{
		"phone_number": claims.PhoneNumber,
	}

	// Set header and send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
