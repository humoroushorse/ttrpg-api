package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/humoroushorse/go_auth/pkg/auth/models"
)

var (
	// ErrMissingToken is returned when no authorization token is provided
	ErrMissingToken = errors.New("missing authorization token")
	// ErrInvalidToken is returned when the token is invalid
	ErrInvalidToken = errors.New("invalid authorization token")
	// ErrExpiredToken is returned when the token has expired
	ErrExpiredToken = errors.New("token has expired")
	// ErrInvalidSignature is returned when the token signature is invalid
	ErrInvalidSignature = errors.New("invalid token signature")
)

// JWTConfig holds configuration for JWT validation
type JWTConfig struct {
	// KeycloakURL is the base URL of the Keycloak server
	KeycloakURL string
	// Realm is the Keycloak realm name
	Realm string
	// ClientID is the client ID for this application
	ClientID string
	// PublicKey is the RSA public key for token verification (optional, will fetch from Keycloak if not provided)
	PublicKey *rsa.PublicKey
	// RefreshInterval is how often to refresh the public key from Keycloak
	RefreshInterval time.Duration
	// SkipValidation skips token validation (for testing only)
	SkipValidation bool
}

// JWTValidator handles JWT token validation
type JWTValidator struct {
	config    JWTConfig
	publicKey *rsa.PublicKey
	mu        sync.RWMutex
	lastFetch time.Time
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(config JWTConfig) *JWTValidator {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 1 * time.Hour
	}

	return &JWTValidator{
		config:    config,
		publicKey: config.PublicKey,
	}
}

// JWTAuth creates middleware for JWT authentication
func JWTAuth(validator *JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token, err := extractToken(r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}

			// Validate token and extract user
			user, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}

			// Add user to context
			ctx := WithUser(r.Context(), user)

			// Continue with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateToken validates a JWT token and returns the user information
func (v *JWTValidator) ValidateToken(ctx context.Context, tokenString string) (*models.User, error) {
	// Skip validation if configured (for testing)
	if v.config.SkipValidation {
		return &models.User{
			Username: "test-user",
			Email:    "test@example.com",
			IsActive: true,
		}, nil
	}

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get public key
		publicKey, err := v.getPublicKey(ctx)
		if err != nil {
			return nil, err
		}

		return publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidSignature
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Extract claims
	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Convert claims to user
	user, err := claims.ToUser()
	if err != nil {
		return nil, fmt.Errorf("failed to convert claims to user: %w", err)
	}

	return user, nil
}

// getPublicKey retrieves the public key for token verification
func (v *JWTValidator) getPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	v.mu.RLock()
	// Check if we have a cached key and it's still fresh
	if v.publicKey != nil && time.Since(v.lastFetch) < v.config.RefreshInterval {
		key := v.publicKey
		v.mu.RUnlock()
		return key, nil
	}
	v.mu.RUnlock()

	// Need to fetch or refresh the key
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check after acquiring write lock
	if v.publicKey != nil && time.Since(v.lastFetch) < v.config.RefreshInterval {
		return v.publicKey, nil
	}

	// Fetch public key from Keycloak
	key, err := v.fetchPublicKeyFromKeycloak(ctx)
	if err != nil {
		// If we have a cached key, use it even if expired
		if v.publicKey != nil {
			return v.publicKey, nil
		}
		return nil, err
	}

	v.publicKey = key
	v.lastFetch = time.Now()

	return key, nil
}

// fetchPublicKeyFromKeycloak fetches the public key from Keycloak's JWKS endpoint
func (v *JWTValidator) fetchPublicKeyFromKeycloak(ctx context.Context) (*rsa.PublicKey, error) {
	// Construct JWKS URL
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", v.config.KeycloakURL, v.config.Realm)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch JWKS: status %d", resp.StatusCode)
	}

	// Parse JWKS response
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Find the first RSA key for signing
	for _, key := range jwks.Keys {
		if key.Kty == "RSA" && (key.Use == "sig" || key.Use == "") {
			// Decode modulus
			nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
			if err != nil {
				continue
			}

			// Decode exponent
			eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
			if err != nil {
				continue
			}

			// Convert exponent bytes to int
			var eInt int
			for _, b := range eBytes {
				eInt = eInt<<8 + int(b)
			}

			// Create RSA public key
			publicKey := &rsa.PublicKey{
				N: new(big.Int).SetBytes(nBytes),
				E: eInt,
			}

			return publicKey, nil
		}
	}

	return nil, errors.New("no suitable RSA key found in JWKS")
}

// extractToken extracts the JWT token from the Authorization header or cookies
func extractToken(r *http.Request) (string, error) {
	// First, try to get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Check for Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1], nil
		}
		return "", ErrInvalidToken
	}

	// If no Authorization header, try to get token from cookie
	cookie, err := r.Cookie("access_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	// No token found in header or cookie
	return "", ErrMissingToken
}

// WithUser adds a user to the context
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}

// GetUser extracts the user from the context
func GetUser(ctx context.Context) (*models.User, error) {
	user, ok := ctx.Value(UserKey).(*models.User)
	if !ok || user == nil {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}

// UserFromContext is an alias for GetUser for convenience
func UserFromContext(ctx context.Context) (*models.User, error) {
	return GetUser(ctx)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
