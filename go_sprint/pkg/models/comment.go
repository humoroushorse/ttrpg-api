package models

import (
	"time"

	"github.com/google/uuid"
)

// Comment represents a comment on a work item
type Comment struct {
	ID         uuid.UUID    `json:"id"`
	WorkItemID uuid.UUID    `json:"work_item_id"`
	AuthorID   uuid.UUID    `json:"author_id"`
	Content    string       `json:"content"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	DeletedAt  *interface{} `json:"deleted_at,omitempty"`
	DeletedBy  *uuid.UUID   `json:"deleted_by,omitempty"`
}

// ActivityLog represents an activity log entry
type ActivityLog struct {
	ID         uuid.UUID              `json:"id"`
	EntityType string                 `json:"entity_type"`
	EntityID   uuid.UUID              `json:"entity_id"`
	Action     string                 `json:"action"`
	UserID     uuid.UUID              `json:"user_id"`
	Changes    map[string]interface{} `json:"changes,omitempty"`
	TraceID    *uuid.UUID             `json:"trace_id,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}
