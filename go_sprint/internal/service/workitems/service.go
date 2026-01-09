package workitems

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/humoroushorse/go_sprint/pkg/pagination"
)

var (
	// ErrWorkItemNotFound is returned when a work item is not found
	ErrWorkItemNotFound = errors.New("work item not found")
	// ErrInvalidWorkItemType is returned when an invalid work item type is provided
	ErrInvalidWorkItemType = errors.New("invalid work item type")
	// ErrInvalidParentType is returned when parent-child relationship is invalid
	ErrInvalidParentType = errors.New("invalid parent-child relationship")
	// ErrInvalidStatusTransition is returned when a status transition is not allowed
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	// ErrWorkItemHasDependencies is returned when trying to delete a work item with dependencies
	ErrWorkItemHasDependencies = errors.New("work item has dependencies or children")
	// ErrRequiredFieldMissing is returned when a required field is missing
	ErrRequiredFieldMissing = errors.New("required field missing")
)

// WorkItemRepository defines the interface for work item data access
type WorkItemRepository interface {
	CreateWorkItem(ctx context.Context, params workitems.CreateWorkItemParams) (workitems.SprintManagementWorkItem, error)
	GetWorkItemByID(ctx context.Context, id pgtype.UUID) (workitems.SprintManagementWorkItem, error)
	UpdateWorkItem(ctx context.Context, params workitems.UpdateWorkItemParams) (workitems.SprintManagementWorkItem, error)
	ListWorkItems(ctx context.Context, params workitems.ListWorkItemsParams) ([]workitems.SprintManagementWorkItem, error)
	ListWorkItemsByType(ctx context.Context, params workitems.ListWorkItemsByTypeParams) ([]workitems.SprintManagementWorkItem, error)
	ListWorkItemsByStatus(ctx context.Context, params workitems.ListWorkItemsByStatusParams) ([]workitems.SprintManagementWorkItem, error)
	HasDependencies(ctx context.Context, id pgtype.UUID) (bool, error)
	HasChildren(ctx context.Context, id pgtype.UUID) (bool, error)
	SoftDeleteWorkItem(ctx context.Context, params workitems.SoftDeleteWorkItemParams) error
}

// Service provides business logic for work item operations
type Service struct {
	repo   WorkItemRepository
	logger *slog.Logger
}

// NewService creates a new work item service
func NewService(repo WorkItemRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// CreateWorkItemRequest represents a request to create a work item
type CreateWorkItemRequest struct {
	Type        models.WorkItemType  `json:"type"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Priority    models.PriorityLevel `json:"priority"`
	StoryPoints *int                 `json:"story_points,omitempty"`
	AssigneeID  *uuid.UUID           `json:"assignee_id,omitempty"`
	ReporterID  uuid.UUID            `json:"reporter_id"`
	ParentID    *uuid.UUID           `json:"parent_id,omitempty"`
	SprintID    *uuid.UUID           `json:"sprint_id,omitempty"`
}

// UpdateWorkItemRequest represents a request to update a work item
type UpdateWorkItemRequest struct {
	Title       *string                `json:"title,omitempty"`
	Description *string                `json:"description,omitempty"`
	Status      *models.WorkItemStatus `json:"status,omitempty"`
	Priority    *models.PriorityLevel  `json:"priority,omitempty"`
	StoryPoints *int                   `json:"story_points,omitempty"`
	AssigneeID  *uuid.UUID             `json:"assignee_id,omitempty"`
	SprintID    *uuid.UUID             `json:"sprint_id,omitempty"`
}

// CreateWorkItem creates a new work item with validation
func (s *Service) CreateWorkItem(ctx context.Context, req CreateWorkItemRequest) (*models.WorkItem, error) {
	logger := s.getLogger(ctx)

	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("validation failed", slog.String("error", err.Error()))
		return nil, err
	}

	// Validate parent-child relationship if parent is specified
	if req.ParentID != nil {
		if err := s.validateParentChildRelationship(ctx, req.Type, *req.ParentID); err != nil {
			logger.Error("parent-child validation failed", slog.String("error", err.Error()))
			return nil, err
		}
	}

	// Convert to repository params
	params := workitems.CreateWorkItemParams{
		Type:       workitems.SprintManagementWorkItemType(req.Type),
		Title:      req.Title,
		Priority:   workitems.SprintManagementPriorityLevel(req.Priority),
		Status:     workitems.SprintManagementWorkItemStatusTodo, // Default status
		ReporterID: uuidToPgtype(req.ReporterID),
	}

	if req.Description != "" {
		params.Description = &req.Description
	}
	if req.StoryPoints != nil {
		sp := int32(*req.StoryPoints)
		params.StoryPoints = &sp
	}
	if req.AssigneeID != nil {
		params.AssigneeID = uuidToPgtype(*req.AssigneeID)
	}
	if req.ParentID != nil {
		params.ParentID = uuidToPgtype(*req.ParentID)
	}
	if req.SprintID != nil {
		params.SprintID = uuidToPgtype(*req.SprintID)
	}

	// Create work item
	workItem, err := s.repo.CreateWorkItem(ctx, params)
	if err != nil {
		logger.Error("failed to create work item", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create work item: %w", err)
	}

	result := s.toModel(workItem)
	logger.Info("work item created", slog.Any("work_item", result))

	return result, nil
}

// GetWorkItem retrieves a work item by ID
func (s *Service) GetWorkItem(ctx context.Context, id uuid.UUID) (*models.WorkItem, error) {
	logger := s.getLogger(ctx)

	workItem, err := s.repo.GetWorkItemByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get work item", slog.String("id", id.String()), slog.String("error", err.Error()))
		return nil, ErrWorkItemNotFound
	}

	result := s.toModel(workItem)
	logger.Info("work item retrieved", slog.Any("work_item", result))

	return result, nil
}

// UpdateWorkItem updates a work item with validation
func (s *Service) UpdateWorkItem(ctx context.Context, id uuid.UUID, req UpdateWorkItemRequest) (*models.WorkItem, error) {
	logger := s.getLogger(ctx)

	// Get existing work item
	existing, err := s.repo.GetWorkItemByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("work item not found", slog.String("id", id.String()))
		return nil, ErrWorkItemNotFound
	}

	// Validate status transition if status is being updated
	if req.Status != nil {
		if err := s.validateStatusTransition(ctx, existing, *req.Status); err != nil {
			logger.Error("invalid status transition", slog.String("error", err.Error()))
			return nil, err
		}
	}

	// Build update params
	params := workitems.UpdateWorkItemParams{
		ID: uuidToPgtype(id),
	}

	// Type (use existing, cannot be changed)
	params.Type = workitems.NullSprintManagementWorkItemType{
		SprintManagementWorkItemType: existing.Type,
		Valid:                        true,
	}

	// Title
	if req.Title != nil {
		params.Title = req.Title
	} else {
		params.Title = &existing.Title
	}

	// Priority
	if req.Priority != nil {
		params.Priority = workitems.NullSprintManagementPriorityLevel{
			SprintManagementPriorityLevel: workitems.SprintManagementPriorityLevel(*req.Priority),
			Valid:                         true,
		}
	} else {
		params.Priority = workitems.NullSprintManagementPriorityLevel{
			SprintManagementPriorityLevel: existing.Priority,
			Valid:                         true,
		}
	}

	// Status
	if req.Status != nil {
		params.Status = workitems.NullSprintManagementWorkItemStatus{
			SprintManagementWorkItemStatus: workitems.SprintManagementWorkItemStatus(*req.Status),
			Valid:                          true,
		}
	} else {
		params.Status = workitems.NullSprintManagementWorkItemStatus{
			SprintManagementWorkItemStatus: existing.Status,
			Valid:                          true,
		}
	}

	// Description
	if req.Description != nil {
		params.Description = req.Description
	} else {
		params.Description = existing.Description
	}

	// Story Points
	if req.StoryPoints != nil {
		sp := int32(*req.StoryPoints)
		params.StoryPoints = &sp
	} else {
		params.StoryPoints = existing.StoryPoints
	}

	// Assignee ID
	if req.AssigneeID != nil {
		params.AssigneeID = uuidToPgtype(*req.AssigneeID)
	} else {
		params.AssigneeID = existing.AssigneeID
	}

	// Sprint ID
	if req.SprintID != nil {
		params.SprintID = uuidToPgtype(*req.SprintID)
	} else {
		params.SprintID = existing.SprintID
	}

	// Parent ID (use existing, cannot be changed via update)
	params.ParentID = existing.ParentID

	// Update work item
	updated, err := s.repo.UpdateWorkItem(ctx, params)
	if err != nil {
		logger.Error("failed to update work item", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update work item: %w", err)
	}

	result := s.toModel(updated)
	logger.Info("work item updated", slog.Any("work_item", result))

	return result, nil
}

// DeleteWorkItem soft deletes a work item after checking for dependencies
func (s *Service) DeleteWorkItem(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	logger := s.getLogger(ctx)

	// Check if work item exists
	_, err := s.repo.GetWorkItemByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("work item not found", slog.String("id", id.String()))
		return ErrWorkItemNotFound
	}

	// Check for dependencies
	hasDeps, err := s.repo.HasDependencies(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to check dependencies", slog.String("error", err.Error()))
		return fmt.Errorf("failed to check dependencies: %w", err)
	}
	if hasDeps {
		logger.Warn("cannot delete work item with dependencies", slog.String("id", id.String()))
		return ErrWorkItemHasDependencies
	}

	// Check for children
	hasChildren, err := s.repo.HasChildren(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to check children", slog.String("error", err.Error()))
		return fmt.Errorf("failed to check children: %w", err)
	}
	if hasChildren {
		logger.Warn("cannot delete work item with children", slog.String("id", id.String()))
		return ErrWorkItemHasDependencies
	}

	// Soft delete
	params := workitems.SoftDeleteWorkItemParams{
		ID:        uuidToPgtype(id),
		DeletedBy: uuidToPgtype(deletedBy),
	}

	if err := s.repo.SoftDeleteWorkItem(ctx, params); err != nil {
		logger.Error("failed to delete work item", slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete work item: %w", err)
	}

	logger.Info("work item deleted", slog.String("id", id.String()))
	return nil
}

// ListWorkItems retrieves a list of work items with optional filters and cursor-based pagination
func (s *Service) ListWorkItems(ctx context.Context, typeFilter *models.WorkItemType, statusFilter *models.WorkItemStatus, cursor *pagination.Cursor, limit int32) ([]*models.WorkItem, error) {
	logger := s.getLogger(ctx)

	// If filters are provided and cursor is not, use the filtered queries
	// Note: The filtered queries don't support cursor pagination, so we only use them without cursor
	if cursor == nil {
		// Type filter only
		if typeFilter != nil && statusFilter == nil {
			items, err := s.repo.ListWorkItemsByType(ctx, workitems.ListWorkItemsByTypeParams{
				Type:   workitems.SprintManagementWorkItemType(*typeFilter),
				Limit:  limit,
				Offset: 0,
			})
			if err != nil {
				logger.Error("failed to list work items by type", slog.String("error", err.Error()))
				return nil, fmt.Errorf("failed to list work items: %w", err)
			}

			result := make([]*models.WorkItem, len(items))
			for i, item := range items {
				result[i] = s.toModel(item)
			}

			logger.Info("work items listed by type", slog.Int("count", len(result)), slog.String("type", string(*typeFilter)))
			return result, nil
		}

		// Status filter only
		if statusFilter != nil && typeFilter == nil {
			items, err := s.repo.ListWorkItemsByStatus(ctx, workitems.ListWorkItemsByStatusParams{
				Status: workitems.SprintManagementWorkItemStatus(*statusFilter),
				Limit:  limit,
				Offset: 0,
			})
			if err != nil {
				logger.Error("failed to list work items by status", slog.String("error", err.Error()))
				return nil, fmt.Errorf("failed to list work items: %w", err)
			}

			result := make([]*models.WorkItem, len(items))
			for i, item := range items {
				result[i] = s.toModel(item)
			}

			logger.Info("work items listed by status", slog.Int("count", len(result)), slog.String("status", string(*statusFilter)))
			return result, nil
		}

		// Both filters - need to filter in memory since we don't have a combined query
		if typeFilter != nil && statusFilter != nil {
			// Get by type first (usually more restrictive)
			items, err := s.repo.ListWorkItemsByType(ctx, workitems.ListWorkItemsByTypeParams{
				Type:   workitems.SprintManagementWorkItemType(*typeFilter),
				Limit:  limit * 2, // Get more to account for filtering
				Offset: 0,
			})
			if err != nil {
				logger.Error("failed to list work items by type", slog.String("error", err.Error()))
				return nil, fmt.Errorf("failed to list work items: %w", err)
			}

			// Filter by status in memory
			result := make([]*models.WorkItem, 0, len(items))
			for _, item := range items {
				if item.Status == workitems.SprintManagementWorkItemStatus(*statusFilter) {
					result = append(result, s.toModel(item))
					if len(result) >= int(limit) {
						break
					}
				}
			}

			logger.Info("work items listed by type and status", slog.Int("count", len(result)), slog.String("type", string(*typeFilter)), slog.String("status", string(*statusFilter)))
			return result, nil
		}
	}

	// No filters or cursor provided - use the cursor-based query
	params := workitems.ListWorkItemsParams{
		Limit:           limit,
		CursorTimestamp: pgtype.Timestamptz{Valid: false},
		CursorID:        pgtype.UUID{Valid: false},
	}

	// If cursor is provided, add cursor filters
	if cursor != nil && cursor.ID != "" {
		// Parse cursor ID
		cursorID, err := uuid.Parse(cursor.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor ID: %w", err)
		}

		// Convert to pgtype.UUID
		pgUUID := pgtype.UUID{
			Bytes: cursorID,
			Valid: true,
		}

		// Convert timestamp to pgtype.Timestamptz
		pgTimestamp := pgtype.Timestamptz{
			Time:  cursor.Timestamp,
			Valid: true,
		}

		// Set cursor parameters for filtering
		params.CursorTimestamp = pgTimestamp
		params.CursorID = pgUUID
	}

	items, err := s.repo.ListWorkItems(ctx, params)
	if err != nil {
		logger.Error("failed to list work items", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	result := make([]*models.WorkItem, len(items))
	for i, item := range items {
		result[i] = s.toModel(item)
	}

	logger.Info("work items listed", slog.Int("count", len(result)))
	return result, nil
}

// validateCreateRequest validates the create work item request
func (s *Service) validateCreateRequest(req CreateWorkItemRequest) error {
	// Validate type
	if !isValidWorkItemType(req.Type) {
		return fmt.Errorf("%w: %s", ErrInvalidWorkItemType, req.Type)
	}

	// Validate required fields
	if req.Title == "" {
		return fmt.Errorf("%w: title", ErrRequiredFieldMissing)
	}
	if req.Description == "" {
		return fmt.Errorf("%w: description", ErrRequiredFieldMissing)
	}
	if req.Priority == "" {
		return fmt.Errorf("%w: priority", ErrRequiredFieldMissing)
	}
	if req.ReporterID == uuid.Nil {
		return fmt.Errorf("%w: reporter_id", ErrRequiredFieldMissing)
	}

	return nil
}

// validateParentChildRelationship validates parent-child relationships
func (s *Service) validateParentChildRelationship(ctx context.Context, childType models.WorkItemType, parentID uuid.UUID) error {
	// Get parent work item
	parent, err := s.repo.GetWorkItemByID(ctx, uuidToPgtype(parentID))
	if err != nil {
		return fmt.Errorf("parent work item not found: %w", err)
	}

	parentType := models.WorkItemType(parent.Type)

	// Validate relationship rules:
	// - Stories can have Epic parents
	// - Defects can have Epic or Story parents
	// - Epics cannot have parents
	switch childType {
	case models.WorkItemTypeEpic:
		return fmt.Errorf("%w: epics cannot have parents", ErrInvalidParentType)
	case models.WorkItemTypeStory:
		if parentType != models.WorkItemTypeEpic {
			return fmt.Errorf("%w: stories can only have epic parents", ErrInvalidParentType)
		}
	case models.WorkItemTypeDefect:
		if parentType != models.WorkItemTypeEpic && parentType != models.WorkItemTypeStory {
			return fmt.Errorf("%w: defects can only have epic or story parents", ErrInvalidParentType)
		}
	}

	return nil
}

// validateStatusTransition validates status transitions
func (s *Service) validateStatusTransition(ctx context.Context, workItem workitems.SprintManagementWorkItem, newStatus models.WorkItemStatus) error {
	currentStatus := models.WorkItemStatus(workItem.Status)

	// Define valid transitions
	validTransitions := map[models.WorkItemStatus][]models.WorkItemStatus{
		models.WorkItemStatusTodo: {
			models.WorkItemStatusInProgress,
			models.WorkItemStatusBlocked,
		},
		models.WorkItemStatusInProgress: {
			models.WorkItemStatusInReview,
			models.WorkItemStatusBlocked,
			models.WorkItemStatusTodo,
		},
		models.WorkItemStatusInReview: {
			models.WorkItemStatusDone,
			models.WorkItemStatusInProgress,
			models.WorkItemStatusBlocked,
		},
		models.WorkItemStatusBlocked: {
			models.WorkItemStatusTodo,
			models.WorkItemStatusInProgress,
		},
		models.WorkItemStatusDone: {
			models.WorkItemStatusInProgress, // Allow reopening
		},
	}

	// Check if transition is valid
	allowedStatuses, exists := validTransitions[currentStatus]
	if !exists {
		return fmt.Errorf("%w: unknown current status %s", ErrInvalidStatusTransition, currentStatus)
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatusTransition, currentStatus, newStatus)
}

// isValidWorkItemType checks if a work item type is valid
func isValidWorkItemType(t models.WorkItemType) bool {
	switch t {
	case models.WorkItemTypeEpic, models.WorkItemTypeStory, models.WorkItemTypeDefect:
		return true
	default:
		return false
	}
}

// toModel converts repository model to domain model
func (s *Service) toModel(wi workitems.SprintManagementWorkItem) *models.WorkItem {
	result := &models.WorkItem{
		ID:         pgtypeToUUID(wi.ID),
		Type:       models.WorkItemType(wi.Type),
		Title:      wi.Title,
		Status:     models.WorkItemStatus(wi.Status),
		Priority:   models.PriorityLevel(wi.Priority),
		ReporterID: pgtypeToUUID(wi.ReporterID),
		CreatedAt:  wi.CreatedAt.Time,
		UpdatedAt:  wi.UpdatedAt.Time,
	}

	if wi.Description != nil {
		result.Description = *wi.Description
	}
	if wi.StoryPoints != nil {
		sp := int(*wi.StoryPoints)
		result.StoryPoints = &sp
	}
	if wi.AssigneeID.Valid {
		assigneeID := pgtypeToUUID(wi.AssigneeID)
		result.AssigneeID = &assigneeID
	}
	if wi.ParentID.Valid {
		parentID := pgtypeToUUID(wi.ParentID)
		result.ParentID = &parentID
	}
	if wi.SprintID.Valid {
		sprintID := pgtypeToUUID(wi.SprintID)
		result.SprintID = &sprintID
	}
	if wi.DeletedAt.Valid {
		deletedAt := wi.DeletedAt.Time
		result.DeletedAt = &deletedAt
	}
	if wi.DeletedBy.Valid {
		deletedBy := pgtypeToUUID(wi.DeletedBy)
		result.DeletedBy = &deletedBy
	}

	return result
}

// getLogger gets logger from context or returns default
func (s *Service) getLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// Helper functions for UUID conversion
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: id != uuid.Nil,
	}
}

func pgtypeToUUID(pg pgtype.UUID) uuid.UUID {
	if !pg.Valid {
		return uuid.Nil
	}
	return pg.Bytes
}
