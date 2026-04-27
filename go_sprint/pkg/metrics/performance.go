package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Database query duration histogram
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"operation", "table"},
	)

	// Cache hit/miss counter
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_name"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_name"},
	)

	// API response time histogram
	APIResponseDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_response_duration_seconds",
			Help:    "Duration of API responses in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint", "status"},
	)

	// Pagination metrics
	PaginationPageSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pagination_page_size",
			Help:    "Size of paginated responses",
			Buckets: []float64{10, 25, 50, 100, 200, 500},
		},
		[]string{"endpoint"},
	)

	PaginationCursorUsage = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pagination_cursor_usage_total",
			Help: "Total number of cursor-based pagination requests",
		},
		[]string{"endpoint", "has_cursor"},
	)

	// Query performance metrics
	QueryRowsReturned = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "query_rows_returned",
			Help:    "Number of rows returned by queries",
			Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000},
		},
		[]string{"operation", "table"},
	)
)

// Timer is a helper for timing operations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// ObserveDatabaseQuery records the duration of a database query
func (t *Timer) ObserveDatabaseQuery(operation, table string) {
	duration := time.Since(t.start).Seconds()
	DatabaseQueryDuration.WithLabelValues(operation, table).Observe(duration)
}

// ObserveAPIResponse records the duration of an API response
func (t *Timer) ObserveAPIResponse(method, endpoint, status string) {
	duration := time.Since(t.start).Seconds()
	APIResponseDuration.WithLabelValues(method, endpoint, status).Observe(duration)
}

// RecordCacheHit records a cache hit
func RecordCacheHit(cacheName string) {
	CacheHits.WithLabelValues(cacheName).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(cacheName string) {
	CacheMisses.WithLabelValues(cacheName).Inc()
}

// RecordPaginationPageSize records the size of a paginated response
func RecordPaginationPageSize(endpoint string, size int) {
	PaginationPageSize.WithLabelValues(endpoint).Observe(float64(size))
}

// RecordPaginationCursorUsage records cursor usage in pagination
func RecordPaginationCursorUsage(endpoint string, hasCursor bool) {
	cursorStr := "false"
	if hasCursor {
		cursorStr = "true"
	}
	PaginationCursorUsage.WithLabelValues(endpoint, cursorStr).Inc()
}

// RecordQueryRowsReturned records the number of rows returned by a query
func RecordQueryRowsReturned(operation, table string, rows int) {
	QueryRowsReturned.WithLabelValues(operation, table).Observe(float64(rows))
}
