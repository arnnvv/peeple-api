package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/arnnvv/peeple-api/pkg/token"
)

// SSE client representation
type Client struct {
	id          string
	w           http.ResponseWriter
	flusher     http.Flusher
	closeChan   chan struct{}
	lastEventID string
}

// Event structure for SSE
type Event struct {
	ID        string    `json:"id"`
	EventType string    `json:"event"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	// Map of active client connections
	clients      = make(map[string]*Client)
	clientsMutex sync.Mutex

	// Channel for broadcasting events
	eventChan = make(chan Event, 10)
)

// Function to broadcast events to all connected admin clients
func broadcastToAdmins(event Event) {
	eventJSON, err := json.Marshal(event.Data)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return
	}

	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	for _, client := range clients {
		select {
		case <-client.closeChan:
			// Client has disconnected
			continue
		default:
			// Send event to client
			fmt.Fprintf(client.w, "id: %s\n", event.ID)
			fmt.Fprintf(client.w, "event: %s\n", event.EventType)
			fmt.Fprintf(client.w, "data: %s\n\n", eventJSON)
			client.lastEventID = event.ID
			client.flusher.Flush()
		}
	}
}

// Handler for admin SSE events
func AdminEventsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if client supports SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Get claims from context (already validated by middleware)
	claims, ok := r.Context().Value(token.ClaimsContextKey).(*token.Claims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a client
	clientID := fmt.Sprintf("%d-%d", claims.UserID, time.Now().UnixNano())
	client := &Client{
		id:        clientID,
		w:         w,
		flusher:   flusher,
		closeChan: make(chan struct{}),
	}

	// Register client
	clientsMutex.Lock()
	clients[clientID] = client
	clientsMutex.Unlock()

	// Remove client when connection closes
	defer func() {
		clientsMutex.Lock()
		delete(clients, clientID)
		clientsMutex.Unlock()
	}()

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\n")
	fmt.Fprintf(w, "data: {\"message\": \"Connected to admin events\"}\n\n")
	flusher.Flush()

	// Keep connection alive with heartbeats and check for client disconnect
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send heartbeat
			fmt.Fprintf(w, ": heartbeat %s\n\n", time.Now().Format(time.RFC3339))
			flusher.Flush()
		case <-r.Context().Done():
			// Client disconnected
			close(client.closeChan)
			return
		}
	}
}

// Function to create and broadcast a minimal verification event
func BroadcastVerificationEvent(userID uint, photoURL string, status string) {
	eventID := fmt.Sprintf("ping-%d", time.Now().UnixNano())

	// Minimal data - just a ping value of 1
	data := 1

	event := Event{
		ID:        eventID,
		EventType: "ping", // Simple event type
		Data:      data,   // Just send the number 1
		Timestamp: time.Now(),
	}

	// Send to event channel
	go broadcastToAdmins(event)
}
