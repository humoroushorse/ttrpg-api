package comments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/humoroushorse/go_sprint/internal/repository/comments"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

var (
	// ErrCommentNotFound is returned when a comment is not found
	ErrCommentNotFound = errors.New("comment not found")
	// ErrEmptyContent is returned when comment content is empty
	ErrEmptyContent = errors.New("comment content cannot be empty")
	// ErrUnauthorized is returned when user is not authorized to perform action
	ErrUnauthorized = errors.New("unauthorized to perform this action")
	// ErrActivityLogCreationFailed is returned when activity log creation fails
	ErrActivityLogCreationFailed = errors.New("failed to create activity log")
)

// CommentRepository defines the interface for comment data access
type CommentRepository interface {
	CreateComment(ctx context.Context, params comments.CreateCommentParams) (comments.SprintManagementComment, error)
	GetCommentByID(ctx context.Context, id pgtype.UUID) (comments.SprintManagementComment, error)
	UpdateComment(ctx context.Context, params comments.UpdateCommentParams) (comments.SprintManagementComment, error)
	SoftDeleteComment(ctx context.Context, params comments.SoftDeleteCommentParams) error
	RestoreComment(ctx context.Context, id pgtype.UUID) error
	ListCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) ([]comments.SprintManagementComment, error)
	CountCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) (int64, error)
	CreateActivityLog(ctx context.Context, params comments.CreateActivityLogParams) (comments.SprintManagementActivityLog, error)
	ListActivityLogsByEntity(ctx context.Context, params comments.ListActivityLogsByEntityParams) ([]comments.SprintManagementActivityLog, error)
	ListActivityLogsByUser(ctx context.Context, params comments.ListActivityLogsByUserParams) ([]comments.SprintManagementActivityLog, error)
	ListRecentActivityLogs(ctx context.Context, params comments.ListRecentActivityLogsParams) ([]comments.SprintManagementActivityLog, error)
}

// Service provides business logic for comments and activity tracking
type Service struct {
	repo   CommentRepository
	logger *slog.Logger
}

// NewService creates a new comments service
func NewService(repo CommentRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	WorkItemID uuid.UUID `json:"work_item_id"`
	AuthorID   uuid.UUID `json:"author_id"`
	Content    string    `json:"content"`
}

// UpdateCommentRequest represents a request to update a comment
type UpdateCommentRequest struct {
	Content string `json:"content"`
}

// ActivityFeedRequest represents a request for activity feed
type ActivityFeedRequest struct {
	EntityType string     `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID `json:"entity_id,omitempty"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Limit      int32      `json:"limit"`
	Offset     int32      `json:"offset"`
}

// CreateComment creates a new comment and logs the activity
func (s *Service) CreateComment(ctx context.Context, req CreateCommentRequest) (*models.Comment, error) {
	logger := s.getLogger(ctx)

	// Validate content
	if err := s.validateContent(req.Content); err != nil {
		logger.Error("validation failed", slog.String("error", err.Error()))
		return nil, err
	}

	// Convert UUIDs to pgtype.UUID
	workItemID := uuidToPgtype(req.WorkItemID)
	authorID := uuidToPgtype(req.AuthorID)

	// Create comment
	params := comments.CreateCommentParams{
		WorkItemID: workItemID,
		AuthorID:   authorID,
		Content:    req.Content,
	}

	comment, err := s.repo.CreateComment(ctx, params)
	if err != nil {
		logger.Error("failed to create comment", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	// Log activity
	if err := s.logActivity(ctx, "comment", pgtypeToUUID(comment.ID), "commented", req.AuthorID, nil); err != nil {
		logger.Warn("failed to log activity", slog.String("error", err.Error()))
	}

	logger.Info("comment created",
		slog.String("comment_id", pgtypeToUUID(comment.ID).String()),
		slog.String("work_item_id", req.WorkItemID.String()),
	)

	return s.toModel(comment), nil
}

// GetComment retrieves a comment by ID
func (s *Service) GetComment(ctx context.Context, id uuid.UUID) (*models.Comment, error) {
	logger := s.getLogger(ctx)

	comment, err := s.repo.GetCommentByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get comment", slog.String("error", err.Error()))
		return nil, ErrCommentNotFound
	}

	return s.toModel(comment), nil
}

// UpdateComment updates a comment and logs the activity
func (s *Service) UpdateComment(ctx context.Context, id uuid.UUID, req UpdateCommentRequest, userID uuid.UUID) (*models.Comment, error) {
	logger := s.getLogger(ctx)

	// Validate content
	if err := s.validateContent(req.Content); err != nil {
		logger.Error("validation failed", slog.String("error", err.Error()))
		return nil, err
	}

	// Get existing comment to check authorization
	existing, err := s.repo.GetCommentByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get comment", slog.String("error", err.Error()))
		return nil, ErrCommentNotFound
	}

	// Check if user is the author
	if pgtypeToUUID(existing.AuthorID) != userID {
		logger.Warn("unauthorized update attempt",
			slog.String("comment_id", id.String()),
			slog.String("user_id", userID.String()),
		)
		return nil, ErrUnauthorized
	}

	// Update comment
	params := comments.UpdateCommentParams{
		ID:      uuidToPgtype(id),
		Content: req.Content,
	}

	comment, err := s.repo.UpdateComment(ctx, params)
	if err != nil {
		logger.Error("failed to update comment", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	// Log activity with changes
	changes := map[string]interface{}{
		"before": existing.Content,
		"after":  req.Content,
	}
	if err := s.logActivity(ctx, "comment", id, "updated", userID, changes); err != nil {
		logger.Warn("failed to log activity", slog.String("error", err.Error()))
	}

	logger.Info("comment updated", slog.String("comment_id", id.String()))

	return s.toModel(comment), nil
}

// DeleteComment soft deletes a comment and logs the activity
func (s *Service) DeleteComment(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	logger := s.getLogger(ctx)

	// Get existing comment to check authorization
	existing, err := s.repo.GetCommentByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get comment", slog.String("error", err.Error()))
		return ErrCommentNotFound
	}

	// Check if user is the author
	if pgtypeToUUID(existing.AuthorID) != userID {
		logger.Warn("unauthorized delete attempt",
			slog.String("comment_id", id.String()),
			slog.String("user_id", userID.String()),
		)
		return ErrUnauthorized
	}

	// Soft delete comment
	params := comments.SoftDeleteCommentParams{
		ID:        uuidToPgtype(id),
		DeletedBy: uuidToPgtype(userID),
	}

	if err := s.repo.SoftDeleteComment(ctx, params); err != nil {
		logger.Error("failed to delete comment", slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	// Log activity
	if err := s.logActivity(ctx, "comment", id, "deleted", userID, nil); err != nil {
		logger.Warn("failed to log activity", slog.String("error", err.Error()))
	}

	logger.Info("comment deleted", slog.String("comment_id", id.String()))

	return nil
}

// ListCommentsByWorkItem retrieves all comments for a work item
func (s *Service) ListCommentsByWorkItem(ctx context.Context, workItemID uuid.UUID) ([]*models.Comment, error) {
	logger := s.getLogger(ctx)

	commentList, err := s.repo.ListCommentsByWorkItem(ctx, uuidToPgtype(workItemID))
	if err != nil {
		logger.Error("failed to list comments", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list comments: %w", err)
	}

	result := make([]*models.Comment, len(commentList))
	for i, c := range commentList {
		result[i] = s.toModel(c)
	}

	return result, nil
}

// GetActivityFeed retrieves activity logs based on filters
func (s *Service) GetActivityFeed(ctx context.Context, req ActivityFeedRequest) ([]*models.ActivityLog, error) {
	logger := s.getLogger(ctx)

	var activityLogs []comments.SprintManagementActivityLog
	var err error

	// Determine which query to use based on filters
	if req.EntityType != "" && req.EntityID != nil {
		params := comments.ListActivityLogsByEntityParams{
			EntityType: req.EntityType,
			EntityID:   uuidToPgtype(*req.EntityID),
			Limit:      req.Limit,
			Offset:     req.Offset,
		}
		activityLogs, err = s.repo.ListActivityLogsByEntity(ctx, params)
	} else if req.UserID != nil {
		params := comments.ListActivityLogsByUserParams{
			UserID: uuidToPgtype(*req.UserID),
			Limit:  req.Limit,
			Offset: req.Offset,
		}
		activityLogs, err = s.repo.ListActivityLogsByUser(ctx, params)
	} else {
		params := comments.ListRecentActivityLogsParams{
			Limit:  req.Limit,
			Offset: req.Offset,
		}
		activityLogs, err = s.repo.ListRecentActivityLogs(ctx, params)
	}

	if err != nil {
		logger.Error("failed to get activity feed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get activity feed: %w", err)
	}

	result := make([]*models.ActivityLog, len(activityLogs))
	for i, log := range activityLogs {
		result[i] = s.activityLogToModel(log)
	}

	return result, nil
}

// LogWorkItemActivity logs activity for work item changes
func (s *Service) LogWorkItemActivity(ctx context.Context, entityID uuid.UUID, action string, userID uuid.UUID, changes map[string]interface{}) error {
	return s.logActivity(ctx, "work_item", entityID, action, userID, changes)
}

// LogSprintActivity logs activity for sprint changes
func (s *Service) LogSprintActivity(ctx context.Context, entityID uuid.UUID, action string, userID uuid.UUID, changes map[string]interface{}) error {
	return s.logActivity(ctx, "sprint", entityID, action, userID, changes)
}

// logActivity creates an activity log entry
func (s *Service) logActivity(ctx context.Context, entityType string, entityID uuid.UUID, action string, userID uuid.UUID, changes map[string]interface{}) error {
	logger := s.getLogger(ctx)

	// Convert changes to JSON
	var changesJSON []byte
	var err error
	if changes != nil {
		changesJSON, err = json.Marshal(changes)
		if err != nil {
			logger.Error("failed to marshal changes", slog.String("error", err.Error()))
			return fmt.Errorf("failed to marshal changes: %w", err)
		}
	}

	// Get trace ID from context if available
	traceID := s.getTraceID(ctx)

	params := comments.CreateActivityLogParams{
		EntityType: entityType,
		EntityID:   uuidToPgtype(entityID),
		Action:     action,
		UserID:     uuidToPgtype(userID),
		Changes:    changesJSON,
		TraceID:    uuidToPgtype(traceID),
	}

	_, err = s.repo.CreateActivityLog(ctx, params)
	if err != nil {
		logger.Error("failed to create activity log", slog.String("error", err.Error()))
		return ErrActivityLogCreationFailed
	}

	return nil
}

// validateContent validates comment content
func (s *Service) validateContent(content string) error {
	if len(content) == 0 {
		return ErrEmptyContent
	}
	return nil
}

// toModel converts repository comment to domain model
func (s *Service) toModel(c comments.SprintManagementComment) *models.Comment {
	return &models.Comment{
		ID:         pgtypeToUUID(c.ID),
		WorkItemID: pgtypeToUUID(c.WorkItemID),
		AuthorID:   pgtypeToUUID(c.AuthorID),
		Content:    c.Content,
		CreatedAt:  c.CreatedAt.Time,
		UpdatedAt:  c.UpdatedAt.Time,
		DeletedAt:  pgtypeToTimePtr(c.DeletedAt),
		DeletedBy:  pgtypeToUUIDPtr(c.DeletedBy),
	}
}

// activityLogToModel converts repository activity log to domain model
func (s *Service) activityLogToModel(log comments.SprintManagementActivityLog) *models.ActivityLog {
	var changes map[string]interface{}
	if len(log.Changes) > 0 {
		_ = json.Unmarshal(log.Changes, &changes)
	}

	return &models.ActivityLog{
		ID:         pgtypeToUUID(log.ID),
		EntityType: log.EntityType,
		EntityID:   pgtypeToUUID(log.EntityID),
		Action:     log.Action,
		UserID:     pgtypeToUUID(log.UserID),
		Changes:    changes,
		TraceID:    pgtypeToUUIDPtr(log.TraceID),
		CreatedAt:  log.CreatedAt.Time,
	}
}

// getLogger retrieves logger from context or returns default
func (s *Service) getLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	return s.logger
}

// getTraceID retrieves trace ID from context
func (s *Service) getTraceID(ctx context.Context) uuid.UUID {
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		if id, err := uuid.Parse(traceID); err == nil {
			return id
		}
	}
	return uuid.New()
}

// Helper functions for UUID conversion
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

func pgtypeToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return id.Bytes
}

func pgtypeToUUIDPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	uid := uuid.UUID(id.Bytes)
	return &uid
}

func pgtypeToTimePtr(ts pgtype.Timestamptz) *interface{} {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	var result interface{} = t
	return &result
}
