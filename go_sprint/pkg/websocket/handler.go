package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Configure allowed origins from config
		// For now, allow all origins (should be restricted in production)
		return true
	},
}

// AuthValidator is an interface for validating JWT tokens
type AuthValidator interface {
	ValidateToken(ctx context.Context, token string) (userID string, err error)
}

// Handler handles WebSocket connection requests
type Handler struct {
	hub           *Hub
	authValidator AuthValidator
	logger        *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, authValidator AuthValidator, logger *slog.Logger) *Handler {
	return &Handler{
		hub:           hub,
		authValidator: authValidator,
		logger:        logger,
	}
}

// ServeHTTP handles WebSocket upgrade requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from query parameter or Authorization header
	token := h.extractToken(r)
	if token == "" {
		h.logger.Warn("websocket connection rejected: no token provided")
		http.Error(w, "Unauthorized: no token provided", http.StatusUnauthorized)
		return
	}

	// Validate token and get user ID
	userID, err := h.authValidator.ValidateToken(r.Context(), token)
	if err != nil {
		h.logger.Warn("websocket connection rejected: invalid token",
			slog.String("error", err.Error()),
		)
		http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade connection",
			slog.String("error", err.Error()),
		)
		return
	}

	// Create new client
	client := NewClient(h.hub, conn, userID, h.logger)

	// Register client with hub
	h.hub.register <- client

	h.logger.Info("websocket connection established",
		slog.String("user_id", userID),
		slog.String("remote_addr", r.RemoteAddr),
	)

	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// extractToken extracts JWT token from request
func (h *Handler) extractToken(r *http.Request) string {
	// Try query parameter first (for WebSocket connections)
	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	// Try Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Remove "Bearer " prefix if present
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		return authHeader
	}

	return ""
}

// GetHub returns the hub instance
func (h *Handler) GetHub() *Hub {
	return h.hub
}
