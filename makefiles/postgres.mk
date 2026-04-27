# postgres.mk

# this file is imported
#    do not import other files
COMMON_REPOSITORY_ROOT=~/projects/ttrpg/ttrpg-api
CONTAINER_RUNNER := $(if $(CONTAINER_RUNNER),$(CONTAINER_RUNNER),"docker")
CONTAINER_POSTGRES_IMAGE=postgres:18-alpine

################################################################################
# App
################################################################################
CONTAINER_POSTGRES_PORT=5432
POSTGRES_CONTAINER_NAME=ttrpg-pg
POSTGRES_DATABASE_NAME=ttrpg-pg

.PHONY: postgres-clean
postgres-clean: # removes local volume mount data
	@echo "removing postgres local volume mount defined in ~/projects/ttrpg/ttrpg-api/deploy/postgres/postgres.compose.yml"
	rm -rf ~/projects/ttrpg/volume-mounts/postgres

.PHONY: postgres-up
postgres-up:
	# start container if not already running
	${CONTAINER_RUNNER} start ${POSTGRES_CONTAINER_NAME} || ${CONTAINER_RUNNER} run --name ${POSTGRES_CONTAINER_NAME} -d\
		-e POSTGRES_USER=postgres\
		-e POSTGRES_PASSWORD=admin\
		-e POSTGRES_DB=${POSTGRES_DATABASE_NAME}\
		-p ${CONTAINER_POSTGRES_PORT}:5432 ${CONTAINER_POSTGRES_IMAGE}
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} pg_isready -U postgres -h localhost -p ${CONTAINER_POSTGRES_PORT} > /dev/null 2>&1 && break; \
		echo "  waiting... ($$i/10)"; \
		sleep 2; \
	done
	@echo "Creating application databases (if they don't exist)..."
	@${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'auth'" | grep -q 1 || \
		${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -c "CREATE DATABASE auth;"
	@${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'sprint_management'" | grep -q 1 || \
		${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -c "CREATE DATABASE sprint_management;"
	@${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'dnd'" | grep -q 1 || \
		${CONTAINER_RUNNER} exec ${POSTGRES_CONTAINER_NAME} psql -U postgres -c "CREATE DATABASE dnd;"
	@echo "✅ PostgreSQL ready with databases: auth, sprint_management, dnd"

.PHONY: postgres-compose-up
postgres-compose-up:
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ../deploy/postgres/postgres.compose.yml \
		up -d

.PHONY: postgres-compose-down
postgres-compose-down:
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ../deploy/postgres/postgres.compose.yml \
		down

.PHONY: postgres-down
postgres-down:
	${CONTAINER_RUNNER} kill ${POSTGRES_CONTAINER_NAME}
	${CONTAINER_RUNNER} rm ${POSTGRES_CONTAINER_NAME}

.PHONY: postgres-kube-up
postgres-kube-up:
	echo "hello, workl"

################################################################################
# Testing
################################################################################
CONTAINER_POSTGRES_TEST_PORT=5433
POSTGRES_CONTAINER_NAME_TESTING=ttrpg-pg-testing
CONTAINER_POSTGRES_TEST_CONN=postgresql://postgres:admin@localhost:${CONTAINER_POSTGRES_TEST_PORT}/${POSTGRES_DATABASE_NAME}

.PHONY: postgres-up-testing
postgres-up-testing:
	# start container if not already running
	${CONTAINER_RUNNER} start ${POSTGRES_CONTAINER_NAME_TESTING} || ${CONTAINER_RUNNER} run --name ${POSTGRES_CONTAINER_NAME_TESTING} -d\
		-e POSTGRES_USER=postgres\
		-e POSTGRES_PASSWORD=admin\
		-e POSTGRES_DB=${POSTGRES_DATABASE_NAME}\
		-p ${CONTAINER_POSTGRES_TEST_PORT}:5432 ${CONTAINER_POSTGRES_IMAGE}

.PHONY: postgres-down-testing
postgres-down-testing:
	${CONTAINER_RUNNER} kill ${POSTGRES_CONTAINER_NAME_TESTING}
	${CONTAINER_RUNNER} rm ${POSTGRES_CONTAINER_NAME_TESTING}
