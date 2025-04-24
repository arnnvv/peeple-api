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

func ToggleReplaceReactionHandler(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("INFO: User %d attempting reaction '%s' on message %d. Validating message access...", reactorUserID, req.Emoji, req.MessageID)

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
	log.Printf("DEBUG: Message access validation passed for user %d on message %d.", reactorUserID, req.MessageID)

	log.Printf("DEBUG: Checking existing reaction by user %d on message %d", reactorUserID, req.MessageID)
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

	if !reactionExists {
		// Case 1: No existing reaction -> Add the new one using UPSERT
		log.Printf("INFO: No existing reaction found. Adding '%s' for user %d on message %d.", req.Emoji, reactorUserID, req.MessageID)
		addParams := migrations.UpsertMessageReactionParams{
			MessageID: req.MessageID,
			UserID:    reactorUserID,
			Emoji:     req.Emoji,
		}
		addedReaction, addErr := queries.UpsertMessageReaction(ctx, addParams)
		if addErr != nil {
			log.Printf("ERROR: Failed to add new reaction for user %d: %v", reactorUserID, addErr)
			respondReactionError(w, http.StatusInternalServerError, "Failed to add reaction")
			return
		}
		utils.RespondWithJSON(w, http.StatusCreated, ReactionResponse{
			Success:    true,
			Message:    "Reaction added.",
			Reaction:   &addedReaction,
			WasRemoved: false,
		})

	} else {
		// Case 2: Reaction already exists. Compare emojis.
		if existingReaction.Emoji == req.Emoji {
			// Case 2a: Same emoji -> Delete the reaction (toggle off)
			log.Printf("INFO: Existing reaction ('%s') matches new ('%s'). Removing reaction for user %d on message %d.", existingReaction.Emoji, req.Emoji, reactorUserID, req.MessageID)
			deleteParams := migrations.DeleteMessageReactionByUserParams{
				MessageID: req.MessageID,
				UserID:    reactorUserID,
			}
			cmdTag, delErr := queries.DeleteMessageReactionByUser(ctx, deleteParams)
			if delErr != nil {
				log.Printf("ERROR: Failed to delete reaction for user %d on message %d: %v", reactorUserID, req.MessageID, delErr)
				respondReactionError(w, http.StatusInternalServerError, "Failed to remove reaction")
				return
			}
			removed := cmdTag.RowsAffected() > 0
			utils.RespondWithJSON(w, http.StatusOK, ReactionResponse{
				Success:    true,
				Message:    "Reaction removed.",
				Reaction:   nil,
				WasRemoved: removed,
			})

		} else {
			// Case 2b: Different emoji -> Update the reaction (replace) using UPSERT
			log.Printf("INFO: Existing reaction ('%s') differs from new ('%s'). Updating reaction for user %d on message %d.", existingReaction.Emoji, req.Emoji, reactorUserID, req.MessageID)
			updateParams := migrations.UpsertMessageReactionParams{
				MessageID: req.MessageID,
				UserID:    reactorUserID,
				Emoji:     req.Emoji,
			}
			updatedReaction, updateErr := queries.UpsertMessageReaction(ctx, updateParams)
			if updateErr != nil {
				log.Printf("ERROR: Failed to update reaction for user %d: %v", reactorUserID, updateErr)
				respondReactionError(w, http.StatusInternalServerError, "Failed to update reaction")
				return
			}
			utils.RespondWithJSON(w, http.StatusOK, ReactionResponse{
				Success:    true,
				Message:    "Reaction updated.",
				Reaction:   &updatedReaction,
				WasRemoved: false,
			})
		}
	}
}

func respondReactionError(w http.ResponseWriter, code int, message string) {
	utils.RespondWithJSON(w, code, ReactionResponse{Success: false, Message: message})
}
