package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
)

type GetConversationRequest struct {
	OtherUserID int32 `json:"other_user_id"`
}

type GetConversationResponse struct {
	Success  bool                     `json:"success"`
	Message  string                   `json:"message,omitempty"`
	Messages []migrations.ChatMessage `json:"messages"`
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

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, GetConversationResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, GetConversationResponse{Success: false, Message: "Authentication required"})
		return
	}
	requestingUserID := int32(claims.UserID)

	var req GetConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: GetConversationHandler: Invalid request body for user %d: %v", requestingUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()
	otherUserID := req.OtherUserID

	if otherUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Valid other_user_id is required in request body"})
		return
	}
	if otherUserID == requestingUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, GetConversationResponse{Success: false, Message: "Cannot fetch conversation with yourself"})
		return
	}

	_, err := queries.GetUserByID(ctx, otherUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondWithJSON(w, http.StatusNotFound, GetConversationResponse{Success: false, Message: "The other user does not exist"})
		} else {
			log.Printf("ERROR: GetConversationHandler: Error checking existence of user %d: %v", otherUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, GetConversationResponse{Success: false, Message: "Error checking user existence"})
		}
		return
	}

	log.Printf("INFO: GetConversationHandler: Fetching FULL conversation between %d and %d (from request body)", requestingUserID, otherUserID)

	params := migrations.GetConversationMessagesParams{
		SenderUserID:    requestingUserID,
		RecipientUserID: otherUserID,
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

	log.Printf("INFO: GetConversationHandler: Found %d total messages for conversation between %d and %d.", len(messages), requestingUserID, otherUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetConversationResponse{
		Success:  true,
		Messages: messages,
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
