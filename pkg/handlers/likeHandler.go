package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dailyStandardLikeLimit = 15
const maxCommentLength = 140

type ContentLikeRequest struct {
	LikedUserID       int32                      `json:"liked_user_id"`
	ContentType       migrations.ContentLikeType `json:"content_type"`
	ContentIdentifier string                     `json:"content_identifier"`
	Comment           *string                    `json:"comment,omitempty"`
	InteractionType   *string                    `json:"interaction_type,omitempty"`
}

type LikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func LikeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	pool := db.GetPool()

	if queries == nil || pool == nil {
		log.Println("ERROR: LikeHandler: Database connection not available.")
		utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Database connection error"})
		return
	}

	if r.Method != http.MethodPost {
		utils.RespondWithJSON(w, http.StatusMethodNotAllowed, LikeResponse{Success: false, Message: "Method Not Allowed: Use POST"})
		return
	}

	claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil || claims.UserID <= 0 {
		utils.RespondWithJSON(w, http.StatusUnauthorized, LikeResponse{Success: false, Message: "Authentication required"})
		return
	}
	likerUserID := int32(claims.UserID)

	// might be useful elsewhere
	// _, err := queries.GetUserByID(ctx, likerUserID)
	// if err != nil {
	// 	log.Printf("ERROR: LikeHandler: Failed to fetch liker user %d: %v", likerUserID, err)
	// 	utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error fetching user data"})
	// 	return
	// }

	var req ContentLikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: LikeHandler: Invalid request body for user %d: %v", likerUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	if req.LikedUserID <= 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Valid liked_user_id is required"})
		return
	}
	if req.LikedUserID == likerUserID {
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Cannot like yourself"})
		return
	}
	switch req.ContentType {
	case migrations.ContentLikeTypeMedia, migrations.ContentLikeTypePromptStory, migrations.ContentLikeTypePromptMytype, migrations.ContentLikeTypePromptGettingpersonal, migrations.ContentLikeTypePromptDatevibes, migrations.ContentLikeTypeAudioPrompt:
	default:
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Invalid content_type specified"})
		return
	}
	if req.ContentIdentifier == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "content_identifier is required"})
		return
	}

	contentValid, validationErr := validateContentInput(ctx, queries, req.LikedUserID, req.ContentType, req.ContentIdentifier)
	if validationErr != nil {
		log.Printf("ERROR: LikeHandler: Error validating content input for user %d liking %d: %v", likerUserID, req.LikedUserID, validationErr)
		utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error validating content"})
		return
	}
	if !contentValid {
		log.Printf("WARN: LikeHandler: Content invalid/not found for user %d liking %d: Type=%s, Identifier=%s", likerUserID, req.LikedUserID, req.ContentType, req.ContentIdentifier)
		utils.RespondWithJSON(w, http.StatusNotFound, LikeResponse{Success: false, Message: "The content you tried to like does not exist or is invalid"})
		return
	}
	log.Printf("INFO: Content validation passed for like request: User=%d -> User=%d", likerUserID, req.LikedUserID)

	isMutualLike := false
	checkLikeParams := migrations.CheckLikeExistsParams{
		LikerUserID: req.LikedUserID,
		LikedUserID: likerUserID,
	}
	existsResult, checkErr := queries.CheckLikeExists(ctx, checkLikeParams)
	if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
		log.Printf("WARN: LikeHandler: Failed to check for existing reverse like (user %d -> %d): %v", req.LikedUserID, likerUserID, checkErr)
	} else if checkErr == nil {
		isMutualLike = existsResult
	}
	log.Printf("INFO: LikeHandler: Is mutual like scenario? %t (Liker: %d, Liked: %d)", isMutualLike, likerUserID, req.LikedUserID)

	commentText := ""
	commentProvided := false
	if req.Comment != nil {
		commentText = strings.TrimSpace(*req.Comment)
		if commentText != "" {
			commentProvided = true
			if utf8.RuneCountInString(commentText) > maxCommentLength {
				utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: fmt.Sprintf("Comment exceeds maximum length of %d characters", maxCommentLength)})
				return
			}
		}
	}

	if !isMutualLike && !commentProvided {
		log.Printf("WARN: LikeHandler: Comment required for initial like from user %d to %d", likerUserID, req.LikedUserID)
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Comment is required when sending an initial like"})
		return
	}
	if isMutualLike && !commentProvided {
		log.Printf("INFO: LikeHandler: Allowing empty comment for mutual like between %d and %d", likerUserID, req.LikedUserID)
	}

	interactionType := migrations.LikeInteractionTypeStandard
	if req.InteractionType != nil && strings.ToLower(*req.InteractionType) == string(migrations.LikeInteractionTypeRose) {
		interactionType = migrations.LikeInteractionTypeRose
	}

	log.Printf("INFO: Like attempt: User=%d -> User=%d, Type=%s, Content=%s:%s, CommentPresent=%t", likerUserID, req.LikedUserID, interactionType, req.ContentType, req.ContentIdentifier, commentProvided)

	addLikeParams := migrations.AddContentLikeParams{
		LikerUserID:       likerUserID,
		LikedUserID:       req.LikedUserID,
		ContentType:       req.ContentType,
		ContentIdentifier: req.ContentIdentifier,
		Comment:           pgtype.Text{String: commentText, Valid: commentProvided},
		InteractionType:   interactionType,
	}

	var likeErr error
	if interactionType == migrations.LikeInteractionTypeRose {
		likeErr = handleRoseLike(ctx, queries, pool, addLikeParams)
		if likeErr != nil {
			log.Printf("ERROR: LikeHandler: Failed rose like: User=%d -> User=%d, Error=%v", likerUserID, req.LikedUserID, likeErr)
			status := http.StatusInternalServerError
			msg := "Failed to process rose like"
			if errors.Is(likeErr, ErrInsufficientConsumables) {
				status = http.StatusForbidden
				msg = likeErr.Error()
			} else if errors.Is(likeErr, ErrLikeAlreadyExists) {
				status = http.StatusConflict
				msg = likeErr.Error()
			}
			utils.RespondWithJSON(w, status, LikeResponse{Success: false, Message: msg})
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Rose sent successfully"})
	} else {
		likeErr = handleStandardLike(ctx, queries, addLikeParams)
		if likeErr != nil {
			log.Printf("ERROR: LikeHandler: Failed standard like: User=%d -> User=%d, Error=%v", likerUserID, req.LikedUserID, likeErr)
			status := http.StatusInternalServerError
			msg := "Failed to process like"
			if errors.Is(likeErr, ErrLikeLimitReached) {
				status = http.StatusForbidden
				msg = likeErr.Error()
			} else if errors.Is(likeErr, ErrLikeAlreadyExists) {
				status = http.StatusConflict
				msg = likeErr.Error()
			}
			utils.RespondWithJSON(w, status, LikeResponse{Success: false, Message: msg})
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Liked successfully"})
	}
}

var ErrInsufficientConsumables = errors.New("insufficient consumables")
var ErrLikeLimitReached = errors.New("daily like limit reached")
var ErrLikeAlreadyExists = errors.New("you have already liked this specific item")

func handleRoseLike(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, params migrations.AddContentLikeParams) error {
	consumable, err := queries.GetUserConsumable(ctx, migrations.GetUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: handleRoseLike: No roses found for user %d", params.LikerUserID)
			return ErrInsufficientConsumables
		}
		log.Printf("ERROR: handleRoseLike: DB error checking rose balance for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("db error checking balance: %w", err)
	}
	if consumable.Quantity <= 0 {
		log.Printf("WARN: handleRoseLike: Insufficient roses (0) for user %d", params.LikerUserID)
		return ErrInsufficientConsumables
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("ERROR: handleRoseLike: Failed to begin transaction for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("begin transaction error: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)

	_, err = qtx.DecrementUserConsumable(ctx, migrations.DecrementUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: handleRoseLike: Failed to decrement rose for user %d (likely insufficient quantity): %v", params.LikerUserID, err)
			return ErrInsufficientConsumables
		}
		log.Printf("ERROR: handleRoseLike: Failed to use rose for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("failed to use rose: %w", err)
	}
	log.Printf("DEBUG: handleRoseLike: Rose decremented successfully for user %d", params.LikerUserID)

	_, err = qtx.AddContentLike(ctx, params)
	// Because of "ON CONFLICT DO NOTHING", AddContentLike might not return an error on conflict,
	// but it also might not return the row (check sqlc docs for RETURNING behavior with DO NOTHING).
	// If it *does* return pgx.ErrNoRows on conflict when RETURNING is used, handle it.
	// If it returns successfully with no row on conflict, we can't easily detect the conflict here.
	// The current AddContentLike returns the row, so pgx.ErrNoRows IS likely returned on conflict.
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: handleRoseLike: Like for item already exists (Conflict Detected): User=%d -> User=%d, Type=%s, ID=%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			// Rollback the transaction because the rose was decremented but the like wasn't new.
			return ErrLikeAlreadyExists
		}
		log.Printf("ERROR: handleRoseLike: Failed to record like for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("failed to record like: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("ERROR: handleRoseLike: Failed to commit transaction for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("commit transaction error: %w", err)
	}

	log.Printf("INFO: Rose like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return nil
}

func handleStandardLike(ctx context.Context, queries *migrations.Queries, params migrations.AddContentLikeParams) error {
	_, err := queries.GetActiveSubscription(ctx, migrations.GetActiveSubscriptionParams{UserID: params.LikerUserID, FeatureType: migrations.PremiumFeatureTypeUnlimitedLikes})
	hasUnlimitedLikes := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("ERROR: handleStandardLike: DB error checking subscription for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("db error checking subscription: %w", err)
	}

	if !hasUnlimitedLikes {
		log.Printf("INFO: handleStandardLike: Checking daily like limit for User=%d", params.LikerUserID)
		count, countErr := queries.CountRecentStandardLikes(ctx, params.LikerUserID)
		if countErr != nil {
			log.Printf("ERROR: handleStandardLike: DB error counting likes for user %d: %v", params.LikerUserID, countErr)
			return fmt.Errorf("db error counting likes: %w", countErr)
		}
		if count >= dailyStandardLikeLimit {
			log.Printf("WARN: handleStandardLike: Daily like limit reached for User=%d (%d/%d)", params.LikerUserID, count, dailyStandardLikeLimit)
			return ErrLikeLimitReached
		}
		log.Printf("INFO: handleStandardLike: Processing standard like (limited %d/%d): User=%d -> User=%d", count, dailyStandardLikeLimit, params.LikerUserID, params.LikedUserID)
	} else {
		log.Printf("INFO: handleStandardLike: Processing standard like (unlimited): User=%d -> User=%d", params.LikerUserID, params.LikedUserID)
	}

	_, err = queries.AddContentLike(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: handleStandardLike: Like for item already exists (Conflict Detected): User=%d -> User=%d, Type=%s, ID=%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			return ErrLikeAlreadyExists
		}
		log.Printf("ERROR: handleStandardLike: Failed to record like for user %d: %v", params.LikerUserID, err)
		return fmt.Errorf("failed to record like: %w", err)
	}

	log.Printf("INFO: Standard like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return nil
}

func validateContentInput(ctx context.Context, queries *migrations.Queries, likedUserID int32, contentType migrations.ContentLikeType, contentIdentifier string) (bool, error) {
	log.Printf("DEBUG: Validating content: User=%d, Type=%s, Identifier=%s", likedUserID, contentType, contentIdentifier)
	likedUser, err := queries.GetUserByID(ctx, likedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch liked user %d: %w", likedUserID, err)
	}

	switch contentType {
	case migrations.ContentLikeTypeMedia:
		index, err := strconv.Atoi(contentIdentifier)
		if err != nil {
			log.Printf("WARN: Invalid media index '%s' for user %d: %v", contentIdentifier, likedUserID, err)
			return false, nil
		}
		isValidIndex := index >= 0 && index < len(likedUser.MediaUrls)
		if !isValidIndex {
			log.Printf("WARN: Media index %d out of bounds for user %d (has %d media)", index, likedUserID, len(likedUser.MediaUrls))
		}
		return isValidIndex, nil
	case migrations.ContentLikeTypeAudioPrompt:
		isValid := contentIdentifier == "0" && likedUser.AudioPromptQuestion.Valid && likedUser.AudioPromptAnswer.Valid && likedUser.AudioPromptAnswer.String != ""
		if !isValid {
			log.Printf("WARN: Invalid audio prompt like for user %d: Identifier='%s', AudioQuestionValid=%t, AudioAnswerValid=%t", likedUserID, contentIdentifier, likedUser.AudioPromptQuestion.Valid, likedUser.AudioPromptAnswer.Valid)
		}
		return isValid, nil
	case migrations.ContentLikeTypePromptStory:
		prompts, err := queries.GetUserStoryTimePrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, fmt.Errorf("db error fetching story prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: Story prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptMytype:
		prompts, err := queries.GetUserMyTypePrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, fmt.Errorf("db error fetching mytype prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: MyType prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptGettingpersonal:
		prompts, err := queries.GetUserGettingPersonalPrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, fmt.Errorf("db error fetching gettingpersonal prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: GettingPersonal prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptDatevibes:
		prompts, err := queries.GetUserDateVibesPrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, fmt.Errorf("db error fetching datevibes prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: DateVibes prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	default:
		log.Printf("ERROR: Unknown content type in validation: %s", contentType)
		return false, fmt.Errorf("unknown content_type for validation: %s", contentType)
	}
}
