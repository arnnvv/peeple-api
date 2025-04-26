package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetConversationRequest struct {
	OtherUserID int32 `json:"other_user_id"`
}

type GetConversationResponse struct {
	Success             bool                          `json:"success"`
	Message             string                        `json:"message,omitempty"`
	OtherUserIsOnline   bool                          `json:"other_user_is_online"`
	OtherUserLastOnline *time.Time                    `json:"other_user_last_online,omitempty"`
	Messages            []ConversationMessageResponse `json:"messages"`
}

type RepliedToInfo struct {
	MessageID               int64   `json:"message_id"`
	SenderID                int32   `json:"sender_id"`
	TextSnippet             *string `json:"text_snippet,omitempty"`
	RepliedMessageMediaType *string `json:"media_type,omitempty"`
}

type ConversationMessageResponse struct {
	ID                  int64              `json:"id"`
	SenderUserID        int32              `json:"sender_user_id"`
	RecipientUserID     int32              `json:"recipient_user_id"`
	MessageText         pgtype.Text        `json:"message_text"`
	MediaUrl            pgtype.Text        `json:"media_url"`
	MediaType           pgtype.Text        `json:"media_type"`
	SentAt              pgtype.Timestamptz `json:"sent_at"`
	IsRead              bool               `json:"is_read"`
	Reactions           json.RawMessage    `json:"reactions"`
	CurrentUserReaction *string            `json:"current_user_reaction,omitempty"`
	ReplyTo             *RepliedToInfo     `json:"reply_to,omitempty"`
}

func GetConversationHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries, errDb := db.GetDB()
	if errDb != nil || queries == nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError,
			GetConversationResponse{
				Success: false,
				Message: "Database connection error",
			})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed,
			GetConversationResponse{
				Success: false,
				Message: "Method Not Allowed: Use POST",
			})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized,
			GetConversationResponse{
				Success: false,
				Message: "Authentication required",
			})
		return
	}
	requestingUserID := int32(claims.UserID)

	var req GetConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest,
			GetConversationResponse{
				Success: false,
				Message: "Invalid request body format",
			})
		return
	}
	defer r.Body.Close()
	otherUserID := req.OtherUserID

	if otherUserID <= 0 || otherUserID == requestingUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest,
			GetConversationResponse{
				Success: false,
				Message: "Valid other_user_id (different from self) is required",
			})
		return
	}

	otherUserData, userErr := queries.GetUserByID(ctx, otherUserID)
	if userErr != nil {
		if errors.Is(userErr, pgx.ErrNoRows) {
			utils.RespondWithJSON(w, http.StatusNotFound,
				GetConversationResponse{
					Success: false,
					Message: "The other user does not exist",
				})
		} else {
			log.Printf("ERROR: GetConversationHandler: Error fetching other user %d data: %v", otherUserID, userErr)
			utils.RespondWithJSON(w, http.StatusInternalServerError,
				GetConversationResponse{
					Success: false,
					Message: "Error checking user existence",
				})
		}
		return
	}

	mutualLikeParams := migrations.CheckMutualLikeExistsParams{LikerUserID: requestingUserID, LikedUserID: otherUserID}
	mutualLikeResult, checkErr := queries.CheckMutualLikeExists(ctx, mutualLikeParams)
	if checkErr != nil {
		log.Printf("ERROR: GetConversationHandler: Failed to check mutual like between %d and %d: %v", requestingUserID, otherUserID, checkErr)
		utils.RespondWithJSON(w, http.StatusInternalServerError,
			GetConversationResponse{
				Success: false,
				Message: "Error checking match status",
			})
		return
	}
	if !mutualLikeResult.Valid || !mutualLikeResult.Bool {
		utils.RespondWithJSON(w, http.StatusForbidden,
			GetConversationResponse{
				Success:  false,
				Message:  "You can only view conversations with users you have matched with.",
				Messages: []ConversationMessageResponse{},
			})
		return
	}

	log.Printf("INFO: GetConversationHandler: Fetching conversation between %d and %d", requestingUserID, otherUserID)

	params := migrations.GetConversationMessagesParams{
		SenderUserID:    requestingUserID,
		RecipientUserID: otherUserID,
	}
	dbMessages, err := queries.GetConversationMessages(ctx, params)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: GetConversationHandler: queries.GetConversationMessages failed: %v", err)
		utils.RespondWithJSON(w, http.StatusInternalServerError,
			GetConversationResponse{
				Success: false,
				Message: "Error retrieving conversation",
			})
		return
	}

	responseMessages := make([]ConversationMessageResponse, 0, len(dbMessages))
	var lastMessageID int64 = 0
	if len(dbMessages) > 0 {
		userReactionsMap, userReactionsErr := fetchUserReactionsForMessages(ctx, queries, dbMessages, requestingUserID)
		if userReactionsErr != nil {
			log.Printf("WARN: GetConversationHandler: Failed to pre-fetch user reactions for user %d: %v. Proceeding without CurrentUserReaction.", requestingUserID, userReactionsErr)
			userReactionsMap = make(map[int64]string)
		}

		for _, msg := range dbMessages {
			var currentUserReaction *string
			if emoji, found := userReactionsMap[msg.ID]; found {
				tempEmoji := emoji
				currentUserReaction = &tempEmoji
			}
			reactionsJSON := msg.ReactionsData
			if len(reactionsJSON) == 0 || string(reactionsJSON) == "null" {
				reactionsJSON = []byte("{}")
			}
			var replyInfo *RepliedToInfo = nil
			if msg.ReplyToMessageID.Valid && msg.RepliedMessageSenderID.Valid {
				replyInfo = &RepliedToInfo{
					MessageID:               msg.ReplyToMessageID.Int64,
					SenderID:                msg.RepliedMessageSenderID.Int32,
					TextSnippet:             nil,
					RepliedMessageMediaType: nil,
				}
				if snippetValue := msg.RepliedMessageTextSnippet; snippetValue != nil {
					snippetStr, ok := snippetValue.(string)
					if ok && snippetStr != "" {
						replyInfo.TextSnippet = &snippetStr
					}
				}
				if msg.RepliedMessageMediaType.Valid {
					mediaType := msg.RepliedMessageMediaType.String
					replyInfo.RepliedMessageMediaType = &mediaType
				}
			}

			responseMsg := ConversationMessageResponse{
				ID:                  msg.ID,
				SenderUserID:        msg.SenderUserID,
				RecipientUserID:     msg.RecipientUserID,
				MessageText:         msg.MessageText,
				MediaUrl:            msg.MediaUrl,
				MediaType:           msg.MediaType,
				SentAt:              msg.SentAt,
				IsRead:              msg.IsRead,
				Reactions:           json.RawMessage(reactionsJSON),
				CurrentUserReaction: currentUserReaction,
				ReplyTo:             replyInfo,
			}
			responseMessages = append(responseMessages, responseMsg)
			if msg.ID > lastMessageID {
				lastMessageID = msg.ID
			}
		}
	} else if dbMessages == nil {
		responseMessages = []ConversationMessageResponse{}
	}

	log.Printf("INFO: GetConversationHandler: Successfully processed %d messages for conversation between %d and %d.", len(responseMessages), requestingUserID, otherUserID)

	utils.RespondWithJSON(w, http.StatusOK, GetConversationResponse{
		Success:             true,
		OtherUserIsOnline:   otherUserData.IsOnline,
		OtherUserLastOnline: pgTimestampToTimePtr(otherUserData.LastOnline),
		Messages:            responseMessages,
	})

	if lastMessageID > 0 {
		go func(lastID int64) {
			bgCtx := context.Background()
			queriesBG, errDbBG := db.GetDB()
			if errDbBG != nil || queriesBG == nil {
				log.Printf("WARN: GetConversationHandler Goroutine: Cannot get DB queries: %v", errDbBG)
				return
			}
			markReadParams := migrations.MarkMessagesAsReadUntilParams{RecipientUserID: requestingUserID, SenderUserID: otherUserID, ID: lastID}
			_, errMark := queriesBG.MarkMessagesAsReadUntil(bgCtx, markReadParams)
			if errMark != nil {
				log.Printf("WARN: GetConversationHandler Goroutine: Failed mark read until ID %d: %v", lastID, errMark)
			} else {
				log.Printf("INFO: GetConversationHandler Goroutine: Marked messages read until ID %d", lastID)
			}
		}(lastMessageID)
	}
}

func fetchUserReactionsForMessages(ctx context.Context, queries *migrations.Queries, messages []migrations.GetConversationMessagesRow, userID int32) (map[int64]string, error) {
	if len(messages) == 0 {
		return make(map[int64]string), nil
	}

	messageIDs := make([]int64, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}

	params := migrations.GetUserReactionsForMessagesParams{
		UserID:  userID,
		Column2: messageIDs,
	}

	reactions, err := queries.GetUserReactionsForMessages(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return make(map[int64]string), nil
		}
		return nil, fmt.Errorf("failed to execute GetUserReactionsForMessages: %w", err)
	}

	resultMap := make(map[int64]string, len(reactions))
	for _, reaction := range reactions {
		resultMap[reaction.MessageID] = reaction.Emoji
	}

	return resultMap, nil
}
