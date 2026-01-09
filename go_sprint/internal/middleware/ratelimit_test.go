package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_AllowRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	userID := "test-user"
	limit := 60

	// First request should be allowed
	allowed, _ := rl.allowRequest(userID, limit)
	if !allowed {
		t.Error("First request should be allowed")
	}

	// Multiple requests within limit should be allowed
	for i := 0; i < 50; i++ {
		allowed, _ := rl.allowRequest(userID, limit)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+2)
		}
	}
}

func TestRateLimiter_ExceedLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 5, // Very low limit for testing
		BurstSize:         2,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	userID := "test-user"
	limit := 5

	// Consume all tokens plus burst
	allowedCount := 0
	for i := 0; i < 10; i++ {
		allowed, _ := rl.allowRequest(userID, limit)
		if allowed {
			allowedCount++
		}
	}

	// Should allow initial tokens + burst
	if allowedCount < 5 {
		t.Errorf("Expected at least 5 allowed requests, got %d", allowedCount)
	}

	// Should eventually deny requests
	if allowedCount >= 10 {
		t.Error("Expected some requests to be denied")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 60, // 1 token per second
		BurstSize:         0,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	userID := "test-user"
	limit := 60

	// Consume all tokens
	for i := 0; i < 65; i++ {
		rl.allowRequest(userID, limit)
	}

	// Wait for tokens to refill (2 seconds = 2 tokens)
	time.Sleep(2 * time.Second)

	// Should allow at least 1 request after refill
	allowed, _ := rl.allowRequest(userID, limit)
	if !allowed {
		t.Error("Request should be allowed after token refill")
	}
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         20,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	userID := "test-user"
	limit := 100

	// Simulate concurrent requests
	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := rl.allowRequest(userID, limit)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should allow most requests within limit
	if allowedCount < 40 {
		t.Errorf("Expected at least 40 allowed requests, got %d", allowedCount)
	}
}

func TestRateLimiter_MultipleUsers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         2,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	limit := 10

	// Each user should have independent rate limits
	user1Allowed := 0
	user2Allowed := 0

	for i := 0; i < 15; i++ {
		allowed, _ := rl.allowRequest("user1", limit)
		if allowed {
			user1Allowed++
		}

		allowed, _ = rl.allowRequest("user2", limit)
		if allowed {
			user2Allowed++
		}
	}

	// Both users should have similar allowed counts
	if user1Allowed < 8 || user2Allowed < 8 {
		t.Errorf("Expected both users to have at least 8 allowed requests, got user1=%d, user2=%d",
			user1Allowed, user2Allowed)
	}
}

func TestRateLimiter_EndpointSpecificLimits(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		EndpointLimits: map[string]int{
			"/api/v1/search": 10,
			"/api/v1/import": 5,
		},
	}
	rl := NewRateLimiter(config, logger)

	// Test default limit
	defaultLimit := rl.getLimitForEndpoint("/api/v1/workitems")
	if defaultLimit != 60 {
		t.Errorf("Expected default limit of 60, got %d", defaultLimit)
	}

	// Test specific limit
	searchLimit := rl.getLimitForEndpoint("/api/v1/search")
	if searchLimit != 10 {
		t.Errorf("Expected search limit of 10, got %d", searchLimit)
	}

	importLimit := rl.getLimitForEndpoint("/api/v1/import")
	if importLimit != 5 {
		t.Errorf("Expected import limit of 5, got %d", importLimit)
	}
}

func TestRateLimitMiddleware_Integration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 5,
		BurstSize:         2,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with rate limit middleware
	middleware := rl.RateLimitMiddleware()
	wrappedHandler := middleware(handler)

	// Test multiple requests
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/v1/test", nil)

		// Add user context
		ctx := context.WithValue(req.Context(), UserContextKey, "test-user")
		ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace")
		ctx = context.WithValue(ctx, "logger", logger)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(rr, req)

		if rr.Code == http.StatusOK {
			successCount++
		} else if rr.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// Should have some successful requests and some rate limited
	if successCount < 5 {
		t.Errorf("Expected at least 5 successful requests, got %d", successCount)
	}

	if rateLimitedCount == 0 {
		t.Error("Expected some requests to be rate limited")
	}

	t.Logf("Success: %d, Rate Limited: %d", successCount, rateLimitedCount)
}

func TestRateLimiter_Metrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         2,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	// Make some requests
	for i := 0; i < 20; i++ {
		rl.allowRequest("user1", 10)
	}

	for i := 0; i < 15; i++ {
		rl.allowRequest("user2", 10)
	}

	// Check metrics
	metrics := rl.GetMetrics()

	if metrics.TotalRequests != 35 {
		t.Errorf("Expected 35 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.RateLimited == 0 {
		t.Error("Expected some rate limited requests")
	}

	if metrics.UniqueUsers != 2 {
		t.Errorf("Expected 2 unique users, got %d", metrics.UniqueUsers)
	}

	t.Logf("Metrics: Total=%d, RateLimited=%d, UniqueUsers=%d",
		metrics.TotalRequests, metrics.RateLimited, metrics.UniqueUsers)
}

func TestRateLimiter_ResetMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 10,
		BurstSize:         2,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	// Make some requests
	for i := 0; i < 10; i++ {
		rl.allowRequest("user1", 10)
	}

	// Reset metrics
	rl.ResetMetrics()

	// Check metrics are reset
	metrics := rl.GetMetrics()

	if metrics.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests after reset, got %d", metrics.TotalRequests)
	}

	if metrics.RateLimited != 0 {
		t.Errorf("Expected 0 rate limited requests after reset, got %d", metrics.RateLimited)
	}

	if metrics.UniqueUsers != 0 {
		t.Errorf("Expected 0 unique users after reset, got %d", metrics.UniqueUsers)
	}
}

func TestRateLimiter_GetRemainingTokens(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		EndpointLimits:    map[string]int{},
	}
	rl := NewRateLimiter(config, logger)

	userID := "test-user"
	limit := 60

	// Check initial remaining tokens (should include burst)
	remaining := rl.getRemainingTokens(userID, limit)
	expectedInitial := limit + config.BurstSize
	if remaining != expectedInitial {
		t.Errorf("Expected %d remaining tokens initially, got %d", expectedInitial, remaining)
	}

	// Consume some tokens
	for i := 0; i < 10; i++ {
		rl.allowRequest(userID, limit)
	}

	// Check remaining tokens decreased
	remaining = rl.getRemainingTokens(userID, limit)
	if remaining >= expectedInitial {
		t.Errorf("Expected remaining tokens to decrease from %d, got %d", expectedInitial, remaining)
	}

	t.Logf("Remaining tokens after 10 requests: %d", remaining)
}
