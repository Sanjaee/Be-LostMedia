package websocket

import (
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to clients
type Hub struct {
	// Registered clients by user ID
	clients map[string]map[*Client]bool

	// Inbound messages from the clients
	broadcast chan *Message

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Callback for user presence changes (userID, online bool)
	onPresenceChange func(userID string, online bool)
}

// Message represents a WebSocket message
type Message struct {
	UserID  string                 `json:"user_id,omitempty"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// SetPresenceCallback sets a callback for when users come online/offline
func (h *Hub) SetPresenceCallback(cb func(userID string, online bool)) {
	h.onPresenceChange = cb
}

// GetOnlineUserIDs returns a list of all currently connected user IDs
func (h *Hub) GetOnlineUserIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.clients))
	for uid := range h.clients {
		ids = append(ids, uid)
	}
	return ids
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			isNew := h.clients[client.UserID] == nil || len(h.clients[client.UserID]) == 0
			if h.clients[client.UserID] == nil {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			h.clients[client.UserID][client] = true
			h.mu.Unlock()
			log.Printf("Client registered: UserID=%s, Total clients for user: %d", client.UserID, len(h.clients[client.UserID]))

			// Broadcast presence: user came online (only if first connection)
			if isNew {
				h.broadcastPresence(client.UserID, true)
				if h.onPresenceChange != nil {
					h.onPresenceChange(client.UserID, true)
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			wasLast := false
			if clients, ok := h.clients[client.UserID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.UserID)
						wasLast = true
					}
				}
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: UserID=%s", client.UserID)

			// Broadcast presence: user went offline (only if last connection closed)
			if wasLast {
				h.broadcastPresence(client.UserID, false)
				if h.onPresenceChange != nil {
					h.onPresenceChange(client.UserID, false)
				}
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			if message.UserID != "" {
				// Send to specific user
				if clients, ok := h.clients[message.UserID]; ok {
					for client := range clients {
						select {
						case client.send <- message:
						default:
							close(client.send)
							delete(clients, client)
						}
					}
				}
			} else {
				// Broadcast to all clients
				for _, clients := range h.clients {
					for client := range clients {
						select {
						case client.send <- message:
						default:
							close(client.send)
							delete(clients, client)
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID string, payload map[string]interface{}) {
	message := &Message{
		UserID:  userID,
		Type:    "notification",
		Payload: payload,
	}

	select {
	case h.broadcast <- message:
	default:
		log.Printf("Broadcast channel full, dropping message for user: %s", userID)
	}
}

// BroadcastToAll sends a message to all connected clients
func (h *Hub) BroadcastToAll(payload map[string]interface{}) {
	message := &Message{
		Type:    "broadcast",
		Payload: payload,
	}

	select {
	case h.broadcast <- message:
	default:
		log.Printf("Broadcast channel full, dropping broadcast message")
	}
}

// GetClientCount returns the number of connected clients for a user
func (h *Hub) GetClientCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.clients[userID]; ok {
		return len(clients)
	}
	return 0
}

// GetTotalClientCount returns the total number of connected clients
func (h *Hub) GetTotalClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}

// broadcastPresence broadcasts a user_presence event to all connected clients
func (h *Hub) broadcastPresence(userID string, online bool) {
	h.BroadcastToAll(map[string]interface{}{
		"type":    "user_presence",
		"user_id": userID,
		"online":  online,
	})
}
