# Auth Package - Shared Authentication Library

This package provides shared authentication functionality that can be imported by other services.

## Overview

The `pkg/auth` package is a shared library that provides:

- JWT validation middleware with Keycloak integration
- User models with secure logging (LogValuer interface)
- Auth service client for inter-service communication
- Trace ID middleware for request correlation
- Role-based authorization middleware
- Common authentication types and utilities

## Package Structure

```
pkg/auth/
├── middleware/      # JWT validation and authentication middleware
│   ├── jwt.go       # JWT token validation
│   ├── auth.go      # Role-based authorization
│   ├── trace.go     # Trace ID middleware
│   └── jwt_test.go  # Property-based tests
├── models/          # User models and domain types
│   ├── user.go      # User model with LogValuer
│   └── claims.go    # JWT claims structures
├── client/          # Auth service HTTP client
│   └── client.go    # HTTP client for auth service
└── types/           # Shared request/response types
    ├── requests.go  # Request types
    └── responses.go # Response types
```

## Installation

```bash
go get github.com/humoroushorse/go_auth/pkg/auth
```

## Usage in Other Services

### Basic JWT Authentication

```go
import (
    "github.com/humoroushorse/go_auth/pkg/auth/middleware"
)

// Create JWT validator
validator := middleware.NewJWTValidator(middleware.JWTConfig{
    KeycloakURL: "http://localhost:8080",
    Realm:       "myrealm",
    ClientID:    "myclient",
})

// Use middleware (TraceID should be first)
router.Use(middleware.TraceID())
router.Use(middleware.JWTAuth(validator))
```

### Role-Based Authorization

```go
// Require specific role
adminRouter := router.PathPrefix("/admin").Subrouter()
adminRouter.Use(middleware.RequireRole(validator, "admin"))

// Require any of multiple roles
apiRouter := router.PathPrefix("/api").Subrouter()
apiRouter.Use(middleware.RequireAnyRole(validator, "user", "admin"))

// Optional authentication (user extracted if present)
publicRouter := router.PathPrefix("/public").Subrouter()
publicRouter.Use(middleware.OptionalAuth(validator))
```

### Extracting User from Context

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.GetUser(r.Context())
    if err != nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Check roles
    if user.HasRole("admin") {
        // Admin-specific logic
    }
    
    // Use user information
    fmt.Fprintf(w, "Hello, %s!", user.Username)
}
```

### Auth Client for Service Communication

```go
import (
    "github.com/humoroushorse/go_auth/pkg/auth/client"
)

// Create auth client
authClient := client.NewAuthClient(client.Config{
    BaseURL: "http://auth-service:8080",
    Timeout: 10 * time.Second,
})

// Validate token
user, err := authClient.ValidateToken(ctx, token)

// Login
tokenResp, err := authClient.Login(ctx, "username", "password")

// Refresh token
newToken, err := authClient.RefreshToken(ctx, refreshToken)

// Logout
err := authClient.Logout(ctx, refreshToken)
```

### Trace ID Middleware

```go
// Add trace ID middleware (should be first in chain)
router.Use(middleware.TraceID())

// Extract trace ID in handlers
func myHandler(w http.ResponseWriter, r *http.Request) {
    traceID := middleware.GetTraceID(r.Context())
    logger.Info("processing request", slog.String("trace_id", traceID))
}
```

## Development

This package is designed to be imported by other services. When developing locally, use Go workspaces or replace directives to reference the local version.

### Go Workspace (Recommended)

```bash
# From parent directory
go work init ./go_auth ./go_sprint
```

### Replace Directive

```go
// In consuming service's go.mod
replace github.com/humoroushorse/go_auth/pkg/auth => ../go_auth/pkg/auth
```

## Components

### Middleware

- **JWTAuth**: Validates JWT tokens and extracts user information
- **RequireRole**: Requires user to have a specific role
- **RequireAnyRole**: Requires user to have any of the specified roles
- **RequireAllRoles**: Requires user to have all of the specified roles
- **OptionalAuth**: Extracts user if token present, but doesn't require it
- **TraceID**: Adds trace ID to request context for distributed tracing

### Models

- **User**: User model with LogValuer interface for secure logging
  - `HasRole(role string) bool`: Check if user has a specific role
  - `HasAnyRole(roles ...string) bool`: Check if user has any of the roles
  - `HasAllRoles(roles ...string) bool`: Check if user has all roles
- **JWTClaims**: JWT claims structure with Keycloak support
  - `ToUser() (*User, error)`: Convert claims to User model
  - `GetRoles() []string`: Extract all roles from claims
  - `HasRole(role string) bool`: Check if claims contain a role

### Client

- **AuthClient**: HTTP-based client for communicating with auth service
  - `ValidateToken(ctx, token) (*User, error)`: Validate JWT token
  - `RefreshToken(ctx, refreshToken) (*TokenResponse, error)`: Refresh token
  - `Login(ctx, username, password) (*TokenResponse, error)`: Login user
  - `Logout(ctx, refreshToken) error`: Logout user

### Types

- **ValidateTokenRequest**: Token validation request structure
- **ValidateTokenResponse**: Token validation response with user info
- **TokenResponse**: Token response with access and refresh tokens
- **ErrorResponse**: Standardized error response format
- **LoginRequest**: Login request structure
- **LogoutRequest**: Logout request structure

## Security

All models implement the `slog.LogValuer` interface to prevent sensitive data (passwords, tokens, API keys, emails) from being logged.

Example:
```go
user := &models.User{
    Username: "john",
    Email:    "john@example.com", // Won't be logged
}

logger.Info("user action", slog.Any("user", user))
// Output: {"user":{"id":"...","username":"john","keycloak_id":"...","is_active":true}}
```

## Testing

The package includes comprehensive property-based tests using `gopter`:

```bash
# Run all tests
go test ./...

# Run property-based tests
go test -v -run TestProperty ./middleware

# Run with coverage
go test -cover ./...
```

### Property-Based Tests

- **Token Validation States**: Tests valid, expired, and invalid signature tokens
- **User Context Extraction**: Tests accurate extraction of user information from JWT claims
- **Role Checking**: Tests role validation logic

## Configuration

### JWT Validator Configuration

```go
config := middleware.JWTConfig{
    KeycloakURL:     "http://localhost:8080",  // Keycloak base URL
    Realm:           "myrealm",                 // Keycloak realm
    ClientID:        "myclient",                // Client ID
    PublicKey:       nil,                       // Optional: provide RSA public key
    RefreshInterval: 1 * time.Hour,             // How often to refresh public key
    SkipValidation:  false,                     // Set to true for testing only
}
```

### Auth Client Configuration

```go
config := client.Config{
    BaseURL: "http://auth-service:8080",  // Auth service URL
    Timeout: 10 * time.Second,             // Request timeout
}
```

## Requirements Validated

- **Requirement 9.1**: Authentication service integration
- **Requirement 9.2**: JWT validation middleware
- **Requirement 9.3**: Trace ID middleware
- **Requirement 5.1**: Context-based trace ID propagation
- **Requirement 5.2**: Request correlation
- **Requirement 5.6**: LogValuer interface for secure logging
- **Requirement 5.7**: Sensitive data protection in logs
- **Requirement 5.8**: Structured logging support

## Error Handling

The library provides specific error types for different failure scenarios:

- `ErrMissingToken`: No authorization token provided
- `ErrInvalidToken`: Token is malformed or invalid
- `ErrExpiredToken`: Token has expired
- `ErrInvalidSignature`: Token signature verification failed

Example:
```go
user, err := validator.ValidateToken(ctx, token)
if err != nil {
    switch err {
    case middleware.ErrExpiredToken:
        // Handle expired token - maybe refresh
    case middleware.ErrInvalidSignature:
        // Handle invalid signature - security issue
    default:
        // Handle other errors
    }
}
```
