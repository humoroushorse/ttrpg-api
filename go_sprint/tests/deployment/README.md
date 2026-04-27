# Deployment Tests

This directory contains tests for validating Docker and Kubernetes deployment configurations for the Sprint Management Service.

## Overview

The deployment tests verify:
- Docker image builds successfully
- Docker security configurations (non-root user, minimal attack surface)
- Docker health checks function correctly
- Kubernetes manifests are valid and properly configured
- Deployment scripts are executable and functional

## Test Files

### docker_test.go
Tests Docker-related functionality:
- `TestDockerBuild`: Verifies Docker image builds successfully
- `TestDockerImageSecurity`: Validates security configurations (non-root user, binary permissions)
- `TestDockerHealthCheck`: Tests container startup and health check endpoints
- `TestDockerComposeValidation`: Validates docker-compose.yml configurations
- `TestDockerImageSize`: Checks image size is reasonable for Alpine-based build
- `TestDockerfileExists`: Verifies Dockerfile exists and contains required directives

### kubernetes_test.go
Tests Kubernetes deployment configurations:
- `TestKubernetesManifestsExist`: Verifies all required K8s manifests exist
- `TestSprintServiceDeployment`: Validates deployment manifest structure and configuration
- `TestKubernetesManifestValidation`: Uses kubectl to validate all manifests
- `TestKubernetesDeploymentScripts`: Validates deployment scripts exist and are executable
- `TestNamespaceConfiguration`: Validates namespace manifest
- `TestServiceConfiguration`: Validates service manifest
- `TestHPAConfiguration`: Validates HorizontalPodAutoscaler configuration
- `TestConfigMapExists`: Validates ConfigMap manifest

## Requirements

### Docker Tests
- Docker installed and running
- Docker Compose installed
- Sufficient disk space for building images

### Kubernetes Tests
- kubectl installed (optional, tests will skip if not available)
- Access to Kubernetes cluster (optional, for full validation)

## Running Tests

### Run All Deployment Tests
```bash
cd go_sprint
go test ./tests/deployment/... -v
```

### Run Only Docker Tests
```bash
go test ./tests/deployment/... -v -run Docker
```

### Run Only Kubernetes Tests
```bash
go test ./tests/deployment/... -v -run Kubernetes
```

### Skip Long-Running Tests
```bash
go test ./tests/deployment/... -v -short
```

### Run Specific Test
```bash
go test ./tests/deployment/... -v -run TestDockerBuild
```

## Test Requirements Validation

These tests validate the following requirements:

### Requirement 23.4: Docker Security and Health
- Non-root user execution in containers
- Health checks configured in Docker
- Multi-stage builds for optimized image size
- Proper file permissions and ownership

### Requirement 23.5: Kubernetes Deployment
- Valid Kubernetes manifests for all components
- Health checks (liveness and readiness probes)
- Resource limits and requests configured
- Security contexts properly set
- HorizontalPodAutoscaler configured
- Service and ingress configurations

## Test Output

Successful test output will show:
```
=== RUN   TestDockerBuild
    docker_test.go:25: Docker build successful
--- PASS: TestDockerBuild (45.23s)

=== RUN   TestSprintServiceDeployment
=== RUN   TestSprintServiceDeployment/Metadata
=== RUN   TestSprintServiceDeployment/Replicas
    kubernetes_test.go:123: Deployment configured with 3 replicas
=== RUN   TestSprintServiceDeployment/Container
=== RUN   TestSprintServiceDeployment/HealthProbes
=== RUN   TestSprintServiceDeployment/Resources
=== RUN   TestSprintServiceDeployment/SecurityContext
--- PASS: TestSprintServiceDeployment (0.05s)
```

## Troubleshooting

### Docker Build Fails
- Ensure Docker daemon is running: `docker ps`
- Check disk space: `df -h`
- Verify go.mod and dependencies are correct

### Kubernetes Validation Fails
- Install kubectl: `brew install kubectl` (macOS) or equivalent
- Verify kubectl is in PATH: `which kubectl`
- Tests will skip if kubectl is not available

### Container Won't Start
- Check logs: `docker logs <container-name>`
- Verify environment variables are set correctly
- Ensure dependencies (PostgreSQL, NATS) are available

## CI/CD Integration

These tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Deployment Tests
  run: |
    cd go_sprint
    go test ./tests/deployment/... -v -short
```

For full integration tests including container startup, remove the `-short` flag and ensure Docker is available in the CI environment.

## Notes

- Docker tests may take several minutes due to image building
- Some tests require Docker daemon to be running
- Kubernetes validation tests are optional and will skip if kubectl is not available
- Tests use a separate image tag (`sprint-service-test:latest`) to avoid conflicts
- Container tests use port 8083 to avoid conflicts with running services
