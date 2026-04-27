# Database Setup Documentation

## Overview

The Sprint Management System uses PostgreSQL with a comprehensive schema design that supports:
- Master/replica database separation for read/write operations
- Two separate schemas: `sprint_management` and `auth`
- Full-text search capabilities
- Soft delete functionality
- Comprehensive audit logging
- UTC timestamp storage

## Schema Architecture

### Schemas

1. **sprint_management** - Contains all application data
   - work_items
   - sprints
   - work_item_dependencies
   - comments
   - activity_logs

2. **auth** - Contains user and authentication data
   - users

## Database Manager

The `pkg/database` package provides a database manager that handles:

### Master/Replica Configuration

```go
cfg := database.Config{
    MasterURL:       "postgres://user:pass@master:5432/db",
    ReplicaURL:      "postgres://user:pass@replica:5432/db",
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,
}

manager, err := database.NewManager(cfg)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// Use master for writes
_, err = manager.Master().ExecContext(ctx, "INSERT INTO ...")

// Use replica for reads
rows, err := manager.Replica().QueryContext(ctx, "SELECT ...")
```

### Local Development

For local development, both master and replica can point to the same database:

```go
cfg := database.Config{
    MasterURL:  "postgres://postgres:admin@localhost:5432/sprint_management",
    ReplicaURL: "postgres://postgres:admin@localhost:5432/sprint_management",
}
```

## Migrations

### Migration Files

Located in `migrations/` directory:

1. **000001_create_schemas.up.sql** - Creates schemas and enables UUID extension
2. **000002_create_core_tables.up.sql** - Creates work_items, sprints, dependencies
3. **000003_create_supporting_tables.up.sql** - Creates comments, activity_logs, users
4. **000004_create_search_indexes.up.sql** - Creates search and performance indexes

### Running Migrations

#### Using Go Code

```go
import "github.com/humoroushorse/go_sprint/pkg/database"

db, _ := sql.Open("postgres", databaseURL)
err := database.MigrateUp(db, "./migrations")
```

#### Using CLI Tool

```bash
go run cmd/migrate/main.go -db "postgres://..." -action up
go run cmd/migrate/main.go -db "postgres://..." -action down
go run cmd/migrate/main.go -db "postgres://..." -action version
```

## Core Tables

### work_items

Stores epics, stories, and defects with:
- Type validation (epic, story, defect)
- Status tracking (todo, in_progress, in_review, done, blocked)
- Priority levels (low, medium, high, critical)
- Parent-child relationships
- Sprint assignment
- Soft delete support

**Key Constraints:**
- `valid_story_points`: Story points must be positive
- `no_self_parent`: Work items cannot be their own parent

### sprints

Time-boxed iterations with:
- Date range validation (end_date > start_date)
- Status tracking (planned, active, completed, cancelled)
- Capacity and velocity tracking
- Soft delete support

**Key Constraints:**
- `valid_date_range`: End date must be after start date
- `valid_capacity`: Capacity points must be non-negative

### work_item_dependencies

Relationship tracking between work items:
- Dependency types: blocks, is_blocked_by, relates_to, duplicates
- Cascade deletion with work items
- Prevents self-dependencies

### comments

Discussion threads on work items:
- Associated with work items (cascade delete)
- Soft delete support
- Content validation (non-empty)

### activity_logs

Comprehensive audit trail:
- Entity type tracking (work_item, sprint, comment, dependency)
- Action tracking (created, updated, deleted, status_changed, etc.)
- JSONB for flexible change tracking
- Trace ID correlation for distributed tracing

### auth.users

User information synced from Keycloak:
- Keycloak ID mapping
- Role-based access control
- Profile information
- Active status tracking

## Indexes

### Full-Text Search

```sql
-- GIN index for full-text search
CREATE INDEX idx_work_items_search ON work_items 
USING gin(to_tsvector('english', title || ' ' || COALESCE(description, '')));

-- Trigram indexes for fuzzy search
CREATE INDEX idx_work_items_title_trgm ON work_items USING gin(title gin_trgm_ops);
```

### Performance Indexes

- Composite indexes for common query patterns
- Partial indexes for active records (WHERE deleted_at IS NULL)
- BRIN indexes for time-series data (activity_logs)

### Cursor Pagination

```sql
CREATE INDEX idx_work_items_cursor ON work_items(created_at DESC, id);
```

## Timestamp Handling

All timestamps are stored in UTC:

```sql
-- Automatic UTC conversion
created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')

-- Trigger for updated_at
CREATE TRIGGER update_work_items_updated_at 
BEFORE UPDATE ON work_items
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### Property-Based Testing

The UTC timestamp storage is validated by Property 12:

```bash
TEST_DATABASE_URL="postgres://..." go test -v -run TestProperty12 ./pkg/database/
```

This test verifies that:
- All timestamps are stored in UTC timezone
- Timestamp values are preserved accurately
- Works across all tables (work_items, sprints, comments, activity_logs, users)

## Soft Delete

All major entities support soft delete:

```sql
deleted_at TIMESTAMPTZ,
deleted_by UUID,

-- Partial index for active records
CREATE INDEX idx_work_items_deleted ON work_items(deleted_at) 
WHERE deleted_at IS NULL;

-- Partial index for soft-deleted records
CREATE INDEX idx_work_items_soft_deleted ON work_items(deleted_at, deleted_by) 
WHERE deleted_at IS NOT NULL;
```

## Health Checks

The database manager provides health check functionality:

```go
ctx := context.Background()
if err := manager.HealthCheck(ctx); err != nil {
    log.Printf("Database unhealthy: %v", err)
}
```

## Connection Pooling

Configurable connection pool settings:

```go
cfg := database.Config{
    MaxOpenConns:    25,  // Maximum open connections
    MaxIdleConns:    5,   // Maximum idle connections
    ConnMaxLifetime: 5 * time.Minute,  // Connection lifetime
}
```

## Testing

### Test Database

Create a separate test database:

```sql
CREATE DATABASE sprint_management_test;
```

### Running Tests

```bash
TEST_DATABASE_URL="postgres://postgres:admin@localhost:5432/sprint_management_test?sslmode=disable" \
  go test -v ./pkg/database/
```

## Production Considerations

### Master/Replica Setup

1. Configure separate master and replica connection strings
2. Route all writes to master
3. Route all reads to replica
4. Monitor replication lag

### Backup Strategy

1. Regular pg_dump backups
2. Point-in-time recovery (PITR) with WAL archiving
3. Test restore procedures regularly

### Monitoring

1. Connection pool metrics
2. Query performance
3. Replication lag
4. Disk usage
5. Index usage statistics

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [lib/pq Driver](https://github.com/lib/pq)
