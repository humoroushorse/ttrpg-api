package dependencies

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/humoroushorse/go_sprint/internal/repository/dependencies"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

var (
	// ErrDependencyNotFound is returned when a dependency is not found
	ErrDependencyNotFound = errors.New("dependency not found")
	// ErrInvalidDependencyType is returned when an invalid dependency type is provided
	ErrInvalidDependencyType = errors.New("invalid dependency type")
	// ErrSelfDependency is returned when trying to create a dependency to itself
	ErrSelfDependency = errors.New("work item cannot depend on itself")
	// ErrDependencyAlreadyExists is returned when a dependency already exists
	ErrDependencyAlreadyExists = errors.New("dependency already exists")
	// ErrHasBlockingDependencies is returned when trying to complete a work item with unresolved blocking dependencies
	ErrHasBlockingDependencies = errors.New("work item has unresolved blocking dependencies")
	// ErrRequiredFieldMissing is returned when a required field is missing
	ErrRequiredFieldMissing = errors.New("required field missing")
)

// DependencyRepository defines the interface for dependency data access
type DependencyRepository interface {
	CreateDependency(ctx context.Context, params dependencies.CreateDependencyParams) (dependencies.SprintManagementWorkItemDependency, error)
	GetDependencyByID(ctx context.Context, id interface{}) (dependencies.SprintManagementWorkItemDependency, error)
	GetDependenciesBySourceID(ctx context.Context, sourceID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error)
	GetDependenciesByTargetID(ctx context.Context, targetID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error)
	GetAllDependenciesForWorkItem(ctx context.Context, workItemID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error)
	DeleteDependency(ctx context.Context, id interface{}) error
	DeleteDependenciesByWorkItemID(ctx context.Context, workItemID interface{}) error
	HasBlockingDependencies(ctx context.Context, targetID interface{}) (bool, error)
	GetBlockingDependencies(ctx context.Context, targetID interface{}) ([]dependencies.GetBlockingDependenciesRow, error)
	DependencyExists(ctx context.Context, params dependencies.DependencyExistsParams) (bool, error)
}

// Service provides business logic for dependency operations
type Service struct {
	repo   DependencyRepository
	logger *slog.Logger
}

// NewService creates a new dependency service
func NewService(repo DependencyRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// CreateDependencyRequest represents a request to create a dependency
type CreateDependencyRequest struct {
	SourceID       uuid.UUID             `json:"source_id"`
	TargetID       uuid.UUID             `json:"target_id"`
	DependencyType models.DependencyType `json:"dependency_type"`
	CreatedBy      uuid.UUID             `json:"created_by"`
}

// CreateDependency creates a new work item dependency with validation
func (s *Service) CreateDependency(ctx context.Context, req CreateDependencyRequest) (*models.WorkItemDependency, error) {
	logger := s.getLogger(ctx)

	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		logger.Error("validation failed", slog.String("error", err.Error()))
		return nil, err
	}

	// Check if dependency already exists
	exists, err := s.repo.DependencyExists(ctx, dependencies.DependencyExistsParams{
		SourceID:       uuidToPgtype(req.SourceID),
		TargetID:       uuidToPgtype(req.TargetID),
		DependencyType: dependencies.SprintManagementDependencyType(req.DependencyType),
	})
	if err != nil {
		logger.Error("failed to check dependency existence", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to check dependency existence: %w", err)
	}
	if exists {
		logger.Warn("dependency already exists",
			slog.String("source_id", req.SourceID.String()),
			slog.String("target_id", req.TargetID.String()),
			slog.String("type", string(req.DependencyType)))
		return nil, ErrDependencyAlreadyExists
	}

	// Create dependency
	params := dependencies.CreateDependencyParams{
		SourceID:       uuidToPgtype(req.SourceID),
		TargetID:       uuidToPgtype(req.TargetID),
		DependencyType: dependencies.SprintManagementDependencyType(req.DependencyType),
		CreatedBy:      uuidToPgtype(req.CreatedBy),
	}

	dep, err := s.repo.CreateDependency(ctx, params)
	if err != nil {
		logger.Error("failed to create dependency", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to create dependency: %w", err)
	}

	result := s.toModel(dep)
	logger.Info("dependency created", slog.Any("dependency", result))

	return result, nil
}

// GetDependency retrieves a dependency by ID
func (s *Service) GetDependency(ctx context.Context, id uuid.UUID) (*models.WorkItemDependency, error) {
	logger := s.getLogger(ctx)

	dep, err := s.repo.GetDependencyByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("failed to get dependency", slog.String("id", id.String()), slog.String("error", err.Error()))
		return nil, ErrDependencyNotFound
	}

	result := s.toModel(dep)
	logger.Info("dependency retrieved", slog.Any("dependency", result))

	return result, nil
}

// GetDependenciesForWorkItem retrieves all dependencies for a work item
func (s *Service) GetDependenciesForWorkItem(ctx context.Context, workItemID uuid.UUID) ([]*models.WorkItemDependency, error) {
	logger := s.getLogger(ctx)

	deps, err := s.repo.GetAllDependenciesForWorkItem(ctx, uuidToPgtype(workItemID))
	if err != nil {
		logger.Error("failed to get dependencies", slog.String("work_item_id", workItemID.String()), slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}

	result := make([]*models.WorkItemDependency, len(deps))
	for i, dep := range deps {
		result[i] = s.toModel(dep)
	}

	logger.Info("dependencies retrieved", slog.String("work_item_id", workItemID.String()), slog.Int("count", len(result)))
	return result, nil
}

// DeleteDependency deletes a dependency by ID
func (s *Service) DeleteDependency(ctx context.Context, id uuid.UUID) error {
	logger := s.getLogger(ctx)

	// Check if dependency exists
	_, err := s.repo.GetDependencyByID(ctx, uuidToPgtype(id))
	if err != nil {
		logger.Error("dependency not found", slog.String("id", id.String()))
		return ErrDependencyNotFound
	}

	// Delete dependency
	if err := s.repo.DeleteDependency(ctx, uuidToPgtype(id)); err != nil {
		logger.Error("failed to delete dependency", slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete dependency: %w", err)
	}

	logger.Info("dependency deleted", slog.String("id", id.String()))
	return nil
}

// DeleteDependenciesForWorkItem deletes all dependencies for a work item
// This is called when a work item is deleted
func (s *Service) DeleteDependenciesForWorkItem(ctx context.Context, workItemID uuid.UUID) error {
	logger := s.getLogger(ctx)

	if err := s.repo.DeleteDependenciesByWorkItemID(ctx, uuidToPgtype(workItemID)); err != nil {
		logger.Error("failed to delete dependencies for work item",
			slog.String("work_item_id", workItemID.String()),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to delete dependencies: %w", err)
	}

	logger.Info("dependencies deleted for work item", slog.String("work_item_id", workItemID.String()))
	return nil
}

// ValidateDependencyResolution checks if a work item can be completed
// Returns error if there are unresolved blocking dependencies
func (s *Service) ValidateDependencyResolution(ctx context.Context, workItemID uuid.UUID) error {
	logger := s.getLogger(ctx)

	hasBlocking, err := s.repo.HasBlockingDependencies(ctx, uuidToPgtype(workItemID))
	if err != nil {
		logger.Error("failed to check blocking dependencies", slog.String("error", err.Error()))
		return fmt.Errorf("failed to check blocking dependencies: %w", err)
	}

	if hasBlocking {
		// Get the blocking dependencies for detailed error message
		blockingDeps, err := s.repo.GetBlockingDependencies(ctx, uuidToPgtype(workItemID))
		if err != nil {
			logger.Error("failed to get blocking dependencies", slog.String("error", err.Error()))
			return ErrHasBlockingDependencies
		}

		logger.Warn("work item has unresolved blocking dependencies",
			slog.String("work_item_id", workItemID.String()),
			slog.Int("blocking_count", len(blockingDeps)))

		return fmt.Errorf("%w: %d unresolved blocking dependencies", ErrHasBlockingDependencies, len(blockingDeps))
	}

	return nil
}

// GetBlockingDependencies retrieves all unresolved blocking dependencies for a work item
func (s *Service) GetBlockingDependencies(ctx context.Context, workItemID uuid.UUID) ([]uuid.UUID, error) {
	logger := s.getLogger(ctx)

	blockingDeps, err := s.repo.GetBlockingDependencies(ctx, uuidToPgtype(workItemID))
	if err != nil {
		logger.Error("failed to get blocking dependencies", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get blocking dependencies: %w", err)
	}

	result := make([]uuid.UUID, len(blockingDeps))
	for i, dep := range blockingDeps {
		result[i] = pgtypeToUUID(dep.SourceID)
	}

	logger.Info("blocking dependencies retrieved",
		slog.String("work_item_id", workItemID.String()),
		slog.Int("count", len(result)))

	return result, nil
}

// validateCreateRequest validates the create dependency request
func (s *Service) validateCreateRequest(req CreateDependencyRequest) error {
	// Validate required fields
	if req.SourceID == uuid.Nil {
		return fmt.Errorf("%w: source_id", ErrRequiredFieldMissing)
	}
	if req.TargetID == uuid.Nil {
		return fmt.Errorf("%w: target_id", ErrRequiredFieldMissing)
	}
	if req.CreatedBy == uuid.Nil {
		return fmt.Errorf("%w: created_by", ErrRequiredFieldMissing)
	}

	// Validate no self-dependency
	if req.SourceID == req.TargetID {
		return ErrSelfDependency
	}

	// Validate dependency type
	if !isValidDependencyType(req.DependencyType) {
		return fmt.Errorf("%w: %s", ErrInvalidDependencyType, req.DependencyType)
	}

	return nil
}

// isValidDependencyType checks if a dependency type is valid
func isValidDependencyType(t models.DependencyType) bool {
	switch t {
	case models.DependencyTypeBlocks,
		models.DependencyTypeIsBlockedBy,
		models.DependencyTypeRelatesTo,
		models.DependencyTypeDuplicates:
		return true
	default:
		return false
	}
}

// toModel converts repository model to domain model
func (s *Service) toModel(dep dependencies.SprintManagementWorkItemDependency) *models.WorkItemDependency {
	return &models.WorkItemDependency{
		ID:             pgtypeToUUID(dep.ID),
		SourceID:       pgtypeToUUID(dep.SourceID),
		TargetID:       pgtypeToUUID(dep.TargetID),
		DependencyType: models.DependencyType(dep.DependencyType),
		CreatedAt:      dep.CreatedAt.Time,
		CreatedBy:      pgtypeToUUID(dep.CreatedBy),
	}
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
