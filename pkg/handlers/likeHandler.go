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
	"github.com/jackc/pgx/v5" // Import pgx specifically for pgx.ErrNoRows
	// "github.com/jackc/pgx/v5/pgconn" // Not needed if only checking ErrNoRows
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dailyStandardLikeLimit = 15
const maxCommentLength = 140

// ContentLikeRequest defined...
type ContentLikeRequest struct {
	LikedUserID       int32                      `json:"liked_user_id"`
	ContentType       migrations.ContentLikeType `json:"content_type"`
	ContentIdentifier string                     `json:"content_identifier"`
	Comment           *string                    `json:"comment,omitempty"`
	InteractionType   *string                    `json:"interaction_type,omitempty"`
}

// LikeResponse defined...
type LikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// LikeHandler handles liking specific content items
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

	likerUser, err := queries.GetUserByID(ctx, likerUserID)
	if err != nil {
		log.Printf("ERROR: LikeHandler: Failed to fetch liker user %d: %v", likerUserID, err)
		utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error fetching user data"})
		return
	}

	var req ContentLikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("ERROR: LikeHandler: Invalid request body for user %d: %v", likerUserID, err)
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Invalid request body format"})
		return
	}
	defer r.Body.Close()

	// --- Basic Input Validation ---
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

	// --- Comment Validation ---
	commentText := ""
	commentProvided := false
	if req.Comment != nil {
		commentText = strings.TrimSpace(*req.Comment)
		commentProvided = true
		if utf8.RuneCountInString(commentText) > maxCommentLength {
			utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: fmt.Sprintf("Comment exceeds maximum length of %d characters", maxCommentLength)})
			return
		}
	}
	if likerUser.Gender.Valid && likerUser.Gender.GenderEnum == migrations.GenderEnumMan && !commentProvided {
		utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Comment is required when sending a like"})
		return
	}

	// --- Content Existence/Index Validation (Go Code) ---
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

	// --- Interaction Type & Premium Checks ---
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

	// Execute based on type
	if interactionType == migrations.LikeInteractionTypeRose {
		err = handleRoseLike(ctx, queries, pool, addLikeParams) // Pass pool
		if err != nil {
			log.Printf("ERROR: LikeHandler: Failed rose like: User=%d -> User=%d, Error=%v", likerUserID, req.LikedUserID, err)
			status := http.StatusInternalServerError
			msg := "Failed to process rose like"
			if errors.Is(err, ErrInsufficientConsumables) {
				status = http.StatusForbidden
				msg = err.Error()
			}
			utils.RespondWithJSON(w, status, LikeResponse{Success: false, Message: msg})
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Rose sent successfully"})
	} else {
		err = handleStandardLike(ctx, queries, addLikeParams)
		if err != nil {
			log.Printf("ERROR: LikeHandler: Failed standard like: User=%d -> User=%d, Error=%v", likerUserID, req.LikedUserID, err)
			status := http.StatusInternalServerError
			msg := "Failed to process like"
			if errors.Is(err, ErrLikeLimitReached) {
				status = http.StatusForbidden
				msg = err.Error()
			}
			utils.RespondWithJSON(w, status, LikeResponse{Success: false, Message: msg})
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Liked successfully"})
	}
}

var ErrInsufficientConsumables = errors.New("insufficient consumables")
var ErrLikeLimitReached = errors.New("daily like limit reached")

// handleRoseLike checks/uses a rose and inserts the like record within a transaction.
func handleRoseLike(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, params migrations.AddContentLikeParams) error {
	consumable, err := queries.GetUserConsumable(ctx, migrations.GetUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientConsumables
		}
		return fmt.Errorf("db error checking balance: %w", err)
	}
	if consumable.Quantity <= 0 {
		return ErrInsufficientConsumables
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction error: %w", err)
	}
	defer tx.Rollback(ctx)
	qtx := queries.WithTx(tx)

	_, err = qtx.DecrementUserConsumable(ctx, migrations.DecrementUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientConsumables
		}
		return fmt.Errorf("failed to use rose: %w", err)
	}

	// *** Call AddContentLike and handle pgx.ErrNoRows for conflict ***
	_, err = qtx.AddContentLike(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Check for no rows when conflict happens
			log.Printf("WARN: handleRoseLike: Like for item already exists (Conflict Triggered): User=%d -> User=%d, Type=%s, ID=%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			// Conflict is not an application error in this case, treat as success
			err = nil // Clear error
		} else {
			// Other DB errors
			return fmt.Errorf("failed to record like: %w", err)
		}
	}
	// If err is nil (insert succeeded or conflict handled), proceed to commit

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction error: %w", err)
	}
	log.Printf("INFO: Rose like processed: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return nil
}

// handleStandardLike checks limits/subscriptions and inserts the like record.
func handleStandardLike(ctx context.Context, queries *migrations.Queries, params migrations.AddContentLikeParams) error {
	_, err := queries.GetActiveSubscription(ctx, migrations.GetActiveSubscriptionParams{UserID: params.LikerUserID, FeatureType: migrations.PremiumFeatureTypeUnlimitedLikes})
	hasUnlimitedLikes := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("db error checking subscription: %w", err)
	}

	if hasUnlimitedLikes {
		log.Printf("INFO: Processing standard like (unlimited): User=%d -> User=%d", params.LikerUserID, params.LikedUserID)
		// *** Call AddContentLike and handle pgx.ErrNoRows for conflict ***
		_, err = queries.AddContentLike(ctx, params)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) { // Check for no rows when conflict happens
				log.Printf("WARN: handleStandardLike (unlimited): Like for item already exists (Conflict Triggered): User=%d -> User=%d, Type=%s, ID=%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
				err = nil // Clear error
			} else {
				return fmt.Errorf("failed to record like: %w", err)
			}
		}
		// If err is nil (insert succeeded or conflict handled), log success
		if err == nil {
			log.Printf("INFO: Standard like (unlimited) processed: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
		}
		return err // Return nil if successful or conflict occurred, or the actual error otherwise
	}

	log.Printf("INFO: Checking daily like limit for User=%d", params.LikerUserID)
	count, err := queries.CountRecentStandardLikes(ctx, params.LikerUserID)
	if err != nil {
		return fmt.Errorf("db error counting likes: %w", err)
	}
	if count >= dailyStandardLikeLimit {
		log.Printf("WARN: Daily like limit reached for User=%d (%d/%d)", params.LikerUserID, count, dailyStandardLikeLimit)
		return ErrLikeLimitReached
	}

	log.Printf("INFO: Processing standard like (limited %d/%d): User=%d -> User=%d", count, dailyStandardLikeLimit, params.LikerUserID, params.LikedUserID)
	// *** Call AddContentLike and handle pgx.ErrNoRows for conflict ***
	_, err = queries.AddContentLike(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Check for no rows when conflict happens
			log.Printf("WARN: handleStandardLike (limited): Like for item already exists (Conflict Triggered): User=%d -> User=%d, Type=%s, ID=%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			err = nil // Clear error
		} else {
			return fmt.Errorf("failed to record like: %w", err)
		}
	}
	// If err is nil (insert succeeded or conflict handled), log success
	if err == nil {
		log.Printf("INFO: Standard like (limited) processed: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	}
	return err // Return nil if successful or conflict occurred, or the actual error otherwise
}

// validateContentInput defined...
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
			log.Printf("WARN: Invalid audio prompt like for user %d: Identifier='%s', AudioValid=%t", likedUserID, contentIdentifier, likedUser.AudioPromptQuestion.Valid)
		}
		return isValid, nil
	case migrations.ContentLikeTypePromptStory:
		prompts, err := queries.GetUserStoryTimePrompts(ctx, likedUserID)
		if err != nil {
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
