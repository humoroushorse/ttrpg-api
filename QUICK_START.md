# Quick Start Guide

## Authentication is Working! 🎉

The authentication service is now fully integrated with cookie support.

## Start Services

```bash
./start-all-services.sh
```

## Test Authentication Flow

### 1. Register a User

```bash
curl -X POST http://localhost:8082/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "Test123!",
    "first_name": "Test",
    "last_name": "User"
  }'
```

### 2. Login (Sets HTTP-only Cookies)

```bash
curl -v -X POST http://localhost:8082/api/v1/auth/login \
  -c cookies.txt \
  -d "username=testuser&password=Test123!"
```

### 3. Get User Info (Using Cookies)

```bash
curl -X GET http://localhost:8082/api/v1/auth/user \
  -b cookies.txt
```

### 4. Or Use Authorization Header

```bash
# Extract token from login response
TOKEN="your_access_token_here"

curl -X GET http://localhost:8082/api/v1/auth/user \
  -H "Authorization: Bearer $TOKEN"
```

## Using Swagger UI

1. Open http://localhost:8082/swagger/
2. Try `/api/v1/auth/login` endpoint
3. Cookies are automatically set in your browser
4. Try `/api/v1/auth/user` - works automatically with cookies!

## Architecture

```
Client (Browser/API)
       ↓
go_sprint (8082) - Reverse Proxy
       ↓
go_auth (8081) - Auth Service
       ↓
Keycloak (8080) - Identity Provider
```

## Features

✅ User registration
✅ User login with username/password
✅ HTTP-only cookies (secure, can't be accessed by JavaScript)
✅ Token refresh
✅ User logout
✅ Get user information
✅ Support for both cookies AND Authorization header
✅ Reverse proxy integration
✅ Single Swagger UI for all endpoints

## Next Steps

1. Implement JWT validation middleware in go_sprint
2. Protect sprint/workitem endpoints with authentication
3. Add user context to handlers
4. Implement role-based access control (RBAC)
