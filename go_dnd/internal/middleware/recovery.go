package middleware

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
)

func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger := LoggerFromContext(r.Context())
				logger.Error("panic recovered", "error", err, "stack", string(debug.Stack()))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error":    "internal server error",
					"trace_id": GetTraceIDFromContext(r.Context()),
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
