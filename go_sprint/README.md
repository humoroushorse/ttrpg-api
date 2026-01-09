# Go Sprint Management Service

A production-ready sprint management microservice built in Go, providing comprehensive functionality for managing software development workflows similar to Jira or Trello.

## Overview

The Sprint Management Service is a microservice-based application that provides enterprise-grade sprint and work item management capabilities with real-time collaboration, comprehensive observability, and scalable architecture.

### Key Features

- **Work Item Management**: Create and manage epics, stories, and defects with full lifecycle tracking
- **Sprint Planning**: Time-boxed iterations with capacity planning and velocity tracking
- **Real-time Collaboration**: WebSocket-based live updates for team synchronization
- **Advanced Search**: Full-text search with filtering and saved searches
- **Reporting & Analytics**: Sprint reports, burndown charts, velocity metrics, and custom dashboards
- **Audit Trail**: Comprehensive change tracking and activity logging
- **Data Import/Export**: Bulk operations with CSV, JSON, and Excel support
- **Soft Delete**: Recoverable deletion with configurable retention periods
- **Comprehensive Backup**: Automated backups with disaster recovery procedures

## Architecture

### Design Principles

- **API-First Development**: OpenAPI 3+ specifications with automated code generation
- **Domain-Driven Design**: Code organized by business domains, not technical layers
- **Type-Safe SQL**: SQLC for compile-time SQL validation and Go code generation
- **Microservice Communication**: NATS messaging for inter-service communication
- **Real-time Updates**: WebSocket support for live collaboration
- **Database Scalability**: PostgreSQL with master/replica read/write separation
- **Observability**: Structured logging, Prometheus metrics, and distributed tracing
- **Security**: JWT authentication, rate limiting, and comprehensive input validation

## Project Structure

```
go_sprint/
├── cmd/
│   ├── server/              # Main service binary
│   └── migrate/             # Database migration tool
├── internal/                # Private application code
│   ├── handlers/            # HTTP request handlers
│   ├── middleware/          # HTTP middleware (auth, logging, CORS, security)
│   ├── repository/          # Data access layer (SQLC-generated)
│   │   ├── workitems/       # Work item repository
│   │   ├── sprints/         # Sprint repository
│   │   ├── comments/        # Comments repository
│   │   └── dependencies/    # Dependencies repository
│   └── service/             # Business logic layer
│       ├── workitems/       # Work item service
│       ├── sprints/         # Sprint service
│       ├── comments/        # Comments service
│       ├── dependencies/    # Dependencies service
│       ├── search/          # Search service
│       ├── estimation/      # Sprint planning and estimation
│       ├── burndown/        # Burndown chart calculations
│       ├── reporting/       # Reports and analytics
│       ├── importexport/    # Data import/export
│       └── softdelete/      # Soft delete management
├── pkg/                     # Public reusable packages
│   ├── database/            # Database connection management
│   ├── nats/                # NATS client wrapper
│   ├── websocket/           # WebSocket hub and client management
│   ├── config/              # Configuration management
│   ├── logging/             # Structured logging with slog
│   ├── metrics/             # Prometheus metrics and health checks
│   ├── cache/               # Caching layer
│   ├── pagination/          # Cursor-based pagination
│   └── backup/              # Automated backup system
├── api/                     # API specifications and generated code
│   ├── openapi/             # OpenAPI 3+ specifications
│   │   ├── main.yaml        # Main API spec
│   │   ├── workitems.yaml   # Work items endpoints
│   │   └── sprints.yaml     # Sprints endpoints
│   ├── generated/           # Generated server code (oapi-codegen)
│   └── frontend/            # Frontend integration assets
│       ├── api-types.ts     # TypeScript interfaces
│       ├── angular-services.example.ts
│       └── websocket.example.ts
├── migrations/              # Database migrations (golang-migrate)
│   ├── 000001_create_schemas.up.sql
│   ├── 000002_create_core_tables.up.sql
│   └── ...
├── docs/                    # Comprehensive documentation
│   ├── api-usage-example.md
│   ├── database-setup.md
│   ├── configuration-and-logging.md
│   ├── nats.md              # NATS subject patterns
│   ├── frontend-integration.md
│   ├── backup-restore.md
│   ├── disaster-recovery.md
│   ├── docker-deployment.md
│   ├── kubernetes-deployment.md
│   ├── import-export-templates.md
│   ├── operational-runbook.md
│   ├── troubleshooting-guide.md
│   └── diagrams/            # Mermaid diagrams (ERD, C4, sequences)
├── k8s/                     # Kubernetes manifests
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── secrets.yaml
│   ├── sprint-service-deployment.yaml
│   ├── ingress.yaml
│   └── ...
├── scripts/                 # Utility scripts
│   ├── backup.sh            # Database backup
│   ├── restore.sh           # Database restore
│   ├── backup-system.sh     # Comprehensive system backup
│   ├── verify-system.sh     # System health verification
│   └── generate-api.sh      # API code generation
├── tests/                   # Test suites
│   └── deployment/          # Deployment tests
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Local development environment
├── docker-compose.prod.yml  # Production-like environment
├── Makefile                 # Build and development tasks
└── README.md                # This file
```

### Domain Organization

Each domain follows a consistent layered architecture:

1. **API Layer**: OpenAPI specifications defining the contract
2. **Handler Layer**: HTTP request/response handling and validation
3. **Service Layer**: Business logic, orchestration, and validation
4. **Repository Layer**: Data access using SQLC-generated type-safe code
5. **SQL Layer**: Raw SQL queries with SQLC annotations for code generation

## Dependencies

### Shared Authentication Library

This service imports the shared authentication library from `go_auth`:

```go
import (
    "github.com/humoroushorse/go_auth/pkg/auth/middleware"
    "github.com/humoroushorse/go_auth/pkg/auth/models"
)
```

## Local Development

### Using Go Workspaces (Recommended)

```bash
# From the parent directory containing both go_auth and go_sprint
go work init ./go_auth ./go_sprint

# Now you can work on both repositories simultaneously
cd go_sprint
go run cmd/server/main.go
```

### Using Replace Directive

Add to `go.mod` for local development:

```go
replace github.com/humoroushorse/go_auth/pkg/auth => ../go_auth/pkg/auth
```

### Docker Compose

```bash
# Start all dependencies (PostgreSQL, NATS, Keycloak, Auth Service)
docker-compose up -d

# Run sprint service
make run
```

## NATS Subject Patterns

The Sprint Management Service uses the following NATS subject patterns:

### Work Item Operations
- `sprint.{trace_id}.workitem.create.request`
- `sprint.{trace_id}.workitem.create.response`
- `sprint.{trace_id}.workitem.update.request`
- `sprint.{trace_id}.workitem.update.response`

### Sprint Operations
- `sprint.{trace_id}.sprint.create.request`
- `sprint.{trace_id}.sprint.create.response`
- `sprint.{trace_id}.sprint.close.request`
- `sprint.{trace_id}.sprint.close.response`

### Real-time Notifications
- `sprint.{trace_id}.notification.workitem.created`
- `sprint.{trace_id}.notification.workitem.updated`
- `sprint.{trace_id}.notification.sprint.status_changed`

See [docs/nats.md](docs/nats.md) for detailed documentation.

## Requirements

### System Requirements

- **Go**: 1.21 or higher
- **PostgreSQL**: 15 or higher
- **NATS**: 2.10 or higher
- **Keycloak**: 22 or higher (via Auth Service)
- **Docker**: 20.10+ (for containerized deployment)
- **Kubernetes**: 1.25+ (for production deployment)

### Development Tools

- **make**: Build automation
- **golang-migrate**: Database migrations
- **oapi-codegen**: OpenAPI code generation
- **sqlc**: Type-safe SQL code generation
- **Docker Compose**: Local development environment

## Getting Started

### Quick Start (Docker Compose)

The fastest way to get started is using Docker Compose, which sets up all dependencies:

```bash
# Clone the repository
git clone https://github.com/humoroushorse/go_sprint
cd go_sprint

# Start all services (PostgreSQL, NATS, Keycloak, Auth Service, Sprint Service)
docker-compose up -d

# Check service health
curl http://localhost:8082/health/ready

# View logs
docker-compose logs -f sprint-service

# Stop all services
docker-compose down
```

The service will be available at:
- **API**: http://localhost:8082/api/v1
- **Metrics**: http://localhost:8082/metrics
- **WebSocket**: ws://localhost:8082/ws

### Local Development Setup

For active development, you'll want to run the service locally:

#### 1. Set Up Dependencies

```bash
# Start infrastructure services
docker-compose up -d postgres nats keycloak auth-service

# Wait for services to be ready
./scripts/wait-for-services.sh
```

#### 2. Configure Environment

```bash
# Copy example configuration
cp .env.example .env

# Edit configuration as needed
vim .env

# Required environment variables:
export DATABASE_MASTER_URL="postgres://postgres:postgres@localhost:5432/sprint_management?sslmode=disable"
export DATABASE_REPLICA_URL="postgres://postgres:postgres@localhost:5432/sprint_management?sslmode=disable"
export NATS_URL="nats://localhost:4222"
export AUTH_SERVICE_URL="http://localhost:8081"
export KEYCLOAK_URL="http://localhost:8080"
export KEYCLOAK_REALM="sprint-management"
```

#### 3. Run Database Migrations

```bash
# Install golang-migrate if not already installed
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
make migrate-up

# Verify migrations
make migrate-version
```

#### 4. Generate Code

```bash
# Generate API server code from OpenAPI specs
make generate-api

# Generate database code from SQL queries
make generate-sqlc
```

#### 5. Run the Service

```bash
# Install dependencies
go mod download

# Run the service
make run

# Or with hot reload (requires air)
make dev
```

#### 6. Verify Installation

```bash
# Check health
curl http://localhost:8080/health/ready

# Create a test work item
curl -X POST http://localhost:8080/api/v1/workitems \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "story",
    "title": "Test Story",
    "description": "Testing the API",
    "priority": "medium"
  }'
```

### Using Go Workspaces (Multi-Repository Development)

If you're developing both `go_auth` and `go_sprint` simultaneously:

```bash
# From the parent directory containing both repositories
go work init ./go_auth ./go_sprint

# Now changes in go_auth/pkg/auth are immediately available in go_sprint
cd go_sprint
go run cmd/server/main.go
```

### Using Replace Directive (Alternative)

For local development without workspaces, add to `go.mod`:

```go
replace github.com/humoroushorse/go_auth/pkg/auth => ../go_auth/pkg/auth
```

## Configuration

The service uses a combination of environment variables and YAML configuration files for flexibility across different deployment environments.

### Environment Variables

```bash
# Database Configuration
DATABASE_MASTER_URL="postgres://user:pass@master:5432/sprint_management?sslmode=disable"
DATABASE_REPLICA_URL="postgres://user:pass@replica:5432/sprint_management?sslmode=disable"
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5

# NATS Configuration
NATS_URL="nats://nats-server:4222"
NATS_RECONNECT_WAIT=2s
NATS_MAX_RECONNECTS=10

# Authentication
AUTH_SERVICE_URL="http://auth-service:8080"
KEYCLOAK_URL="http://keycloak:8080"
KEYCLOAK_REALM="sprint-management"
KEYCLOAK_CLIENT_ID="sprint-client"

# Logging
LOG_LEVEL="info"          # debug, info, warn, error
LOG_FORMAT="json"         # json or console
LOG_ENABLE_COLORS=false   # true for local development

# Server
SERVER_PORT=8080
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s

# Metrics
METRICS_ENABLED=true
METRICS_PORT=8081

# WebSocket
WEBSOCKET_PING_INTERVAL=30s
WEBSOCKET_PONG_WAIT=60s

# Backup
BACKUP_SCHEDULE="0 2 * * *"  # Daily at 2 AM
BACKUP_RETENTION_DAYS=30
BACKUP_PATH="/backups"
```

### Configuration File (config.yaml)

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 30s

database:
  master_url: ${DATABASE_MASTER_URL}
  replica_url: ${DATABASE_REPLICA_URL}
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

nats:
  url: ${NATS_URL}
  reconnect_wait: 2s
  max_reconnects: 10

auth:
  service_url: ${AUTH_SERVICE_URL}
  keycloak_url: ${KEYCLOAK_URL}
  realm: ${KEYCLOAK_REALM}
  client_id: ${KEYCLOAK_CLIENT_ID}

logging:
  level: info
  format: json
  enable_colors: false

metrics:
  enabled: true
  port: 8081
  path: /metrics

websocket:
  ping_interval: 30s
  pong_wait: 60s
  write_wait: 10s
  max_message_size: 512

rate_limiting:
  enabled: true
  requests_per_minute: 100
  burst: 20

cors:
  allowed_origins:
    - "https://sprint.yourdomain.com"
  allow_credentials: true

security:
  enable_https_redirect: true
  enable_hsts: true
  csp_policy: "default-src 'self'"

backup:
  schedule: "0 2 * * *"
  retention_days: 30
  backup_path: "/backups"
  include_schema: true
  include_data: true
```

See [Configuration and Logging Guide](docs/configuration-and-logging.md) for detailed configuration options.

## Testing

The project includes comprehensive test coverage with unit tests, integration tests, and property-based tests.

### Running Tests

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests (requires Docker)
make test-integration

# Run with coverage report
make test-coverage

# Run specific test
go test -v ./internal/service/workitems/...

# Run property-based tests
go test -v ./pkg/pagination/... -run Property
```

### Test Organization

```
tests/
├── unit/                    # Unit tests (70% of tests)
│   ├── service/             # Business logic tests
│   ├── repository/          # Data access tests
│   └── handlers/            # HTTP handler tests
├── integration/             # Integration tests (20% of tests)
│   ├── database/            # Database integration
│   ├── nats/                # NATS messaging
│   └── api/                 # API contract tests
└── deployment/              # Deployment tests (10% of tests)
    ├── docker_test.go       # Docker deployment
    └── kubernetes_test.go   # Kubernetes deployment
```

### Property-Based Testing

The project uses property-based testing for critical correctness properties:

```bash
# Run all property tests
go test -v ./... -run Property

# Examples of tested properties:
# - Work item type validation
# - Sprint date validation
# - Dependency resolution
# - Soft delete behavior
# - Pagination consistency
```

### Test Coverage

Current test coverage: **85%+**

```bash
# Generate coverage report
make test-coverage

# View coverage in browser
go tool cover -html=coverage.out
```

### Integration Test Requirements

Integration tests require Docker to spin up test containers:

```bash
# Install testcontainers-go dependencies
go get github.com/testcontainers/testcontainers-go

# Run integration tests
make test-integration
```

## Backup and Restore

The system includes comprehensive backup and restore capabilities:

```bash
# Create database backup
make backup

# Create comprehensive system backup (database + config + docs)
make backup-system

# List available backups
make backup-list

# Restore from backup
make restore BACKUP_FILE=./backups/sprint_management_20240115_120000.sql

# Verify system health
make verify-system
```

### Automated Backups

Configure automated backups in your application:

```go
import "github.com/humoroushorse/go_sprint/pkg/backup"

config := backup.BackupConfig{
    Schedule:      "0 2 * * *",  // Daily at 2 AM
    RetentionDays: 30,            // Keep 30 days
    BackupPath:    "/backups",
    Host:          "localhost",
    Port:          "5432",
    Username:      "postgres",
    Database:      "sprint_management",
    IncludeSchema: true,
    IncludeData:   true,
}

scheduler := backup.NewBackupScheduler(config, logger)
scheduler.Start(ctx, 24*time.Hour)
defer scheduler.Stop()
```

See [docs/backup-restore.md](docs/backup-restore.md) for detailed documentation and [docs/disaster-recovery.md](docs/disaster-recovery.md) for disaster recovery procedures.

## API Documentation

### OpenAPI Specifications

The API is defined using OpenAPI 3+ specifications organized by domain:

- **Main Spec**: `api/openapi/main.yaml`
- **Work Items**: `api/openapi/workitems.yaml`
- **Sprints**: `api/openapi/sprints.yaml`
- **Combined**: `api/openapi/combined.yaml` (generated)

### API Endpoints

#### Work Items

```bash
# Create work item
POST /api/v1/workitems

# Get work item
GET /api/v1/workitems/{id}

# Update work item
PUT /api/v1/workitems/{id}

# Delete work item (soft delete)
DELETE /api/v1/workitems/{id}

# List work items with filtering
GET /api/v1/workitems?status=in_progress&assignee={user_id}

# Search work items
GET /api/v1/workitems/search?q=authentication

# Get work item dependencies
GET /api/v1/workitems/{id}/dependencies

# Add dependency
POST /api/v1/workitems/{id}/dependencies
```

#### Sprints

```bash
# Create sprint
POST /api/v1/sprints

# Get sprint
GET /api/v1/sprints/{id}

# Update sprint
PUT /api/v1/sprints/{id}

# Close sprint
POST /api/v1/sprints/{id}/close

# Get sprint metrics
GET /api/v1/sprints/{id}/metrics

# Get burndown chart
GET /api/v1/sprints/{id}/burndown
```

#### Comments

```bash
# Add comment
POST /api/v1/workitems/{id}/comments

# List comments
GET /api/v1/workitems/{id}/comments

# Update comment
PUT /api/v1/comments/{id}

# Delete comment
DELETE /api/v1/comments/{id}
```

#### Reports

```bash
# Get sprint report
GET /api/v1/reports/sprints/{id}

# Get team velocity
GET /api/v1/reports/velocity?team={team_id}

# Get work item cycle time
GET /api/v1/reports/cycle-time
```

### Frontend Integration

TypeScript interfaces and Angular service examples are available in `api/frontend/`:

- **TypeScript Types**: `api/frontend/api-types.ts`
- **Angular Services**: `api/frontend/angular-services.example.ts`
- **WebSocket Client**: `api/frontend/websocket.example.ts`
- **Component Examples**: `api/frontend/component.example.ts`

See [Frontend Integration Guide](docs/frontend-integration.md) for detailed integration instructions.

### API Usage Examples

See [API Usage Examples](docs/api-usage-example.md) for comprehensive examples of all API operations.

## License

MIT


## Deployment

### Docker Deployment

#### Build Docker Image

```bash
# Build image
docker build -t sprint-management:latest .

# Or using make
make docker-build
```

#### Run with Docker

```bash
# Run container
docker run -d \
  --name sprint-management \
  --network sprint-network \
  -p 8080:8080 \
  -e DATABASE_MASTER_URL="$DATABASE_MASTER_URL" \
  -e DATABASE_REPLICA_URL="$DATABASE_REPLICA_URL" \
  -e NATS_URL="$NATS_URL" \
  sprint-management:latest

# Check logs
docker logs -f sprint-management

# Check health
curl http://localhost:8080/health/ready
```

#### Docker Compose (Production)

```bash
# Start production environment
docker-compose -f docker-compose.prod.yml up -d

# Scale service
docker-compose -f docker-compose.prod.yml up -d --scale sprint-service=3

# View logs
docker-compose -f docker-compose.prod.yml logs -f

# Stop environment
docker-compose -f docker-compose.prod.yml down
```

See [Docker Deployment Guide](docs/docker-deployment.md) for detailed instructions.

### Kubernetes Deployment

#### Prerequisites

- Kubernetes cluster (1.25+)
- kubectl configured
- Helm (optional, for dependency management)

#### Deploy to Kubernetes

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Create secrets
kubectl apply -f k8s/secrets.yaml

# Create configmap
kubectl apply -f k8s/configmap.yaml

# Deploy PostgreSQL
kubectl apply -f k8s/postgres-deployment.yaml

# Deploy NATS
kubectl apply -f k8s/nats-deployment.yaml

# Deploy Auth Service
kubectl apply -f k8s/auth-service-deployment.yaml

# Deploy Sprint Service
kubectl apply -f k8s/sprint-service-deployment.yaml

# Create ingress
kubectl apply -f k8s/ingress.yaml

# Verify deployment
kubectl get pods -n sprint-management
kubectl get services -n sprint-management
```

#### Using Deployment Script

```bash
# Deploy all components
./k8s/deploy.sh

# Validate deployment
./k8s/validate.sh
```

#### Horizontal Pod Autoscaling

```bash
# Enable autoscaling
kubectl autoscale deployment sprint-management \
  --cpu-percent=70 \
  --min=3 \
  --max=10 \
  -n sprint-management

# Check autoscaler status
kubectl get hpa -n sprint-management
```

#### Rolling Updates

```bash
# Update image
kubectl set image deployment/sprint-management \
  sprint-management=sprint-management:v1.2.3 \
  -n sprint-management

# Monitor rollout
kubectl rollout status deployment/sprint-management -n sprint-management

# Rollback if needed
kubectl rollout undo deployment/sprint-management -n sprint-management
```

See [Kubernetes Deployment Guide](docs/kubernetes-deployment.md) for comprehensive deployment instructions.

## Monitoring and Observability

### Health Checks

```bash
# Liveness probe (is service alive?)
curl http://localhost:8080/health/live

# Readiness probe (can service handle traffic?)
curl http://localhost:8080/health/ready
```

### Metrics

Prometheus metrics are exposed at `/metrics`:

```bash
# View all metrics
curl http://localhost:8081/metrics

# Key metrics:
# - http_requests_total: Total HTTP requests
# - http_request_duration_seconds: Request latency
# - database_queries_total: Database query count
# - database_query_duration_seconds: Query latency
# - nats_messages_published_total: NATS messages published
# - work_items_created_total: Work items created
# - sprint_velocity_points: Sprint velocity
# - active_sprints_count: Active sprints
# - websocket_connections_active: Active WebSocket connections
```

### Logging

Structured logging with trace correlation:

```bash
# View logs (Kubernetes)
kubectl logs -f deployment/sprint-management -n sprint-management

# Filter by log level
kubectl logs deployment/sprint-management -n sprint-management | grep '"level":"ERROR"'

# Follow specific trace
kubectl logs deployment/sprint-management -n sprint-management | grep '"trace_id":"<trace-id>"'
```

### Distributed Tracing

All requests include trace IDs for correlation across services:

```bash
# Make request with trace ID
curl -H "X-Trace-ID: my-trace-123" http://localhost:8080/api/v1/workitems

# Follow trace across services
kubectl logs deployment/sprint-management -n sprint-management | grep "my-trace-123"
kubectl logs deployment/auth-service -n sprint-management | grep "my-trace-123"
```

## Operations

### Database Operations

#### Migrations

```bash
# Check current version
make migrate-version

# Run migrations
make migrate-up

# Rollback one migration
make migrate-down

# Force specific version (use with caution)
make migrate-force VERSION=5
```

#### Backups

```bash
# Create database backup
make backup

# Create comprehensive system backup (database + config + docs)
make backup-system

# List available backups
make backup-list

# Restore from backup
make restore BACKUP_FILE=./backups/sprint_management_20240115_120000.sql

# Verify system after restore
make verify-system
```

See [Backup and Restore Guide](docs/backup-restore.md) and [Disaster Recovery](docs/disaster-recovery.md) for detailed procedures.

### Data Import/Export

```bash
# Import work items from CSV
curl -X POST http://localhost:8080/api/v1/admin/import \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -F "file=@workitems.csv" \
  -F "format=csv"

# Export work items to JSON
curl -X GET "http://localhost:8080/api/v1/admin/export?format=json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -o export.json

# Export sprint report to Excel
curl -X GET "http://localhost:8080/api/v1/admin/export?format=excel&sprint_id={id}" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -o sprint_report.xlsx
```

See [Import/Export Templates](docs/import-export-templates.md) for data format specifications.

### Maintenance

```bash
# Clean up soft-deleted items (older than retention period)
curl -X POST http://localhost:8080/api/v1/admin/cleanup-deleted \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"retention_days": 30}'

# Rebuild search indexes
curl -X POST http://localhost:8080/api/v1/admin/reindex \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Database vacuum and analyze
psql $DATABASE_MASTER_URL -c "VACUUM ANALYZE;"
```

## Troubleshooting

### Common Issues

#### Service Won't Start

```bash
# Check logs
kubectl logs <pod-name> -n sprint-management

# Verify configuration
kubectl get configmap sprint-management-config -n sprint-management -o yaml

# Test database connection
psql $DATABASE_MASTER_URL -c "SELECT 1"

# Test NATS connection
curl http://nats-server:8222/varz
```

#### High Error Rate

```bash
# Check recent errors
kubectl logs --tail=100 deployment/sprint-management -n sprint-management | \
  grep '"level":"ERROR"'

# Check service health
curl http://localhost:8080/health/ready

# Check dependencies
curl http://auth-service:8080/health/ready
curl http://nats-server:8222/varz
```

#### Slow Performance

```bash
# Check metrics
curl http://localhost:8081/metrics | grep duration

# Check database slow queries
psql $DATABASE_MASTER_URL -c "
  SELECT query, mean_exec_time, calls
  FROM pg_stat_statements
  ORDER BY mean_exec_time DESC
  LIMIT 10;
"

# Check connection pool
curl http://localhost:8081/metrics | grep database_connections
```

See [Troubleshooting Guide](docs/troubleshooting-guide.md) for comprehensive troubleshooting procedures.

## Documentation

### Comprehensive Guides

- **[API Usage Examples](docs/api-usage-example.md)**: Complete API usage with examples
- **[Database Setup](docs/database-setup.md)**: Database configuration and schema
- **[Configuration and Logging](docs/configuration-and-logging.md)**: Configuration options and logging setup
- **[NATS Messaging](docs/nats.md)**: NATS subject patterns and messaging
- **[Frontend Integration](docs/frontend-integration.md)**: Angular/TypeScript integration guide
- **[Backup and Restore](docs/backup-restore.md)**: Backup procedures and restoration
- **[Disaster Recovery](docs/disaster-recovery.md)**: Disaster recovery procedures
- **[Docker Deployment](docs/docker-deployment.md)**: Docker deployment guide
- **[Kubernetes Deployment](docs/kubernetes-deployment.md)**: Kubernetes deployment guide
- **[Import/Export Templates](docs/import-export-templates.md)**: Data import/export formats
- **[Operational Runbook](docs/operational-runbook.md)**: Operations and maintenance procedures
- **[Troubleshooting Guide](docs/troubleshooting-guide.md)**: Common issues and solutions

### Architecture Diagrams

Visual documentation is available in `docs/diagrams/`:

- **ERD**: Entity Relationship Diagrams
- **C4 Diagrams**: System context, containers, and components
- **Sequence Diagrams**: NATS message flows and API interactions
- **User Flows**: Major user journeys and workflows

## Contributing

### Development Workflow

1. Create a feature branch
2. Make changes with tests
3. Run tests and linting: `make test lint`
4. Commit with conventional commits
5. Create pull request

### Code Standards

- Follow Go best practices and conventions
- Write tests for all new functionality
- Update documentation for API changes
- Use structured logging with trace IDs
- Implement proper error handling

### Pre-commit Hooks

```bash
# Install pre-commit hooks
make install-hooks

# Hooks will run:
# - go fmt
# - go vet
# - golangci-lint
# - tests
```

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

### Getting Help

- **Documentation**: See [docs/](docs/) directory
- **Issues**: GitHub Issues for bug reports and feature requests
- **Discussions**: GitHub Discussions for questions and community support

### Emergency Contacts

For production issues, see [Operational Runbook](docs/operational-runbook.md) for on-call contacts and escalation procedures.

## Acknowledgments

Built with:
- [Go](https://golang.org/)
- [PostgreSQL](https://www.postgresql.org/)
- [NATS](https://nats.io/)
- [Keycloak](https://www.keycloak.org/)
- [oapi-codegen](https://github.com/deepmap/oapi-codegen)
- [sqlc](https://sqlc.dev/)
- [golang-migrate](https://github.com/golang-migrate/migrate)
