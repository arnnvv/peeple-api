package ws

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arnnvv/peeple-api/migrations"
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

var ErrInsufficientConsumables = errors.New("insufficient consumables (e.g., roses)")
var ErrLikeLimitReached = errors.New("daily like limit reached")
var ErrLikeAlreadyExists = errors.New("you have already liked this specific item")

func ProcessLike(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, hub *Hub, likerUserID int32, req ContentLikeRequest) error {
	likerUser, err := queries.GetUserByID(ctx, likerUserID)
	if err != nil {
		log.Printf("ERROR: ProcessLike: Failed to fetch liker user %d: %v", likerUserID, err)
		return fmt.Errorf("liker user not found")
	}

	if req.LikedUserID <= 0 {
		return errors.New("valid liked_user_id is required")
	}
	if req.LikedUserID == likerUserID {
		return errors.New("cannot like yourself")
	}
	if req.ContentType == "" {
		return errors.New("content_type is required")
	}
	if req.ContentIdentifier == "" {
		return errors.New("content_identifier is required")
	}

	var contentTypeEnum migrations.ContentLikeType
	err = contentTypeEnum.Scan(req.ContentType)
	if err != nil {
		return fmt.Errorf("invalid content_type specified: %s", req.ContentType)
	}

	_, err = queries.GetUserByID(ctx, req.LikedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user you tried to like does not exist")
		}
		log.Printf("ERROR: ProcessLike: Failed to fetch liked user %d: %v", req.LikedUserID, err)
		return errors.New("error checking liked user")
	}

	reverseLikeExists := false
	reverseLikeParams := migrations.CheckLikeExistsParams{
		LikerUserID: req.LikedUserID,
		LikedUserID: likerUserID,
	}
	existsResult, checkErr := queries.CheckLikeExists(ctx, reverseLikeParams)
	if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
		log.Printf("WARN: ProcessLike: Failed check reverse like (%d -> %d): %v", req.LikedUserID, likerUserID, checkErr)
	} else if checkErr == nil {
		reverseLikeExists = existsResult
	}

	contentValid, validationErr := validateContentInput(ctx, queries, req.LikedUserID, contentTypeEnum, req.ContentIdentifier)
	if validationErr != nil {
		log.Printf("ERROR: ProcessLike: Error validating content (%d -> %d): %v", likerUserID, req.LikedUserID, validationErr)
		return errors.New("error validating content")
	}
	if !contentValid {
		isProfileLike := contentTypeEnum == migrations.ContentLikeTypeProfile && req.ContentIdentifier == profileLikeIdentifier
		if isProfileLike && reverseLikeExists {
			log.Printf("INFO: ProcessLike: Allowing 'profile' like back: User %d -> User %d.", likerUserID, req.LikedUserID)
			contentValid = true
		} else {
			message := "The specific content you tried to like does not exist or is invalid."
			if isProfileLike && !reverseLikeExists {
				message = "Generic profile like is only allowed when liking someone back."
			}
			log.Printf("WARN: ProcessLike: Content invalid/not found (%d -> %d): Type=%s, ID=%s", likerUserID, req.LikedUserID, contentTypeEnum, req.ContentIdentifier)
			return errors.New(message)
		}
	}

	commentText := ""
	commentProvided := false
	if req.Comment != nil {
		commentText = strings.TrimSpace(*req.Comment)
		if commentText != "" {
			commentProvided = true
			if utf8.RuneCountInString(commentText) > maxCommentLength {
				return fmt.Errorf("comment exceeds maximum length of %d characters", maxCommentLength)
			}
		}
	}

	isProfileLikeAttempt := contentTypeEnum == migrations.ContentLikeTypeProfile && req.ContentIdentifier == profileLikeIdentifier
	commentRequired := !isProfileLikeAttempt && !reverseLikeExists && !commentProvided &&
		likerUser.Gender.Valid && likerUser.Gender.GenderEnum == migrations.GenderEnumMan
	if commentRequired {
		log.Printf("WARN: ProcessLike: Comment required from male user %d to %d", likerUserID, req.LikedUserID)
		return errors.New("comment is required when sending an initial like on specific content")
	}

	interactionType := migrations.LikeInteractionTypeStandard
	if req.InteractionType != nil && strings.ToLower(*req.InteractionType) == string(migrations.LikeInteractionTypeRose) {
		interactionType = migrations.LikeInteractionTypeRose
	}

	addLikeParams := migrations.AddContentLikeParams{
		LikerUserID:       likerUserID,
		LikedUserID:       req.LikedUserID,
		ContentType:       contentTypeEnum,
		ContentIdentifier: req.ContentIdentifier,
		Comment:           pgtype.Text{String: commentText, Valid: commentProvided},
		InteractionType:   interactionType,
	}

	var likeErr error
	var savedLike migrations.Like
	if interactionType == migrations.LikeInteractionTypeRose {
		savedLike, likeErr = handleRoseLikeAndGet(ctx, queries, pool, addLikeParams)
	} else {
		savedLike, likeErr = handleStandardLikeAndGet(ctx, queries, addLikeParams)
	}
	if likeErr != nil {
		log.Printf("ERROR: ProcessLike: Failed %s like: User=%d -> User=%d, Error=%v", interactionType, likerUserID, req.LikedUserID, likeErr)
		return likeErr
	}

	isNowMutualLike, checkErr := queries.CheckMutualLikeExists(ctx, migrations.CheckMutualLikeExistsParams{
		LikerUserID: likerUserID,
		LikedUserID: req.LikedUserID,
	})
	if checkErr != nil {
		log.Printf("WARN: ProcessLike: Failed check mutual like after like (%d -> %d): %v", likerUserID, req.LikedUserID, checkErr)
	} else {
		go func(
			bgCtx context.Context,
			q *migrations.Queries,
			h *Hub,
			isMatch bool,
			likerID int32,
			likedID int32,
			likeData migrations.Like,
		) {
			if isMatch {
				log.Printf("ProcessLike INFO: Match occurred between %d and %d!", likerID, likedID)
				basicInfoA, errA := q.GetBasicMatchInfo(bgCtx, likerID)
				if errA != nil {
					log.Printf("ProcessLike ERROR: Failed get match info %d: %v", likerID, errA)
					return
				}
				basicInfoB, errB := q.GetBasicMatchInfo(bgCtx, likedID)
				if errB != nil {
					log.Printf("ProcessLike ERROR: Failed get match info %d: %v", likedID, errB)
					return
				}

				matchInfoForA := WsMatchInfo{
					MatchedUserID:         likedID,
					Name:                  buildFullName(basicInfoB.Name, basicInfoB.LastName),
					FirstProfilePicURL:    getFirstMediaURL(basicInfoB.MediaUrls),
					IsOnline:              basicInfoB.IsOnline,
					LastOnline:            pgTimestampToTimePtr(basicInfoB.LastOnline),
					InitiatingLikerUserID: likedID,
				}
				h.BroadcastNewMatch(likerID, matchInfoForA)

				matchInfoForB := WsMatchInfo{
					MatchedUserID:         likerID,
					Name:                  buildFullName(basicInfoA.Name, basicInfoA.LastName),
					FirstProfilePicURL:    getFirstMediaURL(basicInfoA.MediaUrls),
					IsOnline:              basicInfoA.IsOnline,
					LastOnline:            pgTimestampToTimePtr(basicInfoA.LastOnline),
					InitiatingLikerUserID: likerID,
				}
				h.BroadcastNewMatch(likedID, matchInfoForB)

			} else {
				log.Printf("ProcessLike INFO: New like (no match) from %d to %d.", likerID, likedID)
				basicInfoLiker, errLiker := q.GetBasicUserInfo(bgCtx, likerID)
				if errLiker != nil {
					log.Printf("ProcessLike ERROR: Failed get basic info %d: %v", likerID, errLiker)
					return
				}

				var commentPtr *string
				if likeData.Comment.Valid {
					commentPtr = &likeData.Comment.String
				}
				likerInfoPayload := WsBasicLikerInfo{
					LikerUserID:        likerID,
					Name:               buildFullName(basicInfoLiker.Name, basicInfoLiker.LastName),
					FirstProfilePicURL: getFirstMediaURL(basicInfoLiker.MediaUrls),
					IsRose:             likeData.InteractionType == migrations.LikeInteractionTypeRose,
					LikeComment:        commentPtr,
					LikedAt:            likeData.CreatedAt,
				}
				h.BroadcastNewLike(likedID, likerInfoPayload)
			}
		}(context.Background(), queries, hub, isNowMutualLike.Bool, likerUserID, req.LikedUserID, savedLike)
	}

	return nil
}

func handleRoseLikeAndGet(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, params migrations.AddContentLikeParams) (migrations.Like, error) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		return migrations.Like{}, fmt.Errorf("begin transaction error: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := queries.WithTx(tx)

	_, err = qtx.DecrementUserConsumable(ctx, migrations.DecrementUserConsumableParams{
		UserID:         params.LikerUserID,
		ConsumableType: migrations.PremiumFeatureTypeRose},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return migrations.Like{}, ErrInsufficientConsumables
		}
		return migrations.Like{}, fmt.Errorf("failed to use rose: %w", err)
	}

	savedLike, err := qtx.AddContentLike(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("handleRoseLikeAndGet WARN: Like already exists (conflict detected) for %d -> %d (%s:%s)", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			return migrations.Like{}, ErrLikeAlreadyExists
		}
		log.Printf("handleRoseLikeAndGet ERROR: Failed to record like for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("failed to record like: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return migrations.Like{}, fmt.Errorf("commit transaction error: %w", err)
	}

	log.Printf("INFO: Rose like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return savedLike, nil
}

func handleStandardLikeAndGet(ctx context.Context, queries *migrations.Queries, params migrations.AddContentLikeParams) (migrations.Like, error) {
	_, err := queries.GetActiveSubscription(ctx, migrations.GetActiveSubscriptionParams{
		UserID:      params.LikerUserID,
		FeatureType: migrations.PremiumFeatureTypeUnlimitedLikes},
	)
	hasUnlimitedLikes := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("handleStandardLikeAndGet ERROR: DB error checking subscription for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("db error checking subscription: %w", err)
	}

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

	savedLike, err := queries.AddContentLike(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("handleStandardLikeAndGet WARN: Like already exists (conflict detected) for %d -> %d (%s:%s)", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
			return migrations.Like{}, ErrLikeAlreadyExists
		}
		log.Printf("handleStandardLikeAndGet ERROR: Failed to record like for user %d: %v", params.LikerUserID, err)
		return migrations.Like{}, fmt.Errorf("failed to record like: %w", err)
	}

	log.Printf("INFO: Standard like processed successfully: User=%d -> User=%d, Content=%s:%s", params.LikerUserID, params.LikedUserID, params.ContentType, params.ContentIdentifier)
	return savedLike, nil
}

func validateContentInput(ctx context.Context, queries *migrations.Queries, likedUserID int32, contentType migrations.ContentLikeType, contentIdentifier string) (bool, error) {
	if contentType == migrations.ContentLikeTypeProfile && contentIdentifier == profileLikeIdentifier {
		log.Printf("DEBUG: validateContentInput: Allowing 'profile' type within function (should be pre-validated). User=%d", likedUserID)
		return true, nil
	}

	log.Printf("DEBUG: Validating specific content: User=%d, Type=%s, Identifier=%s", likedUserID, contentType, contentIdentifier)

	likedUser, err := queries.GetUserByID(ctx, likedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("WARN: validateContentInput: Liked user %d not found.", likedUserID)
			return false, nil
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
		isValid := contentIdentifier == "0" && likedUser.AudioPromptQuestion.Valid && likedUser.AudioPromptAnswer.Valid && likedUser.AudioPromptAnswer.String != ""
		if !isValid {
			log.Printf("WARN: validateContentInput: Invalid audio prompt like for user %d: Identifier='%s', AudioQuestionValid=%t, AudioAnswerValid=%t", likedUserID, contentIdentifier, likedUser.AudioPromptQuestion.Valid, likedUser.AudioPromptAnswer.Valid)
		}
		return isValid, nil
	case migrations.ContentLikeTypePromptStory:
		prompts, err := queries.GetUserStoryTimePrompts(ctx, likedUserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			log.Printf("ERROR: validateContentInput: DB error fetching story prompts for user %d: %v", likedUserID, err)
			return false, fmt.Errorf("db error fetching story prompts: %w", err)
		}
		for _, p := range prompts {
			if string(p.Question) == contentIdentifier {
				return true, nil
			}
		}
		log.Printf("WARN: validateContentInput: Story prompt '%s' not found for user %d", contentIdentifier, likedUserID)
		return false, nil
	case migrations.ContentLikeTypePromptMytype:
		prompts, err := queries.GetUserMyTypePrompts(ctx, likedUserID)
		if err != nil {
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
		if err != nil {
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
		if err != nil {
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
	default:
		log.Printf("ERROR: validateContentInput: Unknown content type encountered: %s", contentType)
		return false, fmt.Errorf("unknown content_type for validation: %s", contentType)
	}
}

func buildFullName(name, lastName pgtype.Text) string {
	var fullName strings.Builder
	if name.Valid && name.String != "" {
		fullName.WriteString(name.String)
	}
	if lastName.Valid && lastName.String != "" {
		if fullName.Len() > 0 {
			fullName.WriteString(" ")
		}
		fullName.WriteString(lastName.String)
	}
	return fullName.String()
}

func getFirstMediaURL(mediaUrls []string) string {
	if len(mediaUrls) > 0 && mediaUrls[0] != "" {
		return mediaUrls[0]
	}
	return ""
}

func pgTimestampToTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
