# NATS Subject Documentation - Sprint Management Service

This document describes all NATS subjects used by the Sprint Management Service for inter-service communication.

## Overview

The Sprint Management Service uses NATS for:
- Authentication token validation via Auth Service
- Real-time notifications for work item and sprint changes
- Inter-service communication for distributed operations

## Subject Pattern

All Sprint Service subjects follow the standardized pattern:

```
sprint.{trace_id}.{resource}.{action}.{status}
```

## Subject Definitions

### Work Item Operations

#### Create Work Item

**Request**: `sprint.{trace_id}.workitem.create.request`

**Publisher**: Sprint Service (internal)  
**Subscriber**: Sprint Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "work_item": {
    "type": "story",
    "title": "User login feature",
    "description": "Implement user authentication",
    "priority": "high",
    "story_points": 5,
    "assignee_id": "user-uuid",
    "sprint_id": "sprint-uuid"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Response**: `sprint.{trace_id}.workitem.create.response`

**Publisher**: Sprint Service  
**Subscriber**: Sprint Service (internal)

**Success Response**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "work_item": {
    "id": "work-item-uuid",
    "type": "story",
    "title": "User login feature",
    "status": "todo",
    "created_at": "2024-01-15T10:30:00Z"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Update Work Item

**Request**: `sprint.{trace_id}.workitem.update.request`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "work_item_id": "work-item-uuid",
  "updates": {
    "status": "in_progress",
    "assignee_id": "new-user-uuid"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Response**: `sprint.{trace_id}.workitem.update.response`

#### Delete Work Item

**Request**: `sprint.{trace_id}.workitem.delete.request`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "work_item_id": "work-item-uuid",
  "soft_delete": true,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Response**: `sprint.{trace_id}.workitem.delete.response`

### Sprint Operations

#### Create Sprint

**Request**: `sprint.{trace_id}.sprint.create.request`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "sprint": {
    "name": "Sprint 1",
    "description": "First sprint of the project",
    "start_date": "2024-01-15",
    "end_date": "2024-01-29",
    "capacity_points": 40
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Response**: `sprint.{trace_id}.sprint.create.response`

#### Close Sprint

**Request**: `sprint.{trace_id}.sprint.close.request`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "sprint_id": "sprint-uuid",
  "move_incomplete_to_backlog": true,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Response**: `sprint.{trace_id}.sprint.close.response`

**Success Response**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "sprint_id": "sprint-uuid",
  "status": "completed",
  "metrics": {
    "committed_points": 40,
    "completed_points": 35,
    "completion_rate": 87.5,
    "velocity": 35
  },
  "moved_items": ["work-item-1", "work-item-2"],
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Real-time Notifications

#### Work Item Created

**Event**: `sprint.{trace_id}.notification.workitem.created`

**Publisher**: Sprint Service  
**Subscriber**: WebSocket Service, Notification Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "workitem.created",
  "work_item": {
    "id": "work-item-uuid",
    "type": "story",
    "title": "User login feature",
    "assignee_id": "user-uuid",
    "sprint_id": "sprint-uuid"
  },
  "created_by": "user-uuid",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Work Item Updated

**Event**: `sprint.{trace_id}.notification.workitem.updated`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "workitem.updated",
  "work_item_id": "work-item-uuid",
  "changes": {
    "status": {
      "old": "todo",
      "new": "in_progress"
    },
    "assignee_id": {
      "old": "user-1",
      "new": "user-2"
    }
  },
  "updated_by": "user-uuid",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Sprint Status Changed

**Event**: `sprint.{trace_id}.notification.sprint.status_changed`

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "sprint.status_changed",
  "sprint_id": "sprint-uuid",
  "old_status": "active",
  "new_status": "completed",
  "changed_by": "user-uuid",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Authentication Integration

#### Token Validation Request

**Request**: `auth.{trace_id}.validate.request`

**Publisher**: Sprint Service  
**Subscriber**: Auth Service

See [Auth Service NATS Documentation](../../go_auth/docs/nats.md) for details.

## Wildcard Subscriptions

### Tracing All Sprint Messages

```
sprint.{trace_id}.>
```

Subscribe to all messages for a specific trace ID.

### All Work Item Notifications

```
sprint.*.notification.workitem.*
```

Subscribe to all work item notifications across all traces.

### All Sprint Notifications

```
sprint.*.notification.sprint.*
```

Subscribe to all sprint notifications across all traces.

### All Sprint Service Messages

```
sprint.>
```

Subscribe to all messages published by the Sprint Service.

## Message Headers

All NATS messages include standard headers:

```go
headers := nats.Header{
    "Trace-ID":       traceID,
    "Correlation-ID": correlationID,
    "Timestamp":      time.Now().UTC().Format(time.RFC3339),
    "Service":        "sprint",
    "Version":        "v1",
    "Content-Type":   "application/json",
    "User-ID":        userID,
}
```

## Error Handling

### Error Codes

- `WORK_ITEM_NOT_FOUND`: Work item does not exist
- `SPRINT_NOT_FOUND`: Sprint does not exist
- `INVALID_STATUS_TRANSITION`: Invalid status change
- `SPRINT_CLOSED`: Cannot modify closed sprint
- `DEPENDENCY_VIOLATION`: Operation violates dependencies
- `VALIDATION_ERROR`: Input validation failed
- `UNAUTHORIZED`: User lacks permissions
- `INTERNAL_ERROR`: Internal server error

### Error Response Format

```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "field": "additional context"
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## NATS Client Wrapper

The Sprint Service provides a NATS client wrapper (`pkg/nats`) that simplifies NATS operations with:

- Automatic trace ID correlation
- Standardized message format
- Connection management with reconnection logic
- Built-in logging and error handling
- Subject pattern helpers

### Client Configuration

```go
import (
    "log/slog"
    "time"
    
    natsclient "github.com/humoroushorse/go_sprint/pkg/nats"
)

cfg := natsclient.Config{
    URL:            "nats://localhost:4222",
    ReconnectWait:  2 * time.Second,
    MaxReconnects:  10,
    ConnectionName: "sprint-service",
    RequestTimeout: 5 * time.Second,
}

logger := slog.Default()

client, err := natsclient.NewClient(cfg, logger)
if err != nil {
    log.Fatalf("Failed to create NATS client: %v", err)
}
defer client.Close()
```

### Subject Pattern Helpers

The client provides helper functions for constructing standardized subjects:

```go
import natsclient "github.com/humoroushorse/go_sprint/pkg/nats"

// Work item subjects
subject := natsclient.WorkItemCreateRequest(traceID)
// Returns: "sprint.{trace_id}.workitem.create.request"

// Sprint subjects
subject := natsclient.SprintCloseRequest(traceID)
// Returns: "sprint.{trace_id}.sprint.close.request"

// Notification subjects
subject := natsclient.NotificationWorkItemCreated(traceID)
// Returns: "sprint.{trace_id}.notification.workitem.created"

// Wildcard patterns
pattern := natsclient.AllWorkItemMessages()
// Returns: "sprint.*.workitem.*.*"

pattern := natsclient.AllNotifications()
// Returns: "sprint.*.notification.*.*"
```

## Usage Examples

### Example 1: Publishing a Message

```go
package main

import (
    "context"
    "log"
    
    natsclient "github.com/humoroushorse/go_sprint/pkg/nats"
)

func PublishWorkItemCreated(client *natsclient.Client, ctx context.Context, workItem WorkItem) error {
    traceID := natsclient.GetTraceID(ctx)
    subject := natsclient.NotificationWorkItemCreated(traceID)
    
    data := map[string]interface{}{
        "work_item_id": workItem.ID,
        "type":         workItem.Type,
        "title":        workItem.Title,
        "created_by":   workItem.ReporterID,
    }
    
    return client.Publish(ctx, subject, data)
}
```

### Example 2: Request-Response Pattern

```go
func ValidateToken(client *natsclient.Client, ctx context.Context, token string) (*User, error) {
    traceID := natsclient.GetTraceID(ctx)
    subject := natsclient.AuthValidateRequest(traceID)
    
    requestData := map[string]interface{}{
        "token": token,
    }
    
    // Send request and wait for response
    response, err := client.Request(ctx, subject, requestData, 5*time.Second)
    if err != nil {
        return nil, fmt.Errorf("token validation failed: %w", err)
    }
    
    // Check for errors in response
    if errData, ok := response.Data["error"]; ok {
        return nil, fmt.Errorf("validation error: %v", errData)
    }
    
    // Parse user data from response
    userData := response.Data["user"]
    // ... parse user data ...
    
    return user, nil
}
```

### Example 3: Subscribing to Messages

```go
func SubscribeToWorkItemNotifications(client *natsclient.Client) error {
    // Subscribe to all work item notifications
    pattern := natsclient.AllWorkItemMessages()
    
    _, err := client.Subscribe(pattern, func(ctx context.Context, msg *natsclient.Message) error {
        traceID := msg.TraceID
        
        log.Printf("Received work item notification (trace: %s)", traceID)
        
        // Extract work item data
        workItemID, ok := msg.Data["work_item_id"].(string)
        if !ok {
            return fmt.Errorf("invalid work item ID")
        }
        
        eventType, ok := msg.Data["event_type"].(string)
        if !ok {
            return fmt.Errorf("invalid event type")
        }
        
        // Process notification
        log.Printf("Work item %s: %s", workItemID, eventType)
        
        // Broadcast to WebSocket clients
        broadcastToWebSocket(msg.Data)
        
        return nil
    })
    
    return err
}
```

### Example 4: Queue Subscription for Load Balancing

```go
func SubscribeToWorkItemProcessing(client *natsclient.Client) error {
    subject := natsclient.WorkItemCreateRequest("*")
    queueGroup := "work-item-processors"
    
    _, err := client.QueueSubscribe(subject, queueGroup, func(ctx context.Context, msg *natsclient.Message) error {
        // Multiple instances will share the load
        log.Printf("Processing work item creation (trace: %s)", msg.TraceID)
        
        // Extract request data
        workItemData := msg.Data["work_item"]
        
        // Process work item creation
        workItem, err := createWorkItem(ctx, workItemData)
        if err != nil {
            log.Printf("Failed to create work item: %v", err)
            return err
        }
        
        // Publish response
        responseSubject := natsclient.WorkItemCreateResponse(msg.TraceID)
        responseData := map[string]interface{}{
            "work_item": workItem,
            "success":   true,
        }
        
        return client.Publish(ctx, responseSubject, responseData)
    })
    
    return err
}
```

### Example 5: Trace ID Correlation

```go
func HandleHTTPRequest(w http.ResponseWriter, r *http.Request, client *natsclient.Client) {
    // Extract or generate trace ID
    traceID := r.Header.Get("X-Trace-ID")
    if traceID == "" {
        traceID = uuid.New().String()
    }
    
    // Add trace ID to context
    ctx := natsclient.WithTraceID(r.Context(), traceID)
    
    // All NATS operations will use this trace ID
    workItem := WorkItem{
        Type:  "story",
        Title: "New feature",
    }
    
    // Publish with automatic trace ID correlation
    subject := natsclient.NotificationWorkItemCreated(traceID)
    err := client.Publish(ctx, subject, workItem)
    if err != nil {
        http.Error(w, "Failed to publish notification", http.StatusInternalServerError)
        return
    }
    
    // Trace ID is automatically included in all messages
    w.Header().Set("X-Trace-ID", traceID)
    w.WriteHeader(http.StatusOK)
}
```

### Example 6: Using Subject Builder

```go
func BuildCustomSubject(traceID string) string {
    // For custom subject patterns
    subject := natsclient.NewSubjectBuilder().
        WithTraceID(traceID).
        WithResource("custom-resource").
        WithAction("custom-action").
        WithStatus("request").
        Build()
    
    // Returns: "sprint.{trace_id}.custom-resource.custom-action.request"
    return subject
}
```

## Testing

### Integration Tests

The NATS client includes comprehensive integration tests that verify:

- Connection management and reconnection logic
- Message publishing and subscription
- Request-response patterns
- Trace ID correlation across messages
- Queue group load balancing

Run integration tests (requires running NATS server):

```bash
# Start NATS server
make nats-up

# Run tests
go test -v ./pkg/nats/... -run TestNATSIntegration

# Stop NATS server
make nats-down
```

Tests will automatically skip if NATS server is not available.

### Unit Testing with NATS

For unit testing code that uses NATS, consider:

1. **Mock the NATS client**: Create an interface and mock implementation
2. **Use test containers**: Spin up NATS in Docker for integration tests
3. **Test message handlers**: Test handler functions independently

Example mock interface:

```go
type NATSPublisher interface {
    Publish(ctx context.Context, subject string, data interface{}) error
}

type MockNATSPublisher struct {
    PublishedMessages []MockMessage
}

type MockMessage struct {
    Subject string
    Data    interface{}
}

func (m *MockNATSPublisher) Publish(ctx context.Context, subject string, data interface{}) error {
    m.PublishedMessages = append(m.PublishedMessages, MockMessage{
        Subject: subject,
        Data:    data,
    })
    return nil
}
```

## Performance Considerations

- **Timeouts**: All requests should have reasonable timeouts (default: 5 seconds)
- **Retries**: Implement exponential backoff for transient failures
- **Batching**: Batch notifications when possible to reduce message volume
- **Connection Pooling**: Reuse NATS connections across requests
- **Queue Groups**: Use queue groups for load balancing across service instances

## Security

- **Authorization**: Validate user permissions before processing requests
- **Input Validation**: Sanitize all inputs to prevent injection attacks
- **Encryption**: Use TLS for NATS connections in production
- **Authentication**: NATS server requires authentication in production
- **Subject Permissions**: Configure subject-level permissions in NATS

## Monitoring

Monitor these metrics for Sprint Service NATS operations:

- Message publish rate by subject
- Message processing latency
- Error rate by error code
- Active subscriptions
- Queue depth
- Notification delivery rate

## Related Documentation

- [Root NATS Documentation](../../deploy/nats/README.md)
- [Auth Service NATS Documentation](../../go_auth/docs/nats.md)
- [NATS Official Documentation](https://docs.nats.io/)
