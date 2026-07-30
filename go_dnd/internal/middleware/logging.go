package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}

		logger := slog.Default().With(
			slog.String("trace_id", traceID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)

		ctx := logging.WithLogger(r.Context(), logger)
		ctx = context.WithValue(ctx, TraceIDContextKey, traceID)

		w.Header().Set("X-Trace-ID", traceID)

		start := time.Now()
		logger.Info("request started")

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		logger.Info("request completed",
			slog.Int("status", rw.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func GetTraceIDFromRequest(r *http.Request) string {
	return GetTraceIDFromContext(r.Context())
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
