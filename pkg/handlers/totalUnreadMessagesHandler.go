package handlers

import (
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type UnreadCountResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message,omitempty"`
	UnreadCount int64  `json:"unread_count"`
}

func GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: GetUnreadCountHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnreadCountResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, UnreadCountResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, UnreadCountResponse{Success: false, Message: "Authentication required"})
		return
	}
	userID := int32(claims.UserID)

	log.Printf("INFO: GetUnreadCountHandler: Fetching total unread count for user %d", userID)

	unreadCount, err := queries.GetTotalUnreadCount(ctx, userID)
	if err != nil {
		// While pgx.ErrNoRows isn't expected for COUNT(*), check anyway just in case
		// if errors.Is(err, pgx.ErrNoRows) { // Unlikely for COUNT(*)
		// 	log.Printf("INFO: GetUnreadCountHandler: No unread messages found for user %d (Count=0).", userID)
		// 	utils.RespondWithJSON(w, http.StatusOK, UnreadCountResponse{Success: true, UnreadCount: 0})
		// 	return
		// }
		log.Printf("ERROR: GetUnreadCountHandler: Failed to fetch unread count for user %d: %v", userID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, UnreadCountResponse{Success: false, Message: "Failed to retrieve unread message count"})
		return
	}

	log.Printf("INFO: GetUnreadCountHandler: User %d has %d unread messages.", userID, unreadCount)

	utils.RespondWithJSON(w, http.StatusOK, UnreadCountResponse{
		Success:     true,
		UnreadCount: unreadCount,
	})
}
