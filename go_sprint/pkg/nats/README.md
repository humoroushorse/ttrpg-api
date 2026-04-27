# NATS Client Package

This package provides a NATS client wrapper for the Sprint Management Service with standardized patterns for inter-service communication.

## Features

- **Connection Management**: Automatic reconnection with configurable retry logic
- **Trace ID Correlation**: Automatic trace ID propagation across all messages
- **Standardized Message Format**: Consistent message structure with metadata
- **Subject Pattern Helpers**: Pre-defined functions for common subject patterns
- **Logging Integration**: Built-in structured logging with slog
- **Error Handling**: Comprehensive error handling and reporting
- **Queue Groups**: Support for load-balanced message processing

## Quick Start

### Installation

```bash
go get github.com/nats-io/nats.go
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "time"
    
    natsclient "github.com/humoroushorse/go_sprint/pkg/nats"
)

func main() {
    // Configure client
    cfg := natsclient.Config{
        URL:            "nats://localhost:4222",
        ReconnectWait:  2 * time.Second,
        MaxReconnects:  10,
        ConnectionName: "sprint-service",
        RequestTimeout: 5 * time.Second,
    }
    
    logger := slog.Default()
    
    // Create client
    client, err := natsclient.NewClient(cfg, logger)
    if err != nil {
        log.Fatalf("Failed to create NATS client: %v", err)
    }
    defer client.Close()
    
    // Publish a message
    ctx := context.Background()
    traceID := "550e8400-e29b-41d4-a716-446655440000"
    ctx = natsclient.WithTraceID(ctx, traceID)
    
    subject := natsclient.NotificationWorkItemCreated(traceID)
    data := map[string]interface{}{
        "work_item_id": "work-123",
        "type":         "story",
        "title":        "New feature",
    }
    
    err = client.Publish(ctx, subject, data)
    if err != nil {
        log.Fatalf("Failed to publish: %v", err)
    }
}
```

## Package Structure

```
pkg/nats/
├── client.go       # NATS client wrapper with connection management
├── subjects.go     # Subject pattern helpers and constants
├── messages.go     # Message types and validation helpers
├── client_test.go  # Integration tests
└── README.md       # This file
```

## Components

### Client (`client.go`)

The main NATS client wrapper that provides:

- `NewClient()`: Create a new NATS client with configuration
- `Publish()`: Publish a message to a subject
- `Request()`: Send a request and wait for response
- `Subscribe()`: Subscribe to a subject with handler
- `QueueSubscribe()`: Subscribe with queue group for load balancing
- `Close()`: Close the NATS connection
- `IsConnected()`: Check connection status

### Subject Patterns (`subjects.go`)

Helper functions for constructing standardized NATS subjects:

**Work Item Subjects:**
- `WorkItemCreateRequest(traceID)`
- `WorkItemUpdateRequest(traceID)`
- `WorkItemDeleteRequest(traceID)`
- `WorkItemGetRequest(traceID)`

**Sprint Subjects:**
- `SprintCreateRequest(traceID)`
- `SprintUpdateRequest(traceID)`
- `SprintCloseRequest(traceID)`
- `SprintGetRequest(traceID)`

**Notification Subjects:**
- `NotificationWorkItemCreated(traceID)`
- `NotificationWorkItemUpdated(traceID)`
- `NotificationSprintStatusChanged(traceID)`

**Wildcard Patterns:**
- `AllWorkItemMessages()` - `"sprint.*.workitem.*.*"`
- `AllSprintMessages()` - `"sprint.*.sprint.*.*"`
- `AllNotifications()` - `"sprint.*.notification.*.*"`
- `AllServiceMessages()` - `"sprint.>"`

### Message Types (`messages.go`)

Predefined message structures:

- `WorkItemMessage`: Work item data structure
- `SprintMessage`: Sprint data structure
- `ErrorMessage`: Error response structure
- `NotificationMessage`: Notification event structure
- `Response`: Standardized response wrapper

Validation helpers:
- `ValidateWorkItemType()`
- `ValidateWorkItemStatus()`
- `ValidatePriority()`
- `ValidateSprintStatus()`

## Configuration

### Config Structure

```go
type Config struct {
    URL              string        // NATS server URL
    ReconnectWait    time.Duration // Wait time between reconnection attempts
    MaxReconnects    int           // Maximum number of reconnection attempts
    ConnectionName   string        // Client connection name
    RequestTimeout   time.Duration // Default timeout for requests
}
```

### Default Values

- `ReconnectWait`: 2 seconds
- `MaxReconnects`: 10
- `ConnectionName`: "sprint-service"
- `RequestTimeout`: 5 seconds

## Message Format

All messages follow a standardized format:

```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "correlation_id": "660e8400-e29b-41d4-a716-446655440001",
  "timestamp": "2024-01-15T10:30:00Z",
  "service": "sprint",
  "version": "v1",
  "data": {
    "field1": "value1",
    "field2": "value2"
  }
}
```

### Message Headers

```
Trace-ID: 550e8400-e29b-41d4-a716-446655440000
Correlation-ID: 660e8400-e29b-41d4-a716-446655440001
Timestamp: 2024-01-15T10:30:00Z
Service: sprint
Version: v1
```

## Testing

### Running Integration Tests

Integration tests require a running NATS server:

```bash
# Start NATS using Docker
docker run -d --name nats -p 4222:4222 -p 8222:8222 nats:latest

# Run tests
go test -v ./pkg/nats/...

# Stop NATS
docker stop nats && docker rm nats
```

Or using the Makefile:

```bash
make nats-up
go test -v ./pkg/nats/...
make nats-down
```

### Test Coverage

The test suite covers:

- Client creation and configuration
- Connection management
- Message publishing and subscription
- Request-response patterns
- Trace ID correlation
- Queue group load balancing
- Reconnection behavior
- Error handling

Tests automatically skip if NATS server is not available.

## Examples

See the [NATS Documentation](../../docs/nats.md) for comprehensive examples including:

- Publishing messages
- Request-response patterns
- Subscribing to messages
- Queue subscriptions
- Trace ID correlation
- Custom subject patterns

## Best Practices

1. **Always use trace IDs**: Include trace ID in context for all operations
2. **Handle errors**: Check and log all errors from NATS operations
3. **Set timeouts**: Always set reasonable timeouts for requests
4. **Use queue groups**: For load balancing across multiple instances
5. **Close connections**: Always defer `client.Close()` after creation
6. **Validate messages**: Use validation helpers before processing
7. **Log appropriately**: Use structured logging with trace correlation

## Performance Tips

- Reuse NATS connections across requests
- Use queue groups for horizontal scaling
- Batch notifications when possible
- Set appropriate timeouts
- Monitor connection health
- Use wildcard subscriptions carefully

## Troubleshooting

### Connection Issues

```go
if !client.IsConnected() {
    log.Println("NATS client is not connected")
    // Check NATS server status
    // Verify network connectivity
    // Check configuration
}
```

### Message Not Received

- Verify subscription is established before publishing
- Check subject pattern matches
- Ensure NATS server is running
- Check for errors in handler function
- Verify trace ID is correct

### Timeout Errors

- Increase `RequestTimeout` in configuration
- Check NATS server performance
- Verify responder is subscribed and processing
- Check network latency

## Related Documentation

- [Sprint Service NATS Documentation](../../docs/nats.md)
- [Root NATS Documentation](../../../deploy/nats/README.md)
- [NATS Official Documentation](https://docs.nats.io/)

## License

Part of the Sprint Management System.
