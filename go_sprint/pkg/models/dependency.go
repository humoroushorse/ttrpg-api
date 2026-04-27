package models

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// DependencyType represents the type of dependency between work items
type DependencyType string

const (
	DependencyTypeBlocks      DependencyType = "blocks"
	DependencyTypeIsBlockedBy DependencyType = "is_blocked_by"
	DependencyTypeRelatesTo   DependencyType = "relates_to"
	DependencyTypeDuplicates  DependencyType = "duplicates"
)

// WorkItemDependency represents a dependency relationship between two work items
// Implements slog.LogValuer for safe logging
type WorkItemDependency struct {
	ID             uuid.UUID      `json:"id"`
	SourceID       uuid.UUID      `json:"source_id"`
	TargetID       uuid.UUID      `json:"target_id"`
	DependencyType DependencyType `json:"dependency_type"`
	CreatedAt      time.Time      `json:"created_at"`
	CreatedBy      uuid.UUID      `json:"created_by"`
}

// LogValue implements slog.LogValuer to control logging output
func (d WorkItemDependency) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", d.ID.String()),
		slog.String("source_id", d.SourceID.String()),
		slog.String("target_id", d.TargetID.String()),
		slog.String("dependency_type", string(d.DependencyType)),
		slog.String("created_by", d.CreatedBy.String()),
	)
}
