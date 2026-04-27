# keycloak.mk
# Makefile for Keycloak identity provider operations

# This file is imported - do not import other files
COMMON_REPOSITORY_ROOT=~/projects/ttrpg/ttrpg-api
CONTAINER_RUNNER := $(if $(CONTAINER_RUNNER),$(CONTAINER_RUNNER),"docker")
CONTAINER_KEYCLOAK_IMAGE=keycloak/keycloak:26.5

################################################################################
# Keycloak Configuration
################################################################################
CONTAINER_KEYCLOAK_PORT=8080
CONTAINER_KEYCLOAK_MGMT_PORT=9000
KEYCLOAK_CONTAINER_NAME=ttrpg-keycloak
KEYCLOAK_COMPOSE_FILE=${COMMON_REPOSITORY_ROOT}/deploy/keycloak/keycloak.compose.yml
POSTGRES_COMPOSE_FILE=${COMMON_REPOSITORY_ROOT}/deploy/postgres/postgres.compose.yml

.PHONY: keycloak-up
keycloak-up: # start Keycloak (and postgres) via docker compose
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ${POSTGRES_COMPOSE_FILE} \
		-f ${KEYCLOAK_COMPOSE_FILE} \
		up -d
	@if curl -sf http://localhost:${CONTAINER_KEYCLOAK_MGMT_PORT}/health/ready > /dev/null 2>&1; then \
		echo "✅ Keycloak already ready at http://localhost:${CONTAINER_KEYCLOAK_PORT}"; \
	else \
		echo "Waiting for Keycloak to be ready..."; \
		READY=0; for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
			curl -sf http://localhost:${CONTAINER_KEYCLOAK_MGMT_PORT}/health/ready > /dev/null 2>&1 && READY=1 && break; \
			echo "  waiting... ($$i/15)"; \
			sleep 4; \
		done; \
		if [ "$$READY" = "1" ]; then \
			echo "✅ Keycloak ready at http://localhost:${CONTAINER_KEYCLOAK_PORT}"; \
		else \
			echo "❌ Keycloak failed to start. Check logs: docker logs ${KEYCLOAK_CONTAINER_NAME}"; \
			exit 1; \
		fi; \
	fi
	@$(MAKE) keycloak-configure

.PHONY: keycloak-configure
keycloak-configure: # create realms and clients in Keycloak for local dev
	@echo "Configuring Keycloak realms and clients..."
	@KC_TOKEN=$$(curl -sf -X POST "http://localhost:${CONTAINER_KEYCLOAK_PORT}/realms/master/protocol/openid-connect/token" \
		-d "client_id=admin-cli" \
		-d "username=admin" \
		-d "password=admin" \
		-d "grant_type=password" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])"); \
	if [ -z "$$KC_TOKEN" ]; then \
		echo "❌ Failed to get admin token"; \
		exit 1; \
	fi; \
	for REALM in ttrpg sprint-management; do \
		REALM_STATUS=$$(curl -sf -o /dev/null -w "%{http_code}" "http://localhost:${CONTAINER_KEYCLOAK_PORT}/realms/$$REALM" 2>/dev/null || echo "000"); \
		if [ "$$REALM_STATUS" = "200" ]; then \
			echo "  ⚠️  Realm '$$REALM' already exists"; \
		else \
			echo "  Creating realm '$$REALM'..."; \
			curl -sf -X POST "http://localhost:${CONTAINER_KEYCLOAK_PORT}/admin/realms" \
				-H "Authorization: Bearer $$KC_TOKEN" \
				-H "Content-Type: application/json" \
				-d "{\"realm\":\"$$REALM\",\"enabled\":true,\"registrationAllowed\":true,\"loginWithEmailAllowed\":true,\"duplicateEmailsAllowed\":false,\"resetPasswordAllowed\":true,\"editUsernameAllowed\":false,\"bruteForceProtected\":true}"; \
			echo "  ✅ Realm '$$REALM' created"; \
		fi; \
		echo "  Disabling 'verify profile' required action on '$$REALM'..."; \
		curl -sf -X PUT "http://localhost:${CONTAINER_KEYCLOAK_PORT}/admin/realms/$$REALM" \
			-H "Authorization: Bearer $$KC_TOKEN" \
			-H "Content-Type: application/json" \
			-d "{\"requiredActions\":[]}"; \
	done; \
	for REALM_CLIENT in ttrpg:auth-service sprint-management:sprint-management-ui; do \
		REALM=$${REALM_CLIENT%%:*}; \
		CLIENT=$${REALM_CLIENT##*:}; \
		CLIENT_EXISTS=$$(curl -sf "http://localhost:${CONTAINER_KEYCLOAK_PORT}/admin/realms/$$REALM/clients?clientId=$$CLIENT" \
			-H "Authorization: Bearer $$KC_TOKEN" | python3 -c "import sys,json; data=json.load(sys.stdin); print('yes' if len(data)>0 else 'no')" 2>/dev/null || echo "no"); \
		if [ "$$CLIENT_EXISTS" = "yes" ]; then \
			echo "  ⚠️  Client '$$CLIENT' already exists in realm '$$REALM'"; \
		else \
			echo "  Creating client '$$CLIENT' in realm '$$REALM'..."; \
			curl -sf -X POST "http://localhost:${CONTAINER_KEYCLOAK_PORT}/admin/realms/$$REALM/clients" \
				-H "Authorization: Bearer $$KC_TOKEN" \
				-H "Content-Type: application/json" \
				-d "{\"clientId\":\"$$CLIENT\",\"enabled\":true,\"publicClient\":true,\"directAccessGrantsEnabled\":true,\"standardFlowEnabled\":true,\"implicitFlowEnabled\":false,\"redirectUris\":[\"*\"],\"webOrigins\":[\"*\"],\"protocol\":\"openid-connect\"}"; \
			echo "  ✅ Client '$$CLIENT' created in realm '$$REALM'"; \
		fi; \
	done; \
	echo "✅ Keycloak configuration complete"

.PHONY: keycloak-down
keycloak-down: # stop Keycloak (leaves postgres running)
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ${POSTGRES_COMPOSE_FILE} \
		-f ${KEYCLOAK_COMPOSE_FILE} \
		stop keycloak

.PHONY: keycloak-logs
keycloak-logs: # view Keycloak container logs
	${CONTAINER_RUNNER} logs -f ${KEYCLOAK_CONTAINER_NAME}

.PHONY: keycloak-health
keycloak-health: # check Keycloak health
	@echo "Keycloak Health Check:"
	@curl -sf http://localhost:${CONTAINER_KEYCLOAK_MGMT_PORT}/health/ready && echo " ✓ Keycloak is healthy" || echo " ✗ Keycloak is unhealthy"

.PHONY: keycloak-help
keycloak-help: # show Keycloak makefile help
	@echo "Keycloak Makefile Targets:"
	@echo "  keycloak-up          - Start Keycloak (with postgres) via compose"
	@echo "  keycloak-down        - Stop Keycloak container"
	@echo "  keycloak-configure   - Create realms and clients for local dev"
	@echo "  keycloak-logs        - View Keycloak logs"
	@echo "  keycloak-health      - Check Keycloak health"
