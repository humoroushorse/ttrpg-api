package types

// ValidateTokenRequest represents a request to validate a JWT token
type ValidateTokenRequest struct {
	Token   string `json:"token"`
	TraceID string `json:"trace_id,omitempty"`
}

// RefreshTokenRequest represents a request to refresh a JWT token
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	TraceID      string `json:"trace_id,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TraceID  string `json:"trace_id,omitempty"`
}

// LogoutRequest represents a logout request
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
}
