# nats.mk
# Makefile for NATS message broker operations

# This file is imported - do not import other files
COMMON_REPOSITORY_ROOT=~/projects/ttrpg/ttrpg-api
CONTAINER_RUNNER := $(if $(CONTAINER_RUNNER),$(CONTAINER_RUNNER),"docker")
CONTAINER_NATS_IMAGE=nats:2.10-alpine

################################################################################
# NATS Configuration
################################################################################
CONTAINER_NATS_CLIENT_PORT=4222
CONTAINER_NATS_HTTP_PORT=8222
CONTAINER_NATS_CLUSTER_PORT=6222
NATS_CONTAINER_NAME=sprint-nats

.PHONY: nats-clean
nats-clean: # removes local volume mount data
	@echo "removing NATS local volume mount"
	rm -rf ~/projects/ttrpg/volume-mounts/nats

.PHONY: nats-up
nats-up: # start NATS container
	# start container if not already running
	${CONTAINER_RUNNER} start ${NATS_CONTAINER_NAME} || ${CONTAINER_RUNNER} run --name ${NATS_CONTAINER_NAME} -d\
		-p ${CONTAINER_NATS_CLIENT_PORT}:4222\
		-p ${CONTAINER_NATS_HTTP_PORT}:8222\
		-p ${CONTAINER_NATS_CLUSTER_PORT}:6222\
		${CONTAINER_NATS_IMAGE}\
		--jetstream\
		--store_dir=/data\
		--http_port=8222\
		--max_payload=1048576\
		--max_connections=1000

.PHONY: nats-compose-up
nats-compose-up: # start NATS using docker-compose
	${CONTAINER_RUNNER} compose \
		-f ${COMMON_REPOSITORY_ROOT}/deploy/nats/docker-compose.yml \
		up -d

.PHONY: nats-compose-down
nats-compose-down: # stop NATS using docker-compose
	${CONTAINER_RUNNER} compose \
		-f ${COMMON_REPOSITORY_ROOT}/deploy/nats/docker-compose.yml \
		down

.PHONY: nats-down
nats-down: # stop and remove NATS container
	${CONTAINER_RUNNER} kill ${NATS_CONTAINER_NAME}
	${CONTAINER_RUNNER} rm ${NATS_CONTAINER_NAME}

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
	@echo "  nats-up              - Start NATS container"
	@echo "  nats-down            - Stop NATS container"
	@echo "  nats-compose-up      - Start NATS using docker-compose"
	@echo "  nats-compose-down    - Stop NATS using docker-compose"
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
