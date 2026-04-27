# Sequence Diagram: NATS Messaging Flows

This diagram shows the detailed message flows using NATS for inter-service communication.

## JWT Token Validation Flow

```mermaid
sequenceDiagram
    participant Client as Frontend Client
    participant API as Sprint Management API
    participant NATS as NATS Broker
    participant Auth as Auth Service
    participant KC as Keycloak

    Note over Client,KC: User Authentication and Token Validation

    Client->>API: POST /api/v1/workitems<br/>{Authorization: Bearer JWT}
    
    Note over API: Extract trace ID from<br/>request or generate new UUID
    
    API->>API: Extract JWT from<br/>Authorization header
    
    API->>NATS: Publish: auth.{trace_id}.validate.request<br/>{token: "JWT", trace_id: "..."}
    
    Note over NATS: Route message to<br/>auth service subscribers
    
    NATS->>Auth: Deliver: auth.{trace_id}.validate.request
    
    Auth->>Auth: Parse JWT token
    
    Auth->>KC: GET /realms/sprint-management/protocol/openid-connect/certs
    KC-->>Auth: Public keys for validation
    
    Auth->>Auth: Validate JWT signature<br/>and expiration
    
    alt Token Valid
        Auth->>Auth: Extract user claims<br/>(user_id, roles, etc.)
        
        Auth->>NATS: Publish: auth.{trace_id}.validate.response<br/>{valid: true, user: {...}}
        
        NATS->>API: Deliver: auth.{trace_id}.validate.response
        
        API->>API: Store user context<br/>in request context
        
        API->>API: Process business logic
        
        API-->>Client: 201 Created<br/>{work_item: {...}}
    else Token Invalid
        Auth->>NATS: Publish: auth.{trace_id}.validate.response<br/>{valid: false, error: "..."}
        
        NATS->>API: Deliver: auth.{trace_id}.validate.response
        
        API-->>Client: 401 Unauthorized<br/>{error: "Invalid token"}
    end
```

## Work Item Creation with Notification

```mermaid
sequenceDiagram
    participant Client as Frontend Client
    participant API as Sprint Management API
    participant DB as PostgreSQL Master
    participant NATS as NATS Broker
    participant WS as WebSocket Hub
    participant WSClient as WebSocket Clients

    Note over Client,WSClient: Work Item Creation with Real-time Notification

    Client->>API: POST /api/v1/workitems<br/>{title, description, type}
    
    API->>API: Generate trace ID
    
    API->>API: Validate JWT<br/>(via NATS, see above)
    
    API->>API: Validate business rules<br/>(required fields, type, etc.)
    
    API->>DB: INSERT INTO work_items<br/>VALUES (...)
    DB-->>API: Work item created<br/>{id, created_at, ...}
    
    Note over API: Publish notification<br/>for real-time updates
    
    API->>NATS: Publish: sprint.{trace_id}.notification.workitem.created<br/>{work_item: {...}, sprint_id: "..."}
    
    Note over NATS: Broadcast to all<br/>notification subscribers
    
    NATS->>WS: Deliver: sprint.{trace_id}.notification.workitem.created
    
    WS->>WS: Identify clients in<br/>sprint room
    
    loop For each client in room
        WS->>WSClient: WebSocket Message<br/>{type: "workitem.created", data: {...}}
    end
    
    API-->>Client: 201 Created<br/>{work_item: {...}}
    
    Note over WSClient: UI updates automatically<br/>without page refresh
```

## Sprint Closure Workflow

```mermaid
sequenceDiagram
    participant Client as Frontend Client
    participant API as Sprint Management API
    participant DB as PostgreSQL Master
    participant NATS as NATS Broker
    participant WS as WebSocket Hub

    Note over Client,WS: Sprint Closure with Work Item Migration

    Client->>API: POST /api/v1/sprints/{id}/close
    
    API->>API: Generate trace ID
    
    API->>API: Validate JWT
    
    API->>DB: BEGIN TRANSACTION
    
    API->>DB: SELECT * FROM sprints<br/>WHERE id = {id}
    DB-->>API: Sprint data
    
    API->>API: Validate sprint can be closed<br/>(status = 'active')
    
    API->>DB: SELECT * FROM work_items<br/>WHERE sprint_id = {id}<br/>AND status != 'done'
    DB-->>API: Incomplete work items
    
    loop For each incomplete work item
        API->>DB: UPDATE work_items<br/>SET sprint_id = NULL<br/>WHERE id = {item_id}
        
        API->>NATS: Publish: sprint.{trace_id}.notification.workitem.updated<br/>{work_item: {...}, change: "moved_to_backlog"}
    end
    
    API->>DB: UPDATE sprints<br/>SET status = 'completed',<br/>completed_points = ...<br/>WHERE id = {id}
    
    API->>DB: INSERT INTO activity_logs<br/>VALUES (...)
    
    API->>DB: COMMIT TRANSACTION
    
    API->>NATS: Publish: sprint.{trace_id}.notification.sprint.status_changed<br/>{sprint: {...}, status: "completed"}
    
    NATS->>WS: Deliver notifications
    
    WS->>WS: Broadcast to sprint room
    
    API-->>Client: 200 OK<br/>{sprint: {...}, moved_items: [...]}
```

## User Synchronization from Keycloak

```mermaid
sequenceDiagram
    participant Admin as Administrator
    participant Sprint as Sprint Management API
    participant NATS as NATS Broker
    participant Auth as Auth Service
    participant KC as Keycloak
    participant DB as PostgreSQL Master

    Note over Admin,DB: Periodic User Synchronization

    Admin->>Sprint: POST /api/v1/admin/sync-users
    
    Sprint->>Sprint: Generate trace ID
    
    Sprint->>NATS: Publish: auth.{trace_id}.user.sync.request<br/>{full_sync: true}
    
    NATS->>Auth: Deliver: auth.{trace_id}.user.sync.request
    
    Auth->>KC: GET /admin/realms/sprint-management/users
    KC-->>Auth: List of all users
    
    loop For each Keycloak user
        Auth->>DB: SELECT * FROM auth.users<br/>WHERE keycloak_id = {kc_id}
        
        alt User exists
            Auth->>DB: UPDATE auth.users<br/>SET email = ..., roles = ...<br/>WHERE keycloak_id = {kc_id}
        else User doesn't exist
            Auth->>DB: INSERT INTO auth.users<br/>VALUES (...)
        end
    end
    
    Auth->>NATS: Publish: auth.{trace_id}.user.sync.response<br/>{synced: 42, created: 5, updated: 37}
    
    NATS->>Sprint: Deliver: auth.{trace_id}.user.sync.response
    
    Sprint-->>Admin: 200 OK<br/>{result: {...}}
```

## Distributed Tracing Across Services

```mermaid
sequenceDiagram
    participant Client as Frontend Client
    participant Sprint as Sprint Management API
    participant NATS as NATS Broker
    participant Auth as Auth Service
    participant Log as Log Aggregation

    Note over Client,Log: Trace ID Propagation for Debugging

    Client->>Sprint: GET /api/v1/workitems/{id}<br/>{X-Trace-ID: "abc-123"}
    
    Note over Sprint: Use provided trace ID<br/>or generate new one
    
    Sprint->>Sprint: Log: "Received request"<br/>{trace_id: "abc-123", method: "GET"}
    Sprint->>Log: Ship log entry
    
    Sprint->>NATS: Publish: auth.abc-123.validate.request<br/>{trace_id: "abc-123", token: "..."}
    
    Sprint->>Sprint: Log: "Sent auth request"<br/>{trace_id: "abc-123"}
    Sprint->>Log: Ship log entry
    
    NATS->>Auth: Deliver: auth.abc-123.validate.request
    
    Auth->>Auth: Log: "Received auth request"<br/>{trace_id: "abc-123"}
    Auth->>Log: Ship log entry
    
    Auth->>Auth: Validate token
    
    Auth->>Auth: Log: "Token validated"<br/>{trace_id: "abc-123", valid: true}
    Auth->>Log: Ship log entry
    
    Auth->>NATS: Publish: auth.abc-123.validate.response<br/>{trace_id: "abc-123", valid: true}
    
    NATS->>Sprint: Deliver: auth.abc-123.validate.response
    
    Sprint->>Sprint: Log: "Received auth response"<br/>{trace_id: "abc-123", valid: true}
    Sprint->>Log: Ship log entry
    
    Sprint->>Sprint: Process request
    
    Sprint->>Sprint: Log: "Request completed"<br/>{trace_id: "abc-123", duration: "45ms"}
    Sprint->>Log: Ship log entry
    
    Sprint-->>Client: 200 OK<br/>{work_item: {...}}
    
    Note over Log: All logs correlated by<br/>trace_id: "abc-123"<br/>for easy debugging
```

## NATS Subject Patterns

### Sprint Management Service

#### Work Item Operations
```
sprint.{trace_id}.workitem.create.request
sprint.{trace_id}.workitem.create.response
sprint.{trace_id}.workitem.update.request
sprint.{trace_id}.workitem.update.response
sprint.{trace_id}.workitem.delete.request
sprint.{trace_id}.workitem.delete.response
```

#### Sprint Operations
```
sprint.{trace_id}.sprint.create.request
sprint.{trace_id}.sprint.create.response
sprint.{trace_id}.sprint.close.request
sprint.{trace_id}.sprint.close.response
```

#### Real-time Notifications
```
sprint.{trace_id}.notification.workitem.created
sprint.{trace_id}.notification.workitem.updated
sprint.{trace_id}.notification.workitem.deleted
sprint.{trace_id}.notification.sprint.status_changed
sprint.{trace_id}.notification.comment.added
```

### Authentication Service

#### Token Validation
```
auth.{trace_id}.validate.request
auth.{trace_id}.validate.response
```

#### User Management
```
auth.{trace_id}.user.sync.request
auth.{trace_id}.user.sync.response
auth.{trace_id}.user.create.request
auth.{trace_id}.user.create.response
```

### Wildcard Subscriptions

#### Service-specific Tracing
```
sprint.{trace_id}.*.*.*    # All sprint service messages for trace
auth.{trace_id}.*.*        # All auth service messages for trace
```

#### Notification Monitoring
```
sprint.*.notification.*    # All notifications from sprint service
*.*.notification.*         # All notifications from any service
```

#### Debug Subscriptions
```
sprint.>                   # All sprint service messages
auth.>                     # All auth service messages
>                          # All messages (use with caution)
```

## Message Format

### Request Message
```json
{
  "trace_id": "abc-123-def-456",
  "timestamp": "2024-01-15T10:30:00Z",
  "correlation_id": "req-789",
  "payload": {
    "token": "eyJhbGciOiJSUzI1NiIs...",
    "user_id": "user-uuid"
  }
}
```

### Response Message
```json
{
  "trace_id": "abc-123-def-456",
  "timestamp": "2024-01-15T10:30:00.123Z",
  "correlation_id": "req-789",
  "success": true,
  "payload": {
    "valid": true,
    "user": {
      "id": "user-uuid",
      "username": "john.doe",
      "roles": ["developer"]
    }
  },
  "error": null
}
```

### Notification Message
```json
{
  "trace_id": "abc-123-def-456",
  "timestamp": "2024-01-15T10:30:00.456Z",
  "type": "workitem.created",
  "entity_type": "work_item",
  "entity_id": "work-item-uuid",
  "sprint_id": "sprint-uuid",
  "user_id": "user-uuid",
  "payload": {
    "work_item": {
      "id": "work-item-uuid",
      "title": "New Feature",
      "type": "story",
      "status": "todo"
    }
  }
}
```

## Error Handling

### Timeout Handling
```
1. Sprint API publishes request
2. Sprint API waits for response (5 second timeout)
3. If timeout:
   - Log error with trace ID
   - Return 503 Service Unavailable to client
   - Retry with exponential backoff (optional)
```

### Connection Failure
```
1. NATS connection lost
2. Service attempts reconnection
3. Buffered messages queued
4. On reconnection:
   - Resubscribe to subjects
   - Flush queued messages
   - Resume normal operation
```

### Invalid Message Format
```
1. Service receives malformed message
2. Log error with trace ID and message content
3. Send error response (if request/response pattern)
4. Continue processing other messages
```

## Performance Considerations

### Message Size
- Keep messages small (< 1MB)
- Use references instead of embedding large objects
- Compress large payloads if necessary

### Throughput
- NATS can handle millions of messages per second
- Bottleneck typically in message processing, not NATS
- Scale services horizontally for higher throughput

### Latency
- Typical NATS latency: < 1ms
- End-to-end latency depends on processing time
- Use async patterns for non-critical operations

## Monitoring

### NATS Metrics
```bash
# View NATS monitoring endpoint
curl http://nats-server:8222/varz

# Key metrics:
# - connections: Active connections
# - in_msgs: Messages received
# - out_msgs: Messages sent
# - slow_consumers: Slow consumer count
```

### Application Metrics
```prometheus
# Message publish rate
rate(nats_messages_published_total[5m])

# Message receive rate
rate(nats_messages_received_total[5m])

# Message processing duration
histogram_quantile(0.95, rate(nats_message_duration_seconds_bucket[5m]))
```

## Related Documentation

- [NATS Subject Patterns](../nats.md): Complete subject documentation
- [Auth Service NATS](../../go_auth/docs/nats.md): Auth service subjects
- [Container Diagram](c4-container.md): System architecture
- [Troubleshooting Guide](../troubleshooting-guide.md): NATS troubleshooting
