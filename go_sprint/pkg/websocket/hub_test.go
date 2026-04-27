package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hub := NewHub(ctx, logger)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}

	if hub.rooms == nil {
		t.Error("rooms map not initialized")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}

	if hub.register == nil {
		t.Error("register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("unregister channel not initialized")
	}
}

func TestHubGetStats(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hub := NewHub(ctx, logger)

	stats := hub.GetStats()

	if stats["total_clients"] != 0 {
		t.Errorf("expected 0 clients, got %v", stats["total_clients"])
	}

	if stats["total_rooms"] != 0 {
		t.Errorf("expected 0 rooms, got %v", stats["total_rooms"])
	}
}

func TestHubBroadcastMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Create test message
	msg := Message{
		Type:    "test",
		Data:    "test data",
		TraceID: "test-trace-id",
	}

	err := hub.BroadcastMessage(msg)
	if err != nil {
		t.Fatalf("BroadcastMessage failed: %v", err)
	}

	// Note: The timestamp is set on the message copy inside BroadcastMessage,
	// not on the original message passed in. This is expected behavior.

	// Cancel context to stop hub
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestHubBroadcastToRoomMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Create test message
	msg := Message{
		Type:    "test",
		Data:    "test data",
		TraceID: "test-trace-id",
	}

	room := "sprint-123"
	err := hub.BroadcastToRoomMessage(room, msg)
	if err != nil {
		t.Fatalf("BroadcastToRoomMessage failed: %v", err)
	}

	// Note: The room is set on the message copy inside BroadcastToRoomMessage,
	// not on the original message passed in. This is expected behavior.

	// Cancel context to stop hub
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestHubSendToUserMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Create test message
	msg := Message{
		Type:    "test",
		Data:    "test data",
		TraceID: "test-trace-id",
	}

	userID := "user-123"
	err := hub.SendToUserMessage(userID, msg)
	if err != nil {
		t.Fatalf("SendToUserMessage failed: %v", err)
	}

	// Cancel context to stop hub
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestMessageSerialization(t *testing.T) {
	msg := Message{
		Type:      "work_item_created",
		Room:      "sprint-123",
		Data:      map[string]string{"id": "work-456"},
		TraceID:   "trace-789",
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("expected type %s, got %s", msg.Type, decoded.Type)
	}

	if decoded.Room != msg.Room {
		t.Errorf("expected room %s, got %s", msg.Room, decoded.Room)
	}

	if decoded.TraceID != msg.TraceID {
		t.Errorf("expected trace_id %s, got %s", msg.TraceID, decoded.TraceID)
	}
}

func TestHubShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	hub := NewHub(ctx, logger)

	// Start hub in background
	done := make(chan bool)
	go func() {
		hub.Run()
		done <- true
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for hub to shutdown with timeout
	select {
	case <-done:
		// Hub shutdown successfully
	case <-time.After(1 * time.Second):
		t.Fatal("hub did not shutdown within timeout")
	}

	// Verify hub was cleaned up
	stats := hub.GetStats()
	if stats["total_clients"] != 0 {
		t.Errorf("expected 0 clients after shutdown, got %v", stats["total_clients"])
	}

	if stats["total_rooms"] != 0 {
		t.Errorf("expected 0 rooms after shutdown, got %v", stats["total_rooms"])
	}
}
