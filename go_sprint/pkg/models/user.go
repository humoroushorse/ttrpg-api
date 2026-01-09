package models

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
// Implements slog.LogValuer to control what gets logged
type User struct {
	ID             uuid.UUID `json:"id"`
	KeycloakID     string    `json:"keycloak_id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	ProfilePicture string    `json:"profile_picture_url"`
	Roles          []string  `json:"roles"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Sensitive fields that should not be logged
	Password     string `json:"-"`
	APIKey       string `json:"-"`
	RefreshToken string `json:"-"`
}

// LogValue implements slog.LogValuer to control logging output
// This prevents sensitive data like email, password, and tokens from being logged
func (u User) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("username", u.Username),
		slog.Bool("is_active", u.IsActive),
	}

	// Only include ID if it's not zero
	if u.ID != uuid.Nil {
		attrs = append([]slog.Attr{slog.String("id", u.ID.String())}, attrs...)
	}

	return slog.GroupValue(attrs...)
	// Deliberately exclude email, password, API key, and other sensitive data
}
