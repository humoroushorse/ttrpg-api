# NATS Subject Documentation - Auth Service

This document describes all NATS subjects used by the Auth Service for inter-service communication.

## Overview

The Auth Service uses NATS for:
- Token validation requests from other services
- User synchronization with Keycloak
- Authentication events and notifications

## Subject Pattern

All Auth Service subjects follow the standardized pattern:

```
auth.{trace_id}.{action}.{status}
```

## Subject Definitions

### Token Validation

#### Request: `auth.{trace_id}.validate.request`

Validates a JWT token and returns user information.

**Publisher**: Sprint Service, other services  
**Subscriber**: Auth Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Response: `auth.{trace_id}.validate.response`

Returns validation result and user information.

**Publisher**: Auth Service  
**Subscriber**: Sprint Service, other services

**Success Response**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "valid": true,
  "user": {
    "id": "user-uuid",
    "username": "john.doe",
    "email": "john.doe@example.com",
    "roles": ["user", "developer"],
    "keycloak_id": "keycloak-user-id"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

**Error Response**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "valid": false,
  "error": {
    "code": "INVALID_TOKEN",
    "message": "Token has expired"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### User Synchronization

#### Request: `auth.{trace_id}.user.sync.request`

Synchronizes user data from Keycloak to local database.

**Publisher**: Auth Service (internal), Admin tools  
**Subscriber**: Auth Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "keycloak_id": "keycloak-user-id",
  "force": false,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Response: `auth.{trace_id}.user.sync.response`

Returns synchronization result.

**Publisher**: Auth Service  
**Subscriber**: Admin tools

**Success Response**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "success": true,
  "user_id": "user-uuid",
  "synced_at": "2024-01-15T10:30:00Z"
}
```

### Authentication Events

#### Event: `auth.{trace_id}.event.login`

Published when a user successfully logs in.

**Publisher**: Auth Service  
**Subscriber**: Audit Service, Analytics Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "login",
  "user_id": "user-uuid",
  "username": "john.doe",
  "ip_address": "192.168.1.100",
  "user_agent": "Mozilla/5.0...",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### Event: `auth.{trace_id}.event.logout`

Published when a user logs out.

**Publisher**: Auth Service  
**Subscriber**: Audit Service, Analytics Service

**Message Format**:
```json
{
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "logout",
  "user_id": "user-uuid",
  "username": "john.doe",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Wildcard Subscriptions

### Tracing All Auth Messages

```
auth.{trace_id}.*.*
```

Subscribe to all messages for a specific trace ID.

### Monitoring All Auth Events

```
auth.*.event.*
```

Subscribe to all authentication events across all traces.

### All Auth Service Messages

```
auth.>
```

Subscribe to all messages published by the Auth Service.

## Message Headers

All NATS messages include standard headers:

```go
headers := nats.Header{
    "Trace-ID":       traceID,
    "Correlation-ID": correlationID,
    "Timestamp":      time.Now().UTC().Format(time.RFC3339),
    "Service":        "auth",
    "Version":        "v1",
    "Content-Type":   "application/json",
}
```

## Error Handling

### Error Codes

- `INVALID_TOKEN`: Token is malformed or invalid
- `EXPIRED_TOKEN`: Token has expired
- `REVOKED_TOKEN`: Token has been revoked
- `INSUFFICIENT_PERMISSIONS`: User lacks required permissions
- `USER_NOT_FOUND`: User does not exist
- `KEYCLOAK_ERROR`: Error communicating with Keycloak
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

## Usage Examples

### Go Client Example

```go
package main

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
    "github.com/nats-io/nats.go"
)

type ValidateTokenRequest struct {
    TraceID   string    `json:"trace_id"`
    Token     string    `json:"token"`
    Timestamp time.Time `json:"timestamp"`
}

type ValidateTokenResponse struct {
    TraceID   string    `json:"trace_id"`
    Valid     bool      `json:"valid"`
    User      *User     `json:"user,omitempty"`
    Error     *Error    `json:"error,omitempty"`
    Timestamp time.Time `json:"timestamp"`
}

func ValidateToken(ctx context.Context, nc *nats.Conn, token string) (*ValidateTokenResponse, error) {
    traceID := uuid.New().String()
    
    // Create request
    req := ValidateTokenRequest{
        TraceID:   traceID,
        Token:     token,
        Timestamp: time.Now().UTC(),
    }
    
    reqData, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    
    // Publish request and wait for response
    subject := "auth." + traceID + ".validate.request"
    responseSubject := "auth." + traceID + ".validate.response"
    
    // Subscribe to response
    sub, err := nc.SubscribeSync(responseSubject)
    if err != nil {
        return nil, err
    }
    defer sub.Unsubscribe()
    
    // Publish request
    if err := nc.Publish(subject, reqData); err != nil {
        return nil, err
    }
    
    // Wait for response with timeout
    msg, err := sub.NextMsgWithContext(ctx)
    if err != nil {
        return nil, err
    }
    
    // Parse response
    var resp ValidateTokenResponse
    if err := json.Unmarshal(msg.Data, &resp); err != nil {
        return nil, err
    }
    
    return &resp, nil
}
```

## Performance Considerations

- **Timeouts**: All requests should have reasonable timeouts (default: 5 seconds)
- **Retries**: Implement exponential backoff for transient failures
- **Caching**: Cache validation results for short periods to reduce load
- **Connection Pooling**: Reuse NATS connections across requests

## Security

- **Token Transmission**: Tokens are transmitted in message payloads, not subjects
- **Encryption**: Use TLS for NATS connections in production
- **Authentication**: NATS server requires authentication in production
- **Authorization**: Subject-level permissions restrict which services can publish/subscribe

## Monitoring

Monitor these metrics for Auth Service NATS operations:

- Message publish rate
- Message processing latency
- Error rate by error code
- Active subscriptions
- Queue depth

## Related Documentation

- [Root NATS Documentation](../../deploy/nats/README.md)
- [Sprint Service NATS Documentation](../../go_sprint/docs/nats.md)
- [NATS Official Documentation](https://docs.nats.io/)
