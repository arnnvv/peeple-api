package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"unicode/utf8"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/arnnvv/peeple-api/pkg/ws"
	"github.com/jackc/pgx/v5"
)

type ReactionRequest struct {
	MessageID int64  `json:"message_id"`
	Emoji     string `json:"emoji"`
}

type ReactionResponse struct {
	Success    bool                        `json:"success"`
	Message    string                      `json:"message"`
	Reaction   *migrations.MessageReaction `json:"reaction,omitempty"`
	WasRemoved bool                        `json:"was_removed"`
}

func ToggleReplaceReactionHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		queries, errDb := db.GetDB()
		if errDb != nil || queries == nil {
			respondReactionError(w, http.StatusInternalServerError, "Database connection error")
			return
		}

		if r.Method != http.MethodPost {
			respondReactionError(w, http.StatusMethodNotAllowed, "Method Not Allowed: Use POST")
			return
		}

		claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
		if !ok || claims == nil || claims.UserID <= 0 {
			respondReactionError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		reactorUserID := int32(claims.UserID)

		var req ReactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondReactionError(w, http.StatusBadRequest, "Invalid request body format")
			return
		}
		defer r.Body.Close()

		if req.MessageID <= 0 {
			respondReactionError(w, http.StatusBadRequest, "Valid message_id is required")
			return
		}
		if req.Emoji == "" || utf8.RuneCountInString(req.Emoji) > 10 {
			respondReactionError(w, http.StatusBadRequest, "Valid emoji is required (1-10 characters)")
			return
		}

		msgParticipants, err := queries.GetMessageSenderRecipient(ctx, req.MessageID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
				log.Printf("WARN: Reaction attempt failed: Message ID %d not found.", req.MessageID)
				respondReactionError(w, http.StatusNotFound, fmt.Sprintf("Message with ID %d not found.", req.MessageID))
			} else {
				log.Printf("ERROR: Failed to fetch message participants for validation (MsgID: %d): %v", req.MessageID, err)
				respondReactionError(w, http.StatusInternalServerError, "Error validating message")
			}
			return
		}
		if msgParticipants.SenderUserID != reactorUserID && msgParticipants.RecipientUserID != reactorUserID {
			log.Printf("WARN: Reaction attempt failed: User %d is not part of the conversation for message ID %d (Sender: %d, Recipient: %d).",
				reactorUserID, req.MessageID, msgParticipants.SenderUserID, msgParticipants.RecipientUserID)
			respondReactionError(w, http.StatusForbidden, "You can only react to messages in your conversations.")
			return
		}

		existingReaction, err := queries.GetSingleReactionByUser(ctx, migrations.GetSingleReactionByUserParams{
			MessageID: req.MessageID,
			UserID:    reactorUserID,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, sql.ErrNoRows) {
			log.Printf("ERROR: Failed to check existing reaction for user %d on message %d: %v", reactorUserID, req.MessageID, err)
			respondReactionError(w, http.StatusInternalServerError, "Failed to check existing reaction")
			return
		}
		reactionExists := !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, sql.ErrNoRows)

		var finalReaction *migrations.MessageReaction
		var wasRemoved bool
		var responseStatus int
		var responseMessage string

		if !reactionExists {
			addParams := migrations.UpsertMessageReactionParams{MessageID: req.MessageID, UserID: reactorUserID, Emoji: req.Emoji}
			added, addErr := queries.UpsertMessageReaction(ctx, addParams)
			if addErr != nil {
				log.Printf("ERROR: Failed to add new reaction for user %d: %v", reactorUserID, addErr)
				respondReactionError(w, http.StatusInternalServerError, "Failed to add reaction")
				return
			}
			finalReaction = &added
			wasRemoved = false
			responseStatus = http.StatusCreated
			responseMessage = "Reaction added."

		} else {
			if existingReaction.Emoji == req.Emoji {
				deleteParams := migrations.DeleteMessageReactionByUserParams{MessageID: req.MessageID, UserID: reactorUserID}
				cmdTag, delErr := queries.DeleteMessageReactionByUser(ctx, deleteParams)
				if delErr != nil {
					log.Printf("ERROR: Failed to delete reaction for user %d on message %d: %v", reactorUserID, req.MessageID, delErr)
					respondReactionError(w, http.StatusInternalServerError, "Failed to remove reaction")
					return
				}
				finalReaction = nil
				wasRemoved = cmdTag.RowsAffected() > 0
				responseStatus = http.StatusOK
				responseMessage = "Reaction removed."

			} else {
				updateParams := migrations.UpsertMessageReactionParams{MessageID: req.MessageID, UserID: reactorUserID, Emoji: req.Emoji}
				updated, updateErr := queries.UpsertMessageReaction(ctx, updateParams)
				if updateErr != nil {
					log.Printf("ERROR: Failed to update reaction for user %d: %v", reactorUserID, updateErr)
					respondReactionError(w, http.StatusInternalServerError, "Failed to update reaction")
					return
				}
				finalReaction = &updated
				wasRemoved = false
				responseStatus = http.StatusOK
				responseMessage = "Reaction updated."
			}
		}

		if hub != nil {
			participants := []int32{msgParticipants.SenderUserID, msgParticipants.RecipientUserID}
			emojiToSend := ""
			if finalReaction != nil {
				emojiToSend = finalReaction.Emoji
			}
			hub.BroadcastReaction(req.MessageID, reactorUserID, emojiToSend, wasRemoved, participants)
		} else {
			log.Println("WARN: ReactionHandler: Hub is nil, cannot broadcast reaction update.")
		}

		utils.RespondWithJSON(w, responseStatus, ReactionResponse{
			Success:    true,
			Message:    responseMessage,
			Reaction:   finalReaction,
			WasRemoved: wasRemoved,
		})
	}
}

func respondReactionError(w http.ResponseWriter, code int, message string) {
	utils.RespondWithJSON(w, code, ReactionResponse{Success: false, Message: message})
}
