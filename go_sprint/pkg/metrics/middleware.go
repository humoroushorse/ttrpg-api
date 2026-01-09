package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

// HTTPMetricsMiddleware creates middleware for collecting HTTP metrics
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status and size
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			size:           0,
		}

		// Get request size
		requestSize := r.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}

		// Process request
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)
		endpoint := r.URL.Path
		method := r.Method

		RecordHTTPRequest(method, endpoint, status, duration, requestSize, rw.size)
	})
}

// DatabaseMetricsWrapper wraps database operations with metrics
type DatabaseMetricsWrapper struct {
	operation string
	table     string
	start     time.Time
}

// NewDatabaseMetricsWrapper creates a new database metrics wrapper
func NewDatabaseMetricsWrapper(operation, table string) *DatabaseMetricsWrapper {
	return &DatabaseMetricsWrapper{
		operation: operation,
		table:     table,
		start:     time.Now(),
	}
}

// Record records the database operation metrics
func (d *DatabaseMetricsWrapper) Record(err error, rowsAffected int64) {
	duration := time.Since(d.start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}
	RecordDatabaseOperation(d.operation, d.table, status, duration, rowsAffected)
}

// NATSMetricsWrapper wraps NATS operations with metrics
type NATSMetricsWrapper struct {
	subject string
	start   time.Time
}

// NewNATSMetricsWrapper creates a new NATS metrics wrapper
func NewNATSMetricsWrapper(subject string) *NATSMetricsWrapper {
	return &NATSMetricsWrapper{
		subject: subject,
		start:   time.Now(),
	}
}

// RecordPublish records a NATS publish operation
func (n *NATSMetricsWrapper) RecordPublish(err error) {
	duration := time.Since(n.start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}
	RecordNATSPublish(n.subject, status, duration)
}
