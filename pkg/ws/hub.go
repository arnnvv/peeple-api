package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/jackc/pgx/v5"
)

type Hub struct {
	clients    map[int32]*Client
	register   chan *Client
	unregister chan *Client
	clientsMu  sync.RWMutex
	dbQueries  *migrations.Queries // Assuming this is initialized correctly elsewhere
}

func NewHub(db *migrations.Queries) *Hub {
	if db == nil {
		log.Fatal("FATAL: Hub created without database queries interface")
	}
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[int32]*Client),
		dbQueries:  db,
	}
}

func (h *Hub) Run() {
	// log.Println("Hub: Starting run loop") // Keep essential logs
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			// log.Printf("Hub: Registering client for user %d", client.UserID) // Can be verbose
			if oldClient, exists := h.clients[client.UserID]; exists {
				log.Printf("Hub: Closing stale connection for user %d", client.UserID) // Keep this log
				close(oldClient.Send)
			}
			h.clients[client.UserID] = client
			h.clientsMu.Unlock()

			// --- Set User Online & Broadcast Status ---
			go func(uid int32) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := h.dbQueries.SetUserOnline(ctx, uid)
				if err != nil {
					log.Printf("Hub: Failed to set user %d online in DB: %v", uid, err)
				} else {
					// log.Printf("Hub: Set user %d online in DB.", uid) // Can be verbose
				}
				h.broadcastStatusChange(uid, true)
			}(client.UserID)
			// --- End Set User Online ---

		case client := <-h.unregister:
			h.clientsMu.Lock()
			clientID := client.UserID
			if currentClient, ok := h.clients[clientID]; ok && currentClient == client {
				// log.Printf("Hub: Unregistering client for user %d", clientID) // Can be verbose
				delete(h.clients, clientID)
				// Safely close channel
				select {
				case <-client.Send:
					// Channel already closed
				default:
					close(client.Send)
				}
				h.clientsMu.Unlock()

				// --- Set User Offline & Broadcast Status ---
				go func(uid int32) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					err := h.dbQueries.SetUserOffline(ctx, uid)
					if err != nil {
						log.Printf("Hub: Failed to set user %d offline and update last_online in DB: %v", uid, err)
					} else {
						// log.Printf("Hub: Set user %d offline and updated last_online in DB.", uid) // Can be verbose
					}
					h.broadcastStatusChange(uid, false)
				}(clientID)
				// --- End Set User Offline ---

			} else {
				h.clientsMu.Unlock()
				// log.Printf("Hub: Unregister request for unknown or outdated client %d", clientID) // Can be verbose
			}
		}
	}
}

// SendToUser sends a message directly to a specific user if they are connected.
// Returns true if the user was connected and the message was sent to their channel, false otherwise.
func (h *Hub) SendToUser(userID int32, message []byte) bool {
	h.clientsMu.RLock()
	client, ok := h.clients[userID]
	h.clientsMu.RUnlock()

	if ok {
		select {
		case client.Send <- message:
			return true // Message successfully queued
		case <-time.After(1 * time.Second): // Add a timeout to prevent blocking indefinitely
			log.Printf("Hub WARN: Send channel timeout for user %d. Assuming disconnected.", userID)
			// Trigger unregistration asynchronously to avoid deadlock
			go func(c *Client) { h.unregister <- c }(client)
			return false
		}
	}
	return false // User not connected
}

// getMatchIDs fetches IDs of users mutually liked by the given user.
func (h *Hub) getMatchIDs(ctx context.Context, userID int32) ([]int32, error) {
	matchIDs, err := h.dbQueries.GetMatchIDs(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Hub ERROR: Error fetching match IDs for user %d: %v", userID, err)
		return nil, err // Return the error
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return []int32{}, nil // No matches, return empty slice, not an error
	}
	return matchIDs, nil
}

// broadcastStatusChange notifies a user's matches about their online/offline status.
func (h *Hub) broadcastStatusChange(userID int32, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	matchIDs, err := h.getMatchIDs(ctx, userID)
	if err != nil {
		log.Printf("Hub WARN: Failed to get matches for user %d to broadcast status, aborting broadcast: %v", userID, err)
		return
	}

	if len(matchIDs) == 0 {
		return // No matches to notify
	}

	statusStr := "offline"
	if isOnline {
		statusStr = "online"
	}
	statusMsg := WsMessage{
		Type:   "status_update",
		UserID: &userID,
		Status: &statusStr,
	}
	messageBytes, err := json.Marshal(statusMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal status update message for user %d: %v", userID, err)
		return
	}

	broadcastCount := 0
	for _, matchID := range matchIDs {
		if h.SendToUser(matchID, messageBytes) {
			broadcastCount++
		}
	}
	// log.Printf("Hub: Broadcasted status update (isOnline: %t) for user %d to %d/%d connected matches.", isOnline, userID, broadcastCount, len(matchIDs)) // Can be verbose

	// If the user just came online, send them the status of their connected matches
	if isOnline {
		h.sendStatusesOfMatchesToUser(userID, matchIDs)
	}
}

// sendStatusesOfMatchesToUser sends the online status of connected matches TO the newly connected user.
func (h *Hub) sendStatusesOfMatchesToUser(targetUser int32, matchIDs []int32) {
	if len(matchIDs) == 0 {
		return
	}

	statusesSent := 0
	h.clientsMu.RLock()
	// Create a map of connected match IDs for efficient lookup
	connectedMatches := make(map[int32]bool)
	for _, matchID := range matchIDs {
		if _, isConnected := h.clients[matchID]; isConnected {
			connectedMatches[matchID] = true
		}
	}
	h.clientsMu.RUnlock() // Unlock after reading

	// Iterate only over connected matches
	for matchID := range connectedMatches {
		statusStr := "online" // They must be online if they are in the map
		statusMsg := WsMessage{
			Type:   "status_update",
			UserID: &matchID, // Send the match's ID
			Status: &statusStr,
		}
		messageBytes, err := json.Marshal(statusMsg)
		if err != nil {
			log.Printf("Hub ERROR: Failed to marshal match status for user %d: %v", matchID, err)
			continue // Skip this one if marshalling fails
		}
		// Send the status TO the targetUser
		if h.SendToUser(targetUser, messageBytes) {
			statusesSent++
		}
	}
	// log.Printf("Hub: Sent initial online statuses of %d connected matches to user %d", statusesSent, targetUser) // Can be verbose
}

// BroadcastReaction sends a reaction update to relevant participants.
func (h *Hub) BroadcastReaction(messageID int64, reactorUserID int32, emoji string, isRemoved bool, participants []int32) {
	// log.Printf("Hub: Broadcasting reaction update: MsgID=%d, User=%d, Emoji='%s', Removed=%t, Participants=%v", messageID, reactorUserID, emoji, isRemoved, participants) // Can be verbose
	reactionMsg := WsMessage{
		Type:          "reaction_update",
		MessageID:     &messageID,
		ReactorUserID: &reactorUserID,
		IsRemoved:     &isRemoved,
	}
	if !isRemoved {
		reactionMsg.Emoji = &emoji
	}

	messageBytes, err := json.Marshal(reactionMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal reaction update message for msg %d: %v", messageID, err)
		return
	}

	broadcastCount := 0
	for _, participantID := range participants {
		if h.SendToUser(participantID, messageBytes) {
			broadcastCount++
		}
	}
	// log.Printf("Hub: Broadcasted reaction update for msg %d to %d/%d participants.", messageID, broadcastCount, len(participants)) // Can be verbose
}

// --- NEW: BroadcastNewLike Function ---
func (h *Hub) BroadcastNewLike(recipientUserID int32, likerInfo WsBasicLikerInfo) {
	log.Printf("Hub INFO: Broadcasting new like from User %d to User %d", likerInfo.LikerUserID, recipientUserID)
	wsMsg := WsMessage{
		Type:      "new_like_received",
		LikerInfo: &likerInfo, // Embed the liker info payload
	}

	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal new_like_received message for recipient %d: %v", recipientUserID, err)
		return
	}

	if h.SendToUser(recipientUserID, messageBytes) {
		log.Printf("Hub INFO: Sent new_like_received notification to user %d.", recipientUserID)
	} else {
		log.Printf("Hub INFO: User %d is offline, new_like_received notification not sent in real-time.", recipientUserID)
	}
}

// --- NEW: BroadcastNewMatch Function ---
// Sends a new_match message to a specific target user.
// This needs to be called twice by the handler, once for each user involved in the match.
func (h *Hub) BroadcastNewMatch(targetUserID int32, matchInfo WsMatchInfo) {
	log.Printf("Hub INFO: Broadcasting new match notification to User %d about User %d (Initiating Liker: %d)",
		targetUserID, matchInfo.MatchedUserID, matchInfo.InitiatingLikerUserID)
	wsMsg := WsMessage{
		Type:      "new_match",
		MatchInfo: &matchInfo, // Embed the match info payload
	}

	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal new_match message for target %d: %v", targetUserID, err)
		return
	}

	if h.SendToUser(targetUserID, messageBytes) {
		log.Printf("Hub INFO: Sent new_match notification to user %d.", targetUserID)
	} else {
		log.Printf("Hub INFO: User %d is offline, new_match notification not sent in real-time.", targetUserID)
	}
}

// --- NEW: BroadcastLikeRemoved Function ---
func (h *Hub) BroadcastLikeRemoved(recipientUserID int32, removalInfo WsLikeRemovalInfo) {
	log.Printf("Hub INFO: Broadcasting like removal notification to User %d regarding Liker %d",
		recipientUserID, removalInfo.LikerUserID)
	wsMsg := WsMessage{
		Type:        "like_removed",
		RemovalInfo: &removalInfo, // Embed the removal info payload
	}

	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal like_removed message for recipient %d: %v", recipientUserID, err)
		return
	}

	if h.SendToUser(recipientUserID, messageBytes) {
		log.Printf("Hub INFO: Sent like_removed notification to user %d.", recipientUserID)
	} else {
		log.Printf("Hub INFO: User %d is offline, like_removed notification not sent in real-time.", recipientUserID)
	}
}

// *** ADDED: BroadcastMatchRemoved Function ***
// Sends a message to recipientUserID indicating that unmatcherUserID has unmatched them.
func (h *Hub) BroadcastMatchRemoved(recipientUserID int32, unmatcherUserID int32) {
	log.Printf("Hub INFO: Broadcasting match removal notification to User %d regarding Unmatcher %d",
		recipientUserID, unmatcherUserID)

	// Reuse WsLikeRemovalInfo payload structure:
	// LikerUserID here means the ID of the user whose profile should be removed from the recipient's list.
	payload := &WsLikeRemovalInfo{
		LikerUserID: unmatcherUserID,
	}

	wsMsg := WsMessage{
		Type:        "match_removed", // Use the new distinct type
		RemovalInfo: payload,
	}

	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed to marshal match_removed message for recipient %d: %v", recipientUserID, err)
		return
	}

	if h.SendToUser(recipientUserID, messageBytes) {
		log.Printf("Hub INFO: Sent match_removed notification to user %d.", recipientUserID)
	} else {
		log.Printf("Hub INFO: User %d is offline, match_removed notification not sent in real-time.", recipientUserID)
	}
}

// *** END ADDITION ***
