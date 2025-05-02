package handlers

import (
	"context" // Import context
	"encoding/json"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/arnnvv/peeple-api/pkg/ws" // Import ws package
	"github.com/jackc/pgx/v5/pgconn"
)

type UnmatchRequest struct {
	TargetUserID int32 `json:"target_user_id"`
}

type UnmatchResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// *** MODIFIED FUNCTION SIGNATURE ***
func UnmatchHandler(hub *ws.Hub) http.HandlerFunc {
	// *** WRAP EXISTING LOGIC ***
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		queries, _ := db.GetDB()
		pool, _ := db.GetPool()

		if queries == nil || pool == nil {
			log.Println("ERROR: UnmatchHandler: Database connection not available.")
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database connection error"})
			return
		}

		if r.Method != http.MethodPost {
			utils.RespondWithJSON(w, http.StatusMethodNotAllowed, UnmatchResponse{Success: false, Message: "Method Not Allowed: Use POST"})
			return
		}

		claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
		if !ok || claims == nil || claims.UserID <= 0 {
			utils.RespondWithJSON(w, http.StatusUnauthorized, UnmatchResponse{Success: false, Message: "Authentication required"})
			return
		}
		requesterUserID := int32(claims.UserID)

		var req UnmatchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("ERROR: UnmatchHandler: Invalid request body for user %d: %v", requesterUserID, err)
			utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Invalid request body format"})
			return
		}
		defer r.Body.Close()

		if req.TargetUserID <= 0 {
			utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Valid target_user_id is required"})
			return
		}
		if req.TargetUserID == requesterUserID {
			utils.RespondWithJSON(w, http.StatusBadRequest, UnmatchResponse{Success: false, Message: "Cannot unmatch yourself"})
			return
		}

		log.Printf("INFO: Unmatch attempt: User %d -> User %d", requesterUserID, req.TargetUserID)

		tx, err := pool.Begin(ctx)
		if err != nil {
			log.Printf("ERROR: UnmatchHandler: Failed to begin transaction for user %d: %v", requesterUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database transaction error"})
			return
		}
		// Defer rollback *conditionally* based on the final value of 'err'
		defer func() {
			if p := recover(); p != nil {
				// A panic occurred, rollback
				log.Printf("PANIC recovered in UnmatchHandler, rolling back transaction: %v", p)
				_ = tx.Rollback(context.Background()) // Use background context for rollback
				panic(p)                              // Re-panic after rollback
			} else if err != nil {
				// An error occurred, rollback
				log.Printf("WARN: UnmatchHandler: Rolling back transaction due to error: %v", err)
				_ = tx.Rollback(context.Background())
			}
			// If err is nil, commit already happened or will happen.
		}()

		qtx := queries.WithTx(tx)

		log.Printf("DEBUG: UnmatchHandler: Deleting likes between %d and %d", requesterUserID, req.TargetUserID)
		// Assign error to the outer 'err' variable
		err = qtx.DeleteLikesBetweenUsers(ctx, migrations.DeleteLikesBetweenUsersParams{
			LikerUserID: requesterUserID,
			LikedUserID: req.TargetUserID,
		})
		if err != nil {
			log.Printf("ERROR: UnmatchHandler: Failed to delete likes between %d and %d: %v", requesterUserID, req.TargetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to remove existing connection"})
			// Return here will trigger the deferred rollback
			return
		}

		log.Printf("DEBUG: UnmatchHandler: Adding dislike from %d to %d", requesterUserID, req.TargetUserID)
		// Assign error to the outer 'err' variable
		err = qtx.AddDislike(ctx, migrations.AddDislikeParams{
			DislikerUserID: requesterUserID,
			DislikedUserID: req.TargetUserID,
		})
		if err != nil {
			log.Printf("ERROR: UnmatchHandler: Failed to add dislike from %d to %d: %v", requesterUserID, req.TargetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to record dislike"})
			// Return here will trigger the deferred rollback
			return
		}

		log.Printf("DEBUG: UnmatchHandler: Marking chat messages as read for recipient %d from sender %d", requesterUserID, req.TargetUserID)
		markReadParams := migrations.MarkChatAsReadOnUnmatchParams{
			RecipientUserID: requesterUserID,
			SenderUserID:    req.TargetUserID,
		}
		var cmdTag pgconn.CommandTag // Define cmdTag to capture result
		// Assign error to the outer 'err' variable
		cmdTag, err = qtx.MarkChatAsReadOnUnmatch(ctx, markReadParams)
		if err != nil {
			log.Printf("ERROR: UnmatchHandler: Failed to mark chat messages as read between %d and %d: %v", requesterUserID, req.TargetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Failed to update chat read status"})
			// Return here will trigger the deferred rollback
			return
		}
		log.Printf("DEBUG: UnmatchHandler: Marked %d messages as read.", cmdTag.RowsAffected())

		// Commit the transaction. If successful, set outer 'err' to nil.
		commitErr := tx.Commit(ctx)
		if commitErr != nil {
			err = commitErr // Assign commit error to outer 'err' so rollback happens
			log.Printf("ERROR: UnmatchHandler: Failed to commit transaction for user %d unmatching %d: %v", requesterUserID, req.TargetUserID, err)
			utils.RespondWithJSON(w, http.StatusInternalServerError, UnmatchResponse{Success: false, Message: "Database commit error"})
			// Return here will trigger the deferred rollback
			return
		}
		// If commit was successful, ensure the outer error is nil so the defer doesn't roll back
		err = nil
		log.Printf("INFO: Unmatch processed successfully in DB: User %d -> User %d", requesterUserID, req.TargetUserID)

		// *** ADDED: Send WebSocket notification AFTER successful commit ***
		if hub != nil {
			// Send notification asynchronously
			go func(recipientID int32, unmatcherID int32) {
				log.Printf("UnmatchHandler INFO: Triggering WebSocket match_removed broadcast to %d about %d", recipientID, unmatcherID)
				hub.BroadcastMatchRemoved(recipientID, unmatcherID)
			}(req.TargetUserID, requesterUserID)
		} else {
			log.Printf("WARN: UnmatchHandler: Hub is nil, cannot send WebSocket notification.")
		}
		// *** END ADDITION ***

		// Respond HTTP Success
		utils.RespondWithJSON(w, http.StatusOK, UnmatchResponse{Success: true, Message: "Unmatched successfully"})
	}
	// *** END WRAPPER ***
}
