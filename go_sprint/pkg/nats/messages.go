package nats

import (
	"time"

	"github.com/google/uuid"
)

// Common message types for sprint service

// WorkItemMessage represents a work item in NATS messages
type WorkItemMessage struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	StoryPoints *int      `json:"story_points,omitempty"`
	AssigneeID  *string   `json:"assignee_id,omitempty"`
	ReporterID  string    `json:"reporter_id"`
	ParentID    *string   `json:"parent_id,omitempty"`
	SprintID    *string   `json:"sprint_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SprintMessage represents a sprint in NATS messages
type SprintMessage struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	Status          string    `json:"status"`
	StartDate       string    `json:"start_date"`
	EndDate         string    `json:"end_date"`
	CapacityPoints  *int      `json:"capacity_points,omitempty"`
	CommittedPoints int       `json:"committed_points"`
	CompletedPoints int       `json:"completed_points"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ErrorMessage represents an error response
type ErrorMessage struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// CreateWorkItemRequest represents a request to create a work item
type CreateWorkItemRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	StoryPoints *int    `json:"story_points,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	ReporterID  string  `json:"reporter_id"`
	ParentID    *string `json:"parent_id,omitempty"`
	SprintID    *string `json:"sprint_id,omitempty"`
}

// UpdateWorkItemRequest represents a request to update a work item
type UpdateWorkItemRequest struct {
	ID          string  `json:"id"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	StoryPoints *int    `json:"story_points,omitempty"`
	AssigneeID  *string `json:"assignee_id,omitempty"`
	SprintID    *string `json:"sprint_id,omitempty"`
}

// CreateSprintRequest represents a request to create a sprint
type CreateSprintRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	CapacityPoints *int   `json:"capacity_points,omitempty"`
	CreatedBy      string `json:"created_by"`
}

// UpdateSprintRequest represents a request to update a sprint
type UpdateSprintRequest struct {
	ID             string  `json:"id"`
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	Status         *string `json:"status,omitempty"`
	StartDate      *string `json:"start_date,omitempty"`
	EndDate        *string `json:"end_date,omitempty"`
	CapacityPoints *int    `json:"capacity_points,omitempty"`
}

// CloseSprintRequest represents a request to close a sprint
type CloseSprintRequest struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
}

// NotificationMessage represents a notification event
type NotificationMessage struct {
	EventType  string                 `json:"event_type"`
	EntityID   string                 `json:"entity_id"`
	EntityType string                 `json:"entity_type"`
	UserID     string                 `json:"user_id"`
	Changes    map[string]interface{} `json:"changes,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Helper functions to create messages

// NewWorkItemMessage creates a new work item message
func NewWorkItemMessage(id, itemType, title, description, status, priority, reporterID string) *WorkItemMessage {
	return &WorkItemMessage{
		ID:          id,
		Type:        itemType,
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		ReporterID:  reporterID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
}

// NewSprintMessage creates a new sprint message
func NewSprintMessage(id, name, description, status, startDate, endDate, createdBy string) *SprintMessage {
	return &SprintMessage{
		ID:              id,
		Name:            name,
		Description:     description,
		Status:          status,
		StartDate:       startDate,
		EndDate:         endDate,
		CommittedPoints: 0,
		CompletedPoints: 0,
		CreatedBy:       createdBy,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
}

// NewErrorMessage creates a new error message
func NewErrorMessage(code, message string) *ErrorMessage {
	return &ErrorMessage{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// NewNotificationMessage creates a new notification message
func NewNotificationMessage(eventType, entityID, entityType, userID string) *NotificationMessage {
	return &NotificationMessage{
		EventType:  eventType,
		EntityID:   entityID,
		EntityType: entityType,
		UserID:     userID,
		Changes:    make(map[string]interface{}),
		Timestamp:  time.Now().UTC(),
	}
}

// Response wrapper for standardized responses
type Response struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   *ErrorMessage          `json:"error,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(data map[string]interface{}) *Response {
	return &Response{
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(code, message string) *Response {
	return &Response{
		Success: false,
		Error:   NewErrorMessage(code, message),
	}
}

// Validation helpers

// ValidateWorkItemType validates work item type
func ValidateWorkItemType(itemType string) bool {
	validTypes := map[string]bool{
		"epic":   true,
		"story":  true,
		"defect": true,
	}
	return validTypes[itemType]
}

// ValidateWorkItemStatus validates work item status
func ValidateWorkItemStatus(status string) bool {
	validStatuses := map[string]bool{
		"todo":        true,
		"in_progress": true,
		"in_review":   true,
		"done":        true,
		"blocked":     true,
	}
	return validStatuses[status]
}

// ValidatePriority validates priority level
func ValidatePriority(priority string) bool {
	validPriorities := map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	}
	return validPriorities[priority]
}

// ValidateSprintStatus validates sprint status
func ValidateSprintStatus(status string) bool {
	validStatuses := map[string]bool{
		"planned":   true,
		"active":    true,
		"completed": true,
		"cancelled": true,
	}
	return validStatuses[status]
}

// GenerateID generates a new UUID string
func GenerateID() string {
	return uuid.New().String()
}
