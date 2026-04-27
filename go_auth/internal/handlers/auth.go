package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/humoroushorse/go_auth/internal/service/auth"
	"github.com/humoroushorse/go_auth/pkg/config"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	keycloakService *auth.KeycloakService
	keycloakConfig  *config.KeycloakConfig
	logger          *slog.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(keycloakService *auth.KeycloakService, keycloakConfig *config.KeycloakConfig, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		keycloakService: keycloakService,
		keycloakConfig:  keycloakConfig,
		logger:          logger,
	}
}

// LoginRequest represents the login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterRequest represents the registration request.
// Accepts both camelCase (userName, firstName, lastName) and snake_case (username, first_name, last_name).
type RegisterRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// UnmarshalJSON implements custom unmarshalling to accept both camelCase and snake_case field names.
func (r *RegisterRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["username"]; ok {
		r.Username = v
	}
	if v, ok := raw["userName"]; ok && r.Username == "" {
		r.Username = v
	}
	if v, ok := raw["email"]; ok {
		r.Email = v
	}
	if v, ok := raw["password"]; ok {
		r.Password = v
	}
	if v, ok := raw["first_name"]; ok {
		r.FirstName = v
	}
	if v, ok := raw["firstName"]; ok && r.FirstName == "" {
		r.FirstName = v
	}
	if v, ok := raw["last_name"]; ok {
		r.LastName = v
	}
	if v, ok := raw["lastName"]; ok && r.LastName == "" {
		r.LastName = v
	}
	return nil
}

// RefreshRequest represents the refresh token request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string         `json:"error"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// resolveRealm extracts realm from the URL path if present, otherwise returns default.
// Expects paths like /auth/{realm}/login or /auth/login (uses default realm).
func (h *AuthHandler) resolveRealm(r *http.Request) (realm, clientID string) {
	// Path is already cleaned by http.ServeMux
	// For realm-prefixed routes, the realm is set as a path value by the mux pattern
	if rv := r.PathValue("realm"); rv != "" {
		realm = rv
	} else {
		realm = h.keycloakConfig.DefaultRealm
	}
	clientID = h.keycloakConfig.GetClientIDForRealm(realm)
	return
}

// Login handles POST /auth/login and /auth/{realm}/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	realm, clientID := h.resolveRealm(r)

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

	tokenResp, err := h.keycloakService.Login(ctx, realm, clientID, username, password)
	if err != nil {
		h.logger.Error("login failed",
			slog.String("username", username),
			slog.String("realm", realm),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}

	h.logger.Info("user logged in", slog.String("username", username), slog.String("realm", realm))
	SetAuthCookies(w, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.IDToken, tokenResp.ExpiresIn)
	h.respondJSON(w, http.StatusOK, tokenResp)
}

// Refresh handles POST /auth/refresh and /auth/{realm}/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	realm, clientID := h.resolveRealm(r)

	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}
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

	tokenResp, err := h.keycloakService.RefreshToken(ctx, realm, clientID, refreshToken)
	if err != nil {
		h.logger.Error("token refresh failed", slog.String("realm", realm), slog.String("error", err.Error()))
		h.respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired refresh token")
		return
	}

	h.logger.Info("token refreshed", slog.String("realm", realm))
	SetAuthCookies(w, tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.IDToken, tokenResp.ExpiresIn)
	h.respondJSON(w, http.StatusOK, tokenResp)
}

// Logout handles POST /auth/logout and /auth/{realm}/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	realm, clientID := h.resolveRealm(r)

	refreshToken := ""
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		refreshToken = cookie.Value
	}
	if refreshToken == "" {
		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}
	if refreshToken == "" {
		h.logger.Info("logout called with no refresh token")
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.keycloakService.Logout(ctx, realm, clientID, refreshToken); err != nil {
		h.logger.Error("logout failed", slog.String("realm", realm), slog.String("error", err.Error()))
	}

	ClearAuthCookies(w)
	h.logger.Info("user logged out", slog.String("realm", realm))
	w.WriteHeader(http.StatusOK)
}

// GetUser handles GET /auth/user and /auth/{realm}/user
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	realm, _ := h.resolveRealm(r)

	accessToken := ""
	if cookie, err := r.Cookie("access_token"); err == nil {
		accessToken = cookie.Value
	}
	if accessToken == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				accessToken = authHeader[7:]
			} else {
				accessToken = authHeader
			}
		}
	}
	if accessToken == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "No access token provided (cookie or Authorization header)")
		return
	}

	userInfo, err := h.keycloakService.GetUserInfo(ctx, realm, accessToken)
	if err != nil {
		h.logger.Error("failed to get user info", slog.String("realm", realm), slog.String("error", err.Error()))
		h.respondError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired access token")
		return
	}

	h.logger.Debug("user info retrieved", slog.String("user_id", userInfo.Sub), slog.String("realm", realm))
	h.respondJSON(w, http.StatusOK, userInfo)
}

// Register handles POST /auth/register and /auth/{realm}/register (also POST /auth/user and /auth/{realm}/user)
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	realm, _ := h.resolveRealm(r)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		h.respondError(w, http.StatusBadRequest, "invalid_request", "Username, email, and password are required")
		return
	}
	if len(req.Password) < 8 {
		h.respondError(w, http.StatusBadRequest, "weak_password", "Password must be at least 8 characters long")
		return
	}

	userID, err := h.keycloakService.CreateUser(ctx, realm, req.Username, req.Email, req.Password, req.FirstName, req.LastName)
	if err != nil {
		h.logger.Error("user registration failed",
			slog.String("username", req.Username),
			slog.String("realm", realm),
			slog.String("error", err.Error()),
		)
		h.respondError(w, http.StatusBadRequest, "registration_failed", "Failed to create user: "+err.Error())
		return
	}

	h.logger.Info("user registered",
		slog.String("username", req.Username),
		slog.String("user_id", userID),
		slog.String("realm", realm),
	)

	userInfo := map[string]any{
		"sub":                userID,
		"preferred_username": req.Username,
		"email":              req.Email,
		"name":               strings.TrimSpace(req.FirstName + " " + req.LastName),
		"given_name":         req.FirstName,
		"family_name":        req.LastName,
	}

	h.respondJSON(w, http.StatusCreated, userInfo)
}

// UpdateUser handles PUT /auth/user
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	h.respondError(w, http.StatusNotImplemented, "not_implemented", "User update is not yet implemented")
}

// DeleteUser handles DELETE /auth/user
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	h.respondError(w, http.StatusNotImplemented, "not_implemented", "User deletion is not yet implemented")
}

// Helper methods

func (h *AuthHandler) respondJSON(w http.ResponseWriter, status int, data any) {
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

// SetAuthCookies sets authentication cookies
func SetAuthCookies(w http.ResponseWriter, accessToken, refreshToken, idToken string, expiresIn int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Domain:   "localhost",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiresIn,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		Domain:   "localhost",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "id_token",
		Value:    idToken,
		Path:     "/",
		Domain:   "localhost",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   expiresIn,
	})
}

// ClearAuthCookies clears authentication cookies
func ClearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"access_token", "refresh_token", "id_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
}
