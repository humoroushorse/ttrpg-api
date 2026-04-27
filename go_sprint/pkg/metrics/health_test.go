package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"
)

// mockHealthChecker implements HealthChecker for testing
type mockHealthChecker struct {
	shouldFail bool
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context) error {
	if m.shouldFail {
		return errors.New("mock health check failed")
	}
	return nil
}

// mockNATSConnection implements a mock NATS connection
type mockNATSConnection struct {
	connected bool
}

func (m *mockNATSConnection) IsConnected() bool {
	return m.connected
}

func TestLivenessHandler(t *testing.T) {
	handler := NewHealthCheckHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	handler.LivenessHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status.Status)
	}
}

func TestReadinessHandler_Healthy(t *testing.T) {
	// Create mock NATS connection
	mockNATS := &mockNATSConnection{connected: true}

	handler := NewHealthCheckHandler(nil, mockNATS)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status.Status)
	}

	// Check NATS health
	if natsCheck, ok := status.Checks["nats"]; ok {
		if natsCheck.Status != "healthy" {
			t.Errorf("Expected NATS status 'healthy', got '%s'", natsCheck.Status)
		}
	}
}

func TestReadinessHandler_UnhealthyNATS(t *testing.T) {
	// Create mock NATS connection that's disconnected
	mockNATS := &mockNATSConnection{connected: false}

	handler := NewHealthCheckHandler(nil, mockNATS)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", status.Status)
	}

	// Check NATS health
	if natsCheck, ok := status.Checks["nats"]; ok {
		if natsCheck.Status != "unhealthy" {
			t.Errorf("Expected NATS status 'unhealthy', got '%s'", natsCheck.Status)
		}
	}
}

func TestReadinessHandler_WithCustomChecker(t *testing.T) {
	handler := NewHealthCheckHandler(nil, nil)

	// Register a healthy custom checker
	handler.RegisterChecker("custom", &mockHealthChecker{shouldFail: false})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status.Status)
	}

	// Check custom health
	if customCheck, ok := status.Checks["custom"]; ok {
		if customCheck.Status != "healthy" {
			t.Errorf("Expected custom status 'healthy', got '%s'", customCheck.Status)
		}
	} else {
		t.Error("Expected custom check in results")
	}
}

func TestReadinessHandler_WithFailingCustomChecker(t *testing.T) {
	handler := NewHealthCheckHandler(nil, nil)

	// Register a failing custom checker
	handler.RegisterChecker("custom", &mockHealthChecker{shouldFail: true})

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	handler.ReadinessHandler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "unhealthy" {
		t.Errorf("Expected status 'unhealthy', got '%s'", status.Status)
	}

	// Check custom health
	if customCheck, ok := status.Checks["custom"]; ok {
		if customCheck.Status != "unhealthy" {
			t.Errorf("Expected custom status 'unhealthy', got '%s'", customCheck.Status)
		}
		if customCheck.Error == "" {
			t.Error("Expected error message in custom check")
		}
	} else {
		t.Error("Expected custom check in results")
	}
}

func TestStartupHandler(t *testing.T) {
	handler := NewHealthCheckHandler(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/startup", nil)
	w := httptest.NewRecorder()

	handler.StartupHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var status HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", status.Status)
	}
}

func TestHealthCheckError(t *testing.T) {
	err := &HealthCheckError{
		Component: "database",
		Message:   "connection failed",
	}

	expected := "database: connection failed"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestRegisterChecker(t *testing.T) {
	handler := NewHealthCheckHandler(nil, nil)

	// Register multiple checkers
	handler.RegisterChecker("checker1", &mockHealthChecker{shouldFail: false})
	handler.RegisterChecker("checker2", &mockHealthChecker{shouldFail: false})

	if len(handler.checkers) != 2 {
		t.Errorf("Expected 2 checkers, got %d", len(handler.checkers))
	}

	// Verify checkers are registered
	if _, ok := handler.checkers["checker1"]; !ok {
		t.Error("Expected checker1 to be registered")
	}
	if _, ok := handler.checkers["checker2"]; !ok {
		t.Error("Expected checker2 to be registered")
	}
}

func TestHealthStatusJSON(t *testing.T) {
	status := HealthStatus{
		Status: "ok",
		Checks: map[string]CheckResult{
			"database": {
				Status:  "healthy",
				Message: "Database is healthy",
			},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Failed to marshal status: %v", err)
	}

	var decoded HealthStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal status: %v", err)
	}

	if decoded.Status != status.Status {
		t.Errorf("Expected status '%s', got '%s'", status.Status, decoded.Status)
	}

	if len(decoded.Checks) != len(status.Checks) {
		t.Errorf("Expected %d checks, got %d", len(status.Checks), len(decoded.Checks))
	}
}
