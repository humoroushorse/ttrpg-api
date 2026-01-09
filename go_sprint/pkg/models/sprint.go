package models

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// SprintStatus represents the status of a sprint
type SprintStatus string

const (
	SprintStatusPlanned   SprintStatus = "planned"
	SprintStatusActive    SprintStatus = "active"
	SprintStatusCompleted SprintStatus = "completed"
	SprintStatusCancelled SprintStatus = "cancelled"
)

// Sprint represents a sprint
// Implements slog.LogValuer for safe logging
type Sprint struct {
	ID              uuid.UUID    `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	Status          SprintStatus `json:"status"`
	StartDate       time.Time    `json:"start_date"`
	EndDate         time.Time    `json:"end_date"`
	CapacityPoints  *int         `json:"capacity_points,omitempty"`
	CommittedPoints int          `json:"committed_points"`
	CompletedPoints int          `json:"completed_points"`
	CreatedBy       uuid.UUID    `json:"created_by"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	DeletedAt       *time.Time   `json:"deleted_at,omitempty"`
	DeletedBy       *uuid.UUID   `json:"deleted_by,omitempty"`
}

// LogValue implements slog.LogValuer to control logging output
func (s Sprint) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", s.ID.String()),
		slog.String("name", s.Name),
		slog.String("status", string(s.Status)),
		slog.Time("start_date", s.StartDate),
		slog.Time("end_date", s.EndDate),
		slog.Int("committed_points", s.CommittedPoints),
		slog.Int("completed_points", s.CompletedPoints),
	}

	if s.CapacityPoints != nil {
		attrs = append(attrs, slog.Int("capacity_points", *s.CapacityPoints))
	}

	return slog.GroupValue(attrs...)
}
