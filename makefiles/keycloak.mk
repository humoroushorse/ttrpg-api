# keycloak.mk

# this file is imported
#    do not import other files
CONTAINER_RUNNER := $(if $(CONTAINER_RUNNER),$(CONTAINER_RUNNER),"docker")
CONTAINER_KEYCLOAK_IMAGE=quay.io/keycloak/keycloak:22.0.5

################################################################################
# App
################################################################################
CONTAINER_KEYCLOAK_PORT=8080
KEYCLOAK_CONTAINER_NAME=ttrpg-keycloak
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=admin

.PHONY: keycloak-up
keycloak-up:
	# start container if not already running
	${CONTAINER_RUNNER} start ${KEYCLOAK_CONTAINER_NAME} || ${CONTAINER_RUNNER} run --name ${KEYCLOAK_CONTAINER_NAME} -d\
		-e KEYCLOAK_ADMIN=${KEYCLOAK_ADMIN_USER}\
		-e KEYCLOAK_ADMIN_PASSWORD=${KEYCLOAK_ADMIN_PASSWORD}\
		-e JAVA_OPTS="-Xms256m -Xmx512m -XX:+UseSerialGC -Djava.awt.headless=true"\
		-p ${CONTAINER_KEYCLOAK_PORT}:${CONTAINER_KEYCLOAK_PORT}\
		${CONTAINER_KEYCLOAK_IMAGE} start-dev

.PHONY: keycloak-down
keycloak-down:
	${CONTAINER_RUNNER} kill ${KEYCLOAK_CONTAINER_NAME}
	${CONTAINER_RUNNER} rm ${KEYCLOAK_CONTAINER_NAME}

.PHONY: keycloak-logs
keycloak-logs:
	${CONTAINER_RUNNER} logs -f ${KEYCLOAK_CONTAINER_NAME}