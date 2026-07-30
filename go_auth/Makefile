# Import shared makefiles from root
include ../makefiles/postgres.mk
include ../makefiles/keycloak.mk

.PHONY: help run run-sprint build test clean dev-up dev-down

help:
	@echo "Available targets:"
	@echo "  dev-up - Start all dev dependencies (postgres, keycloak)"
	@echo "  dev-down - Stop all dev dependencies"
	@echo "  run    - Run the auth service locally"
	@echo "  build  - Build the auth service binary"
	@echo "  test   - Run tests"
	@echo "  clean  - Clean build artifacts"

################################################################################
# Development Environment
################################################################################

.PHONY: dev-up
dev-up: keycloak-up ## Start all development dependencies (postgres + keycloak via compose)
	@echo "Development environment started"
	@echo "PostgreSQL: localhost:5432"
	@echo "Keycloak:   http://localhost:8080"

.PHONY: dev-down
dev-down: keycloak-down ## Stop all development dependencies
	@echo "Development environment stopped"

run: ## Run auth service locally (ttrpg realm, port 8081)
	@echo "Checking if services are already running..."
	@if lsof -Pi :8081 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "⚠️  Auth service already running on port 8081"; \
	else \
		echo "🔐 Starting auth service on port 8081 (ttrpg realm)..."; \
		SERVER_PORT=8081 KEYCLOAK_DEFAULT_REALM=ttrpg go run cmd/server/main.go; \
	fi

run-sprint: ## Run auth service for sprint-management realm on port 8082
	@echo "Checking if services are already running..."
	@if lsof -Pi :8082 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "⚠️  Auth service (sprint-management) already running on port 8082"; \
	else \
		echo "🔐 Starting auth service on port 8082 (sprint-management realm)..."; \
		SERVER_PORT=8082 KEYCLOAK_DEFAULT_REALM=sprint-management KEYCLOAK_CLIENT_ID=sprint-management-ui go run cmd/server/main.go; \
	fi

stop:
	@echo "Stopping auth service..."
	@if lsof -Pi :8081 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "🛑 Stopping auth service on port 8081..."; \
		kill $$(lsof -ti:8081) 2>/dev/null || true; \
		echo "✅ Auth service stopped"; \
	else \
		echo "✅ Auth service not running on port 8081"; \
	fi

build:
	@echo "Building auth service..."
	go build -o bin/auth-server cmd/server/main.go

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/
