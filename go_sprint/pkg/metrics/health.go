package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
}

// CheckResult represents the result of a single health check
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HealthCheckHandler manages health checks for the service
type HealthCheckHandler struct {
	mu       sync.RWMutex
	checkers map[string]HealthChecker
	db       *sql.DB
	natsConn interface{} // NATS connection interface
}

// NewHealthCheckHandler creates a new health check handler
func NewHealthCheckHandler(db *sql.DB, natsConn interface{}) *HealthCheckHandler {
	return &HealthCheckHandler{
		checkers: make(map[string]HealthChecker),
		db:       db,
		natsConn: natsConn,
	}
}

// RegisterChecker registers a custom health checker
func (h *HealthCheckHandler) RegisterChecker(name string, checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// LivenessHandler handles liveness probe requests
// Liveness checks if the application is running (not deadlocked)
func (h *HealthCheckHandler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := HealthStatus{
			Status:    "ok",
			Timestamp: time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	}
}

// ReadinessHandler handles readiness probe requests
// Readiness checks if the application is ready to serve traffic
func (h *HealthCheckHandler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		checks := make(map[string]CheckResult)
		overallStatus := "ok"

		// Check database connection
		if h.db != nil {
			if err := h.checkDatabase(ctx); err != nil {
				checks["database"] = CheckResult{
					Status: "unhealthy",
					Error:  err.Error(),
				}
				overallStatus = "unhealthy"
			} else {
				checks["database"] = CheckResult{
					Status:  "healthy",
					Message: "Database connection is healthy",
				}
			}
		}

		// Check NATS connection
		if h.natsConn != nil {
			if err := h.checkNATS(ctx); err != nil {
				checks["nats"] = CheckResult{
					Status: "unhealthy",
					Error:  err.Error(),
				}
				overallStatus = "unhealthy"
			} else {
				checks["nats"] = CheckResult{
					Status:  "healthy",
					Message: "NATS connection is healthy",
				}
			}
		}

		// Run custom health checkers
		h.mu.RLock()
		for name, checker := range h.checkers {
			if err := checker.CheckHealth(ctx); err != nil {
				checks[name] = CheckResult{
					Status: "unhealthy",
					Error:  err.Error(),
				}
				overallStatus = "unhealthy"
			} else {
				checks[name] = CheckResult{
					Status:  "healthy",
					Message: "Check passed",
				}
			}
		}
		h.mu.RUnlock()

		status := HealthStatus{
			Status:    overallStatus,
			Timestamp: time.Now().UTC(),
			Checks:    checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if overallStatus == "ok" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(status)
	}
}

// checkDatabase checks if the database is accessible
func (h *HealthCheckHandler) checkDatabase(ctx context.Context) error {
	if h.db == nil {
		return nil
	}
	return h.db.PingContext(ctx)
}

// checkNATS checks if NATS connection is healthy
func (h *HealthCheckHandler) checkNATS(ctx context.Context) error {
	if h.natsConn == nil {
		return nil
	}

	// Type assertion to check if it has IsConnected method
	type natsConnection interface {
		IsConnected() bool
	}

	if nc, ok := h.natsConn.(natsConnection); ok {
		if !nc.IsConnected() {
			return &HealthCheckError{Component: "nats", Message: "NATS is not connected"}
		}
	}

	return nil
}

// HealthCheckError represents a health check error
type HealthCheckError struct {
	Component string
	Message   string
}

func (e *HealthCheckError) Error() string {
	return e.Component + ": " + e.Message
}

// StartupHandler handles startup probe requests
// Startup checks if the application has finished initialization
func (h *HealthCheckHandler) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		checks := make(map[string]CheckResult)
		overallStatus := "ok"

		// Check database is initialized
		if h.db != nil {
			if err := h.checkDatabase(ctx); err != nil {
				checks["database"] = CheckResult{
					Status: "not_ready",
					Error:  err.Error(),
				}
				overallStatus = "not_ready"
			} else {
				checks["database"] = CheckResult{
					Status:  "ready",
					Message: "Database is initialized",
				}
			}
		}

		status := HealthStatus{
			Status:    overallStatus,
			Timestamp: time.Now().UTC(),
			Checks:    checks,
		}

		w.Header().Set("Content-Type", "application/json")
		if overallStatus == "ok" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(status)
	}
}
