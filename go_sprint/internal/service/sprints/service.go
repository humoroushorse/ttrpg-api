package sprints

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/humoroushorse/go_sprint/pkg/pagination"
)

var (
	// ErrSprintNotFound is returned when a sprint is not found
	ErrSprintNotFound = errors.New("sprint not found")
	// ErrInvalidDateRange is returned when end date is not after start date
	ErrInvalidDateRange = errors.New("end date must be after start date")
	// ErrSprintClosed is returned when trying to modify a closed sprint
	ErrSprintClosed = errors.New("cannot modify closed sprint")
	// ErrRequiredFieldMissing is returned when a required field is missing
	ErrRequiredFieldMissing = errors.New("required field missing")
)

// SprintRepository defines the interface for sprint data access
type SprintRepository interface {
	CreateSprint(ctx context.Context, params sprints.CreateSprintParams) (sprints.SprintManagementSprint, error)
	GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error)
	UpdateSprint(ctx context.Context, params sprints.UpdateSprintParams) (sprints.SprintManagementSprint, error)
	UpdateSprintStatus(ctx context.Context, params sprints.UpdateSprintStatusParams) (sprints.SprintManagementSprint, error)
	UpdateSprintMetrics(ctx context.Context, params sprints.UpdateSprintMetricsParams) (sprints.SprintManagementSprint, error)
	MoveWorkItemsToBacklog(ctx context.Context, sprintID pgtype.UUID) error
	GetSprintWithWorkItems(ctx context.Context, id pgtype.UUID) (sprints.GetSprintWithWorkItemsRow, error)
	ListSprints(ctx context.Context, params sprints.ListSprintsParams) ([]sprints.SprintManagementSprint, error)
}

// WorkItemRepository defines the interface for work item data access
type WorkItemRepository interface {
	ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.ListWorkItemsBySprintRow, error)
}

// Service provides business logic for sprint operations
type Service struct {
	sprintRepo   SprintRepository
	workItemRepo WorkItemRepository
	logger       *slog.Logger
}

// NewService creates a new sprint service
func NewService(sprintRepo SprintRepository, workItemRepo WorkItemRepository, logger *slog.Logger) *Service {
	return &Service{
		sprintRepo:   sprintRepo,
		workItemRepo: workItemRepo,
		logger:       logger,
	}
}

// CreateSprintRequest represents a request to create a sprint
type CreateSprintRequest struct {
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	CapacityPoints *int      `json:"capacity_points,omitempty"`
	CreatedBy      uuid.UUID `json:"created_by"`
	UserTimezone   string    `json:"user_timezone,omitempty"` // IANA timezone (e.g., "America/Denver")
}

// UpdateSprintRequest represents a request to update a sprint
type UpdateSprintRequest struct {
	Name           *string    `json:"name,omitempty"`
	Description    *string    `json:"description,omitempty"`
	Status         *string    `json:"status,omitempty"`
	StartDate      *time.Time `json:"start_date,omitempty"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	CapacityPoints *int       `json:"capacity_points,omitempty"`
}

// SprintMetrics represents calculated sprint metrics
type SprintMetrics struct {
	TotalWorkItems      int     `json:"total_work_items"`
	CompletedWorkItems  int     `json:"completed_work_items"`
	InProgressWorkItems int     `json:"in_progress_work_items"`
	TodoWorkItems       int     `json:"todo_work_items"`
	CommittedPoints     int     `json:"committed_points"`
	CompletedPoints     int     `json:"completed_points"`
	CompletionRate      float64 `json:"completion_rate"`
}

// SprintWithMetrics represents a sprint with calculated metrics
type SprintWithMetrics struct {
	Sprint  *models.Sprint `json:"sprint"`
	Metrics *SprintMetrics `json:"metrics"`
}

// CreateSprint creates a new sprint with validation
func (s *Service) CreateSprint(ctx context.Context, req CreateSprintRequest) (*models.Sprint, error) {
	logger := s.getLogger(ctx)

	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("validation failed", slog.String("error", err.Error()))
		return nil, err
	}

	// Validate start date is not in the past (allow today in user's timezone)
	// Load user's timezone, default to UTC if invalid
	loc, err := time.LoadLocation(req.UserTimezone)
	if err != nil {
		loc = time.UTC
	}

	// Get current time in user's timezone and truncate to start of day
	now := time.Now().In(loc)
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	// Parse start date in user's timezone (dates come as YYYY-MM-DD which are parsed as midnight UTC)
	// We need to interpret them as midnight in the user's timezone instead
	startYear, startMonth, startDay := req.StartDate.Date()
	startDate := time.Date(startYear, startMonth, startDay, 0, 0, 0, 0, loc)

	if startDate.Before(nowDate) {
		logger.Error("start date is in the past",
			slog.Time("start_date_parsed", startDate),
			slog.Time("now_date", nowDate),
			slog.String("user_timezone", req.UserTimezone))
		return nil, fmt.Errorf("start date cannot be in the past")
	}

	// Validate date range
	if !req.EndDate.After(req.StartDate) {
		logger.Error("invalid date range",
			slog.Time("start_date", req.StartDate),
			slog.Time("end_date", req.EndDate))
		return nil, ErrInvalidDateRange
	}

	// Convert to repository params
	params := sprints.CreateSprintParams{
		Name:      req.Name,
		Status:    sprints.SprintManagementSprintStatusPlanned,
		StartDate: timeToPgDate(req.StartDate),
		EndDate:   timeToPgDate(req.EndDate),
		CreatedBy: uuidToPgtype(req.CreatedBy),
	}

	if req.Description != "" {
		params.Description = &req.Description
	}
	if req.CapacityPoints != nil {
		cp := int32(*req.CapacityPoints)
		params.CapacityPoints = &cp
	}

	// Create sprint
	sprint, err := s.sprintRepo.CreateSprint(ctx, params)
	if err != nil {
		logger.Error("failed to create sprint", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create sprint: %w", err)
	}

	result := s.toModel(sprint)
	logger.Info("sprint created", slog.Any("sprint", result))

	return result, nil
}

// GetSprint retrieves a sprint by ID
func (s *Service) GetSprint(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	logger := s.getLogger(ctx)

	sprint, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get sprint", slog.String("id", id.String()), slog.String("error", err.Error()))
		return nil, ErrSprintNotFound
	}

	result := s.toModel(sprint)
	logger.Info("sprint retrieved", slog.Any("sprint", result))

	return result, nil
}

// UpdateSprint updates a sprint with validation
func (s *Service) UpdateSprint(ctx context.Context, id uuid.UUID, req UpdateSprintRequest) (*models.Sprint, error) {
	logger := s.getLogger(ctx)

	// Get existing sprint
	existing, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("sprint not found", slog.String("id", id.String()))
		return nil, ErrSprintNotFound
	}

	// Check if sprint is closed
	if existing.Status == sprints.SprintManagementSprintStatusCompleted ||
		existing.Status == sprints.SprintManagementSprintStatusCancelled {
		logger.Error("cannot update closed sprint", slog.String("id", id.String()), slog.String("status", string(existing.Status)))
		return nil, ErrSprintClosed
	}

	// Build update params
	params := sprints.UpdateSprintParams{
		ID: uuidToPgtype(id),
	}

	// Name
	if req.Name != nil {
		params.Name = req.Name
	} else {
		params.Name = &existing.Name
	}

	// Description
	if req.Description != nil {
		params.Description = req.Description
	} else {
		params.Description = existing.Description
	}

	// Start Date
	if req.StartDate != nil {
		params.StartDate = timeToPgDate(*req.StartDate)
	} else {
		params.StartDate = existing.StartDate
	}

	// End Date
	if req.EndDate != nil {
		params.EndDate = timeToPgDate(*req.EndDate)
	} else {
		params.EndDate = existing.EndDate
	}

	// Validate date range
	startDate := pgDateToTime(params.StartDate)
	endDate := pgDateToTime(params.EndDate)
	if !endDate.After(startDate) {
		logger.Error("invalid date range",
			slog.Time("start_date", startDate),
			slog.Time("end_date", endDate))
		return nil, ErrInvalidDateRange
	}

	// Capacity Points
	if req.CapacityPoints != nil {
		cp := int32(*req.CapacityPoints)
		params.CapacityPoints = &cp
	} else {
		params.CapacityPoints = existing.CapacityPoints
	}

	// Status - allow status changes
	if req.Status != nil {
		// Validate status value
		var newStatus sprints.SprintManagementSprintStatus
		switch *req.Status {
		case "planned":
			newStatus = sprints.SprintManagementSprintStatusPlanned
		case "active":
			newStatus = sprints.SprintManagementSprintStatusActive
		case "completed":
			newStatus = sprints.SprintManagementSprintStatusCompleted
		case "cancelled":
			newStatus = sprints.SprintManagementSprintStatusCancelled
		default:
			logger.Error("invalid status value", slog.String("status", *req.Status))
			return nil, fmt.Errorf("invalid status value: %s", *req.Status)
		}
		params.Status = sprints.NullSprintManagementSprintStatus{
			SprintManagementSprintStatus: newStatus,
			Valid:                        true,
		}
	} else {
		params.Status = sprints.NullSprintManagementSprintStatus{
			SprintManagementSprintStatus: existing.Status,
			Valid:                        true,
		}
	}

	// Update sprint
	updated, err := s.sprintRepo.UpdateSprint(ctx, params)
	if err != nil {
		logger.Error("failed to update sprint", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update sprint: %w", err)
	}

	result := s.toModel(updated)
	logger.Info("sprint updated", slog.Any("sprint", result))

	return result, nil
}

// CloseSprint closes a sprint and moves incomplete work items to backlog
func (s *Service) CloseSprint(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	logger := s.getLogger(ctx)

	// Get existing sprint
	existing, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("sprint not found", slog.String("id", id.String()))
		return nil, ErrSprintNotFound
	}

	// Check if sprint is already closed
	if existing.Status == sprints.SprintManagementSprintStatusCompleted ||
		existing.Status == sprints.SprintManagementSprintStatusCancelled {
		logger.Warn("sprint already closed", slog.String("id", id.String()), slog.String("status", string(existing.Status)))
		return s.toModel(existing), nil
	}

	// Calculate final metrics before closing
	metrics, err := s.calculateSprintMetrics(ctx, id)
	if err != nil {
		logger.Error("failed to calculate metrics", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}

	// Update sprint metrics
	committedPoints := int32(metrics.CommittedPoints)
	completedPoints := int32(metrics.CompletedPoints)
	metricsParams := sprints.UpdateSprintMetricsParams{
		ID:              uuidToPgtype(id),
		CommittedPoints: &committedPoints,
		CompletedPoints: &completedPoints,
	}
	_, err = s.sprintRepo.UpdateSprintMetrics(ctx, metricsParams)
	if err != nil {
		logger.Error("failed to update sprint metrics", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update sprint metrics: %w", err)
	}

	// Move incomplete work items to backlog
	if err := s.sprintRepo.MoveWorkItemsToBacklog(ctx, uuidToPgtype(id)); err != nil {
		logger.Error("failed to move work items to backlog", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to move work items to backlog: %w", err)
	}

	// Update sprint status to completed
	statusParams := sprints.UpdateSprintStatusParams{
		ID:     uuidToPgtype(id),
		Status: sprints.SprintManagementSprintStatusCompleted,
	}
	updated, err := s.sprintRepo.UpdateSprintStatus(ctx, statusParams)
	if err != nil {
		logger.Error("failed to update sprint status", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to update sprint status: %w", err)
	}

	result := s.toModel(updated)
	logger.Info("sprint closed",
		slog.Any("sprint", result),
		slog.Int("completed_points", metrics.CompletedPoints),
		slog.Int("committed_points", metrics.CommittedPoints))

	return result, nil
}

// GetSprintMetrics retrieves calculated metrics for a sprint
func (s *Service) GetSprintMetrics(ctx context.Context, id uuid.UUID) (*SprintMetrics, error) {
	logger := s.getLogger(ctx)

	// Verify sprint exists
	_, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("sprint not found", slog.String("id", id.String()))
		return nil, ErrSprintNotFound
	}

	// Calculate metrics
	metrics, err := s.calculateSprintMetrics(ctx, id)
	if err != nil {
		logger.Error("failed to calculate metrics", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}

	logger.Info("sprint metrics calculated", slog.String("sprint_id", id.String()), slog.Any("metrics", metrics))

	return metrics, nil
}

// GetSprintWithMetrics retrieves a sprint with its calculated metrics
func (s *Service) GetSprintWithMetrics(ctx context.Context, id uuid.UUID) (*SprintWithMetrics, error) {
	logger := s.getLogger(ctx)

	// Get sprint
	sprint, err := s.GetSprint(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get metrics
	metrics, err := s.GetSprintMetrics(ctx, id)
	if err != nil {
		return nil, err
	}

	result := &SprintWithMetrics{
		Sprint:  sprint,
		Metrics: metrics,
	}

	logger.Info("sprint with metrics retrieved", slog.String("sprint_id", id.String()))

	return result, nil
}

// calculateSprintMetrics calculates metrics for a sprint
func (s *Service) calculateSprintMetrics(ctx context.Context, sprintID uuid.UUID) (*SprintMetrics, error) {
	// Get all work items in the sprint
	workItems, err := s.workItemRepo.ListWorkItemsBySprint(ctx, uuidToPgtype(sprintID))
	if err != nil {
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	metrics := &SprintMetrics{
		TotalWorkItems: len(workItems),
	}

	// Calculate metrics from work items
	for _, wi := range workItems {
		// Count by status
		switch wi.Status {
		case workitems.SprintManagementWorkItemStatusDone:
			metrics.CompletedWorkItems++
		case workitems.SprintManagementWorkItemStatusInProgress, workitems.SprintManagementWorkItemStatusInReview:
			metrics.InProgressWorkItems++
		case workitems.SprintManagementWorkItemStatusTodo, workitems.SprintManagementWorkItemStatusBlocked:
			metrics.TodoWorkItems++
		}

		// Sum story points
		if wi.StoryPoints != nil {
			points := int(*wi.StoryPoints)
			metrics.CommittedPoints += points
			if wi.Status == workitems.SprintManagementWorkItemStatusDone {
				metrics.CompletedPoints += points
			}
		}
	}

	// Calculate completion rate
	if metrics.CommittedPoints > 0 {
		metrics.CompletionRate = float64(metrics.CompletedPoints) / float64(metrics.CommittedPoints) * 100
	}

	return metrics, nil
}

// ListSprints retrieves a list of sprints with cursor-based pagination and filters
func (s *Service) ListSprints(ctx context.Context, cursor *pagination.Cursor, limit int32, searchQuery *string, statusFilter []string) ([]*models.Sprint, error) {
	logger := s.getLogger(ctx)

	params := sprints.ListSprintsParams{
		Limit:           limit,
		CursorTimestamp: pgtype.Timestamptz{Valid: false},
		CursorID:        pgtype.UUID{Valid: false},
		SearchQuery:     searchQuery,
		StatusFilter:    statusFilter,
	}

	// If cursor is provided, add cursor filters
	if cursor != nil && cursor.ID != "" {
		// Parse cursor ID
		cursorID, err := uuid.Parse(cursor.ID)
		if err != nil {
			logger.Warn("invalid cursor ID", slog.String("cursor_id", cursor.ID), slog.String("error", err.Error()))
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

	items, err := s.sprintRepo.ListSprints(ctx, params)
	if err != nil {
		logger.Error("failed to list sprints", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list sprints: %w", err)
	}

	result := make([]*models.Sprint, len(items))
	for i, item := range items {
		result[i] = s.toModel(item)
	}

	logger.Info("sprints listed", slog.Int("count", len(result)))
	return result, nil
}

// validateCreateRequest validates the create sprint request
func (s *Service) validateCreateRequest(req CreateSprintRequest) error {
	// Validate required fields
	if req.Name == "" {
		return fmt.Errorf("%w: name", ErrRequiredFieldMissing)
	}
	if req.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date", ErrRequiredFieldMissing)
	}
	if req.EndDate.IsZero() {
		return fmt.Errorf("%w: end_date", ErrRequiredFieldMissing)
	}
	if req.CreatedBy == uuid.Nil {
		return fmt.Errorf("%w: created_by", ErrRequiredFieldMissing)
	}

	return nil
}

// toModel converts repository model to domain model
func (s *Service) toModel(sprint sprints.SprintManagementSprint) *models.Sprint {
	result := &models.Sprint{
		ID:              pgtypeToUUID(sprint.ID),
		Name:            sprint.Name,
		Status:          models.SprintStatus(sprint.Status),
		StartDate:       pgDateToTime(sprint.StartDate),
		EndDate:         pgDateToTime(sprint.EndDate),
		CommittedPoints: 0,
		CompletedPoints: 0,
		CreatedBy:       pgtypeToUUID(sprint.CreatedBy),
		CreatedAt:       sprint.CreatedAt.Time,
		UpdatedAt:       sprint.UpdatedAt.Time,
	}

	if sprint.Description != nil {
		result.Description = *sprint.Description
	}
	if sprint.CapacityPoints != nil {
		cp := int(*sprint.CapacityPoints)
		result.CapacityPoints = &cp
	}
	if sprint.CommittedPoints != nil {
		result.CommittedPoints = int(*sprint.CommittedPoints)
	}
	if sprint.CompletedPoints != nil {
		result.CompletedPoints = int(*sprint.CompletedPoints)
	}
	if sprint.DeletedAt.Valid {
		deletedAt := sprint.DeletedAt.Time
		result.DeletedAt = &deletedAt
	}
	if sprint.DeletedBy.Valid {
		deletedBy := pgtypeToUUID(sprint.DeletedBy)
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

// Helper functions for UUID and date conversion
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

func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}
