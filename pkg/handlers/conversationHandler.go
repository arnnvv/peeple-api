package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

const defaultConversationLimit = 50
const defaultConversationOffset = 0

type GetConversationResponse struct {
	Success  bool                     `json:"success"`
	Message  string                   `json:"message,omitempty"`
	Messages []migrations.ChatMessage `json:"messages"`
	HasMore  bool                     `json:"has_more"`
}

func GetConversationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, _ := db.GetDB()
	if queries == nil {
		log.Println("ERROR: GetConversationHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetConversationResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodGet {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, GetConversationResponse{Success: false, Message: "Method Not Allowed: Use GET"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, GetConversationResponse{Success: false, Message: "Authentication required"})
		return
	}
	requestingUserID := int32(claims.UserID)

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "api" || pathParts[1] != "conversation" {
		log.Printf("ERROR: GetConversationHandler: Invalid URL path structure: %s", r.URL.Path)
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Invalid request URL structure. Expected /api/conversation/{userID}"})
		return
	}
	otherUserIDStr := pathParts[len(pathParts)-1]

	otherUserID64, err := strconv.ParseInt(otherUserIDStr, 10, 32)
	if err != nil {
		log.Printf("ERROR: GetConversationHandler: Invalid otherUserID '%s' in path: %v", otherUserIDStr, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Invalid user ID provided in the URL"})
		return
	}
	otherUserID := int32(otherUserID64)

	if otherUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Valid other user ID is required"})
		return
	}
	if otherUserID == requestingUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Cannot fetch conversation with yourself"})
		return
	}

	_, err = queries.GetUserByID(ctx, otherUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithJSON(w, http.StatusNotFound, GetConversationResponse{Success: false, Message: "The other user does not exist"})
		} else {
			log.Printf("ERROR: GetConversationHandler: Error checking existence of user %d: %v", otherUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, GetConversationResponse{Success: false, Message: "Error checking user existence"})
		}
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := defaultConversationLimit
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := defaultConversationOffset
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	log.Printf("INFO: GetConversationHandler: Fetching conversation between %d and %d (limit: %d, offset: %d)", requestingUserID, otherUserID, limit, offset)

	params := migrations.GetConversationMessagesParams{
		SenderUserID:    requestingUserID,
		RecipientUserID: otherUserID,
		Limit:           int32(limit),
		Offset:          int32(offset),
	}

	messages, err := queries.GetConversationMessages(ctx, params)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: GetConversationHandler: Failed to fetch messages between %d and %d: %v", requestingUserID, otherUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, GetConversationResponse{Success: false, Message: "Error retrieving conversation"})
		return
	}

	if messages == nil {
		messages = []migrations.ChatMessage{}
	}

	hasMore := len(messages) == limit

	log.Printf("INFO: GetConversationHandler: Found %d messages for conversation between %d and %d. HasMore: %t", len(messages), requestingUserID, otherUserID, hasMore)

	utils.RespondWithJSON(w, http.StatusOK, GetConversationResponse{
		Success:  true,
		Messages: messages,
		HasMore:  hasMore,
	})

	go func() {
		bgCtx := context.Background()
		markReadParams := migrations.MarkMessagesAsReadParams{
			RecipientUserID: requestingUserID,
			SenderUserID:    otherUserID,
		}
		err := queries.MarkMessagesAsRead(bgCtx, markReadParams)
		if err != nil {
			log.Printf("WARN: GetConversationHandler: Failed to mark messages as read in background between %d and %d: %v", otherUserID, requestingUserID, err)
		} else {
			log.Printf("INFO: GetConversationHandler: Marked messages as read in background from %d to %d", otherUserID, requestingUserID)
		}
	}()
}
