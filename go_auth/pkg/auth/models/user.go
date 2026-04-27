package models

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system with secure logging support
type User struct {
	ID                uuid.UUID `json:"id"`
	KeycloakID        string    `json:"keycloak_id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	FirstName         string    `json:"first_name,omitempty"`
	LastName          string    `json:"last_name,omitempty"`
	ProfilePictureURL string    `json:"profile_picture_url,omitempty"`
	Roles             []string  `json:"roles"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// LogValue implements slog.LogValuer to control logging output
// This prevents sensitive data like email from being logged
func (u User) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", u.ID.String()),
		slog.String("username", u.Username),
		slog.String("keycloak_id", u.KeycloakID),
		slog.Bool("is_active", u.IsActive),
		// Deliberately exclude email and other potentially sensitive data
	)
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (u *User) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles
func (u *User) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !u.HasRole(role) {
			return false
		}
	}
	return true
}
