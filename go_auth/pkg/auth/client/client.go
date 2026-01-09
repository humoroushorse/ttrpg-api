package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/humoroushorse/go_auth/pkg/auth/middleware"
	"github.com/humoroushorse/go_auth/pkg/auth/models"
	"github.com/humoroushorse/go_auth/pkg/auth/types"
)

// AuthClient handles communication with the auth service
type AuthClient struct {
	baseURL    string
	httpClient *http.Client
}

// Config holds configuration for the auth client
type Config struct {
	BaseURL string
	Timeout time.Duration
}

// NewAuthClient creates a new auth service client
func NewAuthClient(config Config) *AuthClient {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	return &AuthClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ValidateToken validates a JWT token via the auth service
func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	traceID := middleware.GetTraceID(ctx)

	req := types.ValidateTokenRequest{
		Token:   token,
		TraceID: traceID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/validate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		httpReq.Header.Set(middleware.TraceIDHeader, traceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("validation failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("validation failed: %s", errResp.Message)
	}

	var validateResp types.ValidateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !validateResp.Valid {
		return nil, fmt.Errorf("token is invalid: %s", validateResp.Error)
	}

	return validateResp.User, nil
}

// RefreshToken refreshes a JWT token via the auth service
func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*types.TokenResponse, error) {
	traceID := middleware.GetTraceID(ctx)

	req := types.RefreshTokenRequest{
		RefreshToken: refreshToken,
		TraceID:      traceID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		httpReq.Header.Set(middleware.TraceIDHeader, traceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("refresh failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("refresh failed: %s", errResp.Message)
	}

	var tokenResp types.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// Login authenticates a user and returns tokens
func (c *AuthClient) Login(ctx context.Context, username, password string) (*types.TokenResponse, error) {
	traceID := middleware.GetTraceID(ctx)

	req := types.LoginRequest{
		Username: username,
		Password: password,
		TraceID:  traceID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		httpReq.Header.Set(middleware.TraceIDHeader, traceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("login failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("login failed: %s", errResp.Message)
	}

	var tokenResp types.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &tokenResp, nil
}

// Logout logs out a user by invalidating their refresh token
func (c *AuthClient) Logout(ctx context.Context, refreshToken string) error {
	traceID := middleware.GetTraceID(ctx)

	req := types.LogoutRequest{
		RefreshToken: refreshToken,
		TraceID:      traceID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/logout", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		httpReq.Header.Set(middleware.TraceIDHeader, traceID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errResp types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("logout failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("logout failed: %s", errResp.Message)
	}

	return nil
}
