package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordHTTPRequest(t *testing.T) {
	// Reset metrics before test
	HTTPRequestsTotal.Reset()

	// Record a request
	RecordHTTPRequest("GET", "/api/workitems", "200", 0.5, 1024, 2048)

	// Verify counter was incremented
	count := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "/api/workitems", "200"))
	if count < 1 {
		t.Errorf("Expected counter to be at least 1, got %f", count)
	}
}

func TestRecordDatabaseOperation(t *testing.T) {
	// Reset metrics before test
	DatabaseOperationsTotal.Reset()

	// Record an operation
	RecordDatabaseOperation("SELECT", "work_items", "success", 0.01, 10)

	// Verify counter was incremented
	count := testutil.ToFloat64(DatabaseOperationsTotal.WithLabelValues("SELECT", "work_items", "success"))
	if count < 1 {
		t.Errorf("Expected counter to be at least 1, got %f", count)
	}
}

func TestUpdateDatabaseConnections(t *testing.T) {
	// Reset metrics before test
	DatabaseConnectionsActive.Reset()
	DatabaseConnectionsIdle.Reset()

	// Update connections
	UpdateDatabaseConnections("master", 10, 5)

	// Verify gauges were set
	active := testutil.ToFloat64(DatabaseConnectionsActive.WithLabelValues("master"))
	if active != 10 {
		t.Errorf("Expected active connections to be 10, got %f", active)
	}

	idle := testutil.ToFloat64(DatabaseConnectionsIdle.WithLabelValues("master"))
	if idle != 5 {
		t.Errorf("Expected idle connections to be 5, got %f", idle)
	}
}

func TestRecordNATSPublish(t *testing.T) {
	// Reset metrics before test
	NATSMessagesPublished.Reset()

	// Record a publish
	RecordNATSPublish("sprint.trace123.workitem.create.request", "success", 0.005)

	// Verify counter was incremented
	count := testutil.ToFloat64(NATSMessagesPublished.WithLabelValues("sprint.trace123.workitem.create.request", "success"))
	if count < 1 {
		t.Errorf("Expected counter to be at least 1, got %f", count)
	}
}

func TestUpdateNATSConnectionStatus(t *testing.T) {
	// Test connected status
	UpdateNATSConnectionStatus(true)
	status := testutil.ToFloat64(NATSConnectionStatus)
	if status != 1 {
		t.Errorf("Expected status to be 1 (connected), got %f", status)
	}

	// Test disconnected status
	UpdateNATSConnectionStatus(false)
	status = testutil.ToFloat64(NATSConnectionStatus)
	if status != 0 {
		t.Errorf("Expected status to be 0 (disconnected), got %f", status)
	}
}

func TestRecordWorkItemCreated(t *testing.T) {
	// Reset metrics before test
	WorkItemsCreated.Reset()

	// Record work item creation
	RecordWorkItemCreated("story")

	// Verify counter was incremented
	count := testutil.ToFloat64(WorkItemsCreated.WithLabelValues("story"))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}
}

func TestRecordWorkItemCompleted(t *testing.T) {
	// Reset metrics before test
	WorkItemsCompleted.Reset()

	// Record work item completion
	RecordWorkItemCompleted("story", 24.0, 48.0)

	// Verify counter was incremented
	count := testutil.ToFloat64(WorkItemsCompleted.WithLabelValues("story"))
	if count < 1 {
		t.Errorf("Expected counter to be at least 1, got %f", count)
	}
}

func TestUpdateSprintsActive(t *testing.T) {
	// Update active sprints
	UpdateSprintsActive(3)

	// Verify gauge was set
	count := testutil.ToFloat64(SprintsActive)
	if count != 3 {
		t.Errorf("Expected active sprints to be 3, got %f", count)
	}
}

func TestRecordSprintCompleted(t *testing.T) {
	// Record sprint completion
	RecordSprintCompleted("sprint-123", 45.0, 90.0, 85.0)

	// Verify counter was incremented
	count := testutil.ToFloat64(SprintsCompleted)
	if count < 1 {
		t.Errorf("Expected counter to be at least 1, got %f", count)
	}
}

func TestUpdateWebSocketConnections(t *testing.T) {
	// Update WebSocket connections
	UpdateWebSocketConnections(5)

	// Verify gauge was set
	count := testutil.ToFloat64(WebSocketConnectionsActive)
	if count != 5 {
		t.Errorf("Expected active WebSocket connections to be 5, got %f", count)
	}
}

func TestRecordWebSocketMessage(t *testing.T) {
	// Reset metrics before test
	WebSocketMessagesPublished.Reset()

	// Record WebSocket message
	RecordWebSocketMessage("workitem.updated")

	// Verify counter was incremented
	count := testutil.ToFloat64(WebSocketMessagesPublished.WithLabelValues("workitem.updated"))
	if count != 1 {
		t.Errorf("Expected counter to be 1, got %f", count)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Verify all metrics are registered with Prometheus
	metrics := []prometheus.Collector{
		HTTPRequestsTotal,
		HTTPRequestDuration,
		DatabaseOperationsTotal,
		DatabaseOperationDuration,
		NATSMessagesPublished,
		NATSMessagesReceived,
		WorkItemsTotal,
		WorkItemsCreated,
		WorkItemsCompleted,
		SprintsActive,
		SprintsCompleted,
		WebSocketConnectionsActive,
	}

	for _, metric := range metrics {
		if metric == nil {
			t.Error("Metric is nil")
		}
	}
}
