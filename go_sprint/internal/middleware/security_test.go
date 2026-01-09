package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := SecurityHeadersMiddleware()
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	// Check security headers
	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", "geolocation=(), microphone=(), camera=()"},
	}

	for _, tt := range tests {
		got := rr.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("Header %s: expected %q, got %q", tt.header, tt.expected, got)
		}
	}

	// Check CSP header contains expected directives
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") {
		t.Error("CSP should contain default-src 'self'")
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("CSP should contain frame-ancestors 'none'")
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "with whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "with HTML tags",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "with null bytes",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "with control characters",
			input:    "Hello\x01\x02World",
			expected: "HelloWorld",
		},
		{
			name:     "with newlines and tabs",
			input:    "Hello\nWorld\tTest",
			expected: "Hello\nWorld\tTest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeInput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContainsSQLInjectionPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "normal text",
			input:    "Hello World",
			expected: false,
		},
		{
			name:     "SQL injection - OR 1=1",
			input:    "' OR 1=1--",
			expected: true,
		},
		{
			name:     "SQL injection - union select",
			input:    "' UNION SELECT * FROM users--",
			expected: true,
		},
		{
			name:     "SQL injection - drop table",
			input:    "; DROP TABLE users;",
			expected: true,
		},
		{
			name:     "SQL injection - exec",
			input:    "exec('malicious code')",
			expected: true,
		},
		{
			name:     "SQL injection - xp_cmdshell",
			input:    "xp_cmdshell 'dir'",
			expected: true,
		},
		{
			name:     "SQL injection - information_schema",
			input:    "SELECT * FROM information_schema.tables",
			expected: true,
		},
		{
			name:     "normal SQL-like text",
			input:    "I selected the best option",
			expected: false, // Changed: normal text shouldn't trigger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsSQLInjectionPatterns(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsSQLInjectionPatterns(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContainsXSSPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "normal text",
			input:    "Hello World",
			expected: false,
		},
		{
			name:     "XSS - script tag",
			input:    "<script>alert('xss')</script>",
			expected: true,
		},
		{
			name:     "XSS - javascript protocol",
			input:    "javascript:alert('xss')",
			expected: true,
		},
		{
			name:     "XSS - onerror",
			input:    "<img src=x onerror=alert('xss')>",
			expected: true,
		},
		{
			name:     "XSS - onload",
			input:    "<body onload=alert('xss')>",
			expected: true,
		},
		{
			name:     "XSS - iframe",
			input:    "<iframe src='malicious.com'></iframe>",
			expected: true,
		},
		{
			name:     "XSS - eval",
			input:    "eval('malicious code')",
			expected: true,
		},
		{
			name:     "XSS - vbscript",
			input:    "vbscript:msgbox('xss')",
			expected: true,
		},
		{
			name:     "XSS - data URI",
			input:    "data:text/html,<script>alert('xss')</script>",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsXSSPatterns(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsXSSPatterns(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContainsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "normal path",
			input:    "/api/v1/workitems",
			expected: false,
		},
		{
			name:     "path traversal - ../",
			input:    "/api/../../etc/passwd",
			expected: true,
		},
		{
			name:     "path traversal - ..\\",
			input:    "/api/..\\..\\windows\\system32",
			expected: true,
		},
		{
			name:     "path traversal - URL encoded",
			input:    "/api/%2e%2e/etc/passwd",
			expected: true,
		},
		{
			name:     "path traversal - mixed encoding",
			input:    "/api/..%2f../etc/passwd",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsPathTraversal(tt.input)
			if result != tt.expected {
				t.Errorf("ContainsPathTraversal(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		minLength int
		maxLength int
		fieldName string
		wantError bool
	}{
		{
			name:      "valid length",
			value:     "Hello",
			minLength: 1,
			maxLength: 10,
			fieldName: "test",
			wantError: false,
		},
		{
			name:      "too short",
			value:     "Hi",
			minLength: 5,
			maxLength: 10,
			fieldName: "test",
			wantError: true,
		},
		{
			name:      "too long",
			value:     "This is a very long string",
			minLength: 1,
			maxLength: 10,
			fieldName: "test",
			wantError: true,
		},
		{
			name:      "exact min length",
			value:     "Hello",
			minLength: 5,
			maxLength: 10,
			fieldName: "test",
			wantError: false,
		},
		{
			name:      "exact max length",
			value:     "HelloWorld",
			minLength: 1,
			maxLength: 10,
			fieldName: "test",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStringLength(tt.value, tt.minLength, tt.maxLength, tt.fieldName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateStringLength() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputSanitizationMiddleware_SQLInjection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := InputSanitizationMiddleware(auditor)
	wrappedHandler := middleware(handler)

	// Test SQL injection in query parameter (URL encoded)
	req := httptest.NewRequest("GET", "/test?id=%27+OR+1%3D1--", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, "test-user")
	ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for SQL injection, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "INVALID_INPUT") {
		t.Error("Response should contain INVALID_INPUT error code")
	}
}

func TestInputSanitizationMiddleware_XSS(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := InputSanitizationMiddleware(auditor)
	wrappedHandler := middleware(handler)

	// Test XSS in query parameter (URL encoded)
	req := httptest.NewRequest("GET", "/test?name=%3Cscript%3Ealert%28%27xss%27%29%3C%2Fscript%3E", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, "test-user")
	ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for XSS, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "INVALID_INPUT") {
		t.Error("Response should contain INVALID_INPUT error code")
	}
}

func TestInputSanitizationMiddleware_PathTraversal(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := InputSanitizationMiddleware(auditor)
	wrappedHandler := middleware(handler)

	// Test path traversal
	req := httptest.NewRequest("GET", "/api/../../etc/passwd", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, "test-user")
	ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for path traversal, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "INVALID_REQUEST") {
		t.Error("Response should contain INVALID_REQUEST error code")
	}
}

func TestInputSanitizationMiddleware_ValidInput(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := InputSanitizationMiddleware(auditor)
	wrappedHandler := middleware(handler)

	// Test valid input
	req := httptest.NewRequest("GET", "/test?name=John&age=30", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, "test-user")
	ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for valid input, got %d", rr.Code)
	}
}

func TestSecurityAuditor_LogSecurityEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	ctx := context.Background()

	tests := []struct {
		name     string
		event    SecurityEvent
		severity string
	}{
		{
			name: "SQL injection attempt",
			event: SecurityEvent{
				EventType: "SQL_INJECTION_ATTEMPT",
				UserID:    "test-user",
				IPAddress: "192.168.1.1",
				Endpoint:  "/api/v1/test",
				Method:    "GET",
				TraceID:   "test-trace",
				Details:   "SQL injection pattern detected",
				Severity:  "high",
			},
			severity: "high",
		},
		{
			name: "XSS attempt",
			event: SecurityEvent{
				EventType: "XSS_ATTEMPT",
				UserID:    "test-user",
				IPAddress: "192.168.1.1",
				Endpoint:  "/api/v1/test",
				Method:    "GET",
				TraceID:   "test-trace",
				Details:   "XSS pattern detected",
				Severity:  "high",
			},
			severity: "high",
		},
		{
			name: "Path traversal attempt",
			event: SecurityEvent{
				EventType: "PATH_TRAVERSAL_ATTEMPT",
				UserID:    "test-user",
				IPAddress: "192.168.1.1",
				Endpoint:  "/api/../../etc/passwd",
				Method:    "GET",
				TraceID:   "test-trace",
				Details:   "Path traversal pattern detected",
				Severity:  "critical",
			},
			severity: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just ensures the logging doesn't panic
			auditor.LogSecurityEvent(ctx, tt.event)
		})
	}
}

func TestAuditAuthenticationMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	auditor := NewSecurityAuditor(logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := AuditAuthenticationMiddleware(auditor)
	wrappedHandler := middleware(handler)

	tests := []struct {
		name   string
		userID string
	}{
		{
			name:   "authenticated user",
			userID: "test-user",
		},
		{
			name:   "unauthenticated user",
			userID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			ctx := context.WithValue(req.Context(), UserContextKey, tt.userID)
			ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}
		})
	}
}
