package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

const (
	// TraceIDKey is the context key for trace ID
	TraceIDKey ContextKey = "trace_id"
	// UserKey is the context key for user information
	UserKey ContextKey = "user"
	// LoggerKey is the context key for logger
	LoggerKey ContextKey = "logger"
)

// TraceIDHeader is the HTTP header name for trace ID
const TraceIDHeader = "X-Trace-ID"

// TraceID middleware extracts or generates a trace ID for request correlation
func TraceID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to get trace ID from header
			traceID := r.Header.Get(TraceIDHeader)

			// If no trace ID provided, generate a new one
			if traceID == "" {
				traceID = uuid.New().String()
			}

			// Validate that it's a valid UUID format
			if _, err := uuid.Parse(traceID); err != nil {
				// If invalid, generate a new one
				traceID = uuid.New().String()
			}

			// Add trace ID to response header
			w.Header().Set(TraceIDHeader, traceID)

			// Add trace ID to context
			ctx := context.WithValue(r.Context(), TraceIDKey, traceID)

			// Continue with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTraceID extracts the trace ID from the context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithTraceID adds a trace ID to the context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}
