package models

import (
	"time"

	"github.com/google/uuid"
)

// Project represents a project for organizing work items
type Project struct {
	ID             uuid.UUID `json:"id" db:"id"`
	Key            string    `json:"key" db:"key"`
	Name           string    `json:"name" db:"name"`
	Description    string    `json:"description" db:"description"`
	CurrentCounter int       `json:"current_counter" db:"current_counter"`
	StartingNumber int       `json:"starting_number" db:"starting_number"`
	CreatedBy      string    `json:"created_by" db:"created_by"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedBy      string    `json:"updated_by" db:"updated_by"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Key            string `json:"key" validate:"required,min=3,max=10,uppercase,alphanum"`
	Name           string `json:"name" validate:"required,min=1,max=255"`
	Description    string `json:"description"`
	StartingNumber int    `json:"starting_number" validate:"min=1"`
}

// UpdateProjectRequest represents a request to update a project
type UpdateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=255"`
	Description string `json:"description"`
}
