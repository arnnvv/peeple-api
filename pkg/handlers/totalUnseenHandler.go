package handlers

import (
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type NotificationCountsResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	UnreadChatCount int64  `json:"unread_chat_count"`
	UnseenLikeCount int64  `json:"unseen_like_count"`
}

func GetUnreadCountHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: GetUnreadCountHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, NotificationCountsResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, NotificationCountsResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, NotificationCountsResponse{Success: false, Message: "Authentication required"})
		return
	}
	userID := int32(claims.UserID)

	log.Printf("INFO: GetUnreadCountHandler: Fetching notification counts for user %d", userID)

	unreadChatCount, errChat := queries.GetTotalUnreadCount(ctx, userID)
	if errChat != nil {
		log.Printf("ERROR: GetUnreadCountHandler: Failed to fetch unread chat count for user %d: %v", userID, errChat)
		utils.RespondWithJSON(w, http.StatusInternalServerError, NotificationCountsResponse{Success: false, Message: "Failed to retrieve unread message count"})
		return
	}
	log.Printf("INFO: GetUnreadCountHandler: User %d has %d unread chat messages.", userID, unreadChatCount)

	unseenLikeCount, errLike := queries.GetUnseenLikeCount(ctx, userID)
	if errLike != nil {
		log.Printf("ERROR: GetUnreadCountHandler: Failed to fetch unseen like count for user %d: %v", userID, errLike)
		utils.RespondWithJSON(w, http.StatusInternalServerError, NotificationCountsResponse{Success: false, Message: "Failed to retrieve unseen like count"})
		return
	}
	log.Printf("INFO: GetUnreadCountHandler: User %d has %d unseen incoming likes (non-mutual).", userID, unseenLikeCount)

	utils.RespondWithJSON(w, http.StatusOK, NotificationCountsResponse{
		Success:         true,
		UnreadChatCount: unreadChatCount,
		UnseenLikeCount: unseenLikeCount,
	})
}
