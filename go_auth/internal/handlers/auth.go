package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/humoroushorse/go_auth/internal/service/auth"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	keycloakService *auth.KeycloakService
	logger          *slog.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(keycloakService *auth.KeycloakService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		keycloakService: keycloakService,
		logger:          logger,
	}
}

// LoginRequest represents the login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest represents the registration request
type RegisterRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// RefreshRequest represents the refresh token request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse form data (OAuth2 password grant uses form encoding)
	if err := r.ParseForm(); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Failed to parse form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Username and password are required")
		return
	}

	// Authenticate with Keycloak
	tokenResp, err := h.keycloakService.Login(ctx, username, password)
	if err != nil {
		h.logger.Error("login failed",
			slog.String("username", username),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	h.logger.Info("user logged in", slog.String("username", username))

	// Set HTTP-only cookies for browser-based clients
	SetAuthCookies(w, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.IDToken, tokenResp.ExpiresIn)

	// Return token response
	h.respondJSON(w, http.StatusOK, tokenResp)
}

// Refresh handles POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get refresh token from cookie first
	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}

	// If not in cookie, try request body
	if refreshToken == "" {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		h.respondError(w, http.StatusNotFound, "no_refresh_token", "No refresh token provided")
		return
	}

	// Refresh token with Keycloak
	tokenResp, err := h.keycloakService.RefreshToken(ctx, refreshToken)
	if err != nil {
		h.logger.Error("token refresh failed", slog.String("error", err.Error()))
		h.respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired refresh token")
		return
	}

	h.logger.Info("token refreshed")

	// Set HTTP-only cookies with new tokens
	SetAuthCookies(w, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.IDToken, tokenResp.ExpiresIn)

	// Return token response
	h.respondJSON(w, http.StatusOK, tokenResp)
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get refresh token from cookie
	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}

	// If not in cookie, try request body
	if refreshToken == "" {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}

	if refreshToken == "" {
		// No token to logout, just return success
		h.logger.Info("logout called with no refresh token")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Logout from Keycloak
	if err := h.keycloakService.Logout(ctx, refreshToken); err != nil {
		h.logger.Error("logout failed", slog.String("error", err.Error()))
		// Don't fail the logout if Keycloak returns an error
	}

	// Clear HTTP-only cookies
	ClearAuthCookies(w)

	h.logger.Info("user logged out")

	w.WriteHeader(http.StatusOK)
}

// GetUser handles GET /auth/user
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try to get access token from cookie first (preferred for browser clients)
	accessToken := ""
	if cookie, err := r.Cookie("access_token"); err == nil {
		accessToken = cookie.Value
	}

	// If not in cookie, try Authorization header (for API clients)
	if accessToken == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// Remove "Bearer " prefix
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				accessToken = authHeader[7:]
			} else {
				accessToken = authHeader
			}
		}
	}

	// If no token found in either location
	if accessToken == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "No access token provided (cookie or Authorization header)")
		return
	}

	// Get user info from Keycloak
	userInfo, err := h.keycloakService.GetUserInfo(ctx, accessToken)
	if err != nil {
		h.logger.Error("failed to get user info", slog.String("error", err.Error()))
		h.respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired access token")
		return
	}

	h.logger.Debug("user info retrieved", slog.String("user_id", userInfo.Sub))

	// Return user info
	h.respondJSON(w, http.StatusOK, userInfo)
}

// Register handles POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Username, email, and password are required")
		return
	}

	// Validate password strength
	if len(req.Password) < 8 {
		h.respondError(w, http.StatusBadRequest, "weak_password", "Password must be at least 8 characters long")
		return
	}

	// Create user in Keycloak
	userID, err := h.keycloakService.CreateUser(ctx, req.Username, req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		h.logger.Error("user registration failed",
			slog.String("username", req.Username),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusBadRequest, "registration_failed", "Failed to create user: "+err.Error())
		return
	}

	h.logger.Info("user registered",
		slog.String("username", req.Username),
		slog.String("user_id", userID),
	)

	// Return user info
	userInfo := map[string]interface{}{
		"sub":                userID,
		"preferred_username": req.Username,
		"email":              req.Email,
		"name":               req.FirstName + " " + req.LastName,
		"given_name":         req.FirstName,
		"family_name":        req.LastName,
	}

	h.respondJSON(w, http.StatusCreated, userInfo)
}

// Helper methods

func (h *AuthHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AuthHandler) respondError(w http.ResponseWriter, status int, errorCode, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}

// SetCookies sets authentication cookies (helper for services that want to use cookies)
func SetAuthCookies(w http.ResponseWriter, accessToken, refreshToken, idToken string, expiresIn int) {
	// Set access token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiresIn,
	})

	// Set refresh token cookie (longer expiration)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	})

	// Set ID token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "id_token",
		Value:    idToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiresIn,
	})
}

// ClearAuthCookies clears authentication cookies
func ClearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "id_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}
