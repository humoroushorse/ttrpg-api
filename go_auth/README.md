# Authentication Service (go_auth)

Reusable authentication service that integrates with Keycloak for user authentication and management.

## Features

- 🔐 User authentication (login/logout)
- 🔄 Token refresh
- 👤 User registration
- 📝 User profile retrieval
- 🔑 JWT token management
- 🌐 OpenAPI/Swagger documentation
- 🔌 Designed to be proxied by other services

## Architecture

This service acts as a bridge between your applications and Keycloak:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────┐
│      go_auth (Port 8081)            │
│  ┌───────────────────────────────┐  │
│  │  POST /auth/login             │  │
│  │  POST /auth/refresh           │  │
│  │  POST /auth/logout            │  │
│  │  POST /auth/register          │  │
│  │  GET  /auth/user              │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│          Keycloak                   │
└─────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Keycloak instance running (default: http://localhost:8080)
- Keycloak realm configured (default: "ttrpg")

### Running Standalone

```bash
cd go_auth
make run
```

The service will start on port 8081.

### Running with Sprint Management Service

Use the root-level startup script to run both services together:

```bash
./start-all-services.sh
```

## Configuration

Configuration is done via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8081 | HTTP server port |
| `KEYCLOAK_URL` | http://localhost:8080 | Keycloak server URL |
| `KEYCLOAK_REALM` | ttrpg | Keycloak realm name |
| `KEYCLOAK_CLIENT_ID` | auth-service | Keycloak client ID |
| `KEYCLOAK_CLIENT_SECRET` | | Keycloak client secret (optional) |
| `KEYCLOAK_ADMIN_USER` | admin | Keycloak admin username |
| `KEYCLOAK_ADMIN_PASS` | admin | Keycloak admin password |
| `LOG_LEVEL` | info | Logging level (debug, info, warn, error) |

## API Endpoints

### Authentication

#### Login
```bash
POST /auth/login
Content-Type: application/x-www-form-urlencoded

username=user@example.com&password=password123
```

Response:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "id_token": "eyJhbGc...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

#### Refresh Token
```bash
POST /auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGc..."
}
```

#### Logout
```bash
POST /auth/logout
Authorization: Bearer <access_token>
```

#### Get User Info
```bash
GET /auth/user
Authorization: Bearer <access_token>
```

Response:
```json
{
  "sub": "user-id",
  "preferred_username": "johndoe",
  "email": "john@example.com",
  "name": "John Doe"
}
```

#### Register User
```bash
POST /auth/register
Content-Type: application/json

{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "SecurePass123!",
  "first_name": "John",
  "last_name": "Doe"
}
```

## Documentation

- **Swagger UI**: http://localhost:8081/swagger/
- **OpenAPI Spec**: http://localhost:8081/api/openapi.yaml

## Using with Other Services

This auth service is designed to be reusable. Other services can:

1. **Proxy auth endpoints** (recommended):
   ```go
   authProxy := httputil.NewSingleHostReverseProxy(authServiceURL)
   mux.Handle("/api/v1/auth/", http.StripPrefix("/api/v1", authProxy))
   ```

2. **Use the auth client library**:
   ```go
   import "github.com/humoroushorse/go_auth/pkg/auth/client"
   
   authClient := client.NewClient("http://localhost:8081")
   userInfo, err := authClient.ValidateToken(ctx, accessToken)
   ```

3. **Use JWT middleware**:
   ```go
   import "github.com/humoroushorse/go_auth/pkg/auth/middleware"
   
   jwtMiddleware := middleware.NewJWTMiddleware(keycloakURL, realm)
   protectedHandler := jwtMiddleware.Authenticate(yourHandler)
   ```

## Development

### Build
```bash
make build
```

### Run Tests
```bash
make test
```

### Clean
```bash
make clean
```

## Project Structure

```
go_auth/
├── api/
│   └── openapi/
│       └── auth.yaml          # OpenAPI specification
├── cmd/
│   └── server/
│       ├── main.go            # Main server entry point
│       └── swagger-ui/        # Swagger UI assets
├── internal/
│   ├── handlers/
│   │   └── auth.go            # HTTP handlers
│   └── service/
│       └── auth/
│           └── keycloak.go    # Keycloak integration
├── pkg/
│   ├── auth/
│   │   ├── client/            # Auth client library
│   │   └── middleware/        # JWT middleware
│   └── config/
│       └── config.go          # Configuration
├── Makefile
└── README.md
```

## Integration Examples

### Example: Sprint Management Service

The `go_sprint` service proxies auth endpoints:

```go
// In go_sprint/cmd/server/main.go
authServiceURL, _ := url.Parse("http://localhost:8081")
authProxy := httputil.NewSingleHostReverseProxy(authServiceURL)
mux.Handle("/api/v1/auth/", http.StripPrefix("/api/v1", authProxy))
```

Users can now access auth endpoints through the sprint service:
- http://localhost:8082/api/v1/auth/login
- http://localhost:8082/api/v1/auth/refresh
- etc.

The sprint service's Swagger UI shows all endpoints (auth + sprint) in one place.

## Security Considerations

- Always use HTTPS in production
- Set `Secure: true` for cookies in production
- Rotate Keycloak client secrets regularly
- Use strong passwords for Keycloak admin account
- Enable rate limiting for auth endpoints
- Monitor failed login attempts

## License

MIT
