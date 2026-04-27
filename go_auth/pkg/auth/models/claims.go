package models

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the claims in a JWT token from Keycloak
type JWTClaims struct {
	jwt.RegisteredClaims
	Email             string             `json:"email,omitempty"`
	EmailVerified     bool               `json:"email_verified,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
	GivenName         string             `json:"given_name,omitempty"`
	FamilyName        string             `json:"family_name,omitempty"`
	Name              string             `json:"name,omitempty"`
	RealmAccess       *Access            `json:"realm_access,omitempty"`
	ResourceAccess    map[string]*Access `json:"resource_access,omitempty"`
}

// Access represents the access roles in Keycloak JWT
type Access struct {
	Roles []string `json:"roles"`
}

// ToUser converts JWT claims to a User model
func (c *JWTClaims) ToUser() (*User, error) {
	// Parse user ID from subject claim
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		// If subject is not a UUID, generate a new one
		// This handles cases where Keycloak uses non-UUID subjects
		userID = uuid.New()
	}

	// Extract roles from realm access
	roles := []string{}
	if c.RealmAccess != nil {
		roles = c.RealmAccess.Roles
	}

	return &User{
		ID:         userID,
		KeycloakID: c.Subject,
		Username:   c.PreferredUsername,
		Email:      c.Email,
		FirstName:  c.GivenName,
		LastName:   c.FamilyName,
		Roles:      roles,
		IsActive:   true,
	}, nil
}

// GetRoles returns all roles from realm and resource access
func (c *JWTClaims) GetRoles() []string {
	roles := []string{}

	// Add realm roles
	if c.RealmAccess != nil {
		roles = append(roles, c.RealmAccess.Roles...)
	}

	// Add resource roles
	for _, access := range c.ResourceAccess {
		if access != nil {
			roles = append(roles, access.Roles...)
		}
	}

	return roles
}

// HasRole checks if the claims contain a specific role
func (c *JWTClaims) HasRole(role string) bool {
	roles := c.GetRoles()
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
