package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dailyStandardLikeLimit = 15 // Define your daily limit

type LikeRequest struct {
	LikedUserID     int32   `json:"liked_user_id"`
	InteractionType *string `json:"interaction_type,omitempty"` // "rose" or omit for standard
}

type LikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func LikeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := r.Context()
	queries := db.GetDB()
	pool := db.GetPool() // Needed for transactions

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

	var req LikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	// Determine interaction type
	interactionType := migrations.LikeInteractionTypeStandard
	if req.InteractionType != nil && strings.ToLower(*req.InteractionType) == string(migrations.LikeInteractionTypeRose) {
		interactionType = migrations.LikeInteractionTypeRose
	}

	log.Printf("Like attempt: User %d -> User %d (Type: %s)", likerUserID, req.LikedUserID, interactionType)

	// --- Logic ---
	if interactionType == migrations.LikeInteractionTypeRose {
		// Handle Rose interaction
		err := handleRoseLike(ctx, queries, pool, likerUserID, req.LikedUserID)
		if err != nil {
			// handleRoseLike sends the response, just log here if needed
			log.Printf("Error handling rose like for user %d -> %d: %v", likerUserID, req.LikedUserID, err)
			// Determine appropriate status code based on error type if needed, but handler does it
			if errors.Is(err, ErrInsufficientConsumables) {
				utils.RespondWithJSON(w, http.StatusForbidden, LikeResponse{Success: false, Message: err.Error()})
			} else {
				utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Failed to process rose like"})
			}
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Rose sent successfully"})

	} else {
		// Handle Standard Like interaction
		err := handleStandardLike(ctx, queries, likerUserID, req.LikedUserID)
		if err != nil {
			log.Printf("Error handling standard like for user %d -> %d: %v", likerUserID, req.LikedUserID, err)
			if errors.Is(err, ErrLikeLimitReached) {
				utils.RespondWithJSON(w, http.StatusForbidden, LikeResponse{Success: false, Message: err.Error()})
			} else {
				utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Failed to process like"})
			}
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: "Liked successfully"})
	}
}

var ErrInsufficientConsumables = errors.New("insufficient consumables")
var ErrLikeLimitReached = errors.New("daily like limit reached")

func handleRoseLike(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, likerUserID, likedUserID int32) error {
	// 1. Check consumable balance (outside transaction is fine for a read)
	consumable, err := queries.GetUserConsumable(ctx, migrations.GetUserConsumableParams{
		UserID:         likerUserID,
		ConsumableType: migrations.PremiumFeatureTypeRose,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("User %d has no rose consumables record.", likerUserID)
			return ErrInsufficientConsumables // No record means 0 balance
		}
		log.Printf("Error checking rose balance for user %d: %v", likerUserID, err)
		return errors.New("database error checking balance") // Generic internal error
	}

	if consumable.Quantity <= 0 {
		log.Printf("User %d has insufficient roses (%d).", likerUserID, consumable.Quantity)
		return ErrInsufficientConsumables
	}

	// 2. Start Transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("Failed to begin transaction for rose like (User %d): %v", likerUserID, err)
		return errors.New("database transaction error")
	}
	defer tx.Rollback(ctx) // Rollback if commit fails or panics

	qtx := queries.WithTx(tx)

	// 3. Decrement Consumable
	_, err = qtx.DecrementUserConsumable(ctx, migrations.DecrementUserConsumableParams{
		UserID:         likerUserID,
		ConsumableType: migrations.PremiumFeatureTypeRose,
	})
	if err != nil {
		// This could happen if balance changed between check and update, or DB error
		log.Printf("Failed to decrement rose consumable for user %d: %v", likerUserID, err)
		// Could check if it was sql.ErrNoRows which implies quantity became 0 concurrently
		if errors.Is(err, pgx.ErrNoRows) { // Check if RETURNING * failed because WHERE quantity > 0 failed
			return ErrInsufficientConsumables
		}
		return errors.New("failed to use rose")
	}

	// 4. Add Like record
	err = qtx.AddLike(ctx, migrations.AddLikeParams{
		LikerUserID:     likerUserID,
		LikedUserID:     likedUserID,
		InteractionType: migrations.LikeInteractionTypeRose,
	})
	if err != nil {
		log.Printf("Failed to add rose like record for user %d -> %d: %v", likerUserID, likedUserID, err)
		return errors.New("failed to record like")
	}

	// 5. Commit Transaction
	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("Failed to commit transaction for rose like (User %d): %v", likerUserID, err)
		return errors.New("database commit error")
	}

	log.Printf("Rose like processed successfully: User %d -> User %d", likerUserID, likedUserID)
	return nil // Success
}

func handleStandardLike(ctx context.Context, queries *migrations.Queries, likerUserID, likedUserID int32) error {
	// 1. Check for unlimited likes subscription
	_, err := queries.GetActiveSubscription(ctx, migrations.GetActiveSubscriptionParams{
		UserID:      likerUserID,
		FeatureType: migrations.PremiumFeatureTypeUnlimitedLikes,
	})

	hasUnlimitedLikes := err == nil // If no error, subscription exists and is active
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// An actual DB error occurred checking subscription
		log.Printf("Error checking unlimited likes subscription for user %d: %v", likerUserID, err)
		return errors.New("database error checking subscription")
	}

	if hasUnlimitedLikes {
		log.Printf("User %d has unlimited likes. Processing standard like.", likerUserID)
		// Add the like directly
		err = queries.AddLike(ctx, migrations.AddLikeParams{
			LikerUserID:     likerUserID,
			LikedUserID:     likedUserID,
			InteractionType: migrations.LikeInteractionTypeStandard,
		})
		if err != nil {
			log.Printf("Failed to add standard like record (unlimited) for user %d -> %d: %v", likerUserID, likedUserID, err)
			return errors.New("failed to record like")
		}
		log.Printf("Standard like (unlimited) processed successfully: User %d -> User %d", likerUserID, likedUserID)
		return nil // Success
	}

	// 2. Check daily limit if no unlimited subscription
	log.Printf("User %d does not have unlimited likes. Checking daily limit.", likerUserID)
	count, err := queries.CountRecentStandardLikes(ctx, likerUserID)
	if err != nil {
		log.Printf("Error counting recent likes for user %d: %v", likerUserID, err)
		return errors.New("database error counting likes")
	}

	if count >= dailyStandardLikeLimit {
		log.Printf("User %d has reached daily like limit (%d/%d).", likerUserID, count, dailyStandardLikeLimit)
		return ErrLikeLimitReached
	}

	// 3. Add the like if limit not reached
	log.Printf("User %d is within daily like limit (%d/%d). Processing standard like.", likerUserID, count, dailyStandardLikeLimit)
	err = queries.AddLike(ctx, migrations.AddLikeParams{
		LikerUserID:     likerUserID,
		LikedUserID:     likedUserID,
		InteractionType: migrations.LikeInteractionTypeStandard,
	})
	if err != nil {
		log.Printf("Failed to add standard like record (limited) for user %d -> %d: %v", likerUserID, likedUserID, err)
		return errors.New("failed to record like")
	}

	log.Printf("Standard like (limited) processed successfully: User %d -> User %d", likerUserID, likedUserID)
	return nil // Success
}
