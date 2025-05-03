package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/migrations"
	"github.com/go-redis/redis_rate/v10"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type Hub struct {
	clients     map[int32]*Client
	register    chan *Client
	unregister  chan *Client
	clientsMu   sync.RWMutex
	dbQueries   *migrations.Queries
	redisClient *redis.Client
	hubContext  context.Context
	hubCancel   context.CancelFunc
	rateLimiter *redis_rate.Limiter
}

func NewHub(db *migrations.Queries, rds *redis.Client, limiter *redis_rate.Limiter) *Hub {
	if db == nil {
		log.Fatal("FATAL: Hub created without database queries interface")
	}
	if rds == nil {
		log.Fatal("FATAL: Hub created without Redis client")
	}
	if limiter == nil {
		log.Fatal("FATAL: Hub created without Rate Limiter")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[int32]*Client),
		dbQueries:   db,
		redisClient: rds,
		hubContext:  ctx,
		hubCancel:   cancel,
		rateLimiter: limiter,
	}
}

func (h *Hub) Run() {
	log.Println("Hub: Starting run loop and Redis subscription...")
	go h.subscribeToMessages()

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
				}
				h.broadcastStatusChange(uid, true)
			}(client.UserID)

		case client := <-h.unregister:
			clientID := client.UserID
			go func(uid int32) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := h.dbQueries.SetUserOffline(ctx, uid)
				if err != nil {
					log.Printf("Hub: Failed to set user %d offline and update last_online in DB: %v", uid, err)
				}
				h.broadcastStatusChange(uid, false)
			}(clientID)

			h.clientsMu.Lock()
			if currentClient, ok := h.clients[clientID]; ok && currentClient == client {
				log.Printf("Hub: Unregistering client for user %d", clientID)
				delete(h.clients, clientID)
				select {
				case <-client.Send:
				default:
					close(client.Send)
				}
			}
			h.clientsMu.Unlock()

		case <-h.hubContext.Done():
			log.Println("Hub: Context cancelled, shutting down Run loop.")
			return
		}
	}
}

func (h *Hub) Stop() {
	log.Println("Hub: Stopping...")
	h.hubCancel()
}

func (h *Hub) subscribeToMessages() {
	pubsub := h.redisClient.Subscribe(h.hubContext, RedisChannelName)
	defer func() {
		err := pubsub.Close()
		if err != nil {
			log.Printf("Hub Subscriber: Error closing pubsub: %v", err)
		}
		log.Println("Hub Subscriber: Stopped.")
	}()

	log.Printf("Hub Subscriber: Subscribed to '%s'", RedisChannelName)
	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			if msg == nil {
				log.Println("Hub Subscriber: Received nil message, channel likely closed.")
				return
			}
			h.handleRedisMessage(msg.Payload)
		case <-h.hubContext.Done():
			log.Println("Hub Subscriber: Context cancelled, exiting.")
			return
		}
	}
}

func (h *Hub) handleRedisMessage(payload string) {
	var redisMsg RedisWsMessage
	err := json.Unmarshal([]byte(payload), &redisMsg)
	if err != nil {
		log.Printf("Hub Subscriber ERROR: Failed to unmarshal message from Redis: %v. Payload: %s", err, payload)
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	switch redisMsg.Type {
	case RedisMsgTypeDirect:
		if redisMsg.TargetUserID != nil {
			targetID := *redisMsg.TargetUserID
			if client, ok := h.clients[targetID]; ok {
				select {
				case client.Send <- redisMsg.OriginalPayload:
				default:
					log.Printf("Hub Subscriber WARN: Send channel full or closed for user %d.", targetID)
				}
			}
		} else {
			log.Printf("Hub Subscriber WARN: Received direct message from Redis without TargetUserID.")
		}

	case RedisMsgTypeBroadcastMatches:
		if len(redisMsg.TargetUserIDs) > 0 {
			for _, targetID := range redisMsg.TargetUserIDs {
				if targetID == redisMsg.SenderUserID {
					continue
				}
				if client, ok := h.clients[targetID]; ok {
					select {
					case client.Send <- redisMsg.OriginalPayload:
					default:
						log.Printf("Hub Subscriber WARN: Send channel full or closed for user %d during broadcast.", targetID)
					}
				}
			}
		} else {
			log.Printf("Hub Subscriber WARN: Received broadcast_matches message from Redis without TargetUserIDs.")
		}
	default:
		log.Printf("Hub Subscriber WARN: Received unknown message type from Redis: '%s'", redisMsg.Type)
	}
}

func (h *Hub) publishToRedis(ctx context.Context, redisMsg RedisWsMessage) error {
	jsonData, err := json.Marshal(redisMsg)
	if err != nil {
		log.Printf("Hub Publish ERROR: Failed to marshal message for Redis: %v", err)
		return err
	}

	err = h.redisClient.Publish(ctx, RedisChannelName, jsonData).Err()
	if err != nil {
		log.Printf("Hub Publish ERROR: Failed to publish message to Redis channel '%s': %v", RedisChannelName, err)
		return err
	}
	return nil
}

func (h *Hub) SendToUser(userID int32, message []byte) bool {
	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeDirect,
		TargetUserID:    &userID,
		OriginalPayload: message,
	}
	err := h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish direct message for user %d via Redis: %v", userID, err)
		return false
	}
	return true
}

func (h *Hub) getMatchIDs(ctx context.Context, userID int32) ([]int32, error) {
	matchIDs, err := h.dbQueries.GetMatchIDs(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Hub ERROR: Error fetching match IDs for user %d: %v", userID, err)
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
		log.Printf("Hub WARN: Failed to get matches for user %d to broadcast status: %v", userID, err)
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
		log.Printf("Hub ERROR: Failed to marshal status update message for user %d: %v", userID, err)
		return
	}

	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeBroadcastMatches,
		TargetUserIDs:   matchIDs,
		OriginalPayload: messageBytes,
		SenderUserID:    userID,
	}
	err = h.publishToRedis(ctx, redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish status change for user %d via Redis: %v", userID, err)
	}

	if isOnline {
		h.sendStatusesOfMatchesToUser(userID, matchIDs)
	}
}

func (h *Hub) sendStatusesOfMatchesToUser(targetUser int32, matchIDs []int32) {
	if len(matchIDs) == 0 {
		return
	}

	h.clientsMu.RLock()
	connectedMatches := make(map[int32]bool)
	for _, matchID := range matchIDs {
		if _, isConnected := h.clients[matchID]; isConnected {
			connectedMatches[matchID] = true
		}
	}
	targetClient, isTargetConnected := h.clients[targetUser]
	h.clientsMu.RUnlock()

	if !isTargetConnected {
		log.Printf("Hub WARN: Cannot send initial statuses to user %d, they disconnected.", targetUser)
		return
	}

	for matchID := range connectedMatches {
		statusStr := "online"
		statusMsg := WsMessage{
			Type:   "status_update",
			UserID: &matchID,
			Status: &statusStr,
		}
		messageBytes, err := json.Marshal(statusMsg)
		if err != nil {
			log.Printf("Hub ERROR: Failed to marshal match status for user %d: %v", matchID, err)
			continue
		}
		select {
		case targetClient.Send <- messageBytes:
		default:
			log.Printf("Hub WARN: Send channel full/closed for target user %d while sending initial statuses.", targetUser)
			return
		}
	}
}

func (h *Hub) BroadcastReaction(messageID int64, reactorUserID int32, emoji string, isRemoved bool, participants []int32) {
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
		log.Printf("Hub ERROR: Failed marshal reaction update msg %d: %v", messageID, err)
		return
	}

	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeBroadcastMatches,
		TargetUserIDs:   participants,
		OriginalPayload: messageBytes,
		SenderUserID:    reactorUserID,
	}
	err = h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish reaction update for msg %d via Redis: %v", messageID, err)
	}
}

func (h *Hub) BroadcastNewLike(recipientUserID int32, likerInfo WsBasicLikerInfo) {
	wsMsg := WsMessage{Type: "new_like_received", LikerInfo: &likerInfo}
	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed marshal new_like msg for %d: %v", recipientUserID, err)
		return
	}
	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeDirect,
		TargetUserID:    &recipientUserID,
		OriginalPayload: messageBytes,
		SenderUserID:    likerInfo.LikerUserID,
	}
	err = h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish new_like_received message for user %d via Redis: %v", recipientUserID, err)
	} else {
		log.Printf("Hub INFO: Published new_like_received notification via Redis for user %d.", recipientUserID)
	}
}

func (h *Hub) BroadcastNewMatch(targetUserID int32, matchInfo WsMatchInfo) {
	wsMsg := WsMessage{Type: "new_match", MatchInfo: &matchInfo}
	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed marshal new_match msg for %d: %v", targetUserID, err)
		return
	}
	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeDirect,
		TargetUserID:    &targetUserID,
		OriginalPayload: messageBytes,
		SenderUserID:    matchInfo.MatchedUserID,
	}
	err = h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish new_match message for user %d via Redis: %v", targetUserID, err)
	} else {
		log.Printf("Hub INFO: Published new_match notification via Redis for user %d.", targetUserID)
	}
}

func (h *Hub) BroadcastLikeRemoved(recipientUserID int32, removalInfo WsLikeRemovalInfo) {
	wsMsg := WsMessage{Type: "like_removed", RemovalInfo: &removalInfo}
	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed marshal like_removed msg for %d: %v", recipientUserID, err)
		return
	}
	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeDirect,
		TargetUserID:    &recipientUserID,
		OriginalPayload: messageBytes,
		SenderUserID:    removalInfo.LikerUserID,
	}
	err = h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish like_removed message for user %d via Redis: %v", recipientUserID, err)
	} else {
		log.Printf("Hub INFO: Published like_removed notification via Redis for user %d.", recipientUserID)
	}
}

func (h *Hub) BroadcastMatchRemoved(recipientUserID int32, unmatcherUserID int32) {
	payload := &WsLikeRemovalInfo{LikerUserID: unmatcherUserID}
	wsMsg := WsMessage{Type: "match_removed", RemovalInfo: payload}
	messageBytes, err := json.Marshal(wsMsg)
	if err != nil {
		log.Printf("Hub ERROR: Failed marshal match_removed msg for %d: %v", recipientUserID, err)
		return
	}
	redisMsg := RedisWsMessage{
		Type:            RedisMsgTypeDirect,
		TargetUserID:    &recipientUserID,
		OriginalPayload: messageBytes,
		SenderUserID:    unmatcherUserID,
	}
	err = h.publishToRedis(context.Background(), redisMsg)
	if err != nil {
		log.Printf("Hub WARN: Failed to publish match_removed message for user %d via Redis: %v", recipientUserID, err)
	} else {
		log.Printf("Hub INFO: Published match_removed notification via Redis for user %d.", recipientUserID)
	}
}
