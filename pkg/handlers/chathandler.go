package handlers

import (
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/ws"
)

func ChatHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
		if !ok || claims == nil || claims.UserID <= 0 {
			log.Println("ERROR: ChatHandler: Authentication claims missing or invalid.")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := int32(claims.UserID)

		ws.ServeWs(hub, w, r, userID)
	}
}
