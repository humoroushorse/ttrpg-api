package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewNotifier(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	if notifier == nil {
		t.Fatal("NewNotifier returned nil")
	}

	if notifier.hub != hub {
		t.Error("notifier hub not set correctly")
	}

	if notifier.natsClient != natsClient {
		t.Error("notifier natsClient not set correctly")
	}
}

func TestNotificationTypes(t *testing.T) {
	tests := []struct {
		name string
		typ  NotificationType
		want string
	}{
		{"work item created", NotificationWorkItemCreated, "work_item_created"},
		{"work item updated", NotificationWorkItemUpdated, "work_item_updated"},
		{"work item deleted", NotificationWorkItemDeleted, "work_item_deleted"},
		{"sprint created", NotificationSprintCreated, "sprint_created"},
		{"sprint updated", NotificationSprintUpdated, "sprint_updated"},
		{"sprint closed", NotificationSprintClosed, "sprint_closed"},
		{"comment added", NotificationCommentAdded, "comment_added"},
		{"comment updated", NotificationCommentUpdated, "comment_updated"},
		{"comment deleted", NotificationCommentDeleted, "comment_deleted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.typ) != tt.want {
				t.Errorf("expected %s, got %s", tt.want, string(tt.typ))
			}
		})
	}
}

func TestNotificationSerialization(t *testing.T) {
	notification := Notification{
		ID:         "notif-123",
		Type:       NotificationWorkItemCreated,
		EntityID:   "work-456",
		EntityType: "work_item",
		UserID:     "user-789",
		Data:       map[string]string{"title": "Test Work Item"},
		TraceID:    "trace-abc",
		Room:       "sprint-xyz",
	}

	data, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("failed to marshal notification: %v", err)
	}

	var decoded Notification
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal notification: %v", err)
	}

	if decoded.ID != notification.ID {
		t.Errorf("expected ID %s, got %s", notification.ID, decoded.ID)
	}

	if decoded.Type != notification.Type {
		t.Errorf("expected type %s, got %s", notification.Type, decoded.Type)
	}

	if decoded.EntityID != notification.EntityID {
		t.Errorf("expected entity_id %s, got %s", notification.EntityID, decoded.EntityID)
	}

	if decoded.EntityType != notification.EntityType {
		t.Errorf("expected entity_type %s, got %s", notification.EntityType, decoded.EntityType)
	}

	if decoded.UserID != notification.UserID {
		t.Errorf("expected user_id %s, got %s", notification.UserID, decoded.UserID)
	}

	if decoded.TraceID != notification.TraceID {
		t.Errorf("expected trace_id %s, got %s", notification.TraceID, decoded.TraceID)
	}

	if decoded.Room != notification.Room {
		t.Errorf("expected room %s, got %s", notification.Room, decoded.Room)
	}
}

func TestSendNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create notification without ID
	notification := Notification{
		Type:       NotificationWorkItemCreated,
		EntityID:   "work-123",
		EntityType: "work_item",
		UserID:     "user-456",
		Data:       map[string]string{"title": "Test"},
		TraceID:    "trace-789",
	}

	err := notifier.SendNotification(ctx, notification)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	// Note: The ID is generated inside SendNotification on a copy,
	// not on the original notification object. This is expected behavior.

	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestSendNotificationWithRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create notification with room
	notification := Notification{
		Type:       NotificationSprintCreated,
		EntityID:   "sprint-123",
		EntityType: "sprint",
		UserID:     "user-456",
		Data:       map[string]string{"name": "Sprint 1"},
		TraceID:    "trace-789",
		Room:       "sprint-123",
	}

	err := notifier.SendNotification(ctx, notification)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestGetStringField(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{
			name: "existing string field",
			m:    map[string]interface{}{"key": "value"},
			key:  "key",
			want: "value",
		},
		{
			name: "non-existing field",
			m:    map[string]interface{}{"key": "value"},
			key:  "missing",
			want: "",
		},
		{
			name: "non-string field",
			m:    map[string]interface{}{"key": 123},
			key:  "key",
			want: "",
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			key:  "key",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringField(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("getStringField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleWorkItemCreated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create event data
	event := map[string]interface{}{
		"id":        "work-123",
		"user_id":   "user-456",
		"trace_id":  "trace-789",
		"sprint_id": "sprint-abc",
		"title":     "Test Work Item",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Handle event
	err = notifier.handleWorkItemCreated(ctx, data)
	if err != nil {
		t.Fatalf("handleWorkItemCreated failed: %v", err)
	}

	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestHandleSprintClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create event data
	event := map[string]interface{}{
		"id":       "sprint-123",
		"user_id":  "user-456",
		"trace_id": "trace-789",
		"name":     "Sprint 1",
		"status":   "closed",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Handle event
	err = notifier.handleSprintClosed(ctx, data)
	if err != nil {
		t.Fatalf("handleSprintClosed failed: %v", err)
	}

	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestHandleCommentAdded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	hub := NewHub(ctx, logger)
	natsClient := NewMockNATSClient()

	notifier := NewNotifier(hub, natsClient, logger)

	// Start hub
	go hub.Run()
	time.Sleep(10 * time.Millisecond)

	// Create event data
	event := map[string]interface{}{
		"id":           "comment-123",
		"user_id":      "user-456",
		"trace_id":     "trace-789",
		"work_item_id": "work-abc",
		"content":      "Test comment",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Handle event
	err = notifier.handleCommentAdded(ctx, data)
	if err != nil {
		t.Fatalf("handleCommentAdded failed: %v", err)
	}

	cancel()
	time.Sleep(10 * time.Millisecond)
}
