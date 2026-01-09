package models

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// WorkItemType represents the type of work item
type WorkItemType string

const (
	WorkItemTypeEpic   WorkItemType = "epic"
	WorkItemTypeStory  WorkItemType = "story"
	WorkItemTypeDefect WorkItemType = "defect"
)

// WorkItemStatus represents the status of a work item
type WorkItemStatus string

const (
	WorkItemStatusTodo       WorkItemStatus = "todo"
	WorkItemStatusInProgress WorkItemStatus = "in_progress"
	WorkItemStatusInReview   WorkItemStatus = "in_review"
	WorkItemStatusDone       WorkItemStatus = "done"
	WorkItemStatusBlocked    WorkItemStatus = "blocked"
)

// PriorityLevel represents the priority of a work item
type PriorityLevel string

const (
	PriorityLow      PriorityLevel = "low"
	PriorityMedium   PriorityLevel = "medium"
	PriorityHigh     PriorityLevel = "high"
	PriorityCritical PriorityLevel = "critical"
)

// WorkItem represents a work item (epic, story, or defect)
// Implements slog.LogValuer for safe logging
type WorkItem struct {
	ID          uuid.UUID      `json:"id"`
	Type        WorkItemType   `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      WorkItemStatus `json:"status"`
	Priority    PriorityLevel  `json:"priority"`
	StoryPoints *int           `json:"story_points,omitempty"`
	AssigneeID  *uuid.UUID     `json:"assignee_id,omitempty"`
	ReporterID  uuid.UUID      `json:"reporter_id"`
	ParentID    *uuid.UUID     `json:"parent_id,omitempty"`
	SprintID    *uuid.UUID     `json:"sprint_id,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   *time.Time     `json:"deleted_at,omitempty"`
	DeletedBy   *uuid.UUID     `json:"deleted_by,omitempty"`
}

// LogValue implements slog.LogValuer to control logging output
func (w WorkItem) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("id", w.ID.String()),
		slog.String("type", string(w.Type)),
		slog.String("title", w.Title),
		slog.String("status", string(w.Status)),
		slog.String("priority", string(w.Priority)),
	}

	if w.AssigneeID != nil {
		attrs = append(attrs, slog.String("assignee_id", w.AssigneeID.String()))
	}
	if w.SprintID != nil {
		attrs = append(attrs, slog.String("sprint_id", w.SprintID.String()))
	}
	if w.StoryPoints != nil {
		attrs = append(attrs, slog.Int("story_points", *w.StoryPoints))
	}

	return slog.GroupValue(attrs...)
}
