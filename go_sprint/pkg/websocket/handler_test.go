package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewHandler(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()

	handler := NewHandler(hub, authValidator, logger)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.hub != hub {
		t.Error("handler hub not set correctly")
	}

	if handler.authValidator != authValidator {
		t.Error("handler authValidator not set correctly")
	}
}

func TestHandlerExtractToken(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()
	handler := NewHandler(hub, authValidator, logger)

	tests := []struct {
		name     string
		setupReq func() *http.Request
		want     string
	}{
		{
			name: "token from query parameter",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws?token=test-token", nil)
				return req
			},
			want: "test-token",
		},
		{
			name: "token from Authorization header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws", nil)
				req.Header.Set("Authorization", "Bearer test-token")
				return req
			},
			want: "test-token",
		},
		{
			name: "token from Authorization header without Bearer",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws", nil)
				req.Header.Set("Authorization", "test-token")
				return req
			},
			want: "test-token",
		},
		{
			name: "no token",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws", nil)
				return req
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			got := handler.extractToken(req)
			if got != tt.want {
				t.Errorf("extractToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandlerServeHTTP_NoToken(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()
	handler := NewHandler(hub, authValidator, logger)

	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "no token provided") {
		t.Errorf("expected error message about no token, got: %s", body)
	}
}

func TestHandlerServeHTTP_InvalidToken(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()
	handler := NewHandler(hub, authValidator, logger)

	req := httptest.NewRequest("GET", "/ws?token=invalid-token", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "invalid token") {
		t.Errorf("expected error message about invalid token, got: %s", body)
	}
}

func TestHandlerGetHub(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()
	handler := NewHandler(hub, authValidator, logger)

	gotHub := handler.GetHub()
	if gotHub != hub {
		t.Error("GetHub() returned wrong hub instance")
	}
}

func TestSimpleAuthValidator(t *testing.T) {
	validator := NewSimpleAuthValidator()

	// Test with no tokens
	_, err := validator.ValidateToken(context.Background(), "test-token")
	if err == nil {
		t.Error("expected error for invalid token, got nil")
	}

	// Add a valid token
	validator.AddToken("valid-token", "user-123")

	// Test with valid token
	userID, err := validator.ValidateToken(context.Background(), "valid-token")
	if err != nil {
		t.Errorf("expected no error for valid token, got: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("expected user ID 'user-123', got: %s", userID)
	}

	// Remove token
	validator.RemoveToken("valid-token")

	// Test after removal
	_, err = validator.ValidateToken(context.Background(), "valid-token")
	if err == nil {
		t.Error("expected error after token removal, got nil")
	}
}

// Integration test with actual WebSocket connection
func TestWebSocketConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	authValidator := NewSimpleAuthValidator()
	authValidator.AddToken("test-token", "user-123")

	handler := NewHandler(hub, authValidator, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=test-token"

	// Connect to WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to websocket: %v", err)
	}
	defer ws.Close()

	// Give time for client to register
	time.Sleep(50 * time.Millisecond)

	// Check hub stats
	stats := hub.GetStats()
	if stats["total_clients"] != 1 {
		t.Errorf("expected 1 client, got %v", stats["total_clients"])
	}

	// Close connection
	ws.Close()
	time.Sleep(50 * time.Millisecond)

	// Check hub stats after disconnect
	stats = hub.GetStats()
	if stats["total_clients"] != 0 {
		t.Errorf("expected 0 clients after disconnect, got %v", stats["total_clients"])
	}
}
