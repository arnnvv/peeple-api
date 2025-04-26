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
	dbQueries  *migrations.Queries
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
	log.Println("Hub: Starting run loop")
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			log.Printf("Hub: Registering client for user %d", client.UserID)
			if oldClient, exists := h.clients[client.UserID]; exists {
				log.Printf("Hub: Closing stale connection for user %d", client.UserID)
				close(oldClient.Send)
			}
			h.clients[client.UserID] = client
			h.clientsMu.Unlock()

			go func(uid int32) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := h.dbQueries.SetUserOnline(ctx, uid)
				if err != nil {
					log.Printf("Hub: Failed to set user %d online in DB: %v", uid, err)
				} else {
					log.Printf("Hub: Set user %d online in DB.", uid)
				}
				h.broadcastStatusChange(uid, true)
			}(client.UserID)

		case client := <-h.unregister:
			h.clientsMu.Lock()
			clientID := client.UserID
			if currentClient, ok := h.clients[clientID]; ok && currentClient == client {
				log.Printf("Hub: Unregistering client for user %d", clientID)
				delete(h.clients, clientID)
				select {
				case <-client.Send:
				default:
					close(client.Send)
				}
				h.clientsMu.Unlock()

				go func(uid int32) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					err := h.dbQueries.SetUserOffline(ctx, uid)
					if err != nil {
						log.Printf("Hub: Failed to set user %d offline and update last_online in DB: %v", uid, err)
					} else {
						log.Printf("Hub: Set user %d offline and updated last_online in DB.", uid)
					}
					h.broadcastStatusChange(uid, false)
				}(clientID)

			} else {
				h.clientsMu.Unlock()
				log.Printf("Hub: Unregister request for unknown or outdated client %d", clientID)
			}
		}
	}
}

func (h *Hub) SendToUser(userID int32, message []byte) bool {
	h.clientsMu.RLock()
	client, ok := h.clients[userID]
	h.clientsMu.RUnlock()
	if ok {
		select {
		case client.Send <- message:
			return true
		case <-time.After(1 * time.Second):
			log.Printf("Hub: Send channel timeout for user %d. Assuming disconnected.", userID)
			go func(c *Client) { h.unregister <- c }(client)
			return false
		}
	}
	return false
}

func (h *Hub) getMatchIDs(ctx context.Context, userID int32) ([]int32, error) {
	matchIDs, err := h.dbQueries.GetMatchIDs(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Hub: Error fetching match IDs for user %d: %v", userID, err)
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return []int32{}, nil
	}
	return matchIDs, nil
}

func (h *Hub) broadcastStatusChange(userID int32, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	matchIDs, err := h.getMatchIDs(ctx, userID)
	if err != nil {
		log.Printf("Hub: Failed to get matches for user %d to broadcast status, aborting broadcast: %v", userID, err)
		return
	}

	if len(matchIDs) == 0 {
		return
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
		log.Printf("Hub: Failed to marshal status update message for user %d: %v", userID, err)
		return
	}

	broadcastCount := 0
	for _, matchID := range matchIDs {
		if h.SendToUser(matchID, messageBytes) {
			broadcastCount++
		}
	}
	log.Printf("Hub: Broadcasted status update (isOnline: %t) for user %d to %d/%d connected matches.", isOnline, userID, broadcastCount, len(matchIDs))

	if isOnline {
		h.sendStatusesOfMatchesToUser(userID, matchIDs)
	}
}

func (h *Hub) sendStatusesOfMatchesToUser(targetUser int32, matchIDs []int32) {
	if len(matchIDs) == 0 {
		return
	}
	statusesSent := 0
	h.clientsMu.RLock()
	connectedMatches := make(map[int32]bool)
	for _, matchID := range matchIDs {
		if _, isConnected := h.clients[matchID]; isConnected {
			connectedMatches[matchID] = true
		}
	}
	h.clientsMu.RUnlock()
	for matchID := range connectedMatches {
		statusStr := "online"
		statusMsg := WsMessage{
			Type:   "status_update",
			UserID: &matchID, Status: &statusStr,
		}
		messageBytes, err := json.Marshal(statusMsg)
		if err != nil {
			log.Printf("Hub: Failed to marshal match status for user %d: %v", matchID, err)
			continue
		}
		if h.SendToUser(targetUser, messageBytes) {
			statusesSent++
		}
	}
	log.Printf("Hub: Sent initial online statuses of %d connected matches to user %d", statusesSent, targetUser)
}

func (h *Hub) BroadcastReaction(messageID int64, reactorUserID int32, emoji string, isRemoved bool, participants []int32) {
	log.Printf("Hub: Broadcasting reaction update: MsgID=%d, User=%d, Emoji='%s', Removed=%t, Participants=%v", messageID, reactorUserID, emoji, isRemoved, participants)
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
		log.Printf("Hub: Failed to marshal reaction update message for msg %d: %v", messageID, err)
		return
	}
	broadcastCount := 0
	for _, participantID := range participants {
		if h.SendToUser(participantID, messageBytes) {
			broadcastCount++
		}
	}
	log.Printf("Hub: Broadcasted reaction update for msg %d to %d/%d participants.", messageID, broadcastCount, len(participants))
}
