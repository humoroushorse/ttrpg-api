# Sprint Management Server

This is the main HTTP server for the Sprint Management Service.

## Features

- ✅ RESTful API for work items and sprints
- ✅ Health check endpoints
- ✅ Prometheus metrics
- ✅ Swagger UI for interactive API testing
- ✅ OpenAPI 3.0 specification
- ✅ WebSocket support for real-time updates
- ✅ Structured logging with trace IDs
- ✅ CORS support
- ✅ Security headers
- ✅ Graceful shutdown

## Running the Server

### Quick Start

From the `go_sprint` directory:

```bash
./start-local.sh
```

### Manual Start

```bash
# Set environment variables
export DATABASE_MASTER_URL="postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable"
export DATABASE_REPLICA_URL="postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable"
export NATS_URL="nats://localhost:4222"

# Run the server
go run cmd/server/main.go
```

## Configuration

The server is configured via environment variables:

### Required

- `DATABASE_MASTER_URL` - PostgreSQL master connection string (writes)
- `DATABASE_REPLICA_URL` - PostgreSQL replica connection string (reads)

### Optional

- `NATS_URL` - NATS server URL (default: empty, runs without messaging)
- `SERVER_PORT` - HTTP server port (default: 8080)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `LOG_FORMAT` - Log format: console or json (default: console)

## Endpoints

### Documentation

- `GET /` - Service information and endpoint list
- `GET /swagger/` - Swagger UI (interactive API documentation)
- `GET /api/openapi.yaml` - OpenAPI 3.0 specification

### Health & Monitoring

- `GET /health/live` - Liveness probe (always returns 200 if server is running)
- `GET /health/ready` - Readiness probe (checks database and NATS connectivity)
- `GET /metrics` - Prometheus metrics

### API

- `GET /api/v1/workitems` - List work items
- `POST /api/v1/workitems` - Create work item
- `GET /api/v1/workitems/{id}` - Get work item by ID
- `PUT /api/v1/workitems/{id}` - Update work item
- `DELETE /api/v1/workitems/{id}` - Delete work item (soft delete)
- `GET /api/v1/workitems/search` - Search work items

- `GET /api/v1/sprints` - List sprints
- `POST /api/v1/sprints` - Create sprint
- `GET /api/v1/sprints/{id}` - Get sprint by ID
- `PUT /api/v1/sprints/{id}` - Update sprint
- `DELETE /api/v1/sprints/{id}` - Delete sprint (soft delete)

### Real-time

- `GET /ws` - WebSocket connection for real-time updates

## Architecture

The server follows a layered architecture:

```
HTTP Request
    ↓
Middleware (logging, tracing, CORS, security)
    ↓
Handler (HTTP request/response)
    ↓
Service (business logic)
    ↓
Repository (data access)
    ↓
Database
```

### Components

- **Handlers** (`internal/handlers/`) - HTTP request handling
- **Services** (`internal/service/`) - Business logic
- **Repositories** (`internal/repository/`) - Data access
- **Middleware** (`internal/middleware/`) - Cross-cutting concerns
- **Infrastructure** (`pkg/`) - Reusable packages

## Swagger UI

The server includes an embedded Swagger UI for interactive API testing:

1. Start the server
2. Open http://localhost:8080/swagger/
3. Browse and test all API endpoints
4. No additional tools required!

The Swagger UI automatically loads the OpenAPI specification from `/api/openapi.yaml`.

## Development

### Adding New Endpoints

1. Define the endpoint in OpenAPI spec (`api/openapi/*.yaml`)
2. Create handler in `internal/handlers/`
3. Register route in `cmd/server/main.go`
4. Add tests

### Adding Middleware

1. Create middleware in `internal/middleware/`
2. Add to middleware chain in `main.go`:
   ```go
   handler := middleware.Chain(
       mux,
       middleware.YourNewMiddleware(),
       // ... other middleware
   )
   ```

## Graceful Shutdown

The server handles SIGINT and SIGTERM signals for graceful shutdown:

1. Stop accepting new connections
2. Wait for active requests to complete (30 second timeout)
3. Close database connections
4. Close NATS connection
5. Exit

## Logging

All logs include:
- Timestamp
- Log level
- Message
- Structured fields (trace_id, user_id, etc.)

Example log output:
```
2024-01-15T10:30:00Z INFO Server starting address=:8080
2024-01-15T10:30:01Z INFO Database connected
2024-01-15T10:30:01Z INFO NATS connected url=nats://localhost:4222
```

## Metrics

Prometheus metrics are exposed at `/metrics`:

- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency
- `database_queries_total` - Database query count
- `database_query_duration_seconds` - Query latency
- Custom business metrics

## Troubleshooting

### Server won't start

Check logs for errors:
- Database connection issues
- Port already in use
- Missing environment variables

### Database connection error

Verify PostgreSQL is running:
```bash
docker-compose ps postgres
```

### NATS connection error

The server will work without NATS (messaging disabled). To enable:
```bash
docker-compose up -d nats
export NATS_URL="nats://localhost:4222"
```

## See Also

- [Quick Start Guide](../../QUICKSTART.md)
- [API Documentation](../../docs/api-usage-example.md)
- [Troubleshooting Guide](../../docs/troubleshooting-guide.md)
