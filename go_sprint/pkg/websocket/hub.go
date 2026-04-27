package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Room-based broadcasting
	rooms map[string]map[*Client]bool

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Logger
	logger *slog.Logger

	// Context for graceful shutdown
	ctx context.Context
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Room      string      `json:"room,omitempty"`
	Data      interface{} `json:"data"`
	TraceID   string      `json:"trace_id"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewHub creates a new Hub instance
func NewHub(ctx context.Context, logger *slog.Logger) *Hub {
	return &Hub{
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		logger:     logger,
		ctx:        ctx,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	h.logger.Info("WebSocket hub started")
	defer h.logger.Info("WebSocket hub stopped")

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info("WebSocket hub shutting down")
			h.shutdown()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient registers a new client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	h.logger.Info("client registered",
		slog.String("user_id", client.userID),
		slog.Int("total_clients", len(h.clients)),
	)
}

// unregisterClient unregisters a client
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		// Remove client from all rooms
		for room := range h.rooms {
			if clients, ok := h.rooms[room]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.rooms, room)
				}
			}
		}

		h.logger.Info("client unregistered",
			slog.String("user_id", client.userID),
			slog.Int("total_clients", len(h.clients)),
		)
	}
}

// broadcastMessage broadcasts a message to all clients
func (h *Hub) broadcastMessage(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			// Client's send channel is full, close it
			close(client.send)
			delete(h.clients, client)
		}
	}
}

// BroadcastToRoom broadcasts a message to all clients in a specific room
func (h *Hub) BroadcastToRoom(room string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[room]; ok {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				// Client's send channel is full, skip
				h.logger.Warn("failed to send message to client",
					slog.String("user_id", client.userID),
					slog.String("room", room),
				)
			}
		}
	}
}

// JoinRoom adds a client to a room
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true

	h.logger.Info("client joined room",
		slog.String("user_id", client.userID),
		slog.String("room", room),
		slog.Int("room_clients", len(h.rooms[room])),
	)
}

// LeaveRoom removes a client from a room
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}

		h.logger.Info("client left room",
			slog.String("user_id", client.userID),
			slog.String("room", room),
		)
	}
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.userID == userID {
			select {
			case client.send <- message:
			default:
				h.logger.Warn("failed to send message to user",
					slog.String("user_id", userID),
				)
			}
		}
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"total_clients": len(h.clients),
		"total_rooms":   len(h.rooms),
	}
}

// shutdown gracefully shuts down the hub
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all client connections
	for client := range h.clients {
		close(client.send)
		client.conn.Close()
	}

	// Clear all data structures
	h.clients = make(map[*Client]bool)
	h.rooms = make(map[string]map[*Client]bool)
}

// BroadcastMessage broadcasts a structured message to all clients
func (h *Hub) BroadcastMessage(msg Message) error {
	msg.Timestamp = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.broadcast <- data
	return nil
}

// BroadcastToRoomMessage broadcasts a structured message to a specific room
func (h *Hub) BroadcastToRoomMessage(room string, msg Message) error {
	msg.Timestamp = time.Now()
	msg.Room = room
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.BroadcastToRoom(room, data)
	return nil
}

// SendToUserMessage sends a structured message to a specific user
func (h *Hub) SendToUserMessage(userID string, msg Message) error {
	msg.Timestamp = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.SendToUser(userID, data)
	return nil
}
