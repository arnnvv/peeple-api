package token

import (
	"encoding/json"
	"net/http"

	"github.com/arnnvv/peeple-api/db"
	"github.com/arnnvv/peeple-api/pkg/enums"
)

var (
	errAdminRequired = ErrorResponse{
		Success: false,
		Message: "Admin access required",
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

		var user db.UserModel
		if result := db.DB.Where("id = ?", claims.UserID).First(&user); result.Error != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{
				Success: false,
				Message: "User not found",
			})
			return
		}

		if user.Role == nil || *user.Role != enums.UserRoleAdmin {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(errAdminRequired)
			return
		}

		next.ServeHTTP(w, r)
	})
}
