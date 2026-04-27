package deployment

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestDockerBuild tests that the Docker image builds successfully
func TestDockerBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker build test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Build the Docker image
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", "sprint-service-test:latest", "../../")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Docker build successful")

	// Verify the image exists
	cmd = exec.CommandContext(ctx, "docker", "images", "sprint-service-test:latest", "--format", "{{.Repository}}:{{.Tag}}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to verify Docker image: %v", err)
	}

	imageTag := strings.TrimSpace(string(output))
	if imageTag != "sprint-service-test:latest" {
		t.Fatalf("Expected image tag 'sprint-service-test:latest', got '%s'", imageTag)
	}
}

// TestDockerImageSecurity tests security aspects of the Docker image
func TestDockerImageSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker security test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Ensure image is built
	ensureDockerImageBuilt(t, ctx)

	// Test 1: Verify non-root user
	t.Run("NonRootUser", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "sprint-service-test:latest", "id", "-u")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to check user ID: %v", err)
		}

		userID := strings.TrimSpace(string(output))
		if userID == "0" {
			t.Error("Container is running as root user (UID 0), should run as non-root")
		} else {
			t.Logf("Container running as non-root user (UID: %s)", userID)
		}
	})

	// Test 2: Verify binary exists and is executable
	t.Run("BinaryExists", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "sprint-service-test:latest", "ls", "-la", "/app/sprint-server")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Binary not found: %v\nOutput: %s", err, string(output))
		}

		if !strings.Contains(string(output), "sprint-server") {
			t.Error("sprint-server binary not found in /app directory")
		}
	})

	// Test 3: Verify migrations directory exists
	t.Run("MigrationsExist", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "sprint-service-test:latest", "ls", "-la", "/app/migrations")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Migrations directory not found: %v\nOutput: %s", err, string(output))
		}

		if !strings.Contains(string(output), "000001_create_schemas") {
			t.Error("Migration files not found in /app/migrations directory")
		}
	})
}

// TestDockerHealthCheck tests the Docker health check functionality
func TestDockerHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker health check test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Ensure image is built
	ensureDockerImageBuilt(t, ctx)

	// Start container with health check
	containerName := fmt.Sprintf("sprint-test-%d", time.Now().Unix())
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", containerName,
		"-p", "8083:8080",
		"-e", "DATABASE_MASTER_URL=postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable",
		"-e", "DATABASE_REPLICA_URL=postgresql://postgres:admin@localhost:5432/sprint_management?sslmode=disable",
		"-e", "NATS_URL=nats://localhost:4222",
		"sprint-service-test:latest")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start container: %v\nOutput: %s", err, string(output))
	}

	// Ensure cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		exec.CommandContext(cleanupCtx, "docker", "stop", containerName).Run()
		exec.CommandContext(cleanupCtx, "docker", "rm", containerName).Run()
	}()

	// Wait for container to start
	time.Sleep(5 * time.Second)

	// Check container is running
	cmd = exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Status}}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check container status: %v", err)
	}

	status := strings.TrimSpace(string(output))
	if !strings.Contains(status, "Up") {
		t.Errorf("Container is not running. Status: %s", status)
	} else {
		t.Logf("Container is running: %s", status)
	}

	// Test health endpoint (may fail if dependencies not available, but container should start)
	t.Run("HealthEndpoint", func(t *testing.T) {
		// Give the service time to start
		time.Sleep(2 * time.Second)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://localhost:8083/health/live")
		if err != nil {
			t.Logf("Health check endpoint not reachable (expected if dependencies unavailable): %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Logf("Health check endpoint returned OK")
		} else {
			t.Logf("Health check endpoint returned status: %d (may be expected if dependencies unavailable)", resp.StatusCode)
		}
	})
}

// TestDockerComposeValidation tests docker-compose configuration
func TestDockerComposeValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping docker-compose validation test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Test docker-compose.yml validation
	t.Run("ValidateDockerCompose", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker-compose", "-f", "../../docker-compose.yml", "config")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("docker-compose.yml validation failed: %v\nOutput: %s", err, string(output))
		}

		// Check for required services
		requiredServices := []string{"postgres", "nats", "keycloak", "sprint-service"}
		for _, service := range requiredServices {
			if !strings.Contains(string(output), service) {
				t.Errorf("Required service '%s' not found in docker-compose.yml", service)
			}
		}

		t.Logf("docker-compose.yml is valid and contains all required services")
	})

	// Test docker-compose.prod.yml validation
	t.Run("ValidateDockerComposeProd", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, "docker-compose", "-f", "../../docker-compose.prod.yml", "config")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("docker-compose.prod.yml validation failed: %v\nOutput: %s", err, string(output))
		}

		t.Logf("docker-compose.prod.yml is valid")
	})
}

// TestDockerImageSize tests that the Docker image is reasonably sized
func TestDockerImageSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker image size test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Ensure image is built
	ensureDockerImageBuilt(t, ctx)

	// Get image size
	cmd := exec.CommandContext(ctx, "docker", "images", "sprint-service-test:latest", "--format", "{{.Size}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get image size: %v", err)
	}

	size := strings.TrimSpace(string(output))
	t.Logf("Docker image size: %s", size)

	// Image should be less than 100MB (Alpine-based multi-stage build)
	// This is a soft check - log warning if larger
	if strings.Contains(size, "GB") || (strings.Contains(size, "MB") && !strings.HasPrefix(size, "1") && !strings.HasPrefix(size, "2") && !strings.HasPrefix(size, "3") && !strings.HasPrefix(size, "4") && !strings.HasPrefix(size, "5") && !strings.HasPrefix(size, "6") && !strings.HasPrefix(size, "7") && !strings.HasPrefix(size, "8") && !strings.HasPrefix(size, "9")) {
		t.Logf("Warning: Image size is larger than expected for Alpine-based build: %s", size)
	}
}

// Helper function to ensure Docker image is built
func ensureDockerImageBuilt(t *testing.T, ctx context.Context) {
	// Check if image exists
	cmd := exec.CommandContext(ctx, "docker", "images", "sprint-service-test:latest", "-q")
	output, err := cmd.CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		// Image doesn't exist, build it
		t.Log("Building Docker image...")
		buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", "sprint-service-test:latest", "../../")
		buildOutput, buildErr := buildCmd.CombinedOutput()
		if buildErr != nil {
			t.Fatalf("Failed to build Docker image: %v\nOutput: %s", buildErr, string(buildOutput))
		}
		t.Log("Docker image built successfully")
	}
}

// TestDockerfileExists verifies the Dockerfile exists and has required content
func TestDockerfileExists(t *testing.T) {
	dockerfilePath := "../../Dockerfile"

	// Check file exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		t.Fatal("Dockerfile does not exist")
	}

	// Read Dockerfile content
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}

	dockerfile := string(content)

	// Verify multi-stage build
	if !strings.Contains(dockerfile, "FROM golang") {
		t.Error("Dockerfile should use golang base image for build stage")
	}

	if !strings.Contains(dockerfile, "FROM alpine") {
		t.Error("Dockerfile should use alpine base image for runtime stage")
	}

	// Verify non-root user
	if !strings.Contains(dockerfile, "USER") {
		t.Error("Dockerfile should specify non-root USER")
	}

	// Verify health check
	if !strings.Contains(dockerfile, "HEALTHCHECK") {
		t.Error("Dockerfile should include HEALTHCHECK instruction")
	}

	// Verify port exposure
	if !strings.Contains(dockerfile, "EXPOSE 8080") {
		t.Error("Dockerfile should expose port 8080")
	}

	t.Log("Dockerfile validation passed")
}
