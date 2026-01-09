package types

import (
	"time"

	"github.com/humoroushorse/go_auth/pkg/auth/models"
)

// ValidateTokenResponse represents the response from token validation
type ValidateTokenResponse struct {
	Valid   bool         `json:"valid"`
	User    *models.User `json:"user,omitempty"`
	Error   string       `json:"error,omitempty"`
	TraceID string       `json:"trace_id,omitempty"`
}

// TokenResponse represents a token response from authentication
type TokenResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	TokenType        string    `json:"token_type"`
	ExpiresIn        int       `json:"expires_in"`
	RefreshExpiresIn int       `json:"refresh_expires_in,omitempty"`
	Scope            string    `json:"scope,omitempty"`
	IssuedAt         time.Time `json:"issued_at"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	TraceID   string    `json:"trace_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// UserSyncResponse represents the response from user synchronization
type UserSyncResponse struct {
	Success bool         `json:"success"`
	User    *models.User `json:"user,omitempty"`
	Error   string       `json:"error,omitempty"`
	TraceID string       `json:"trace_id,omitempty"`
}
