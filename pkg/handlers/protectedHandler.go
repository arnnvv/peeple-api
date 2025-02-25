package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/token"
)

func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to retrieve token claims",
		})
		return
	}

	// Build the JSON response with only UserID
	response := map[string]interface{}{
		"user_id": claims.UserID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
