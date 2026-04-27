package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/nats"
)

// NATSClient interface for NATS operations needed by the notifier
type NATSClient interface {
	Subscribe(subject string, handler func(ctx context.Context, msg *nats.Message) error) (interface{}, error)
	Close() error
	IsConnected() bool
}

// NotificationType represents the type of notification
type NotificationType string

const (
	// Work item notifications
	NotificationWorkItemCreated       NotificationType = "work_item_created"
	NotificationWorkItemUpdated       NotificationType = "work_item_updated"
	NotificationWorkItemDeleted       NotificationType = "work_item_deleted"
	NotificationWorkItemAssigned      NotificationType = "work_item_assigned"
	NotificationWorkItemStatusChanged NotificationType = "work_item_status_changed"

	// Sprint notifications
	NotificationSprintCreated       NotificationType = "sprint_created"
	NotificationSprintUpdated       NotificationType = "sprint_updated"
	NotificationSprintClosed        NotificationType = "sprint_closed"
	NotificationSprintStatusChanged NotificationType = "sprint_status_changed"

	// Comment notifications
	NotificationCommentAdded   NotificationType = "comment_added"
	NotificationCommentUpdated NotificationType = "comment_updated"
	NotificationCommentDeleted NotificationType = "comment_deleted"
)

// Notification represents a real-time notification
type Notification struct {
	ID         string           `json:"id"`
	Type       NotificationType `json:"type"`
	EntityID   string           `json:"entity_id"`
	EntityType string           `json:"entity_type"`       // "work_item", "sprint", "comment"
	UserID     string           `json:"user_id,omitempty"` // User who triggered the action
	Data       interface{}      `json:"data"`
	TraceID    string           `json:"trace_id"`
	Room       string           `json:"room,omitempty"` // Sprint ID or project ID for room-based notifications
}

// Notifier handles real-time notifications via WebSocket and NATS
type Notifier struct {
	hub        *Hub
	natsClient NATSClient
	logger     *slog.Logger
}

// NewNotifier creates a new Notifier instance
func NewNotifier(hub *Hub, natsClient NATSClient, logger *slog.Logger) *Notifier {
	return &Notifier{
		hub:        hub,
		natsClient: natsClient,
		logger:     logger,
	}
}

// Start starts the notifier and subscribes to NATS events
func (n *Notifier) Start(ctx context.Context) error {
	// Subscribe to work item events
	if err := n.subscribeToWorkItemEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to work item events: %w", err)
	}

	// Subscribe to sprint events
	if err := n.subscribeToSprintEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to sprint events: %w", err)
	}

	// Subscribe to comment events
	if err := n.subscribeToCommentEvents(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to comment events: %w", err)
	}

	n.logger.Info("notifier started and subscribed to NATS events")
	return nil
}

// subscribeToWorkItemEvents subscribes to work item NATS events
func (n *Notifier) subscribeToWorkItemEvents(ctx context.Context) error {
	// Subscribe to work item created events
	if _, err := n.natsClient.Subscribe("sprint.*.workitem.created", n.handleWorkItemCreatedNATS); err != nil {
		return err
	}

	// Subscribe to work item updated events
	if _, err := n.natsClient.Subscribe("sprint.*.workitem.updated", n.handleWorkItemUpdatedNATS); err != nil {
		return err
	}

	// Subscribe to work item deleted events
	if _, err := n.natsClient.Subscribe("sprint.*.workitem.deleted", n.handleWorkItemDeletedNATS); err != nil {
		return err
	}

	return nil
}

// subscribeToSprintEvents subscribes to sprint NATS events
func (n *Notifier) subscribeToSprintEvents(ctx context.Context) error {
	// Subscribe to sprint created events
	if _, err := n.natsClient.Subscribe("sprint.*.sprint.created", n.handleSprintCreatedNATS); err != nil {
		return err
	}

	// Subscribe to sprint updated events
	if _, err := n.natsClient.Subscribe("sprint.*.sprint.updated", n.handleSprintUpdatedNATS); err != nil {
		return err
	}

	// Subscribe to sprint closed events
	if _, err := n.natsClient.Subscribe("sprint.*.sprint.closed", n.handleSprintClosedNATS); err != nil {
		return err
	}

	return nil
}

// subscribeToCommentEvents subscribes to comment NATS events
func (n *Notifier) subscribeToCommentEvents(ctx context.Context) error {
	// Subscribe to comment added events
	if _, err := n.natsClient.Subscribe("sprint.*.comment.added", n.handleCommentAddedNATS); err != nil {
		return err
	}

	// Subscribe to comment updated events
	if _, err := n.natsClient.Subscribe("sprint.*.comment.updated", n.handleCommentUpdatedNATS); err != nil {
		return err
	}

	// Subscribe to comment deleted events
	if _, err := n.natsClient.Subscribe("sprint.*.comment.deleted", n.handleCommentDeletedNATS); err != nil {
		return err
	}

	return nil
}

// NATS message handlers that wrap the actual handlers
func (n *Notifier) handleWorkItemCreatedNATS(ctx context.Context, msg *nats.Message) error {
	// msg.Data is already a map[string]interface{}, just marshal it to JSON
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleWorkItemCreated(ctx, data)
}

func (n *Notifier) handleWorkItemUpdatedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleWorkItemUpdated(ctx, data)
}

func (n *Notifier) handleWorkItemDeletedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleWorkItemDeleted(ctx, data)
}

func (n *Notifier) handleSprintCreatedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleSprintCreated(ctx, data)
}

func (n *Notifier) handleSprintUpdatedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleSprintUpdated(ctx, data)
}

func (n *Notifier) handleSprintClosedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleSprintClosed(ctx, data)
}

func (n *Notifier) handleCommentAddedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleCommentAdded(ctx, data)
}

func (n *Notifier) handleCommentUpdatedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleCommentUpdated(ctx, data)
}

func (n *Notifier) handleCommentDeletedNATS(ctx context.Context, msg *nats.Message) error {
	data, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return n.handleCommentDeleted(ctx, data)
}

// handleWorkItemCreated handles work item created events
func (n *Notifier) handleWorkItemCreated(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationWorkItemCreated,
		EntityType: "work_item",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "sprint_id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleWorkItemUpdated handles work item updated events
func (n *Notifier) handleWorkItemUpdated(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationWorkItemUpdated,
		EntityType: "work_item",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "sprint_id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleWorkItemDeleted handles work item deleted events
func (n *Notifier) handleWorkItemDeleted(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationWorkItemDeleted,
		EntityType: "work_item",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "sprint_id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleSprintCreated handles sprint created events
func (n *Notifier) handleSprintCreated(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationSprintCreated,
		EntityType: "sprint",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "id"), // Sprint ID is the room
	}

	return n.sendNotification(ctx, notification)
}

// handleSprintUpdated handles sprint updated events
func (n *Notifier) handleSprintUpdated(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationSprintUpdated,
		EntityType: "sprint",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleSprintClosed handles sprint closed events
func (n *Notifier) handleSprintClosed(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationSprintClosed,
		EntityType: "sprint",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleCommentAdded handles comment added events
func (n *Notifier) handleCommentAdded(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationCommentAdded,
		EntityType: "comment",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "work_item_id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleCommentUpdated handles comment updated events
func (n *Notifier) handleCommentUpdated(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationCommentUpdated,
		EntityType: "comment",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "work_item_id"),
	}

	return n.sendNotification(ctx, notification)
}

// handleCommentDeleted handles comment deleted events
func (n *Notifier) handleCommentDeleted(ctx context.Context, data []byte) error {
	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}

	notification := Notification{
		ID:         uuid.New().String(),
		Type:       NotificationCommentDeleted,
		EntityType: "comment",
		EntityID:   getStringField(event, "id"),
		UserID:     getStringField(event, "user_id"),
		Data:       event,
		TraceID:    getStringField(event, "trace_id"),
		Room:       getStringField(event, "work_item_id"),
	}

	return n.sendNotification(ctx, notification)
}

// sendNotification sends a notification via WebSocket
func (n *Notifier) sendNotification(ctx context.Context, notification Notification) error {
	msg := Message{
		Type:    "notification",
		Data:    notification,
		TraceID: notification.TraceID,
	}

	// If room is specified, broadcast to room
	if notification.Room != "" {
		if err := n.hub.BroadcastToRoomMessage(notification.Room, msg); err != nil {
			n.logger.Error("failed to broadcast notification to room",
				slog.String("notification_id", notification.ID),
				slog.String("room", notification.Room),
				slog.String("error", err.Error()),
			)
			return err
		}
	} else {
		// Otherwise broadcast to all clients
		if err := n.hub.BroadcastMessage(msg); err != nil {
			n.logger.Error("failed to broadcast notification",
				slog.String("notification_id", notification.ID),
				slog.String("error", err.Error()),
			)
			return err
		}
	}

	n.logger.Debug("notification sent",
		slog.String("notification_id", notification.ID),
		slog.String("type", string(notification.Type)),
		slog.String("entity_id", notification.EntityID),
		slog.String("room", notification.Room),
	)

	return nil
}

// SendNotification sends a notification directly (for use outside NATS events)
func (n *Notifier) SendNotification(ctx context.Context, notification Notification) error {
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}
	return n.sendNotification(ctx, notification)
}

// getStringField safely extracts a string field from a map
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
