package ws

import (
	"context"
	"errors"
	"log"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnmatchRequest struct {
	TargetUserID int32 `json:"target_user_id"`
}

type UnmatchResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func ProcessUnmatch(ctx context.Context, queries *migrations.Queries, pool *pgxpool.Pool, hub *Hub, requesterUserID int32, req UnmatchRequest) error {
	if req.TargetUserID <= 0 {
		return errors.New("valid target_user_id is required")
	}
	if req.TargetUserID == requesterUserID {
		return errors.New("cannot unmatch yourself")
	}

	log.Printf("ProcessUnmatch attempt: User %d -> User %d", requesterUserID, req.TargetUserID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("ERROR: ProcessUnmatch: Failed begin transaction user %d: %v", requesterUserID, err)
		return errors.New("database transaction error")
	}
	defer func(txErr *error) {
		if p := recover(); p != nil {
			log.Printf("PANIC recovered in ProcessUnmatch, rolling back: %v", p)
			_ = tx.Rollback(context.Background())
			panic(p)
		} else if *txErr != nil {
			log.Printf("WARN: ProcessUnmatch: Rolling back transaction due to error: %v", *txErr)
			_ = tx.Rollback(context.Background())
		}
	}(&err)

	qtx := queries.WithTx(tx)

	err = qtx.DeleteLikesBetweenUsers(ctx, migrations.DeleteLikesBetweenUsersParams{
		LikerUserID: requesterUserID,
		LikedUserID: req.TargetUserID,
	})
	if err != nil {
		log.Printf("ERROR: ProcessUnmatch: Failed delete likes %d <-> %d: %v", requesterUserID, req.TargetUserID, err)
		return errors.New("failed to remove existing connection")
	}

	err = qtx.AddDislike(ctx, migrations.AddDislikeParams{
		DislikerUserID: requesterUserID,
		DislikedUserID: req.TargetUserID,
	})
	if err != nil {
		log.Printf("ERROR: ProcessUnmatch: Failed add dislike %d -> %d: %v", requesterUserID, req.TargetUserID, err)
		return errors.New("failed to record dislike")
	}

	markReadParams := migrations.MarkChatAsReadOnUnmatchParams{
		RecipientUserID: requesterUserID,
		SenderUserID:    req.TargetUserID,
	}
	var cmdTag pgconn.CommandTag
	cmdTag, err = qtx.MarkChatAsReadOnUnmatch(ctx, markReadParams)
	if err != nil {
		log.Printf("ERROR: ProcessUnmatch: Failed mark chat read %d <-> %d: %v", requesterUserID, req.TargetUserID, err)
		return errors.New("failed to update chat read status")
	}
	log.Printf("DEBUG: ProcessUnmatch: Marked %d chat messages read.", cmdTag.RowsAffected())

	err = tx.Commit(ctx)
	if err != nil {
		log.Printf("ERROR: ProcessUnmatch: Failed commit transaction user %d unmatching %d: %v", requesterUserID, req.TargetUserID, err)
		return errors.New("database commit error")
	}

	err = nil
	log.Printf("INFO: Unmatch processed DB successfully: User %d -> User %d", requesterUserID, req.TargetUserID)

	if hub != nil {
		go func(recipientID int32, unmatcherID int32) {
			log.Printf("ProcessUnmatch INFO: Triggering WS match_removed broadcast to %d about %d", recipientID, unmatcherID)
			hub.BroadcastMatchRemoved(recipientID, unmatcherID)
		}(req.TargetUserID, requesterUserID)
	} else {
		log.Printf("WARN: ProcessUnmatch: Hub is nil, cannot send WS notification.")
	}

	return nil
}
