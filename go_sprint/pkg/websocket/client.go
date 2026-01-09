package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512 KB
)

// Client is a middleman between the websocket connection and the hub
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// User ID for authentication
	userID string

	// Rooms the client is subscribed to
	rooms []string

	// Logger
	logger *slog.Logger
}

// NewClient creates a new Client instance
func NewClient(hub *Hub, conn *websocket.Conn, userID string, logger *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		rooms:  make([]string, 0),
		logger: logger,
	}
}

// readPump pumps messages from the websocket connection to the hub
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("websocket read error",
					slog.String("user_id", c.userID),
					slog.String("error", err.Error()),
				)
			}
			break
		}

		// Handle incoming messages
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the websocket connection
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Client) handleMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		c.logger.Error("failed to unmarshal message",
			slog.String("user_id", c.userID),
			slog.String("error", err.Error()),
		)
		return
	}

	c.logger.Debug("received message",
		slog.String("user_id", c.userID),
		slog.String("type", msg.Type),
		slog.String("room", msg.Room),
	)

	// Handle different message types
	switch msg.Type {
	case "join_room":
		if room, ok := msg.Data.(string); ok {
			c.hub.JoinRoom(c, room)
			c.rooms = append(c.rooms, room)
		}

	case "leave_room":
		if room, ok := msg.Data.(string); ok {
			c.hub.LeaveRoom(c, room)
			// Remove room from client's rooms list
			for i, r := range c.rooms {
				if r == room {
					c.rooms = append(c.rooms[:i], c.rooms[i+1:]...)
					break
				}
			}
		}

	case "ping":
		// Respond with pong
		pongMsg := Message{
			Type:      "pong",
			TraceID:   msg.TraceID,
			Timestamp: time.Now(),
		}
		if data, err := json.Marshal(pongMsg); err == nil {
			c.send <- data
		}

	default:
		c.logger.Warn("unknown message type",
			slog.String("user_id", c.userID),
			slog.String("type", msg.Type),
		)
	}
}

// Send sends a message to the client
func (c *Client) Send(message []byte) {
	select {
	case c.send <- message:
	default:
		c.logger.Warn("client send channel full",
			slog.String("user_id", c.userID),
		)
	}
}

// GetUserID returns the client's user ID
func (c *Client) GetUserID() string {
	return c.userID
}

// GetRooms returns the rooms the client is subscribed to
func (c *Client) GetRooms() []string {
	return c.rooms
}
