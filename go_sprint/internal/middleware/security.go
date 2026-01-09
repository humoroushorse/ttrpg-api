package middleware

import (
	"context"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SecurityEvent represents a security-related event for audit logging
type SecurityEvent struct {
	EventType string
	UserID    string
	IPAddress string
	Endpoint  string
	Method    string
	TraceID   string
	Details   string
	Timestamp time.Time
	Severity  string // "low", "medium", "high", "critical"
}

// SecurityAuditor handles security event logging
type SecurityAuditor struct {
	logger *slog.Logger
}

// NewSecurityAuditor creates a new security auditor
func NewSecurityAuditor(logger *slog.Logger) *SecurityAuditor {
	return &SecurityAuditor{
		logger: logger,
	}
}

// LogSecurityEvent logs a security event with appropriate severity
func (sa *SecurityAuditor) LogSecurityEvent(ctx context.Context, event SecurityEvent) {
	event.Timestamp = time.Now()

	logAttrs := []slog.Attr{
		slog.String("event_type", event.EventType),
		slog.String("user_id", event.UserID),
		slog.String("ip_address", event.IPAddress),
		slog.String("endpoint", event.Endpoint),
		slog.String("method", event.Method),
		slog.String("trace_id", event.TraceID),
		slog.String("details", event.Details),
		slog.String("severity", event.Severity),
		slog.Time("timestamp", event.Timestamp),
	}

	switch event.Severity {
	case "critical":
		sa.logger.LogAttrs(ctx, slog.LevelError, "SECURITY EVENT", logAttrs...)
	case "high":
		sa.logger.LogAttrs(ctx, slog.LevelWarn, "SECURITY EVENT", logAttrs...)
	case "medium":
		sa.logger.LogAttrs(ctx, slog.LevelInfo, "SECURITY EVENT", logAttrs...)
	default:
		sa.logger.LogAttrs(ctx, slog.LevelDebug, "SECURITY EVENT", logAttrs...)
	}
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable XSS protection
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Control referrer information
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy - more restrictive
			csp := "default-src 'self'; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self'; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
			w.Header().Set("Content-Security-Policy", csp)

			// HSTS for HTTPS connections
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			// Permissions Policy (formerly Feature Policy)
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			next.ServeHTTP(w, r)
		})
	}
}

// InputSanitizationMiddleware sanitizes and validates all incoming requests
func InputSanitizationMiddleware(auditor *SecurityAuditor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check for SQL injection patterns in query parameters
			for key, values := range r.URL.Query() {
				for _, value := range values {
					if ContainsSQLInjectionPatterns(value) {
						// Log security event
						auditor.LogSecurityEvent(ctx, SecurityEvent{
							EventType: "SQL_INJECTION_ATTEMPT",
							UserID:    GetUserIDFromContext(ctx),
							IPAddress: r.RemoteAddr,
							Endpoint:  r.URL.Path,
							Method:    r.Method,
							TraceID:   GetTraceIDFromContext(ctx),
							Details:   "SQL injection pattern detected in query parameter: " + key,
							Severity:  "high",
						})

						// Return 400 Bad Request
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte(`{"error":{"code":"INVALID_INPUT","message":"Invalid input detected"}}`))
						return
					}
				}
			}

			// Check for XSS patterns in query parameters
			for key, values := range r.URL.Query() {
				for _, value := range values {
					if ContainsXSSPatterns(value) {
						// Log security event
						auditor.LogSecurityEvent(ctx, SecurityEvent{
							EventType: "XSS_ATTEMPT",
							UserID:    GetUserIDFromContext(ctx),
							IPAddress: r.RemoteAddr,
							Endpoint:  r.URL.Path,
							Method:    r.Method,
							TraceID:   GetTraceIDFromContext(ctx),
							Details:   "XSS pattern detected in query parameter: " + key,
							Severity:  "high",
						})

						// Return 400 Bad Request
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusBadRequest)
						w.Write([]byte(`{"error":{"code":"INVALID_INPUT","message":"Invalid input detected"}}`))
						return
					}
				}
			}

			// Check for path traversal attempts
			if ContainsPathTraversal(r.URL.Path) {
				// Log security event
				auditor.LogSecurityEvent(ctx, SecurityEvent{
					EventType: "PATH_TRAVERSAL_ATTEMPT",
					UserID:    GetUserIDFromContext(ctx),
					IPAddress: r.RemoteAddr,
					Endpoint:  r.URL.Path,
					Method:    r.Method,
					TraceID:   GetTraceIDFromContext(ctx),
					Details:   "Path traversal pattern detected",
					Severity:  "critical",
				})

				// Return 400 Bad Request
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":{"code":"INVALID_REQUEST","message":"Invalid request path"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeInput removes potentially dangerous characters from user input
func SanitizeInput(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)

	// HTML escape to prevent XSS
	input = html.EscapeString(input)

	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove other control characters
	input = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, input)

	return input
}

// ValidateStringLength validates that a string is within acceptable length bounds
func ValidateStringLength(value string, minLength, maxLength int, fieldName string) error {
	length := len(value)

	if length < minLength {
		return &ValidationError{
			Field:   fieldName,
			Message: "value is too short",
			Code:    "STRING_TOO_SHORT",
		}
	}

	if length > maxLength {
		return &ValidationError{
			Field:   fieldName,
			Message: "value is too long",
			Code:    "STRING_TOO_LONG",
		}
	}

	return nil
}

// ContainsSQLInjectionPatterns checks if input contains common SQL injection patterns
func ContainsSQLInjectionPatterns(input string) bool {
	lowerInput := strings.ToLower(input)

	// Common SQL injection patterns
	sqlPatterns := []string{
		"' or '1'='1",
		"' or 1=1",
		"\" or \"1\"=\"1",
		"\" or 1=1",
		"; drop table",
		"; delete from",
		"union select",
		"' union select",
		"exec(",
		"execute(",
		"xp_cmdshell",
		"sp_executesql",
		"--",
		"/*",
		"*/",
		"@@version",
		"information_schema",
	}

	for _, pattern := range sqlPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// ContainsXSSPatterns checks if input contains common XSS patterns
func ContainsXSSPatterns(input string) bool {
	lowerInput := strings.ToLower(input)

	// Common XSS patterns
	xssPatterns := []string{
		"<script",
		"</script>",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"onmouseover=",
		"<iframe",
		"<embed",
		"<object",
		"eval(",
		"expression(",
		"vbscript:",
		"data:text/html",
	}

	for _, pattern := range xssPatterns {
		if strings.Contains(lowerInput, pattern) {
			return true
		}
	}

	return false
}

// ContainsPathTraversal checks if path contains path traversal patterns
func ContainsPathTraversal(path string) bool {
	// Check for common path traversal patterns
	traversalPatterns := []string{
		"../",
		"..\\",
		"..%2f",
		"..%5c",
		"%2e%2e/",
		"%2e%2e\\",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range traversalPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// AuditAuthenticationMiddleware logs authentication events
func AuditAuthenticationMiddleware(auditor *SecurityAuditor) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Check if user is authenticated
			userID := GetUserIDFromContext(ctx)

			if userID == "" {
				// Log unauthenticated access attempt
				auditor.LogSecurityEvent(ctx, SecurityEvent{
					EventType: "UNAUTHENTICATED_ACCESS",
					UserID:    "anonymous",
					IPAddress: r.RemoteAddr,
					Endpoint:  r.URL.Path,
					Method:    r.Method,
					TraceID:   GetTraceIDFromContext(ctx),
					Details:   "Unauthenticated access attempt",
					Severity:  "low",
				})
			} else {
				// Log authenticated access
				auditor.LogSecurityEvent(ctx, SecurityEvent{
					EventType: "AUTHENTICATED_ACCESS",
					UserID:    userID,
					IPAddress: r.RemoteAddr,
					Endpoint:  r.URL.Path,
					Method:    r.Method,
					TraceID:   GetTraceIDFromContext(ctx),
					Details:   "Authenticated access",
					Severity:  "low",
				})
			}

			next.ServeHTTP(w, r)
		})
	}
}
