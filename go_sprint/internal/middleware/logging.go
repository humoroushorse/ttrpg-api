package middleware

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// LoggingMiddleware creates a middleware that logs HTTP requests
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Get or generate trace ID
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}

			// Add trace ID to context
			ctx := context.WithValue(r.Context(), TraceIDContextKey, traceID)

			// Create a response writer wrapper to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Create context-aware logger
			contextLogger := logger.With(
				slog.String("trace_id", traceID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
			)

			// Add logger to context
			ctx = context.WithValue(ctx, LoggerContextKey, contextLogger)

			// Log request start
			contextLogger.Info("request started")

			// Process request
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Log request completion
			duration := time.Since(start)
			contextLogger.Info("request completed",
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

// LoggerFromContext extracts the logger from the request context
func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(LoggerContextKey).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}

// TraceIDMiddleware ensures every request has a trace ID
func TraceIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get or generate trace ID
			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}

			// Add trace ID to response headers
			w.Header().Set("X-Trace-ID", traceID)

			// Add trace ID to context
			ctx := context.WithValue(r.Context(), TraceIDContextKey, traceID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
