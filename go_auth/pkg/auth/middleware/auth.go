package middleware

import (
	"net/http"
)

// RequireRole creates middleware that requires the user to have a specific role
func RequireRole(validator *JWTValidator, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUser(r.Context())
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if !user.HasRole(role) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole creates middleware that requires the user to have any of the specified roles
func RequireAnyRole(validator *JWTValidator, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUser(r.Context())
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if !user.HasAnyRole(roles...) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAllRoles creates middleware that requires the user to have all of the specified roles
func RequireAllRoles(validator *JWTValidator, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := GetUser(r.Context())
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if !user.HasAllRoles(roles...) {
				writeError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates middleware that extracts user information if present but doesn't require it
func OptionalAuth(validator *JWTValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to extract token
			token, err := extractToken(r)
			if err != nil {
				// No token present, continue without user
				next.ServeHTTP(w, r)
				return
			}

			// Try to validate token
			user, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				// Invalid token, continue without user
				next.ServeHTTP(w, r)
				return
			}

			// Add user to context
			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
