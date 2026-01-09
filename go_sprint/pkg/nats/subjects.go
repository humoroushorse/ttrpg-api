package nats

import (
	"fmt"
)

// Subject pattern constants for sprint service
const (
	ServiceName = "sprint"
)

// SubjectBuilder helps construct standardized NATS subjects
type SubjectBuilder struct {
	service  string
	traceID  string
	resource string
	action   string
	status   string
}

// NewSubjectBuilder creates a new subject builder
func NewSubjectBuilder() *SubjectBuilder {
	return &SubjectBuilder{
		service: ServiceName,
	}
}

// WithTraceID sets the trace ID
func (sb *SubjectBuilder) WithTraceID(traceID string) *SubjectBuilder {
	sb.traceID = traceID
	return sb
}

// WithResource sets the resource
func (sb *SubjectBuilder) WithResource(resource string) *SubjectBuilder {
	sb.resource = resource
	return sb
}

// WithAction sets the action
func (sb *SubjectBuilder) WithAction(action string) *SubjectBuilder {
	sb.action = action
	return sb
}

// WithStatus sets the status
func (sb *SubjectBuilder) WithStatus(status string) *SubjectBuilder {
	sb.status = status
	return sb
}

// Build constructs the subject string
func (sb *SubjectBuilder) Build() string {
	return fmt.Sprintf("%s.%s.%s.%s.%s",
		sb.service,
		sb.traceID,
		sb.resource,
		sb.action,
		sb.status,
	)
}

// Predefined subject patterns for common operations

// Work Item subjects
func WorkItemCreateRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.create.request", traceID)
}

func WorkItemCreateResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.create.response", traceID)
}

func WorkItemUpdateRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.update.request", traceID)
}

func WorkItemUpdateResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.update.response", traceID)
}

func WorkItemDeleteRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.delete.request", traceID)
}

func WorkItemDeleteResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.delete.response", traceID)
}

func WorkItemGetRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.get.request", traceID)
}

func WorkItemGetResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.workitem.get.response", traceID)
}

// Sprint subjects
func SprintCreateRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.create.request", traceID)
}

func SprintCreateResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.create.response", traceID)
}

func SprintUpdateRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.update.request", traceID)
}

func SprintUpdateResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.update.response", traceID)
}

func SprintCloseRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.close.request", traceID)
}

func SprintCloseResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.close.response", traceID)
}

func SprintGetRequest(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.get.request", traceID)
}

func SprintGetResponse(traceID string) string {
	return fmt.Sprintf("sprint.%s.sprint.get.response", traceID)
}

// Notification subjects
func NotificationWorkItemCreated(traceID string) string {
	return fmt.Sprintf("sprint.%s.notification.workitem.created", traceID)
}

func NotificationWorkItemUpdated(traceID string) string {
	return fmt.Sprintf("sprint.%s.notification.workitem.updated", traceID)
}

func NotificationWorkItemDeleted(traceID string) string {
	return fmt.Sprintf("sprint.%s.notification.workitem.deleted", traceID)
}

func NotificationSprintStatusChanged(traceID string) string {
	return fmt.Sprintf("sprint.%s.notification.sprint.status_changed", traceID)
}

func NotificationSprintClosed(traceID string) string {
	return fmt.Sprintf("sprint.%s.notification.sprint.closed", traceID)
}

// Wildcard patterns for subscriptions

// AllMessagesForTrace returns wildcard pattern for all messages with a specific trace ID
func AllMessagesForTrace(traceID string) string {
	return fmt.Sprintf("sprint.%s.*.*.*", traceID)
}

// AllWorkItemMessages returns wildcard pattern for all work item messages
func AllWorkItemMessages() string {
	return "sprint.*.workitem.*.*"
}

// AllSprintMessages returns wildcard pattern for all sprint messages
func AllSprintMessages() string {
	return "sprint.*.sprint.*.*"
}

// AllNotifications returns wildcard pattern for all notification messages
func AllNotifications() string {
	return "sprint.*.notification.*.*"
}

// AllServiceMessages returns wildcard pattern for all sprint service messages
func AllServiceMessages() string {
	return "sprint.>"
}

// Auth service subjects (for integration)
func AuthValidateRequest(traceID string) string {
	return fmt.Sprintf("auth.%s.validate.request", traceID)
}

func AuthValidateResponse(traceID string) string {
	return fmt.Sprintf("auth.%s.validate.response", traceID)
}

func AuthUserSyncRequest(traceID string) string {
	return fmt.Sprintf("auth.%s.user.sync.request", traceID)
}

func AuthUserSyncResponse(traceID string) string {
	return fmt.Sprintf("auth.%s.user.sync.response", traceID)
}
