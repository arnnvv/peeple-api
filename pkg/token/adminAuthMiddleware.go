package token

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
)

var (
	errAdminRequired = ErrorResponse{
		Success: false,
		Message: "Admin access required",
	}
	errUserNotFound = ErrorResponse{
		Success: false,
		Message: "User associated with token not found",
	}
	errInternalServer = ErrorResponse{
		Success: false,
		Message: "Internal server error",
	}
)

func AdminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(ClaimsContextKey).(*Claims)
		if !ok || claims == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(errInvalidToken)
			return
		}

		q, err := db.GetDB()
		if err != nil {
			log.Printf("AdminAuthMiddleware: Failed to get database connection: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(errInternalServer)
			return
		}

		user, err := q.GetUserByID(r.Context(), int32(claims.UserID))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(errUserNotFound)
			} else {
				log.Printf("AdminAuthMiddleware: Error fetching user %d: %v", claims.UserID, err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(errInternalServer)
			}
			return
		}

		if user.Role != migrations.UserRoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(errAdminRequired)
			return
		}

		next.ServeHTTP(w, r)
	})
}
