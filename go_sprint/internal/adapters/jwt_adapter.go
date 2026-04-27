package adapters

import (
	"context"

	authmiddleware "github.com/humoroushorse/go_auth/pkg/auth/middleware"
)

// JWTValidatorAdapter adapts the auth middleware JWTValidator to the websocket AuthValidator interface
type JWTValidatorAdapter struct {
	validator *authmiddleware.JWTValidator
}

// NewJWTValidatorAdapter creates a new adapter
func NewJWTValidatorAdapter(validator *authmiddleware.JWTValidator) *JWTValidatorAdapter {
	return &JWTValidatorAdapter{
		validator: validator,
	}
}

// ValidateToken validates a JWT token and returns the user ID
func (a *JWTValidatorAdapter) ValidateToken(ctx context.Context, token string) (string, error) {
	user, err := a.validator.ValidateToken(ctx, token)
	if err != nil {
		return "", err
	}
	return user.ID.String(), nil
}
