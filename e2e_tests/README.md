# E2E API Testing Framework

Playwright-based API testing framework for go_auth and go_sprint services.

## Setup

### Prerequisites

- Node.js 18+ and npm
- Docker and Docker Compose (for test database)
- Access to Keycloak instance

### Installation

1. Install dependencies:
   ```bash
   npm install
   ```

2. Install Playwright browsers:
   ```bash
   npx playwright install
   ```

3. Create `.env` file from template:
   ```bash
   cp .env.example .env
   ```

4. Update `.env` with your credentials (see Configuration section)

## Configuration

The framework supports multiple environments (local, staging, production). Configuration is managed through:

- Environment-specific config files in `lib/config/`
- Credentials loaded from `.env` file
- Environment selection via `ENV` environment variable (defaults to `local`)

### Required Environment Variables

Create a `.env` file with the following variables:

```env
# Keycloak Admin
KEYCLOAK_ADMIN_USERNAME=admin
KEYCLOAK_ADMIN_PASSWORD=your_admin_password

# Test Users
TEST_ADMIN_USERNAME=test_admin
TEST_ADMIN_PASSWORD=your_test_admin_password
TEST_USER_USERNAME=test_user
TEST_USER_PASSWORD=your_test_user_password

# Database
DATABASE_PASSWORD=your_db_password
```

## Running Tests

### All Tests
```bash
npm test
```

### CI/CD (excludes @stress tests)
```bash
npm run test:ci
```

### By Project
```bash
npm run test:go_auth    # go_auth service tests only
npm run test:go_sprint  # go_sprint service tests only
```

### By Tag
```bash
npm run test:health     # Health check tests only
```

### View Test Report
```bash
npm run test:report
```

## Project Structure

```
e2e_tests/
├── lib/                      # Shared libraries and utilities
│   ├── api-client.ts        # Base API client with auth
│   ├── config/              # Environment configurations
│   ├── helpers/             # Test helper functions
│   └── fixtures/            # Test data fixtures
├── tests/                   # Test suites
│   ├── health/             # Health check tests
│   ├── go_auth/            # go_auth service tests
│   └── go_sprint/          # go_sprint service tests
├── reports/                # Generated test reports
├── playwright.config.ts    # Playwright configuration
└── package.json           # Dependencies and scripts
```

## Writing Tests

Tests are organized by project and tagged appropriately:

```typescript
import { test, expect } from '@playwright/test';

test.describe('Work Items API @go_sprint', () => {
  test('should list work items', async ({ request }) => {
    const response = await request.get('/api/v1/workitems');
    expect(response.ok()).toBeTruthy();
  });
});
```

### Test Tags

- `@go_auth` - Tests for go_auth service
- `@go_sprint` - Tests for go_sprint service
- `@health` - Health check tests
- `@smoke` - Smoke tests (run on every commit)
- `@integration` - Integration tests
- `@stress` - Stress tests (excluded from CI/CD)

## Test Database

The framework uses an isolated test database to avoid interfering with development data.

Start the test database:
```bash
docker-compose -f docker-compose.test.yml up -d
```

Stop the test database:
```bash
docker-compose -f docker-compose.test.yml down
```

## Troubleshooting

### Connection Refused Errors

Ensure all services are running:
```bash
docker-compose ps
```

### Authentication Failures

1. Verify Keycloak is accessible
2. Check credentials in `.env` file
3. Ensure test users exist in Keycloak

### Database Connection Issues

1. Verify test database is running
2. Check database connection string in config
3. Ensure migrations have run successfully

## Adding New Tests

1. Create test file in appropriate directory (`tests/go_auth/` or `tests/go_sprint/`)
2. Add appropriate project tag (`@go_auth` or `@go_sprint`)
3. Use shared helpers from `lib/helpers/` for common operations
4. Follow existing test patterns for consistency

## CI/CD Integration

The framework is designed for CI/CD integration:

- Tests tagged with `@stress` are excluded via `npm run test:ci`
- JUnit XML reports are generated for CI systems
- JSON reports available for custom processing
- Screenshots/videos captured for failed tests
