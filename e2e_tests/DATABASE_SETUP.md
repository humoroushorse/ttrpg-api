# Test Database Setup

## Overview

The test database infrastructure provides an isolated PostgreSQL instance for running API tests without affecting development data.

## Configuration

- **Port**: 5433 (different from dev database on 5432)
- **Database**: sprint_management_test
- **User**: postgres
- **Password**: testpass
- **Schemas**: sprint_management, auth

## Starting the Test Database

```bash
docker-compose -f e2e_tests/docker-compose.test.yml up -d
```

## Stopping the Test Database

```bash
docker-compose -f e2e_tests/docker-compose.test.yml down
```

## Removing Test Data

To remove all test data and volumes:

```bash
docker-compose -f e2e_tests/docker-compose.test.yml down -v
```

## Database Helper Usage

The `DatabaseHelper` class provides utilities for managing the test database:

```typescript
import { DatabaseHelper } from './lib/helpers/database';

const db = new DatabaseHelper();

// Connect to database
await db.connect();

// Run migrations
await db.runMigrations();

// Load fixtures
await db.loadFixtures(['work-items', 'sprints']);

// Clean up between tests
await db.truncateAllTables();

// Execute custom queries
const results = await db.query('SELECT * FROM sprint_management.work_items');

// Disconnect
await db.disconnect();
```

## Health Check

The database includes a health check that verifies:
- PostgreSQL is running
- Database is accepting connections
- User can authenticate

Health check runs every 5 seconds with a 10-second startup period.

## Migrations

Migrations are automatically applied from `go_sprint/migrations/` directory in order:
1. 000001_create_schemas.up.sql - Creates schemas and extensions
2. 000002_create_core_tables.up.sql - Creates core tables
3. 000003_create_supporting_tables.up.sql - Creates supporting tables
4. 000004_create_search_indexes.up.sql - Creates search indexes

## Fixtures

Test fixtures are JSON files stored in `lib/fixtures/` directory. Format:

```json
{
  "data": [
    {
      "id": "uuid-here",
      "title": "Test Work Item",
      "status": "todo"
    }
  ]
}
```

Fixture files should be named after the table they populate (e.g., `work-items.json` for `work_items` table).
