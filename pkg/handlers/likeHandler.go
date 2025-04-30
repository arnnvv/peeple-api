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
	"github.com/arnnvv/peeple-api/pkg/ws" // *** ADDED: Import ws package ***
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const dailyStandardLikeLimit = 15
const maxCommentLength = 140
const profileLikeIdentifier = "profile"

type ContentLikeRequest struct {
	LikedUserID       int32   `json:"liked_user_id"`
	ContentType       string  `json:"content_type"`
	ContentIdentifier string  `json:"content_identifier"`
	Comment           *string `json:"comment,omitempty"`
	InteractionType   *string `json:"interaction_type,omitempty"`
}

type LikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// *** ADDED: Dependency Injection for Hub ***
func LikeHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		queries, _ := db.GetDB()
		pool, _ := db.GetPool()

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
			if errors.Is(err, pgx.ErrNoRows) {
				utils.RespondWithJSON(w, http.StatusNotFound, LikeResponse{Success: false, Message: "Liker user not found"})
			} else {
				utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error fetching liker user data"})
			}
			return
		}

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
		if req.ContentType == "" {
			utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "content_type is required"})
			return
		}
		if req.ContentIdentifier == "" {
			utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "content_identifier is required"})
			return
		}

		var contentTypeEnum migrations.ContentLikeType
		err = contentTypeEnum.Scan(req.ContentType)
		if err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: fmt.Sprintf("Invalid content_type specified: %s", req.ContentType)})
			return
		}

		// --- Check if Liked User Exists ---
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				utils.RespondWithJSON(w, http.StatusNotFound, LikeResponse{Success: false, Message: "User you tried to like does not exist."})
			} else {
				log.Printf("ERROR: LikeHandler: Failed to fetch liked user %d: %v", req.LikedUserID, err)
				utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error checking liked user."})
			}
			return
		}
		// --- End Liked User Check ---

		// --- Check for reverse like (B liked A?) ---
		reverseLikeExists := false
		reverseLikeParams := migrations.CheckLikeExistsParams{
			LikerUserID: req.LikedUserID, // User B
			LikedUserID: likerUserID,     // User A
		}
		existsResult, checkErr := queries.CheckLikeExists(ctx, reverseLikeParams)
		if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
			log.Printf("WARN: LikeHandler: Failed to check for existing reverse like (user %d -> %d): %v", req.LikedUserID, likerUserID, checkErr)
			// Continue, but log the warning
		} else if checkErr == nil {
			reverseLikeExists = existsResult
		}
		// --- End Reverse Like Check ---

		// --- Content Validation ---
		var contentValid = false
		var validationErr error
		isProfileLikeAttempt := contentTypeEnum == migrations.ContentLikeTypeProfile && req.ContentIdentifier == profileLikeIdentifier

		if isProfileLikeAttempt {
			// Profile like back IS allowed only if the other user liked first
			if !reverseLikeExists {
				log.Printf("WARN: LikeHandler: User %d attempted 'profile' like on user %d, but no reverse like exists. Forbidden.", likerUserID, req.LikedUserID)
				utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Generic profile like is only allowed when liking someone back."})
				return
			}
			log.Printf("INFO: LikeHandler: Processing 'profile' like back: User %d -> User %d.", likerUserID, req.LikedUserID)
			contentValid = true // Valid scenario for profile like
		} else {
			// Specific content like validation
			log.Printf("INFO: LikeHandler: Processing specific content like: User %d -> User %d, Type=%s, ID=%s", likerUserID, req.LikedUserID, contentTypeEnum, req.ContentIdentifier)
			contentValid, validationErr = validateContentInput(ctx, queries, req.LikedUserID, contentTypeEnum, req.ContentIdentifier)
			if validationErr != nil {
				log.Printf("ERROR: LikeHandler: Error validating specific content input for user %d liking %d: %v", likerUserID, req.LikedUserID, validationErr)
				utils.RespondWithJSON(w, http.StatusInternalServerError, LikeResponse{Success: false, Message: "Error validating content"})
				return
			}
			if !contentValid {
				log.Printf("WARN: LikeHandler: Specific content invalid/not found for user %d liking %d: Type=%s, Identifier=%s", likerUserID, req.LikedUserID, contentTypeEnum, req.ContentIdentifier)
				utils.RespondWithJSON(w, http.StatusNotFound, LikeResponse{Success: false, Message: "The specific content you tried to like does not exist or is invalid."})
				return
			}
			log.Printf("INFO: Specific content validation passed for like request: User=%d -> User=%d", likerUserID, req.LikedUserID)
		}
		// --- End Content Validation ---

		// --- Comment Validation ---
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
		// --- End Comment Validation ---

		// --- Comment Requirement Check ---
		// Comment is required if:
		// 1. It's NOT a profile like attempt (i.e., specific content like)
		// 2. It's NOT a mutual like situation (the other user hasn't liked back yet)
		// 3. The liker is male
		// 4. No comment was provided
		commentRequired := !isProfileLikeAttempt && !reverseLikeExists && !commentProvided &&
			likerUser.Gender.Valid && likerUser.Gender.GenderEnum == migrations.GenderEnumMan

		if commentRequired {
			log.Printf("WARN: LikeHandler: Comment required for initial specific content like from male user %d to %d", likerUserID, req.LikedUserID)
			utils.RespondWithJSON(w, http.StatusBadRequest, LikeResponse{Success: false, Message: "Comment is required when sending an initial like on specific content"})
			return
		}
		// --- End Comment Requirement Check ---

		// --- Determine Interaction Type ---
		interactionType := migrations.LikeInteractionTypeStandard
		if req.InteractionType != nil && strings.ToLower(*req.InteractionType) == string(migrations.LikeInteractionTypeRose) {
			interactionType = migrations.LikeInteractionTypeRose
		}
		// --- End Interaction Type ---

		log.Printf("INFO: Like attempt: User=%d -> User=%d, Type=%s, Content=%s:%s, CommentPresent=%t, IsProfileLike=%t, Interaction=%s",
			likerUserID, req.LikedUserID, contentTypeEnum, req.ContentIdentifier, commentProvided, isProfileLikeAttempt, interactionType)

		// --- Prepare Like Parameters ---
		addLikeParams := migrations.AddContentLikeParams{
			LikerUserID:       likerUserID,
			LikedUserID:       req.LikedUserID,
			ContentType:       contentTypeEnum,
			ContentIdentifier: req.ContentIdentifier,
			Comment:           pgtype.Text{String: commentText, Valid: commentProvided},
			InteractionType:   interactionType,
		}
		// --- End Prepare Like Parameters ---

		// --- Execute Like Logic (Standard or Rose) ---
		var likeErr error
		var savedLike migrations.Like // Variable to store the result of AddContentLike

		if interactionType == migrations.LikeInteractionTypeRose {
			savedLike, likeErr = handleRoseLikeAndGet(ctx, queries, pool, addLikeParams) // Modified function
			if likeErr != nil {
				// Handle errors (InsufficientConsumables, LikeAlreadyExists, DB error)
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
			// Rose sent successfully, proceed to check for match/notification
		} else {
			savedLike, likeErr = handleStandardLikeAndGet(ctx, queries, addLikeParams) // Modified function
			if likeErr != nil {
				// Handle errors (LikeLimitReached, LikeAlreadyExists, DB error)
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
			// Standard like added successfully, proceed to check for match/notification
		}
		// --- End Execute Like Logic ---

		// --- Check for Match and Send Notifications (AFTER successful like) ---
		isNowMutualLike, checkErr := queries.CheckMutualLikeExists(ctx, migrations.CheckMutualLikeExistsParams{
			LikerUserID: likerUserID,
			LikedUserID: req.LikedUserID,
		})
		if checkErr != nil {
			log.Printf("WARN: LikeHandler: Failed to check for mutual like after like from %d to %d: %v", likerUserID, req.LikedUserID, checkErr)
			// Don't fail the request, but log that notifications might be missed
		} else {
			// Use goroutine for notifications to avoid blocking the HTTP response
			go func(
				ctx context.Context, // Pass context if needed by fetch functions
				q *migrations.Queries,
				h *ws.Hub,
				isMatch bool,
				likerID int32,
				likedID int32,
				likeData migrations.Like, // Pass the saved like data
			) {
				if isMatch {
					// --- Handle Match ---
					log.Printf("LikeHandler INFO: Match occurred between %d and %d!", likerID, likedID)

					// Fetch info for User A (Liker)
					basicInfoA, errA := q.GetBasicMatchInfo(ctx, likerID) // NEW QUERY NEEDED
					if errA != nil {
						log.Printf("LikeHandler ERROR: Failed to get match info for user %d: %v", likerID, errA)
						// Continue, but notifications might be incomplete
					}

					// Fetch info for User B (Liked)
					basicInfoB, errB := q.GetBasicMatchInfo(ctx, likedID) // NEW QUERY NEEDED
					if errB != nil {
						log.Printf("LikeHandler ERROR: Failed to get match info for user %d: %v", likedID, errB)
						// Continue, but notifications might be incomplete
					}

					// Notify User A about the match with User B
					if errA == nil && errB == nil {
						matchInfoForA := ws.WsMatchInfo{
							MatchedUserID:      likedID,
							Name:               buildFullName(basicInfoB.Name, basicInfoB.LastName),
							FirstProfilePicURL: getFirstMediaURL(basicInfoB.MediaUrls),
							IsOnline:           basicInfoB.IsOnline,
							LastOnline:         pgTimestampToTimePtr(basicInfoB.LastOnline),
							// User A needs to remove the like notification FROM User B
							InitiatingLikerUserID: likedID,
						}
						h.BroadcastNewMatch(likerID, matchInfoForA)
					}

					// Notify User B about the match with User A
					if errA == nil && errB == nil {
						matchInfoForB := ws.WsMatchInfo{
							MatchedUserID:      likerID,
							Name:               buildFullName(basicInfoA.Name, basicInfoA.LastName),
							FirstProfilePicURL: getFirstMediaURL(basicInfoA.MediaUrls),
							IsOnline:           basicInfoA.IsOnline,
							LastOnline:         pgTimestampToTimePtr(basicInfoA.LastOnline),
							// User B needs to remove the like notification FROM User A (the current liker)
							InitiatingLikerUserID: likerID,
						}
						h.BroadcastNewMatch(likedID, matchInfoForB)
					}
					// --- End Handle Match ---

				} else {
					// --- Handle New Like (No Match) ---
					log.Printf("LikeHandler INFO: New like (no match) from %d to %d.", likerID, likedID)

					// Fetch basic info for the liker (User A)
					basicInfoLiker, errLiker := q.GetBasicUserInfo(ctx, likerID) // NEW QUERY NEEDED
					if errLiker != nil {
						log.Printf("LikeHandler ERROR: Failed to get basic info for liker %d for WS notification: %v", likerID, errLiker)
						return // Cannot send notification without liker info
					}

					// Prepare payload for the liked user (User B)
					var commentPtr *string
					if likeData.Comment.Valid {
						commentPtr = &likeData.Comment.String
					}
					likerInfoPayload := ws.WsBasicLikerInfo{
						LikerUserID:        likerID,
						Name:               buildFullName(basicInfoLiker.Name, basicInfoLiker.LastName),
						FirstProfilePicURL: getFirstMediaURL(basicInfoLiker.MediaUrls),
						IsRose:             likeData.InteractionType == migrations.LikeInteractionTypeRose,
						LikeComment:        commentPtr,
						LikedAt:            likeData.CreatedAt, // Use the timestamp from the saved like
					}

					// Send notification to the liked user (User B)
					h.BroadcastNewLike(likedID, likerInfoPayload)
					// --- End Handle New Like ---
				}
			}(context.Background(), queries, hub, isNowMutualLike.Bool, likerUserID, req.LikedUserID, savedLike) // Pass necessary vars
		}
		// --- End Check for Match and Notifications ---

		// Respond HTTP Success
		message := "Liked successfully"
		if interactionType == migrations.LikeInteractionTypeRose {
			message = "Rose sent successfully"
		}
		utils.RespondWithJSON(w, http.StatusOK, LikeResponse{Success: true, Message: message})

	} // End of outer function brace
}

var ErrInsufficientConsumables = errors.New("insufficient consumables (e.g., roses)")
var ErrLikeLimitReached = errors.New("daily like limit reached")
var ErrLikeAlreadyExists = errors.New("you have already liked this specific item")

// --- MODIFIED: handleRoseLikeAndGet ---
// Returns the saved Like object on success, or error
func handleRoseLikeAndGet(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, params migrations.AddContentLikeParams) (migrations.Like, error) {
	// Check balance
	consumable, err := queries.GetUserConsumable(ctx, migrations.GetUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return migrations.Like{}, ErrInsufficientConsumables
		}
		return migrations.Like{}, fmt.Errorf("db error checking balance: %w", err)
	}
	if consumable.Quantity <= 0 {
		return migrations.Like{}, ErrInsufficientConsumables
	}

	// Start transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return migrations.Like{}, fmt.Errorf("begin transaction error: %w", err)
	}
	defer tx.Rollback(ctx) // Ensure rollback on error

	qtx := queries.WithTx(tx)

	// Decrement consumable
	_, err = qtx.DecrementUserConsumable(ctx, migrations.DecrementUserConsumableParams{UserID: params.LikerUserID, ConsumableType: migrations.PremiumFeatureTypeRose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // Double-check quantity during tx
			return migrations.Like{}, ErrInsufficientConsumables
		}
		return migrations.Like{}, fmt.Errorf("failed to use rose: %w", err)
	}

	// Add the like
	savedLike, err := qtx.AddContentLike(ctx, params)
	if err != nil {
		// Check if the error is because the like already exists (ON CONFLICT DO NOTHING returns no rows)
		// Note: pgx might return ErrNoRows here if the ON CONFLICT clause was triggered.
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("handleRoseLikeAndGet WARN: Like already exists (conflict detected) for %d -> %d (%s:%s)", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			return migrations.Like{}, ErrLikeAlreadyExists
		}
		log.Printf("handleRoseLikeAndGet ERROR: Failed to record like for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("failed to record like: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return migrations.Like{}, fmt.Errorf("commit transaction error: %w", err)
	}

	log.Printf("INFO: Rose like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return savedLike, nil // Return the saved like object
}

// --- MODIFIED: handleStandardLikeAndGet ---
// Returns the saved Like object on success, or error
func handleStandardLikeAndGet(ctx context.Context, queries *migrations.Queries, params migrations.AddContentLikeParams) (migrations.Like, error) {
	// Check for unlimited likes subscription
	_, err := queries.GetActiveSubscription(ctx, migrations.GetActiveSubscriptionParams{UserID: params.LikerUserID, FeatureType: migrations.PremiumFeatureTypeUnlimitedLikes})
	hasUnlimitedLikes := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("handleStandardLikeAndGet ERROR: DB error checking subscription for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("db error checking subscription: %w", err)
	}

	// Check daily limit if no subscription
	if !hasUnlimitedLikes {
		count, countErr := queries.CountRecentStandardLikes(ctx, params.LikerUserID)
		if countErr != nil {
			log.Printf("handleStandardLikeAndGet ERROR: DB error counting likes for user %d: %v", params.LikerUserID, countErr)
			return migrations.Like{}, fmt.Errorf("db error counting likes: %w", countErr)
		}
		if count >= dailyStandardLikeLimit {
			return migrations.Like{}, ErrLikeLimitReached
		}
	}

	// Add the like
	savedLike, err := queries.AddContentLike(ctx, params)
	if err != nil {
		// Check if the error is because the like already exists
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("handleStandardLikeAndGet WARN: Like already exists (conflict detected) for %d -> %d (%s:%s)", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			return migrations.Like{}, ErrLikeAlreadyExists
		}
		log.Printf("handleStandardLikeAndGet ERROR: Failed to record like for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("failed to record like: %w", err)
	}

	log.Printf("INFO: Standard like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return savedLike, nil // Return the saved like object
}

// --- validateContentInput (Keep as is) ---
func validateContentInput(ctx context.Context, queries *migrations.Queries, likedUserID int32, contentType migrations.ContentLikeType, contentIdentifier string) (bool, error) {

	log.Printf("DEBUG: Validating specific content: User=%d, Type=%s, Identifier=%s", likedUserID, contentType, contentIdentifier)

	likedUser, err := queries.GetUserByID(ctx, likedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: validateContentInput: Liked user %d not found.", likedUserID)
			return false, nil // Content is invalid if user doesn't exist
		}
		log.Printf("ERROR: validateContentInput: Failed to fetch liked user %d: %v", likedUserID, err)
		return false, fmt.Errorf("failed to fetch liked user %d: %w", likedUserID, err)
	}

	switch contentType {
	case migrations.ContentLikeTypeMedia:
		index, err := strconv.Atoi(contentIdentifier)
		if err != nil {
			log.Printf("WARN: validateContentInput: Invalid media index '%s' for user %d: %v", contentIdentifier, likedUserID, err)
			return false, nil
		}
		isValidIndex := index >= 0 && index < len(likedUser.MediaUrls)
		if !isValidIndex {
			log.Printf("WARN: validateContentInput: Media index %d out of bounds for user %d (has %d media)", index, likedUserID, len(likedUser.MediaUrls))
		}
		return isValidIndex, nil

	case migrations.ContentLikeTypeAudioPrompt:
		isValid := contentIdentifier == "0" && // Audio identifier is always "0" for now
			likedUser.AudioPromptQuestion.Valid &&
			likedUser.AudioPromptAnswer.Valid && likedUser.AudioPromptAnswer.String != ""
		if !isValid {
			log.Printf("WARN: validateContentInput: Invalid audio prompt like for user %d: Identifier='%s', AudioQuestionValid=%t, AudioAnswerValid=%t", likedUserID, contentIdentifier, likedUser.AudioPromptQuestion.Valid, likedUser.AudioPromptAnswer.Valid)
		}
		return isValid, nil

	case migrations.ContentLikeTypePromptStory:
		prompts, err := queries.GetUserStoryTimePrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil // No prompts, so specific one doesn't exist
			}
			log.Printf("ERROR: validateContentInput: DB error fetching story prompts for user %d: %v", likedUserID, err)
			return false, fmt.Errorf("db error fetching story prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil // Found the specific prompt question
			}
		}
		log.Printf("WARN: validateContentInput: Story prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil // Prompt question not found

	// --- Add similar cases for other prompt types ---
	case migrations.ContentLikeTypePromptMytype:
		prompts, err := queries.GetUserMyTypePrompts(ctx, likedUserID)
		if err != nil { // Handle DB errors
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			log.Printf("ERROR: validateContentInput: DB error fetching mytype prompts for user %d: %v", likedUserID, err)
			return false, fmt.Errorf("db error fetching mytype prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: validateContentInput: MyType prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptGettingpersonal:
		prompts, err := queries.GetUserGettingPersonalPrompts(ctx, likedUserID)
		if err != nil { // Handle DB errors
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			log.Printf("ERROR: validateContentInput: DB error fetching gettingpersonal prompts for user %d: %v", likedUserID, err)
			return false, fmt.Errorf("db error fetching gettingpersonal prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: validateContentInput: GettingPersonal prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptDatevibes:
		prompts, err := queries.GetUserDateVibesPrompts(ctx, likedUserID)
		if err != nil { // Handle DB errors
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			log.Printf("ERROR: validateContentInput: DB error fetching datevibes prompts for user %d: %v", likedUserID, err)
			return false, fmt.Errorf("db error fetching datevibes prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: validateContentInput: DateVibes prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	// --- End prompt cases ---

	case migrations.ContentLikeTypeProfile:
		// This case should have been handled earlier, but defensive check:
		log.Printf("ERROR: validateContentInput: Reached 'profile' type validation unexpectedly. This should be handled earlier.")
		return false, fmt.Errorf("internal error: profile type should not be validated here")

	default:
		log.Printf("ERROR: validateContentInput: Unknown content type encountered: %s", contentType)
		return false, fmt.Errorf("unknown content_type for validation: %s", contentType)
	}
}
