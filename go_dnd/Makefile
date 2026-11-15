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

LOG_FORMAT=console
LOG_ENABLE_COLORS=true

.PHONY: all build clean run test lint fmt vet generate catalog-generate migrate-create schema-check sqlc-generate openapi-generate merge-openapi help \
	vtt-migrate-up vtt-migrate-down vtt-migrate-version vtt-migrate-create

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

generate: catalog-generate sqlc-generate openapi-generate ## Run all code generation

catalog-generate: ## OpenAPI paths and route methods for internal/entities
	go run ./cmd/gencatalog

sqlc-generate: ## Generate Go code from SQL queries
	sqlc generate

openapi-generate: merge-openapi ## Generate server code from OpenAPI spec
	oapi-codegen -config api/oapi-codegen.yaml api/openapi/dnd.yaml

merge-openapi: ## Merge dnd.yaml + auth.yaml into combined-with-auth.yaml
	poetry install --no-interaction -q
	poetry run python3 merge-openapi.py

# migrations/ mirrors py_dnd's Alembic schema and is never applied to the shared database
# (migrations/README.md). py_dnd's `make alembic-upgrade` builds the tables.
migrate-create: ## Create a new mirror migration (usage: make migrate-create NAME=name)
	migrate create -ext sql -dir migrations -seq $(NAME)

schema-check: ## Diff the dnd schema built by Alembic against the one built by migrations/
	../scripts/dnd_schema.sh check

# migrations_vtt/ is go_dnd's own: schema dnd_vtt, applied to the shared database, versioned in
# dnd_vtt.schema_migrations (migrations_vtt/README.md)
VTT_MIGRATIONS=migrations_vtt
VTT_DB_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
VTT_MIGRATE=$(GOCMD) run ./cmd/migrate -schema dnd_vtt -path $(VTT_MIGRATIONS) -db "$(VTT_DB_URL)"

vtt-migrate-up: ## Apply the dnd_vtt migrations to DB_NAME (default ttrpg-pg)
	$(VTT_MIGRATE) -action up

vtt-migrate-down: ## Roll back the last dnd_vtt migration on DB_NAME
	$(VTT_MIGRATE) -action down

vtt-migrate-version: ## Show the dnd_vtt migration version of DB_NAME
	$(VTT_MIGRATE) -action version

vtt-migrate-create: ## Create a new dnd_vtt migration (usage: make vtt-migrate-create NAME=name)
	@if [ -z "$(NAME)" ]; then echo "Error: NAME is required. Usage: make vtt-migrate-create NAME=name"; exit 1; fi
	migrate create -ext sql -dir $(VTT_MIGRATIONS) -seq $(NAME)

dev-up: keycloak-up nats-up ## Start dev dependencies (py_dnd's `make alembic-upgrade` builds the tables)

dev-down: nats-down keycloak-down ## Stop dev dependencies

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
