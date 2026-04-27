# Quick Start Guide

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- Port 8080, 5432, 4222, and 8222 available

## Option 1: Automated Start (Recommended)

```bash
cd go_sprint
./start-local.sh
```

This script will:
1. Start PostgreSQL and NATS
2. Run database migrations
3. Build the server
4. Start the Sprint Management API

## Option 2: Manual Start

### 1. Start Infrastructure

```bash
cd go_sprint
docker-compose up -d postgres nats
```

### 2. Wait for Services

```bash
# Wait for PostgreSQL
until docker-compose exec -T postgres pg_isready -U postgres; do sleep 1; done

# Wait for NATS
until curl -s http://localhost:8222/healthz > /dev/null; do sleep 1; done
```

### 3. Run Migrations

```bash
export DATABASE_MASTER_URL="postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable"

# Create database if it doesn't exist
docker-compose exec -T postgres psql -U postgres -c "CREATE DATABASE sprint_management;" || true

# Run migrations
for migration in migrations/*.up.sql; do
    docker-compose exec -T postgres psql -U postgres -d sprint_management -f - < "$migration"
done
```

### 4. Start the Server

```bash
export DATABASE_MASTER_URL="postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable"
export DATABASE_REPLICA_URL="postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable"
export NATS_URL="nats://localhost:4222"
export LOG_LEVEL="info"
export LOG_FORMAT="console"

go run cmd/server/main.go
```

## Access the API

Once the server is running, you can access:

### Swagger UI (Interactive API Documentation)
**http://localhost:8080/swagger/**

This provides an interactive interface where you can:
- Browse all API endpoints
- See request/response schemas
- Try out API calls directly from the browser
- View example requests and responses

### OpenAPI Specification
**http://localhost:8080/api/openapi.yaml**

Raw OpenAPI 3.0 specification file

### Health Checks

```bash
# Liveness check
curl http://localhost:8080/health/live

# Readiness check
curl http://localhost:8080/health/ready
```

### API Endpoints

```bash
# Get service info
curl http://localhost:8080/

# List work items
curl http://localhost:8080/api/v1/workitems

# Create work item
curl -X POST http://localhost:8080/api/v1/workitems \
  -H "Content-Type: application/json" \
  -d '{
    "type": "story",
    "title": "My First Story",
    "description": "Testing the API",
    "priority": "medium",
    "status": "todo"
  }'

# List sprints
curl http://localhost:8080/api/v1/sprints

# Create sprint
curl -X POST http://localhost:8080/api/v1/sprints \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sprint 1",
    "start_date": "2024-01-15",
    "end_date": "2024-01-29",
    "capacity_points": 80
  }'
```

### Metrics

```bash
# Prometheus metrics
curl http://localhost:8080/metrics
```

## Testing with Swagger UI

1. Open **http://localhost:8080/swagger/** in your browser
2. You'll see all available endpoints organized by tags
3. Click on any endpoint to expand it
4. Click "Try it out" to enable the form
5. Fill in the parameters
6. Click "Execute" to make the request
7. View the response below

### Example: Create a Work Item via Swagger

1. Navigate to **POST /api/v1/workitems**
2. Click "Try it out"
3. Edit the request body:
   ```json
   {
     "type": "story",
     "title": "User Authentication",
     "description": "Implement user login and registration",
     "priority": "high",
     "status": "todo",
     "story_points": 5
   }
   ```
4. Click "Execute"
5. See the response with the created work item including its ID

## Stopping the Server

Press `Ctrl+C` in the terminal where the server is running.

To stop infrastructure services:

```bash
docker-compose down
```

To stop and remove all data:

```bash
docker-compose down -v
```

## Troubleshooting

### Port Already in Use

If port 8080 is already in use, you can change it:

```bash
export SERVER_PORT=8081
go run cmd/server/main.go
```

Then access at http://localhost:8081

### Database Connection Error

Make sure PostgreSQL is running:

```bash
docker-compose ps postgres
docker-compose logs postgres
```

### NATS Connection Error

The server will work without NATS (messaging features will be disabled). To enable NATS:

```bash
docker-compose ps nats
docker-compose logs nats
```

### Migrations Failed

Run migrations manually:

```bash
docker-compose exec postgres psql -U postgres -d sprint_management -f - < migrations/000001_create_schemas.up.sql
docker-compose exec postgres psql -U postgres -d sprint_management -f - < migrations/000002_create_core_tables.up.sql
docker-compose exec postgres psql -U postgres -d sprint_management -f - < migrations/000003_create_supporting_tables.up.sql
docker-compose exec postgres psql -U postgres -d sprint_management -f - < migrations/000004_create_search_indexes.up.sql
```

## Next Steps

- Explore the API using Swagger UI
- Check out the [API Documentation](docs/api-usage-example.md)
- Read the [Configuration Guide](docs/configuration-and-logging.md)
- See [Troubleshooting Guide](docs/troubleshooting-guide.md) for more help

## Development

### Run Tests

```bash
make test
```

### Watch Logs

```bash
# Application logs (in the terminal where server is running)

# PostgreSQL logs
docker-compose logs -f postgres

# NATS logs
docker-compose logs -f nats
```

### Database Access

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U postgres -d sprint_management

# List tables
\dt sprint_management.*

# Query work items
SELECT * FROM sprint_management.work_items;
```

## Production Deployment

For production deployment, see:
- [Docker Deployment Guide](docs/docker-deployment.md)
- [Kubernetes Deployment Guide](docs/kubernetes-deployment.md)
- [Operational Runbook](docs/operational-runbook.md)
