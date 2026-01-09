#!/bin/bash

# Sprint Management Service - Local Startup Script
# This script starts the Sprint Management API using the shared ttrpg-pg database

set -e

echo "🚀 Starting Sprint Management Service..."
echo ""

# Check if ttrpg-pg container is running
if ! docker ps --filter "name=ttrpg-pg" --format "{{.Names}}" | grep -q "ttrpg-pg"; then
    echo "❌ Error: ttrpg-pg database container is not running"
    echo "   Start it with: docker-compose -f deploy/postgres/postgres.compose.yml up -d"
    echo "   (from the repository root)"
    exit 1
fi

echo "✅ Database container (ttrpg-pg) is running"

# Check if sprint_management schema exists
if ! docker exec ttrpg-pg psql -U postgres -d ttrpg-pg -tAc "SELECT 1 FROM information_schema.schemata WHERE schema_name='sprint_management'" | grep -q 1; then
    echo "📦 Creating sprint_management schema..."
    for migration in migrations/*.up.sql; do
        echo "   Running $(basename $migration)..."
        docker exec -i ttrpg-pg psql -U postgres -d ttrpg-pg < "$migration" > /dev/null
    done
    echo "✅ Migrations completed"
else
    echo "✅ sprint_management schema exists"
fi

# Start NATS if not running
if ! docker ps --filter "name=sprint-nats" --format "{{.Names}}" | grep -q "sprint-nats"; then
    echo "📡 Starting NATS..."
    docker-compose up -d nats
    echo "⏳ Waiting for NATS to be ready..."
    sleep 3
fi

echo "✅ NATS is running"
echo ""
echo "🎯 Starting Sprint Management API on port 8082..."
echo ""
echo "   API:        http://localhost:8082/"
echo "   Swagger UI: http://localhost:8082/swagger/"
echo "   Health:     http://localhost:8082/health/ready"
echo "   Metrics:    http://localhost:8082/metrics"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""

# Start the server
SERVER_PORT=8082 go run cmd/server/main.go
