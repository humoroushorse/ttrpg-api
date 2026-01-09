# Database Migrations

This directory contains database migrations for the Sprint Management System using [golang-migrate](https://github.com/golang-migrate/migrate).

## Migration Files

Migrations are numbered sequentially and come in pairs (up/down):

1. **000001_create_schemas** - Creates PostgreSQL schemas (sprint_management, auth)
2. **000002_create_core_tables** - Creates core entity tables (work_items, sprints, work_item_dependencies)
3. **000003_create_supporting_tables** - Creates supporting tables (comments, activity_logs, auth.users)
4. **000004_create_search_indexes** - Creates search and performance indexes

## Running Migrations

### Using Go Code

```go
import "github.com/humoroushorse/go_sprint/pkg/database"

// Run all pending migrations
db, _ := sql.Open("postgres", databaseURL)
err := database.MigrateUp(db, "./migrations")

// Rollback last migration
err := database.MigrateDown(db, "./migrations")

// Get current version
version, dirty, err := database.MigrateVersion(db, "./migrations")
```

### Using CLI

Install the migrate CLI:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Run migrations:
```bash
# Up
migrate -path ./migrations -database "postgres://postgres:admin@localhost:5432/sprint_management?sslmode=disable" up

# Down
migrate -path ./migrations -database "postgres://postgres:admin@localhost:5432/sprint_management?sslmode=disable" down

# Version
migrate -path ./migrations -database "postgres://postgres:admin@localhost:5432/sprint_management?sslmode=disable" version
```

## Database Schema

### Schemas

- **sprint_management** - All application data (work items, sprints, comments, etc.)
- **auth** - User and authentication data

### Core Tables

#### sprint_management.work_items
- Stores epics, stories, and defects
- Supports parent-child relationships
- Includes soft delete support
- Full-text search enabled

#### sprint_management.sprints
- Time-boxed iterations
- Date validation (end_date > start_date)
- Capacity and velocity tracking
- Soft delete support

#### sprint_management.work_item_dependencies
- Relationship types: blocks, is_blocked_by, relates_to, duplicates
- Prevents self-dependencies
- Cascade deletes with work items

#### sprint_management.comments
- Associated with work items
- Soft delete support
- Chronological ordering

#### sprint_management.activity_logs
- Audit trail for all changes
- JSONB for flexible change tracking
- Trace ID correlation

#### auth.users
- User information synced from Keycloak
- Role-based access control
- Profile information

## Timestamp Handling

All timestamps are stored in UTC timezone. The database is configured to:
- Use UTC for all timestamp operations
- Store timestamps with timezone (TIMESTAMPTZ)
- Automatically update `updated_at` columns via triggers

## Indexes

### Full-Text Search
- GIN indexes on work_items for title and description
- Trigram indexes for fuzzy search

### Performance
- Composite indexes for common query patterns
- Partial indexes for soft-deleted items
- BRIN indexes for time-series data (activity_logs)

### Cursor Pagination
- Indexes on (created_at, id) for stable ordering

## Testing

Run the property-based tests to verify schema integrity:

```bash
TEST_DATABASE_URL="postgres://postgres:admin@localhost:5432/sprint_management_test?sslmode=disable" \
  go test -v -run TestProperty12 ./pkg/database/
```

This validates that all timestamps are stored in UTC timezone (Property 12).
