package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/arnnvv/peeple-api/pkg/db"
	"github.com/arnnvv/peeple-api/pkg/token"
	"github.com/arnnvv/peeple-api/pkg/utils"
	"github.com/arnnvv/peeple-api/pkg/ws" // *** ADDED: Import ws package ***
	"github.com/jackc/pgx/v5"
)

type DislikeRequest struct {
	DislikedUserID int32 `json:"disliked_user_id"`
}

type DislikeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// *** ADDED: Dependency Injection for Hub ***
// Modify the function signature to accept the Hub
func DislikeHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx := r.Context()
		queries, _ := db.GetDB()
		// Get Pool for potential transaction if needed later, though not strictly necessary for dislike notification
		// pool, _ := db.GetPool()

		if r.Method != http.MethodPost {
			utils.RespondWithJSON(w, http.StatusMethodNotAllowed, DislikeResponse{Success: false, Message: "Method Not Allowed: Use POST"})
			return
		}

		claims, ok := ctx.Value(token.ClaimsContextKey).(*token.Claims)
		if !ok || claims == nil || claims.UserID <= 0 {
			utils.RespondWithJSON(w, http.StatusUnauthorized, DislikeResponse{Success: false, Message: "Authentication required"})
			return
		}
		dislikerUserID := int32(claims.UserID)

		var req DislikeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Invalid request body format"})
			return
		}
		defer r.Body.Close()

		if req.DislikedUserID <= 0 {
			utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Valid disliked_user_id is required"})
			return
		}

		if req.DislikedUserID == dislikerUserID {
			utils.RespondWithJSON(w, http.StatusBadRequest, DislikeResponse{Success: false, Message: "Cannot dislike yourself"})
			return
		}

		log.Printf("Dislike attempt: User %d -> User %d", dislikerUserID, req.DislikedUserID)

		// --- Check if the disliked user had liked the disliker ---
		// We need to check this *before* adding the dislike, otherwise, the dislike might prevent finding the like
		likeCheckParams := migrations.CheckLikeExistsParams{
			LikerUserID: req.DislikedUserID, // Check if the *disliked* user liked the *disliker*
			LikedUserID: dislikerUserID,
		}
		var hadLikedBack bool = false
		likeExists, checkErr := queries.CheckLikeExists(ctx, likeCheckParams)
		if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
			log.Printf("DislikeHandler WARN: Failed to check if user %d liked user %d before dislike: %v", req.DislikedUserID, dislikerUserID, checkErr)
			// Continue processing the dislike, but log the warning
		} else if checkErr == nil {
			hadLikedBack = likeExists
		}
		// --- End Like Check ---

		// --- Add Dislike to DB ---
		err := queries.AddDislike(ctx, migrations.AddDislikeParams{
			DislikerUserID: dislikerUserID,
			DislikedUserID: req.DislikedUserID,
		})

		if err != nil {
			log.Printf("DislikeHandler ERROR: Error adding dislike for user %d -> %d: %v", dislikerUserID, req.DislikedUserID, err)
			// TODO: Check for specific DB errors if needed (e.g., constraint violation if already disliked)
			utils.RespondWithJSON(w, http.StatusInternalServerError, DislikeResponse{Success: false, Message: "Failed to process dislike"})
			return
		}
		// --- End DB Add ---

		log.Printf("Dislike processed successfully: User %d -> User %d", dislikerUserID, req.DislikedUserID)

		// --- Send WebSocket Notification if necessary ---
		if hadLikedBack && hub != nil { // Check if the dislike removed a like from the other user's screen
			log.Printf("DislikeHandler INFO: Dislike from %d removed a like previously sent by %d. Notifying user %d.", dislikerUserID, req.DislikedUserID, req.DislikedUserID)
			removalInfo := ws.WsLikeRemovalInfo{
				LikerUserID: dislikerUserID, // The ID of the like to remove from the recipient's screen
			}
			// Send notification asynchronously
			go hub.BroadcastLikeRemoved(req.DislikedUserID, removalInfo)
		}
		// --- End WebSocket Notification ---

		// Send success response
		utils.RespondWithJSON(w, http.StatusOK, DislikeResponse{Success: true, Message: "Disliked successfully"})
	}
}
