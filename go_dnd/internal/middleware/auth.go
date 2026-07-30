package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	authmiddleware "github.com/humoroushorse/go_auth/pkg/auth/middleware"
	"github.com/humoroushorse/go_auth/pkg/auth/models"
)

type contextKey string

const (
	UserContextKey    contextKey = "user"
	TraceIDContextKey contextKey = "trace_id"
	LoggerContextKey  contextKey = "logger"
)

func AuthMiddleware(validator *authmiddleware.JWTValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	jwtMiddleware := authmiddleware.JWTAuth(validator)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Debug("authenticating request", slog.String("path", r.URL.Path), slog.String("method", r.Method))
			jwtMiddleware(next).ServeHTTP(w, r)
		})
	}
}

func OptionalAuthMiddleware(validator *authmiddleware.JWTValidator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			hasCookie := false
			if _, err := r.Cookie("access_token"); err == nil {
				hasCookie = true
			}
			if authHeader == "" && !hasCookie {
				next.ServeHTTP(w, r)
				return
			}
			parts := strings.Split(authHeader, " ")
			if authHeader != "" && (len(parts) != 2 || parts[0] != "Bearer") {
				next.ServeHTTP(w, r)
				return
			}
			authmiddleware.JWTAuth(validator)(next).ServeHTTP(w, r)
		})
	}
}

func GetUserFromContext(ctx context.Context) (*models.User, bool) {
	user, err := authmiddleware.UserFromContext(ctx)
	if err == nil && user != nil {
		return user, true
	}
	user, ok := ctx.Value(UserContextKey).(*models.User)
	return user, ok
}

func GetTraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(TraceIDContextKey).(string)
	return traceID
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(LoggerContextKey).(*slog.Logger)
	if !ok || logger == nil {
		return slog.Default()
	}
	return logger
}
