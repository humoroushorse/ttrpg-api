# nats.mk
# Makefile for NATS message broker operations

# This file is imported - do not import other files
COMMON_REPOSITORY_ROOT=~/projects/ttrpg/ttrpg-api
CONTAINER_RUNNER := $(if $(CONTAINER_RUNNER),$(CONTAINER_RUNNER),"docker")
# the image for the testing server below; the dev server's is in deploy/nats/nats.compose.yml
CONTAINER_NATS_IMAGE=nats:2.15.3-alpine

################################################################################
# NATS Configuration
################################################################################
CONTAINER_NATS_HTTP_PORT=8222
NATS_CONTAINER_NAME=ttrpg-nats
# one NATS for every service, in the same compose project as postgres and keycloak
NATS_COMPOSE_FILE=${COMMON_REPOSITORY_ROOT}/deploy/nats/nats.compose.yml

.PHONY: nats-clean
nats-clean: # removes local volume mount data (JetStream's store)
	@echo "removing NATS local volume mount"
	rm -rf ~/projects/ttrpg/volume-mounts/nats

.PHONY: nats-up
nats-up: # start NATS via docker compose (project ttrpg); settings in deploy/nats/nats.conf
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ${NATS_COMPOSE_FILE} \
		up -d --wait nats

.PHONY: nats-down
nats-down: # stop NATS (leaves postgres and keycloak running)
	${CONTAINER_RUNNER} compose \
		-p ttrpg \
		-f ${NATS_COMPOSE_FILE} \
		stop nats

.PHONY: nats-logs
nats-logs: # view NATS container logs
	${CONTAINER_RUNNER} logs -f ${NATS_CONTAINER_NAME}

.PHONY: nats-stats
nats-stats: # view NATS server statistics
	@echo "NATS Server Statistics:"
	@curl -s http://localhost:${CONTAINER_NATS_HTTP_PORT}/varz | jq '.'

.PHONY: nats-connections
nats-connections: # view NATS connections
	@echo "NATS Connections:"
	@curl -s http://localhost:${CONTAINER_NATS_HTTP_PORT}/connz | jq '.'

.PHONY: nats-subscriptions
nats-subscriptions: # view NATS subscriptions
	@echo "NATS Subscriptions:"
	@curl -s http://localhost:${CONTAINER_NATS_HTTP_PORT}/subsz | jq '.'

.PHONY: nats-health
nats-health: # check NATS health
	@echo "NATS Health Check:"
	@curl -s http://localhost:${CONTAINER_NATS_HTTP_PORT}/healthz && echo "✓ NATS is healthy" || echo "✗ NATS is unhealthy"

################################################################################
# Kubernetes Deployment
################################################################################

.PHONY: nats-kube-up
nats-kube-up: # deploy NATS to Kubernetes
	kubectl apply -f ${COMMON_REPOSITORY_ROOT}/deploy/nats/k8s/

.PHONY: nats-kube-down
nats-kube-down: # remove NATS from Kubernetes
	kubectl delete -f ${COMMON_REPOSITORY_ROOT}/deploy/nats/k8s/

.PHONY: nats-kube-status
nats-kube-status: # check NATS status in Kubernetes
	kubectl get pods -l app=nats -n sprint-management
	kubectl get svc -l app=nats -n sprint-management

.PHONY: nats-kube-logs
nats-kube-logs: # view NATS logs in Kubernetes
	kubectl logs -l app=nats -n sprint-management -f

################################################################################
# Testing
################################################################################
CONTAINER_NATS_TEST_CLIENT_PORT=4223
CONTAINER_NATS_TEST_HTTP_PORT=8223
NATS_CONTAINER_NAME_TESTING=sprint-nats-testing

.PHONY: nats-up-testing
nats-up-testing: # start NATS container for testing
	# start container if not already running
	${CONTAINER_RUNNER} start ${NATS_CONTAINER_NAME_TESTING} || ${CONTAINER_RUNNER} run --name ${NATS_CONTAINER_NAME_TESTING} -d\
		-p ${CONTAINER_NATS_TEST_CLIENT_PORT}:4222\
		-p ${CONTAINER_NATS_TEST_HTTP_PORT}:8222\
		${CONTAINER_NATS_IMAGE}\
		--jetstream\
		--store_dir=/data\
		--http_port=8222

.PHONY: nats-down-testing
nats-down-testing: # stop and remove NATS testing container
	${CONTAINER_RUNNER} kill ${NATS_CONTAINER_NAME_TESTING}
	${CONTAINER_RUNNER} rm ${NATS_CONTAINER_NAME_TESTING}

################################################################################
# Utilities
################################################################################

.PHONY: nats-help
nats-help: # show NATS makefile help
	@echo "NATS Makefile Targets:"
	@echo "  nats-up              - Start NATS (compose project ttrpg)"
	@echo "  nats-down            - Stop NATS"
	@echo "  nats-logs            - View NATS logs"
	@echo "  nats-stats           - View NATS statistics"
	@echo "  nats-connections     - View NATS connections"
	@echo "  nats-subscriptions   - View NATS subscriptions"
	@echo "  nats-health          - Check NATS health"
	@echo "  nats-clean           - Remove NATS data volumes"
	@echo ""
	@echo "Kubernetes Targets:"
	@echo "  nats-kube-up         - Deploy NATS to Kubernetes"
	@echo "  nats-kube-down       - Remove NATS from Kubernetes"
	@echo "  nats-kube-status     - Check NATS status in Kubernetes"
	@echo "  nats-kube-logs       - View NATS logs in Kubernetes"
	@echo ""
	@echo "Testing Targets:"
	@echo "  nats-up-testing      - Start NATS for testing"
	@echo "  nats-down-testing    - Stop NATS testing container"
