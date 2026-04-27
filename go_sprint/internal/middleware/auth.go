package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	authmiddleware "github.com/humoroushorse/go_auth/pkg/auth/middleware"
	"github.com/humoroushorse/go_auth/pkg/auth/models"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// UserContextKey is the key for storing user information in context
	UserContextKey contextKey = "user"
	// TraceIDContextKey is the key for storing trace ID in context
	TraceIDContextKey contextKey = "trace_id"
	// LoggerContextKey is the key for storing logger in context
	LoggerContextKey contextKey = "logger"
)

// AuthMiddleware wraps the shared auth library's JWT middleware
// and adapts it for the sprint service
func AuthMiddleware(validator *authmiddleware.JWTValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	// Use the shared JWT middleware from go_auth library
	// The shared middleware already handles token extraction and validation
	jwtMiddleware := authmiddleware.JWTAuth(validator)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Log the authentication attempt
			logger.Debug("authenticating request",
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
			)

			// Use the shared JWT middleware to validate and extract user
			// It will handle token extraction, validation, and error responses
			jwtMiddleware(next).ServeHTTP(w, r)
		})
	}
}

// GetUserFromContext extracts the user from the request context
func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	// Try the auth library's context key first
	user, err := authmiddleware.UserFromContext(ctx)
	if err == nil && user != nil {
		return user, true
	}

	// Fallback to our own context key
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}

// GetTraceIDFromContext extracts the trace ID from the request context
func GetTraceIDFromContext(ctx context.Context) string {
	traceID, ok := ctx.Value(TraceIDContextKey).(string)
	if !ok {
		return ""
	}
	return traceID
}

// OptionalAuthMiddleware is similar to AuthMiddleware but doesn't fail if no auth is provided
// Useful for endpoints that work with or without authentication
func OptionalAuthMiddleware(validator *authmiddleware.JWTValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No auth provided, continue without user context
				next.ServeHTTP(w, r)
				return
			}

			// Auth provided, validate it
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				// Invalid format, continue without user context
				logger.Debug("invalid authorization header format in optional auth",
					slog.String("path", r.URL.Path),
				)
				next.ServeHTTP(w, r)
				return
			}

			// Use the shared JWT middleware
			jwtMiddleware := authmiddleware.JWTAuth(validator)
			jwtMiddleware(next).ServeHTTP(w, r)
		})
	}
}
