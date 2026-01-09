# Running Sprint Management Service Locally

This guide explains how to run the Sprint Management Service with authentication on your local machine.

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- PostgreSQL database (`ttrpg-pg` container running)
- NATS server (optional, for messaging features)
- Keycloak (for authentication)

## Quick Start

### Option 1: Run All Services Together (Recommended)

From the repository root, use the startup script:

```bash
./start-all-services.sh
```

This will start:
- Authentication Service (port 8081)
- Sprint Management Service (port 8082)

### Option 2: Run Sprint Service Only

If you only want to run the sprint service:

```bash
cd go_sprint
./start-local.sh
```

**Note:** Auth endpoints will not work unless the auth service is also running.

## What Gets Started

### Authentication Service (Port 8081)
- Handles user authentication with Keycloak
- Provides login, logout, refresh, and registration endpoints
- Swagger UI: http://localhost:8081/swagger/

### Sprint Management Service (Port 8082)
- Main API for sprint and work item management
- Proxies auth requests to the auth service
- Swagger UI: http://localhost:8082/swagger/ (includes auth endpoints)

## Available Endpoints

### Via Sprint Service (Port 8082)

All endpoints are accessible through the sprint service:

**Authentication (proxied to auth service):**
- `POST /api/v1/auth/login` - Login user
- `POST /api/v1/auth/refresh` - Refresh access token
- `POST /api/v1/auth/logout` - Logout user
- `GET /api/v1/auth/user` - Get current user info
- `POST /api/v1/auth/register` - Register new user

**Sprint Management:**
- `GET /api/v1/workitems` - List work items
- `POST /api/v1/workitems` - Create work item
- `GET /api/v1/sprints` - List sprints
- `POST /api/v1/sprints` - Create sprint

**Health & Monitoring:**
- `GET /health/live` - Liveness check
- `GET /health/ready` - Readiness check
- `GET /metrics` - Prometheus metrics

**Documentation:**
- `GET /swagger/` - Swagger UI (includes auth + sprint endpoints)
- `GET /api/openapi.yaml` - Combined OpenAPI specification

## Authentication Flow

### 1. Register a New User

```bash
curl -X POST http://localhost:8082/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "SecurePass123!",
    "first_name": "Test",
    "last_name": "User"
  }'
```

### 2. Login

```bash
curl -X POST http://localhost:8082/api/v1/auth/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=testuser&password=SecurePass123!"
```

Response:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

### 3. Use Access Token

Use the access token in the Authorization header for protected endpoints:

```bash
curl -X GET http://localhost:8082/api/v1/workitems \
  -H "Authorization: Bearer eyJhbGc..."
```

### 4. Refresh Token

When the access token expires, use the refresh token:

```bash
curl -X POST http://localhost:8082/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "eyJhbGc..."}'
```

## Testing with Swagger UI

The easiest way to test the API is through Swagger UI:

1. Open http://localhost:8082/swagger/
2. Click "Authorize" button
3. Register a new user using `/api/v1/auth/register`
4. Login using `/api/v1/auth/login`
5. Copy the `access_token` from the response
6. Click "Authorize" again and paste the token
7. Now you can test protected endpoints

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────┐
│  go_sprint (Port 8082)              │
│  ┌───────────────────────────────┐  │
│  │ /api/v1/auth/* → Proxy to     │  │
│  │ go_auth service               │  │
│  └───────────────────────────────┘  │
│  ┌───────────────────────────────┐  │
│  │ /api/v1/workitems             │  │
│  │ /api/v1/sprints               │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
       │ (validates tokens)
       ▼
┌─────────────────────────────────────┐
│  go_auth (Port 8081)                │
│  Authentication Service             │
└─────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│  Keycloak                           │
└─────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│  PostgreSQL (ttrpg-pg)              │
└─────────────────────────────────────┘
```

## Troubleshooting

### Auth Service Not Available

If you see "auth_service_unavailable" errors:

1. Check if auth service is running:
   ```bash
   curl http://localhost:8081/health/live
   ```

2. Start the auth service:
   ```bash
   cd go_auth
   make run
   ```

### Database Connection Issues

1. Verify database is running:
   ```bash
   docker ps | grep ttrpg-pg
   ```

2. Check connection:
   ```bash
   psql -h localhost -U postgres -d ttrpg-pg
   ```

### Keycloak Issues

1. Verify Keycloak is running:
   ```bash
   curl http://localhost:8080
   ```

2. Check realm configuration in Keycloak admin console

## Stopping Services

Press `Ctrl+C` in the terminal where services are running.
