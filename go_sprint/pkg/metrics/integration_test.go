package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsIntegration_HTTPWorkflow tests the complete HTTP metrics workflow
func TestMetricsIntegration_HTTPWorkflow(t *testing.T) {
	// Reset metrics
	HTTPRequestsTotal.Reset()

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	wrappedHandler := HTTPMetricsMiddleware(handler)

	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status code %d, got %d", i, http.StatusOK, w.Code)
		}
	}

	// Verify metrics were recorded
	count := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/api/test", "200"))
	if count < 5 {
		t.Errorf("Expected at least 5 requests recorded, got %f", count)
	}
}

// TestMetricsIntegration_DatabaseWorkflow tests the complete database metrics workflow
func TestMetricsIntegration_DatabaseWorkflow(t *testing.T) {
	// Reset metrics
	DatabaseOperationsTotal.Reset()

	// Simulate multiple database operations
	operations := []struct {
		operation string
		table     string
		status    string
	}{
		{"SELECT", "work_items", "success"},
		{"INSERT", "work_items", "success"},
		{"UPDATE", "work_items", "success"},
		{"DELETE", "work_items", "success"},
		{"SELECT", "sprints", "success"},
	}

	for _, op := range operations {
		RecordDatabaseOperation(op.operation, op.table, op.status, 0.01, 1)
	}

	// Verify metrics were recorded
	selectCount := testutil.ToFloat64(DatabaseOperationsTotal.WithLabelValues("SELECT", "work_items", "success"))
	if selectCount < 1 {
		t.Errorf("Expected at least 1 SELECT operation, got %f", selectCount)
	}

	insertCount := testutil.ToFloat64(DatabaseOperationsTotal.WithLabelValues("INSERT", "work_items", "success"))
	if insertCount < 1 {
		t.Errorf("Expected at least 1 INSERT operation, got %f", insertCount)
	}
}

// TestMetricsIntegration_NATSWorkflow tests the complete NATS metrics workflow
func TestMetricsIntegration_NATSWorkflow(t *testing.T) {
	// Reset metrics
	NATSMessagesPublished.Reset()
	NATSMessagesReceived.Reset()

	// Simulate NATS message flow
	subjects := []string{
		"sprint.trace1.workitem.create.request",
		"sprint.trace1.workitem.create.response",
		"sprint.trace2.sprint.update.request",
		"sprint.trace2.sprint.update.response",
	}

	// Publish messages
	for _, subject := range subjects {
		RecordNATSPublish(subject, "success", 0.005)
	}

	// Receive messages
	for _, subject := range subjects {
		RecordNATSReceive(subject)
	}

	// Verify metrics
	publishCount := testutil.ToFloat64(NATSMessagesPublished.WithLabelValues("sprint.trace1.workitem.create.request", "success"))
	if publishCount < 1 {
		t.Errorf("Expected at least 1 published message, got %f", publishCount)
	}

	receiveCount := testutil.ToFloat64(NATSMessagesReceived.WithLabelValues("sprint.trace1.workitem.create.request"))
	if receiveCount < 1 {
		t.Errorf("Expected at least 1 received message, got %f", receiveCount)
	}
}

// TestMetricsIntegration_BusinessMetrics tests business metrics collection
func TestMetricsIntegration_BusinessMetrics(t *testing.T) {
	// Reset metrics
	WorkItemsCreated.Reset()
	WorkItemsCompleted.Reset()

	// Simulate work item lifecycle
	workItemTypes := []string{"story", "defect", "epic"}

	for _, itemType := range workItemTypes {
		// Create work items
		for i := 0; i < 3; i++ {
			RecordWorkItemCreated(itemType)
		}

		// Complete some work items
		for i := 0; i < 2; i++ {
			RecordWorkItemCompleted(itemType, 24.0, 48.0)
		}
	}

	// Verify metrics
	storyCreated := testutil.ToFloat64(WorkItemsCreated.WithLabelValues("story"))
	if storyCreated < 3 {
		t.Errorf("Expected at least 3 stories created, got %f", storyCreated)
	}

	storyCompleted := testutil.ToFloat64(WorkItemsCompleted.WithLabelValues("story"))
	if storyCompleted < 2 {
		t.Errorf("Expected at least 2 stories completed, got %f", storyCompleted)
	}
}

// TestMetricsIntegration_SprintMetrics tests sprint metrics collection
func TestMetricsIntegration_SprintMetrics(t *testing.T) {
	// Update active sprints
	UpdateSprintsActive(3)

	// Verify gauge
	activeCount := testutil.ToFloat64(SprintsActive)
	if activeCount != 3 {
		t.Errorf("Expected 3 active sprints, got %f", activeCount)
	}

	// Complete sprints
	sprints := []struct {
		id                  string
		velocity            float64
		completionRate      float64
		capacityUtilization float64
	}{
		{"sprint-1", 45.0, 90.0, 85.0},
		{"sprint-2", 50.0, 95.0, 90.0},
		{"sprint-3", 40.0, 80.0, 75.0},
	}

	for _, sprint := range sprints {
		RecordSprintCompleted(sprint.id, sprint.velocity, sprint.completionRate, sprint.capacityUtilization)
	}

	// Verify counter increased
	completedCount := testutil.ToFloat64(SprintsCompleted)
	if completedCount < 3 {
		t.Errorf("Expected at least 3 completed sprints, got %f", completedCount)
	}
}

// TestMetricsIntegration_PrometheusExport tests Prometheus metrics export
func TestMetricsIntegration_PrometheusExport(t *testing.T) {
	// Record some metrics
	RecordHTTPRequest("GET", "/api/workitems", "200", 0.1, 1024, 2048)
	RecordDatabaseOperation("SELECT", "work_items", "success", 0.01, 10)
	RecordNATSPublish("sprint.trace.test", "success", 0.005)

	// Create Prometheus handler
	handler := promhttp.Handler()

	// Make request to metrics endpoint
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	// Verify response contains metrics
	body := w.Body.String()
	expectedMetrics := []string{
		"http_requests_total",
		"database_operations_total",
		"nats_messages_published_total",
	}

	for _, metric := range expectedMetrics {
		if !contains(body, metric) {
			t.Errorf("Expected metric '%s' in Prometheus output", metric)
		}
	}
}

// TestHealthCheckIntegration_CompleteWorkflow tests the complete health check workflow
func TestHealthCheckIntegration_CompleteWorkflow(t *testing.T) {
	// Create mock NATS connection
	mockNATS := &mockNATSConnection{connected: true}

	// Create health check handler
	handler := NewHealthCheckHandler(nil, mockNATS)

	// Register custom checker
	handler.RegisterChecker("custom", &mockHealthChecker{shouldFail: false})

	// Test liveness endpoint
	t.Run("Liveness", func(t *testing.T) {
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
	})

	// Test readiness endpoint
	t.Run("Readiness", func(t *testing.T) {
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

		// Verify all checks passed
		if len(status.Checks) == 0 {
			t.Error("Expected health checks in response")
		}

		for name, check := range status.Checks {
			if check.Status != "healthy" {
				t.Errorf("Check '%s' expected to be healthy, got '%s'", name, check.Status)
			}
		}
	})

	// Test startup endpoint
	t.Run("Startup", func(t *testing.T) {
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
	})
}

// TestHealthCheckIntegration_FailureScenarios tests health check failure scenarios
func TestHealthCheckIntegration_FailureScenarios(t *testing.T) {
	// Test with disconnected NATS
	t.Run("DisconnectedNATS", func(t *testing.T) {
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
	})

	// Test with failing custom checker
	t.Run("FailingCustomChecker", func(t *testing.T) {
		handler := NewHealthCheckHandler(nil, nil)
		handler.RegisterChecker("failing", &mockHealthChecker{shouldFail: true})

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

		if check, ok := status.Checks["failing"]; ok {
			if check.Status != "unhealthy" {
				t.Errorf("Expected failing check to be unhealthy, got '%s'", check.Status)
			}
		}
	})
}

// TestTracingIntegration_EndToEnd tests end-to-end tracing workflow
func TestTracingIntegration_EndToEnd(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	// Start parent span
	ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation")
	parentSpan.SetAttribute("user_id", "user-123")

	// Simulate database operation
	err := TraceDatabase(ctx, tracer, "SELECT", "work_items", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Simulate NATS operation
	err = TraceNATS(ctx, tracer, "sprint.trace.test", "publish", func() error {
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// End parent span
	parentSpan.End()

	// Verify span hierarchy
	if parentSpan.TraceID == "" {
		t.Error("Expected trace ID to be set")
	}
	if parentSpan.SpanID == "" {
		t.Error("Expected span ID to be set")
	}
	if parentSpan.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}
	if parentSpan.Status.Code != StatusCodeUnset {
		t.Errorf("Expected status code Unset, got %v", parentSpan.Status.Code)
	}
}

// TestTracingIntegration_NATSPropagation tests trace propagation through NATS
func TestTracingIntegration_NATSPropagation(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	// Start span
	ctx, span := tracer.StartSpan(ctx, "nats-publish")
	traceID := span.TraceID

	// Propagate trace to NATS headers
	headers := PropagateTraceToNATS(ctx)

	// Verify headers contain trace information
	if headers["trace-id"] != traceID {
		t.Errorf("Expected trace-id '%s', got '%s'", traceID, headers["trace-id"])
	}
	if headers["span-id"] != span.SpanID {
		t.Errorf("Expected span-id '%s', got '%s'", span.SpanID, headers["span-id"])
	}

	// Simulate receiving message on another service
	newCtx := context.Background()
	newCtx = ExtractTraceFromNATS(newCtx, headers)

	// Verify trace ID was extracted
	extractedTraceID := GetTraceIDFromContext(newCtx)
	if extractedTraceID != traceID {
		t.Errorf("Expected extracted trace ID '%s', got '%s'", traceID, extractedTraceID)
	}

	// Start child span in new context
	_, childSpan := tracer.StartSpan(newCtx, "nats-receive")

	// Verify child span has same trace ID
	if childSpan.TraceID != traceID {
		t.Errorf("Expected child trace ID '%s', got '%s'", traceID, childSpan.TraceID)
	}
}

// TestPerformanceMonitoringIntegration tests performance monitoring workflow
func TestPerformanceMonitoringIntegration(t *testing.T) {
	tracer := NewTracer("test-service")
	monitor := NewPerformanceMonitor(tracer, 50*time.Millisecond)

	ctx := context.Background()

	// Test fast operation
	err := monitor.MonitorOperation(ctx, "fast-op", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test slow operation (exceeds threshold)
	err = monitor.MonitorOperation(ctx, "slow-op", func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify spans were created
	if len(tracer.spans) < 2 {
		t.Errorf("Expected at least 2 spans, got %d", len(tracer.spans))
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
