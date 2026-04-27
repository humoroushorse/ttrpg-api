# Metrics and Observability Package

This package provides comprehensive observability and monitoring capabilities for the Go Sprint Management System, including Prometheus metrics, health checks, and distributed tracing.

## Features

### 1. Prometheus Metrics

The package exposes detailed metrics for monitoring system performance and business operations.

#### HTTP Metrics
- `http_requests_total` - Total number of HTTP requests
- `http_request_duration_seconds` - Duration of HTTP requests
- `http_request_size_bytes` - Size of HTTP requests
- `http_response_size_bytes` - Size of HTTP responses

#### Database Metrics
- `database_operations_total` - Total number of database operations
- `database_operation_duration_seconds` - Duration of database operations
- `database_connections_active` - Number of active database connections
- `database_connections_idle` - Number of idle database connections
- `database_rows_affected` - Number of rows affected by operations

#### NATS Metrics
- `nats_messages_published_total` - Total number of NATS messages published
- `nats_messages_received_total` - Total number of NATS messages received
- `nats_publish_duration_seconds` - Duration of NATS publish operations
- `nats_connection_status` - NATS connection status (1 = connected, 0 = disconnected)
- `nats_reconnections_total` - Total number of NATS reconnections

#### Business Metrics
- `work_items_total` - Total number of work items by type and status
- `work_items_created_total` - Total number of work items created
- `work_items_completed_total` - Total number of work items completed
- `work_item_cycle_time_hours` - Cycle time of work items (in_progress to done)
- `work_item_lead_time_hours` - Lead time of work items (creation to done)
- `sprints_active` - Number of active sprints
- `sprints_completed_total` - Total number of completed sprints
- `sprint_velocity_points` - Sprint velocity in story points
- `sprint_completion_rate_percent` - Sprint completion rate as percentage
- `sprint_capacity_utilization_percent` - Sprint capacity utilization

#### WebSocket Metrics
- `websocket_connections_active` - Number of active WebSocket connections
- `websocket_messages_published_total` - Total WebSocket messages published

### 2. Health Check Endpoints

The package provides three types of health check endpoints for Kubernetes:

#### Liveness Probe (`/health/live`)
Checks if the application is running and not deadlocked.
- Returns 200 OK if the application is alive
- Simple check that always succeeds if the process is running

#### Readiness Probe (`/health/ready`)
Checks if the application is ready to serve traffic.
- Verifies database connectivity
- Verifies NATS connectivity
- Runs custom health checkers
- Returns 200 OK if all checks pass
- Returns 503 Service Unavailable if any check fails

#### Startup Probe (`/health/startup`)
Checks if the application has finished initialization.
- Verifies database is initialized
- Returns 200 OK if startup is complete
- Returns 503 Service Unavailable if still starting

### 3. Distributed Tracing

OpenTelemetry-compatible distributed tracing for request correlation across services.

#### Features
- Trace ID propagation across HTTP and NATS
- Parent-child span relationships
- Span attributes and events
- Error recording
- Performance monitoring with thresholds

## Usage

### Setting Up Metrics

```go
import "github.com/humoroushorse/go_sprint/pkg/metrics"

// Record HTTP request
metrics.RecordHTTPRequest("GET", "/api/workitems", "200", 0.5, 1024, 2048)

// Record database operation
metrics.RecordDatabaseOperation("SELECT", "work_items", "success", 0.01, 10)

// Record NATS publish
metrics.RecordNATSPublish("sprint.trace.workitem.create", "success", 0.005)

// Record business metrics
metrics.RecordWorkItemCreated("story")
metrics.RecordWorkItemCompleted("story", 24.0, 48.0) // cycle time, lead time in hours
metrics.UpdateSprintsActive(3)
metrics.RecordSprintCompleted("sprint-123", 45.0, 90.0, 85.0) // velocity, completion%, utilization%
```

### Using Middleware

```go
import (
    "net/http"
    "github.com/humoroushorse/go_sprint/pkg/metrics"
)

// Add HTTP metrics middleware
handler := metrics.HTTPMetricsMiddleware(yourHandler)

// Add tracing middleware
tracer := metrics.NewTracer("sprint-service")
handler = metrics.TracingMiddleware(tracer)(handler)
```

### Setting Up Health Checks

```go
import (
    "database/sql"
    "github.com/humoroushorse/go_sprint/pkg/metrics"
)

// Create health check handler
healthHandler := metrics.NewHealthCheckHandler(db, natsConn)

// Register custom health checker
healthHandler.RegisterChecker("custom", customChecker)

// Set up endpoints
http.HandleFunc("/health/live", healthHandler.LivenessHandler())
http.HandleFunc("/health/ready", healthHandler.ReadinessHandler())
http.HandleFunc("/health/startup", healthHandler.StartupHandler())
```

### Using Distributed Tracing

```go
import (
    "context"
    "github.com/humoroushorse/go_sprint/pkg/metrics"
)

// Create tracer
tracer := metrics.NewTracer("sprint-service")

// Start span
ctx, span := tracer.StartSpan(ctx, "operation-name")
defer span.End()

// Set attributes
span.SetAttribute("user_id", "user-123")
span.SetAttribute("work_item_id", "item-456")

// Add events
span.AddEvent("validation_completed", map[string]interface{}{
    "validation_time_ms": 10,
})

// Record errors
if err != nil {
    span.RecordError(err)
    return err
}

// Set status
span.SetStatus(metrics.StatusCodeOK, "")
```

### Tracing Database Operations

```go
err := metrics.TraceDatabase(ctx, tracer, "SELECT", "work_items", func() error {
    // Your database operation
    return db.Query(...)
})
```

### Tracing NATS Operations

```go
err := metrics.TraceNATS(ctx, tracer, "sprint.trace.workitem.create", "publish", func() error {
    // Your NATS publish operation
    return nc.Publish(subject, data)
})
```

### Propagating Trace Context Through NATS

```go
// On the publishing side
headers := metrics.PropagateTraceToNATS(ctx)
// Add headers to NATS message

// On the receiving side
ctx = metrics.ExtractTraceFromNATS(ctx, headers)
// Continue with extracted trace context
```

### Performance Monitoring

```go
monitor := metrics.NewPerformanceMonitor(tracer, 100*time.Millisecond)

err := monitor.MonitorOperation(ctx, "slow-operation", func() error {
    // Your operation
    return doWork()
})
// Automatically alerts if operation exceeds 100ms threshold
```

## Kubernetes Configuration

### Deployment with Health Checks

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sprint-management
spec:
  template:
    spec:
      containers:
      - name: sprint-management
        image: sprint-management:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        startupProbe:
          httpGet:
            path: /health/startup
            port: 8080
          initialDelaySeconds: 0
          periodSeconds: 5
          failureThreshold: 30
```

### Prometheus ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: sprint-management
spec:
  selector:
    matchLabels:
      app: sprint-management
  endpoints:
  - port: metrics
    path: /metrics
    interval: 30s
```

## Metrics Endpoint

Expose Prometheus metrics endpoint:

```go
import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

http.Handle("/metrics", promhttp.Handler())
```

## Custom Health Checkers

Implement custom health checkers:

```go
type MyHealthChecker struct {
    // your dependencies
}

func (c *MyHealthChecker) CheckHealth(ctx context.Context) error {
    // Perform your health check
    if !healthy {
        return errors.New("service is unhealthy")
    }
    return nil
}

// Register the checker
healthHandler.RegisterChecker("my-service", &MyHealthChecker{})
```

## Best Practices

1. **Metrics Collection**
   - Record metrics at appropriate granularity
   - Use labels wisely to avoid cardinality explosion
   - Record both technical and business metrics

2. **Health Checks**
   - Keep health checks lightweight and fast
   - Use readiness checks for dependencies
   - Use liveness checks for deadlock detection
   - Use startup checks for slow initialization

3. **Distributed Tracing**
   - Always propagate trace context across service boundaries
   - Add meaningful attributes to spans
   - Record errors with context
   - Use performance monitoring for critical operations

4. **Performance**
   - Metrics collection has minimal overhead
   - Health checks should complete in < 1 second
   - Tracing adds negligible latency

## Testing

The package includes comprehensive tests:

```bash
# Run all tests
go test ./pkg/metrics/...

# Run specific test suites
go test ./pkg/metrics/... -run TestMetrics
go test ./pkg/metrics/... -run TestHealth
go test ./pkg/metrics/... -run TestTracing
go test ./pkg/metrics/... -run Integration
```

## Requirements Validation

This package validates the following requirements:

- **Requirement 17.1**: Prometheus metrics for HTTP, database, and NATS operations
- **Requirement 17.2**: Health check endpoints for liveness and readiness probes
- **Requirement 17.4**: Custom business metrics (velocity, cycle time, active sprints)
- **Requirement 17.5**: Distributed tracing compatible with OpenTelemetry standards
