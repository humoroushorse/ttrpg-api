package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// RequestsPerMinute is the default rate limit for all endpoints
	RequestsPerMinute int
	// EndpointLimits maps endpoint patterns to specific rate limits
	EndpointLimits map[string]int
	// BurstSize allows temporary bursts above the rate limit
	BurstSize int
}

// DefaultRateLimitConfig returns sensible default rate limiting configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
		EndpointLimits: map[string]int{
			"/api/v1/workitems": 100, // Higher limit for list endpoints
			"/api/v1/sprints":   100,
			"/api/v1/search":    30, // Lower limit for expensive operations
			"/api/v1/reports":   20,
			"/api/v1/import":    5, // Very low limit for bulk operations
			"/api/v1/export":    10,
		},
	}
}

// RateLimiter implements token bucket rate limiting per user
type RateLimiter struct {
	config  RateLimitConfig
	buckets map[string]*tokenBucket
	mu      sync.RWMutex
	logger  *slog.Logger
	metrics *RateLimitMetrics
}

// tokenBucket implements the token bucket algorithm
type tokenBucket struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens per second
	lastRefillTime time.Time
	mu             sync.Mutex
}

// RateLimitMetrics tracks rate limiting statistics
type RateLimitMetrics struct {
	TotalRequests int64
	RateLimited   int64
	UniqueUsers   int64
	mu            sync.RWMutex
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimitConfig, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		config:  config,
		buckets: make(map[string]*tokenBucket),
		logger:  logger,
		metrics: &RateLimitMetrics{},
	}

	// Start cleanup goroutine to remove inactive buckets
	go rl.cleanupInactiveBuckets()

	return rl
}

// RateLimitMiddleware creates middleware that enforces rate limits per user
func (rl *RateLimiter) RateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context (set by auth middleware)
			userID := GetUserIDFromContext(r.Context())
			if userID == "" {
				// Use IP address as fallback for unauthenticated requests
				userID = r.RemoteAddr
			}

			// Get endpoint-specific limit or use default
			limit := rl.getLimitForEndpoint(r.URL.Path)

			// Check if request is allowed (metrics updated inside allowRequest)
			allowed, retryAfter := rl.allowRequest(userID, limit)

			if !allowed {
				// Log rate limit event
				logger := LoggerFromContext(r.Context())
				logger.Warn("rate limit exceeded",
					slog.String("user_id", userID),
					slog.String("endpoint", r.URL.Path),
					slog.Int("limit", limit),
					slog.Duration("retry_after", retryAfter),
				)

				// Return 429 Too Many Requests
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(fmt.Sprintf(`{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"Rate limit exceeded. Please retry after %d seconds."},"trace_id":"%s"}`,
					int(retryAfter.Seconds()),
					GetTraceIDFromContext(r.Context()),
				)))
				return
			}

			// Add rate limit headers to response
			remaining := rl.getRemainingTokens(userID, limit)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// allowRequest checks if a request is allowed under the rate limit
func (rl *RateLimiter) allowRequest(userID string, limit int) (bool, time.Duration) {
	rl.mu.Lock()
	bucket, exists := rl.buckets[userID]
	if !exists {
		bucket = rl.createBucket(limit)
		rl.buckets[userID] = bucket
		rl.metrics.UniqueUsers++
	}
	rl.mu.Unlock()

	allowed, retryAfter := bucket.consume()

	// Update metrics
	rl.metrics.mu.Lock()
	rl.metrics.TotalRequests++
	if !allowed {
		rl.metrics.RateLimited++
	}
	rl.metrics.mu.Unlock()

	return allowed, retryAfter
}

// createBucket creates a new token bucket for a user
func (rl *RateLimiter) createBucket(limit int) *tokenBucket {
	maxTokens := float64(limit)
	refillRate := maxTokens / 60.0 // Convert per-minute to per-second

	return &tokenBucket{
		tokens:         maxTokens + float64(rl.config.BurstSize),
		maxTokens:      maxTokens + float64(rl.config.BurstSize),
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// consume attempts to consume one token from the bucket
func (tb *tokenBucket) consume() (bool, time.Duration) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()
	tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefillTime = now

	// Check if we have tokens available
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true, 0
	}

	// Calculate retry after duration
	tokensNeeded := 1.0 - tb.tokens
	retryAfter := time.Duration(tokensNeeded/tb.refillRate) * time.Second

	return false, retryAfter
}

// getLimitForEndpoint returns the rate limit for a specific endpoint
func (rl *RateLimiter) getLimitForEndpoint(path string) int {
	// Check for exact match first
	if limit, exists := rl.config.EndpointLimits[path]; exists {
		return limit
	}

	// Check for prefix matches
	for pattern, limit := range rl.config.EndpointLimits {
		if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
			return limit
		}
	}

	// Return default limit
	return rl.config.RequestsPerMinute
}

// getRemainingTokens returns the number of remaining tokens for a user
func (rl *RateLimiter) getRemainingTokens(userID string, limit int) int {
	rl.mu.RLock()
	bucket, exists := rl.buckets[userID]
	rl.mu.RUnlock()

	if !exists {
		return limit + rl.config.BurstSize
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on time elapsed (but don't modify the bucket)
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefillTime).Seconds()
	tokens := min(bucket.maxTokens, bucket.tokens+elapsed*bucket.refillRate)

	return int(tokens)
}

// cleanupInactiveBuckets removes buckets that haven't been used recently
func (rl *RateLimiter) cleanupInactiveBuckets() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for userID, bucket := range rl.buckets {
			bucket.mu.Lock()
			inactive := now.Sub(bucket.lastRefillTime) > 10*time.Minute
			bucket.mu.Unlock()

			if inactive {
				delete(rl.buckets, userID)
			}
		}
		rl.mu.Unlock()
	}
}

// GetMetrics returns current rate limiting metrics
func (rl *RateLimiter) GetMetrics() RateLimitMetrics {
	rl.metrics.mu.RLock()
	defer rl.metrics.mu.RUnlock()

	return RateLimitMetrics{
		TotalRequests: rl.metrics.TotalRequests,
		RateLimited:   rl.metrics.RateLimited,
		UniqueUsers:   rl.metrics.UniqueUsers,
	}
}

// ResetMetrics resets all rate limiting metrics
func (rl *RateLimiter) ResetMetrics() {
	rl.metrics.mu.Lock()
	defer rl.metrics.mu.Unlock()

	rl.metrics.TotalRequests = 0
	rl.metrics.RateLimited = 0
	rl.metrics.UniqueUsers = 0
}

// Helper function for min
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// GetUserIDFromContext extracts the user ID from the request context
func GetUserIDFromContext(ctx context.Context) string {
	userID, ok := ctx.Value(UserContextKey).(string)
	if !ok {
		return ""
	}
	return userID
}
