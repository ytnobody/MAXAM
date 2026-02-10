package ws

import (
	"sync"
)

// Hub manages WebSocket client connections and message broadcasting
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	history    []*Message
	maxHistory int
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		history:    make([]*Message, 0, 100),
		maxHistory: 100,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			// Send join notification
			joinMsg := NewJoinMessage(client.ID)
			h.broadcastMessage(joinMsg)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

			// Send leave notification
			leaveMsg := NewLeaveMessage(client.ID)
			h.broadcastMessage(leaveMsg)

		case message := <-h.broadcast:
			h.addToHistory(message)
			h.broadcastMessage(message)
		}
	}
}

// broadcastMessage sends a message to all connected clients
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			// Client send buffer full, skip
		}
	}
}

// addToHistory adds a message to the history buffer
func (h *Hub) addToHistory(message *Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Only store chat messages in history
	if message.Type != TypeChat {
		return
	}

	h.history = append(h.history, message)

	// Trim history if needed
	if len(h.history) > h.maxHistory {
		h.history = h.history[len(h.history)-h.maxHistory:]
	}
}

// GetHistory returns a copy of the message history
func (h *Hub) GetHistory() []*Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*Message, len(h.history))
	copy(result, h.history)
	return result
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message *Message) {
	h.broadcast <- message
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
