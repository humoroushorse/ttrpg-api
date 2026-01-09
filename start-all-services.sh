#!/bin/bash

# Start all TTRPG services
# This script starts go_auth and go_sprint services together

set -e

echo "🚀 Starting TTRPG Services..."
echo ""

# Check if ttrpg-pg database is running
if ! docker ps --format '{{.Names}}' | grep -q "ttrpg-pg"; then
    echo "❌ Error: ttrpg-pg database is not running"
    echo "Please start the database first with: docker-compose up -d"
    exit 1
fi

echo "✅ Database is running"
echo ""

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🛑 Stopping services..."
    kill $AUTH_PID $SPRINT_PID 2>/dev/null || true
    wait $AUTH_PID $SPRINT_PID 2>/dev/null || true
    echo "✅ Services stopped"
}

trap cleanup EXIT INT TERM

# Start go_auth service
echo "🔐 Starting Authentication Service (port 8081)..."
cd go_auth
go run cmd/server/main.go &
AUTH_PID=$!
cd ..

# Wait a moment for auth service to start
sleep 2

# Start go_sprint service
echo "📋 Starting Sprint Management Service (port 8082)..."
cd go_sprint
SERVER_PORT=8082 go run cmd/server/main.go &
SPRINT_PID=$!
cd ..

# Wait a moment for sprint service to start
sleep 2

echo ""
echo "✅ All services started successfully!"
echo ""
echo "📚 Available Services:"
echo "  • Authentication Service:"
echo "    - API: http://localhost:8081"
echo "    - Swagger UI: http://localhost:8081/swagger/"
echo "    - OpenAPI Spec: http://localhost:8081/api/openapi.yaml"
echo ""
echo "  • Sprint Management Service (with auth proxy):"
echo "    - API: http://localhost:8082"
echo "    - Swagger UI: http://localhost:8082/swagger/"
echo "    - OpenAPI Spec: http://localhost:8082/api/openapi.yaml"
echo "    - Auth endpoints: http://localhost:8082/api/v1/auth/*"
echo ""
echo "🔑 Authentication Flow:"
echo "  1. Register: POST http://localhost:8082/api/v1/auth/register"
echo "  2. Login: POST http://localhost:8082/api/v1/auth/login"
echo "  3. Use access_token in Authorization header for protected endpoints"
echo ""
echo "Press Ctrl+C to stop all services"
echo ""

# Wait for both processes
wait $AUTH_PID $SPRINT_PID
