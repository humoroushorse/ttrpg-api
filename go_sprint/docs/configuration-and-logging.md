# Configuration and Logging

This document describes the configuration management and structured logging implementation for the Go Sprint Management System.

## Configuration Management

### Overview

The configuration system provides:
- Environment-based configuration with sensible defaults
- Fail-fast validation at startup
- Type-safe configuration structs
- Support for multiple deployment environments

### Configuration Structure

```go
type Config struct {
    Server   ServerConfig   // HTTP server settings
    Database DatabaseConfig // Database connection settings
    NATS     NATSConfig     // NATS messaging settings
    Auth     AuthConfig     // Authentication settings
    Logging  LoggingConfig  // Logging configuration
    Metrics  MetricsConfig  // Metrics configuration
}
```

### Loading Configuration

```go
import "github.com/humoroushorse/go_sprint/pkg/config"

// Load configuration from environment variables
cfg, err := config.LoadFromEnv()
if err != nil {
    log.Fatal("Failed to load configuration:", err)
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `SERVER_READ_TIMEOUT` | `30s` | HTTP read timeout |
| `SERVER_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| `SERVER_SHUTDOWN_TIMEOUT` | `30s` | Graceful shutdown timeout |
| `DATABASE_MASTER_URL` | `postgres://...` | Master database connection string |
| `DATABASE_REPLICA_URL` | `postgres://...` | Replica database connection string |
| `DATABASE_MAX_OPEN_CONNS` | `25` | Maximum open database connections |
| `DATABASE_MAX_IDLE_CONNS` | `5` | Maximum idle database connections |
| `DATABASE_CONN_MAX_LIFETIME` | `5m` | Maximum connection lifetime |
| `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `NATS_RECONNECT_WAIT` | `2s` | NATS reconnection wait time |
| `NATS_MAX_RECONNECTS` | `10` | Maximum NATS reconnection attempts |
| `AUTH_SERVICE_URL` | `http://localhost:8081` | Auth service URL |
| `KEYCLOAK_URL` | `http://localhost:8080` | Keycloak server URL |
| `KEYCLOAK_REALM` | `sprint-management` | Keycloak realm name |
| `KEYCLOAK_CLIENT_ID` | `sprint-service` | Keycloak client ID |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, console) |
| `LOG_ENABLE_COLORS` | `false` | Enable colored console output |
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `METRICS_PATH` | `/metrics` | Metrics endpoint path |

### Configuration Validation

The configuration system performs comprehensive validation at startup:

- **Server**: Port range (1-65535), positive timeouts
- **Database**: Required URLs, valid connection pool settings
- **NATS**: Required URL, positive reconnection settings
- **Auth**: Required service URLs and credentials
- **Logging**: Valid log level and format
- **Metrics**: Valid path when enabled

If validation fails, the application will exit with a descriptive error message.

## Structured Logging

### Overview

The logging system uses Go's standard `log/slog` package with:
- Structured logging with key-value pairs
- Environment-specific output formats (JSON for production, console for development)
- Context-based logging with trace IDs and user information
- LogValuer interface for sensitive data protection
- Colored console output for local development

### Creating a Logger

```go
import "github.com/humoroushorse/go_sprint/pkg/logging"

// Create logger from configuration
logger := logging.NewLogger(cfg.Logging)
```

### Log Formats

#### JSON Format (Production)

```json
{
  "time": "2024-01-15T10:30:45.123Z",
  "level": "INFO",
  "msg": "work item created",
  "trace_id": "abc123",
  "user_id": "user456",
  "work_item": {
    "id": "work789",
    "type": "story",
    "title": "User login feature",
    "status": "todo"
  }
}
```

#### Console Format (Development)

```
2024-01-15 10:30:45 INFO [trace:abc123] [user:user456] work item created type=story title="User login feature"
```

### Context-Based Logging

```go
import (
    "context"
    "github.com/humoroushorse/go_sprint/pkg/logging"
)

// Add logger to context
ctx := logging.WithLogger(context.Background(), logger)

// Add trace ID
logger = logging.WithTraceID(logger, traceID)

// Add user ID
logger = logging.WithUserID(logger, userID)

// Add request information
logger = logging.WithRequestInfo(logger, "GET", "/api/workitems")

// Retrieve logger from context
logger = logging.FromContext(ctx)
```

### LogValuer Interface for Sensitive Data Protection

All domain models implement the `slog.LogValuer` interface to control what gets logged:

```go
type User struct {
    ID       uuid.UUID
    Username string
    Email    string    // Sensitive - not logged
    Password string    // Sensitive - not logged
    APIKey   string    // Sensitive - not logged
}

// LogValue implements slog.LogValuer
func (u User) LogValue() slog.Value {
    return slog.GroupValue(
        slog.String("id", u.ID.String()),
        slog.String("username", u.Username),
        // Email, password, and API key are deliberately excluded
    )
}
```

### Usage Example

```go
// Create a user with sensitive data
user := models.User{
    Username: "john.doe",
    Email:    "john.doe@example.com",
    Password: "super-secret-password",
    APIKey:   "secret-api-key",
}

// Log the user - sensitive fields are automatically excluded
logger.Info("user authenticated", slog.Any("user", user))

// Output (JSON format):
// {
//   "time": "2024-01-15T10:30:45Z",
//   "level": "INFO",
//   "msg": "user authenticated",
//   "user": {
//     "id": "123e4567-e89b-12d3-a456-426614174000",
//     "username": "john.doe"
//   }
// }
// Note: email, password, and API key are NOT in the output
```

### Log Levels

- **Debug**: Detailed information for debugging
- **Info**: General informational messages
- **Warn**: Warning messages for potentially harmful situations
- **Error**: Error messages for failures

### Best Practices

1. **Always use structured logging**: Use key-value pairs instead of string formatting
   ```go
   // Good
   logger.Info("user created", slog.String("user_id", userID))
   
   // Bad
   logger.Info(fmt.Sprintf("user created: %s", userID))
   ```

2. **Use LogValuer for domain models**: Implement `slog.LogValuer` on all models to control logging
   ```go
   func (m MyModel) LogValue() slog.Value {
       return slog.GroupValue(
           slog.String("id", m.ID.String()),
           // Only include non-sensitive fields
       )
   }
   ```

3. **Include trace IDs**: Always include trace IDs for request correlation
   ```go
   logger = logging.WithTraceID(logger, traceID)
   ```

4. **Use context for logger propagation**: Pass logger through context
   ```go
   ctx := logging.WithLogger(ctx, logger)
   logger := logging.FromContext(ctx)
   ```

5. **Choose appropriate log levels**:
   - Debug: Detailed debugging information
   - Info: Normal operations and state changes
   - Warn: Unexpected but recoverable situations
   - Error: Failures that require attention

## Environment-Specific Configuration

### Local Development

```bash
export LOG_FORMAT=console
export LOG_ENABLE_COLORS=true
export LOG_LEVEL=debug
```

### Production

```bash
export LOG_FORMAT=json
export LOG_ENABLE_COLORS=false
export LOG_LEVEL=info
```

## Testing

### Configuration Tests

```bash
go test ./pkg/config/...
```

### Logging Tests

```bash
go test ./pkg/logging/...
```

### Property-Based Tests

The logging system includes property-based tests to verify:
- LogValuer interface prevents sensitive data logging
- Environment-specific log format switching works correctly
- Log level filtering functions as expected

## Integration with Other Components

### HTTP Middleware

```go
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            traceID := GetTraceID(r.Context())
            contextLogger := logging.WithTraceID(logger, traceID)
            contextLogger = logging.WithRequestInfo(contextLogger, r.Method, r.URL.Path)
            
            ctx := logging.WithLogger(r.Context(), contextLogger)
            
            start := time.Now()
            next.ServeHTTP(w, r.WithContext(ctx))
            
            contextLogger.Info("request completed",
                slog.Duration("duration", time.Since(start)),
            )
        })
    }
}
```

### Service Layer

```go
func (s *WorkItemService) CreateWorkItem(ctx context.Context, req CreateWorkItemRequest) (*WorkItem, error) {
    logger := logging.FromContext(ctx)
    
    logger.Info("creating work item",
        slog.String("type", req.Type),
        slog.String("title", req.Title),
    )
    
    workItem, err := s.repo.Create(ctx, req)
    if err != nil {
        logger.Error("failed to create work item",
            slog.String("error", err.Error()),
        )
        return nil, err
    }
    
    logger.Info("work item created successfully",
        slog.Any("work_item", workItem),
    )
    
    return workItem, nil
}
```

## Troubleshooting

### Configuration Issues

**Problem**: Application fails to start with "configuration validation failed"

**Solution**: Check that all required environment variables are set and have valid values. Review the error message for specific validation failures.

### Logging Issues

**Problem**: Logs are not appearing

**Solution**: Check the `LOG_LEVEL` setting. If set to `error`, only error messages will appear.

**Problem**: Sensitive data appearing in logs

**Solution**: Ensure all domain models implement the `slog.LogValuer` interface correctly.

**Problem**: Colored output not working

**Solution**: Ensure `LOG_FORMAT=console` and `LOG_ENABLE_COLORS=true` are set.

## References

- [Go slog package documentation](https://pkg.go.dev/log/slog)
- [Structured logging best practices](https://go.dev/blog/slog)
- Requirements: 5.3, 5.4, 5.5, 5.6, 5.7, 5.8, 18.1, 18.2, 18.3, 18.4, 18.5, 18.6
