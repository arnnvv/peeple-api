package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
)

type MarkReadUntilRequest struct {
	OtherUserID int32 `json:"other_user_id"`
	MessageID   int64 `json:"message_id"`
}

type MarkReadUntilResponse struct {
	Success              bool   `json:"success"`
	Message              string `json:"message"`
	MessagesMarkedAsRead int64  `json:"messages_marked_as_read"`
}

func MarkReadUntilHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		log.Println("ERROR: MarkReadUntilHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, MarkReadUntilResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, MarkReadUntilResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, MarkReadUntilResponse{Success: false, Message: "Authentication required"})
		return
	}
	recipientUserID := int32(claims.UserID)

	var req MarkReadUntilRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: MarkReadUntilHandler: Invalid request body for user %d: %v", recipientUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkReadUntilResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.OtherUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkReadUntilResponse{Success: false, Message: "Valid other_user_id is required"})
		return
	}
	if req.MessageID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkReadUntilResponse{Success: false, Message: "Valid message_id is required"})
		return
	}
	if req.OtherUserID == recipientUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, MarkReadUntilResponse{Success: false, Message: "Cannot mark messages from yourself as read this way"})
		return
	}

	log.Printf("INFO: MarkReadUntilHandler: User %d marking messages from user %d as read up to message ID %d",
		recipientUserID, req.OtherUserID, req.MessageID)

	params := migrations.MarkMessagesAsReadUntilParams{
		RecipientUserID: recipientUserID,
		SenderUserID:    req.OtherUserID,
		ID:              req.MessageID,
	}

	cmdTag, err := queries.MarkMessagesAsReadUntil(ctx, params)
	if err != nil {
		log.Printf("ERROR: MarkReadUntilHandler: Failed to update messages for user %d from user %d: %v", recipientUserID, req.OtherUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, MarkReadUntilResponse{Success: false, Message: "Failed to update message status"})
		return
	}

	rowsAffected := cmdTag.RowsAffected()
	log.Printf("INFO: MarkReadUntilHandler: Successfully marked %d messages as read for user %d from user %d (up to ID %d)",
		rowsAffected, recipientUserID, req.OtherUserID, req.MessageID)

	utils.RespondWithJSON(w, http.StatusOK, MarkReadUntilResponse{
		Success:              true,
		Message:              "Messages marked as read successfully.",
		MessagesMarkedAsRead: rowsAffected,
	})
}
