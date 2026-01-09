package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_size_bytes",
			Help:    "Size of HTTP requests in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "endpoint"},
	)

	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "Size of HTTP responses in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"method", "endpoint"},
	)

	// Database Metrics
	DatabaseOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "database_operations_total",
			Help: "Total number of database operations",
		},
		[]string{"operation", "table", "status"},
	)

	DatabaseOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_operation_duration_seconds",
			Help:    "Duration of database operations in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"operation", "table"},
	)

	DatabaseConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections_active",
			Help: "Number of active database connections",
		},
		[]string{"pool"},
	)

	DatabaseConnectionsIdle = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections_idle",
			Help: "Number of idle database connections",
		},
		[]string{"pool"},
	)

	DatabaseRowsAffected = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_rows_affected",
			Help:    "Number of rows affected by database operations",
			Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000},
		},
		[]string{"operation", "table"},
	)

	// NATS Metrics
	NATSMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nats_messages_published_total",
			Help: "Total number of NATS messages published",
		},
		[]string{"subject", "status"},
	)

	NATSMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nats_messages_received_total",
			Help: "Total number of NATS messages received",
		},
		[]string{"subject"},
	)

	NATSPublishDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "nats_publish_duration_seconds",
			Help:    "Duration of NATS publish operations in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"subject"},
	)

	NATSConnectionStatus = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "nats_connection_status",
			Help: "NATS connection status (1 = connected, 0 = disconnected)",
		},
	)

	NATSReconnections = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "nats_reconnections_total",
			Help: "Total number of NATS reconnections",
		},
	)

	// Business Metrics - Work Items
	WorkItemsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "work_items_total",
			Help: "Total number of work items",
		},
		[]string{"type", "status"},
	)

	WorkItemsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "work_items_created_total",
			Help: "Total number of work items created",
		},
		[]string{"type"},
	)

	WorkItemsCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "work_items_completed_total",
			Help: "Total number of work items completed",
		},
		[]string{"type"},
	)

	WorkItemCycleTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "work_item_cycle_time_hours",
			Help:    "Cycle time of work items in hours (from in_progress to done)",
			Buckets: []float64{1, 4, 8, 24, 48, 72, 168, 336, 720}, // 1h to 30 days
		},
		[]string{"type"},
	)

	WorkItemLeadTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "work_item_lead_time_hours",
			Help:    "Lead time of work items in hours (from creation to done)",
			Buckets: []float64{1, 4, 8, 24, 48, 72, 168, 336, 720}, // 1h to 30 days
		},
		[]string{"type"},
	)

	// Business Metrics - Sprints
	SprintsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sprints_active",
			Help: "Number of active sprints",
		},
	)

	SprintsCompleted = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "sprints_completed_total",
			Help: "Total number of completed sprints",
		},
	)

	SprintVelocity = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sprint_velocity_points",
			Help:    "Sprint velocity in story points",
			Buckets: []float64{10, 20, 30, 40, 50, 75, 100, 150, 200},
		},
		[]string{"sprint_id"},
	)

	SprintCompletionRate = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sprint_completion_rate_percent",
			Help:    "Sprint completion rate as percentage",
			Buckets: []float64{0, 25, 50, 75, 90, 95, 100},
		},
		[]string{"sprint_id"},
	)

	SprintCapacityUtilization = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sprint_capacity_utilization_percent",
			Help:    "Sprint capacity utilization as percentage",
			Buckets: []float64{0, 25, 50, 75, 90, 100, 125, 150},
		},
		[]string{"sprint_id"},
	)

	// WebSocket Metrics
	WebSocketConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Number of active WebSocket connections",
		},
	)

	WebSocketMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "websocket_messages_published_total",
			Help: "Total number of WebSocket messages published",
		},
		[]string{"event_type"},
	)

	// Cache Metrics (already defined in performance.go, but adding for completeness)
	CacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total number of cache operations",
		},
		[]string{"operation", "cache_name", "status"},
	)
)

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(method, endpoint, status string, duration float64, requestSize, responseSize int64) {
	HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint, status).Observe(duration)
	if requestSize > 0 {
		HTTPRequestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	}
	if responseSize > 0 {
		HTTPResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
	}
}

// RecordDatabaseOperation records a database operation
func RecordDatabaseOperation(operation, table, status string, duration float64, rowsAffected int64) {
	DatabaseOperationsTotal.WithLabelValues(operation, table, status).Inc()
	DatabaseOperationDuration.WithLabelValues(operation, table).Observe(duration)
	if rowsAffected > 0 {
		DatabaseRowsAffected.WithLabelValues(operation, table).Observe(float64(rowsAffected))
	}
}

// UpdateDatabaseConnections updates database connection metrics
func UpdateDatabaseConnections(pool string, active, idle int) {
	DatabaseConnectionsActive.WithLabelValues(pool).Set(float64(active))
	DatabaseConnectionsIdle.WithLabelValues(pool).Set(float64(idle))
}

// RecordNATSPublish records a NATS publish operation
func RecordNATSPublish(subject, status string, duration float64) {
	NATSMessagesPublished.WithLabelValues(subject, status).Inc()
	NATSPublishDuration.WithLabelValues(subject).Observe(duration)
}

// RecordNATSReceive records a NATS message received
func RecordNATSReceive(subject string) {
	NATSMessagesReceived.WithLabelValues(subject).Inc()
}

// UpdateNATSConnectionStatus updates NATS connection status
func UpdateNATSConnectionStatus(connected bool) {
	if connected {
		NATSConnectionStatus.Set(1)
	} else {
		NATSConnectionStatus.Set(0)
	}
}

// RecordNATSReconnection records a NATS reconnection
func RecordNATSReconnection() {
	NATSReconnections.Inc()
}

// UpdateWorkItemsTotal updates the total work items gauge
func UpdateWorkItemsTotal(itemType, status string, count int) {
	WorkItemsTotal.WithLabelValues(itemType, status).Set(float64(count))
}

// RecordWorkItemCreated records a work item creation
func RecordWorkItemCreated(itemType string) {
	WorkItemsCreated.WithLabelValues(itemType).Inc()
}

// RecordWorkItemCompleted records a work item completion
func RecordWorkItemCompleted(itemType string, cycleTimeHours, leadTimeHours float64) {
	WorkItemsCompleted.WithLabelValues(itemType).Inc()
	WorkItemCycleTime.WithLabelValues(itemType).Observe(cycleTimeHours)
	WorkItemLeadTime.WithLabelValues(itemType).Observe(leadTimeHours)
}

// UpdateSprintsActive updates the active sprints gauge
func UpdateSprintsActive(count int) {
	SprintsActive.Set(float64(count))
}

// RecordSprintCompleted records a sprint completion with metrics
func RecordSprintCompleted(sprintID string, velocity float64, completionRate, capacityUtilization float64) {
	SprintsCompleted.Inc()
	SprintVelocity.WithLabelValues(sprintID).Observe(velocity)
	SprintCompletionRate.WithLabelValues(sprintID).Observe(completionRate)
	SprintCapacityUtilization.WithLabelValues(sprintID).Observe(capacityUtilization)
}

// UpdateWebSocketConnections updates WebSocket connection count
func UpdateWebSocketConnections(count int) {
	WebSocketConnectionsActive.Set(float64(count))
}

// RecordWebSocketMessage records a WebSocket message published
func RecordWebSocketMessage(eventType string) {
	WebSocketMessagesPublished.WithLabelValues(eventType).Inc()
}

// RecordCacheOperation records a cache operation
func RecordCacheOperation(operation, cacheName, status string) {
	CacheOperations.WithLabelValues(operation, cacheName, status).Inc()
}
