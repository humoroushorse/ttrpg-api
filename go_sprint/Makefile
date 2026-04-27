# Makefile for go_sprint service

# Import shared makefiles from root
include ../makefiles/nats.mk
include ../makefiles/postgres.mk
include ../makefiles/keycloak.mk

# Go configuration
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=sprint-server
BINARY_PATH=bin/$(BINARY_NAME)

# Logging configuration
LOG_FORMAT=console        # Options: console, json
LOG_ENABLE_COLORS=true    # Options: true, false (only applies when LOG_FORMAT=console)

# Docker configuration
DOCKER_IMAGE=go-sprint
DOCKER_TAG=latest

# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=admin
DB_NAME=sprint_management
DB_MASTER_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
DB_REPLICA_URL=$(DB_MASTER_URL)

################################################################################
# Build Targets
################################################################################

.PHONY: all
all: test build

.PHONY: build
build: ## Build the sprint service binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_PATH) -v cmd/server/main.go

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_PATH)
	rm -rf bin/

.PHONY: run
run: ## Run the sprint service locally (also starts go_auth if needed)
	@echo "Checking if services are already running..."
	@if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "✅ Keycloak already running on port 8080"; \
	else \
		echo "⚠️  Keycloak not running on port 8080 - please start it first"; \
	fi
	@if lsof -Pi :8081 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "✅ Auth service already running on port 8081"; \
	else \
		echo "🔐 Starting auth service on port 8081..."; \
		cd ../go_auth && SERVER_PORT=8081 KEYCLOAK_DEFAULT_REALM=sprint-management KEYCLOAK_CLIENT_ID=sprint-management-ui KEYCLOAK_REALM_CLIENTS=ttrpg:auth-service,sprint-management:sprint-management-ui go run cmd/server/main.go & \
		sleep 2; \
	fi
	@if lsof -Pi :8003 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "⚠️  Sprint service already running on port 8003"; \
	else \
		echo "📋 Starting $(BINARY_NAME) on port 8003 with LOG_FORMAT=$(LOG_FORMAT) LOG_ENABLE_COLORS=$(LOG_ENABLE_COLORS)..."; \
		LOG_FORMAT=$(LOG_FORMAT) LOG_ENABLE_COLORS=$(LOG_ENABLE_COLORS) SERVER_PORT=8003 KEYCLOAK_REALM=sprint-management KEYCLOAK_CLIENT_ID=sprint-management-ui $(GOCMD) run cmd/server/main.go; \
	fi

.PHONY: stop
stop: ## Stop the sprint service and auth service
	@echo "Stopping services..."
	@if lsof -Pi :8003 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "🛑 Stopping sprint service on port 8003..."; \
		kill $$(lsof -ti:8003) 2>/dev/null || true; \
	else \
		echo "✅ Sprint service not running on port 8003"; \
	fi
	@if lsof -Pi :8081 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "🛑 Stopping auth service on port 8081..."; \
		kill $$(lsof -ti:8081) 2>/dev/null || true; \
	else \
		echo "✅ Auth service not running on port 8081"; \
	fi
	@echo "✅ Services stopped"

################################################################################
# Testing Targets
################################################################################

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	$(GOTEST) -v -short ./...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -run Integration ./...

.PHONY: test-coverage
test-coverage: test ## Generate test coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

################################################################################
# Code Quality Targets
################################################################################

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

################################################################################
# Dependency Management
################################################################################

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

.PHONY: deps-vendor
deps-vendor: ## Vendor dependencies
	@echo "Vendoring dependencies..."
	$(GOMOD) vendor

################################################################################
# Database Targets
################################################################################

.PHONY: migrate-up
migrate-up: ## Run database migrations up
	@echo "Running migrations up..."
	migrate -path migrations -database "$(DB_MASTER_URL)" up

.PHONY: migrate-down
migrate-down: ## Run database migrations down
	@echo "Running migrations down..."
	migrate -path migrations -database "$(DB_MASTER_URL)" down

.PHONY: migrate-create
migrate-create: ## Create a new migration (usage: make migrate-create NAME=migration_name)
	@echo "Creating migration: $(NAME)"
	migrate create -ext sql -dir migrations -seq $(NAME)

################################################################################
# SQLC Code Generation
################################################################################

.PHONY: sqlc-generate
sqlc-generate: ## Generate Go code from SQL queries using sqlc
	@echo "Generating code with sqlc..."
	sqlc generate

.PHONY: sqlc-verify
sqlc-verify: ## Verify SQL queries with sqlc
	@echo "Verifying SQL queries..."
	sqlc verify

################################################################################
# OpenAPI Code Generation
################################################################################

.PHONY: openapi-generate
openapi-generate: ## Generate server code from OpenAPI specs
	@echo "Generating server code from OpenAPI specs..."
	./scripts/generate-api.sh

.PHONY: openapi-install-tools
openapi-install-tools: ## Install OpenAPI code generation tools
	@echo "Installing OpenAPI tools..."
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	@echo "✓ oapi-codegen installed"
	@echo ""
	@echo "Optional tools (for documentation and TypeScript):"
	@echo "  npm install -g openapi-typescript"
	@echo "  npm install -g redoc-cli"
	@echo "  npm install -g swagger-markdown"

################################################################################
# Docker Targets
################################################################################

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-push
docker-push: ## Push Docker image to registry
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-compose-up
docker-compose-up: ## Start all services with docker-compose
	@echo "Starting services with docker-compose..."
	docker-compose up -d

.PHONY: docker-compose-down
docker-compose-down: ## Stop all services with docker-compose
	@echo "Stopping services with docker-compose..."
	docker-compose down

################################################################################
# Development Environment
################################################################################

.PHONY: dev-up
dev-up: keycloak-up nats-up migrate-up ## Start all development dependencies and run migrations
	@echo "Development environment started"
	@echo "PostgreSQL: localhost:5432"
	@echo "Keycloak:   http://localhost:8080"
	@echo "NATS: localhost:4222"
	@echo "NATS Monitoring: http://localhost:8222"

.PHONY: dev-down
dev-down: nats-down keycloak-down ## Stop all development dependencies
	@echo "Development environment stopped"

.PHONY: dev-logs
dev-logs: ## View logs from all services
	docker-compose logs -f

################################################################################
# Code Generation
################################################################################

.PHONY: generate
generate: sqlc-generate openapi-generate ## Run all code generation
	@echo "All code generation complete"
	$(GOCMD) generate ./...

################################################################################
# Help
################################################################################

.PHONY: help
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help

################################################################################
# Backup and Restore Targets
################################################################################

.PHONY: backup
backup: ## Create database backup
	@echo "Creating database backup..."
	./scripts/backup.sh

.PHONY: backup-system
backup-system: ## Create comprehensive system backup
	@echo "Creating comprehensive system backup..."
	./scripts/backup-system.sh

.PHONY: restore
restore: ## Restore database from backup (usage: make restore BACKUP_FILE=path/to/backup.sql)
	@echo "Restoring database from backup..."
	@if [ -z "$(BACKUP_FILE)" ]; then \
		echo "Error: BACKUP_FILE not specified"; \
		echo "Usage: make restore BACKUP_FILE=path/to/backup.sql"; \
		exit 1; \
	fi
	./scripts/restore.sh $(BACKUP_FILE)

.PHONY: verify-system
verify-system: ## Verify system health and integrity
	@echo "Verifying system..."
	./scripts/verify-system.sh

.PHONY: backup-list
backup-list: ## List available backups
	@echo "Available backups:"
	@ls -lh backups/sprint_management_*.sql 2>/dev/null || echo "No backups found"

.PHONY: reset
reset: ## Wipe database and start fresh (WARNING: Deletes all data!)
	@echo "⚠️  WARNING: This will delete ALL data!"
	@read -p "Are you sure? Type 'yes' to continue: " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		echo "Dropping and recreating database..."; \
		docker exec ttrpg-pg psql -U postgres -c "DROP DATABASE IF EXISTS sprint_management;"; \
		docker exec ttrpg-pg psql -U postgres -c "CREATE DATABASE sprint_management;"; \
		echo "Running migrations..."; \
		make migrate-up; \
		echo "✅ Database reset complete!"; \
	else \
		echo "Reset cancelled."; \
	fi
