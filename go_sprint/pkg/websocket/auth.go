package websocket

import (
	"context"
	"errors"
)

// SimpleAuthValidator is a simple implementation of AuthValidator for testing
// In production, this should use the actual JWT validation from go_auth
type SimpleAuthValidator struct {
	// validTokens maps tokens to user IDs for testing
	validTokens map[string]string
}

// NewSimpleAuthValidator creates a new SimpleAuthValidator
func NewSimpleAuthValidator() *SimpleAuthValidator {
	return &SimpleAuthValidator{
		validTokens: make(map[string]string),
	}
}

// AddToken adds a valid token for testing
func (v *SimpleAuthValidator) AddToken(token, userID string) {
	v.validTokens[token] = userID
}

// ValidateToken validates a token and returns the user ID
func (v *SimpleAuthValidator) ValidateToken(ctx context.Context, token string) (string, error) {
	if userID, ok := v.validTokens[token]; ok {
		return userID, nil
	}
	return "", errors.New("invalid token")
}

// RemoveToken removes a token
func (v *SimpleAuthValidator) RemoveToken(token string) {
	delete(v.validTokens, token)
}
