include ../makefiles/nats.mk
include ../makefiles/postgres.mk
include ../makefiles/keycloak.mk

GOCMD=go
BINARY_NAME=dnd-server
BINARY_PATH=bin/$(BINARY_NAME)

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=admin
DB_NAME=ttrpg-pg
DB_MASTER_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable&search_path=dnd
DB_MIGRATE_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

LOG_FORMAT=console
LOG_ENABLE_COLORS=true

.PHONY: all build clean run test lint fmt vet generate migrate-up migrate-down migrate-create sqlc-generate openapi-generate merge-openapi help

all: test build

build: ## Build the dnd service binary
	$(GOCMD) build -o $(BINARY_PATH) -v cmd/server/main.go

clean: ## Remove build artifacts
	$(GOCMD) clean
	rm -rf bin/

run: ## Run the dnd service locally
	LOG_FORMAT=$(LOG_FORMAT) LOG_ENABLE_COLORS=$(LOG_ENABLE_COLORS) SERVER_PORT=8004 $(GOCMD) run cmd/server/main.go

test: ## Run all tests
	$(GOCMD) test -v -race -coverprofile=coverage.out ./...

lint: ## Run linters
	golangci-lint run ./...

fmt: ## Format code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

generate: sqlc-generate openapi-generate ## Run all code generation

sqlc-generate: ## Generate Go code from SQL queries
	sqlc generate

openapi-generate: merge-openapi ## Generate server code from OpenAPI spec
	oapi-codegen -config api/oapi-codegen.yaml api/openapi/dnd.yaml

merge-openapi: ## Merge dnd.yaml + auth.yaml into combined-with-auth.yaml
	poetry install --no-interaction -q
	poetry run python3 merge-openapi.py

migrate-up: ## Run migrations up
	migrate -path migrations -database "$(DB_MIGRATE_URL)" up

migrate-down: ## Run migrations down
	migrate -path migrations -database "$(DB_MIGRATE_URL)" down

migrate-create: ## Create a new migration (usage: make migrate-create NAME=name)
	migrate create -ext sql -dir migrations -seq $(NAME)

dev-up: keycloak-up nats-up migrate-up ## Start dev dependencies and run migrations

dev-down: nats-down keycloak-down ## Stop dev dependencies

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
