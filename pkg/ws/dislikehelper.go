package ws

import (
	"context"
	"errors"
	"log"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5"
)

type DislikeRequest struct {
	DislikedUserID int32 `json:"disliked_user_id"`
}

type DislikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func ProcessDislike(ctx context.Context, queries *migrations.Queries, hub *Hub, dislikerUserID int32, req DislikeRequest) error {
	if req.DislikedUserID <= 0 {
		return errors.New("valid disliked_user_id is required")
	}
	if req.DislikedUserID == dislikerUserID {
		return errors.New("cannot dislike yourself")
	}

	log.Printf("ProcessDislike attempt: User %d -> User %d", dislikerUserID, req.DislikedUserID)

	likeCheckParams := migrations.CheckLikeExistsParams{
		LikerUserID: req.DislikedUserID,
		LikedUserID: dislikerUserID,
	}
	hadLikedBack := false
	likeExists, checkErr := queries.CheckLikeExists(ctx, likeCheckParams)
	if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
		log.Printf("ProcessDislike WARN: Failed check like %d -> %d: %v", req.DislikedUserID, dislikerUserID, checkErr)
	} else if checkErr == nil {
		hadLikedBack = likeExists
	}

	err := queries.AddDislike(ctx, migrations.AddDislikeParams{
		DislikerUserID: dislikerUserID,
		DislikedUserID: req.DislikedUserID,
	})
	if err != nil {
		log.Printf("ProcessDislike ERROR: AddDislike %d -> %d: %v", dislikerUserID, req.DislikedUserID, err)
		return errors.New("failed to process dislike")
	}

	log.Printf("ProcessDislike INFO: Dislike processed: User %d -> User %d", dislikerUserID, req.DislikedUserID)

	if hadLikedBack && hub != nil {
		log.Printf("ProcessDislike INFO: Dislike from %d removed like from %d. Notifying user %d.", dislikerUserID, req.DislikedUserID, req.DislikedUserID)
		removalInfo := WsLikeRemovalInfo{
			LikerUserID: dislikerUserID,
		}
		go hub.BroadcastLikeRemoved(req.DislikedUserID, removalInfo)
	}

	return nil
}
