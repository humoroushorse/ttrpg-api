package softdelete

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/repository/comments"
	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/humoroushorse/go_sprint/pkg/config"
	"github.com/jackc/pgx/v5/pgtype"
)

// EntityType represents the type of entity that can be soft deleted
type EntityType string

const (
	EntityTypeWorkItem EntityType = "work_item"
	EntityTypeSprint   EntityType = "sprint"
	EntityTypeComment  EntityType = "comment"
)

// SoftDeletedItem represents a soft-deleted entity with metadata
type SoftDeletedItem struct {
	ID         uuid.UUID  `json:"id"`
	EntityType EntityType `json:"entity_type"`
	DeletedAt  time.Time  `json:"deleted_at"`
	DeletedBy  uuid.UUID  `json:"deleted_by"`
	Data       any        `json:"data"` // The actual entity data
}

// Service provides soft delete operations across all entities
type Service struct {
	workItemsRepo *workitems.Repository
	sprintsRepo   *sprints.Repository
	commentsRepo  *comments.Repository
	config        *config.Config
	logger        *slog.Logger
}

// NewService creates a new soft delete service
func NewService(
	workItemsRepo *workitems.Repository,
	sprintsRepo *sprints.Repository,
	commentsRepo *comments.Repository,
	cfg *config.Config,
	logger *slog.Logger,
) *Service {
	return &Service{
		workItemsRepo: workItemsRepo,
		sprintsRepo:   sprintsRepo,
		commentsRepo:  commentsRepo,
		config:        cfg,
		logger:        logger,
	}
}

// Helper function to convert pgtype.UUID to uuid.UUID
func pgtypeToUUID(pgu pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], pgu.Bytes[:])
	return u
}

// Helper function to convert uuid.UUID to pgtype.UUID
func uuidToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte(u),
		Valid: true,
	}
}

// ListSoftDeleted lists all soft-deleted items of a specific type
func (s *Service) ListSoftDeleted(ctx context.Context, entityType EntityType, limit, offset int) ([]SoftDeletedItem, error) {
	switch entityType {
	case EntityTypeWorkItem:
		workItems, err := s.workItemsRepo.ListSoftDeletedWorkItems(ctx, workitems.ListSoftDeletedWorkItemsParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list soft-deleted work items: %w", err)
		}

		items := make([]SoftDeletedItem, len(workItems))
		for i, wi := range workItems {
			items[i] = SoftDeletedItem{
				ID:         pgtypeToUUID(wi.ID),
				EntityType: EntityTypeWorkItem,
				DeletedAt:  wi.DeletedAt.Time,
				DeletedBy:  pgtypeToUUID(wi.DeletedBy),
				Data:       wi,
			}
		}
		return items, nil

	case EntityTypeSprint:
		sprintsList, err := s.sprintsRepo.ListSoftDeletedSprints(ctx, sprints.ListSoftDeletedSprintsParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list soft-deleted sprints: %w", err)
		}

		items := make([]SoftDeletedItem, len(sprintsList))
		for i, sp := range sprintsList {
			items[i] = SoftDeletedItem{
				ID:         pgtypeToUUID(sp.ID),
				EntityType: EntityTypeSprint,
				DeletedAt:  sp.DeletedAt.Time,
				DeletedBy:  pgtypeToUUID(sp.DeletedBy),
				Data:       sp,
			}
		}
		return items, nil

	case EntityTypeComment:
		commentsList, err := s.commentsRepo.ListSoftDeletedComments(ctx, comments.ListSoftDeletedCommentsParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list soft-deleted comments: %w", err)
		}

		items := make([]SoftDeletedItem, len(commentsList))
		for i, cm := range commentsList {
			items[i] = SoftDeletedItem{
				ID:         pgtypeToUUID(cm.ID),
				EntityType: EntityTypeComment,
				DeletedAt:  cm.DeletedAt.Time,
				DeletedBy:  pgtypeToUUID(cm.DeletedBy),
				Data:       cm,
			}
		}
		return items, nil

	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// Restore restores a soft-deleted item
func (s *Service) Restore(ctx context.Context, entityType EntityType, id uuid.UUID) error {
	pgID := uuidToPgtype(id)

	switch entityType {
	case EntityTypeWorkItem:
		if err := s.workItemsRepo.RestoreWorkItem(ctx, pgID); err != nil {
			return fmt.Errorf("failed to restore work item: %w", err)
		}
		s.logger.Info("Work item restored", "id", id)
		return nil

	case EntityTypeSprint:
		if err := s.sprintsRepo.RestoreSprint(ctx, pgID); err != nil {
			return fmt.Errorf("failed to restore sprint: %w", err)
		}
		s.logger.Info("Sprint restored", "id", id)
		return nil

	case EntityTypeComment:
		if err := s.commentsRepo.RestoreComment(ctx, pgID); err != nil {
			return fmt.Errorf("failed to restore comment: %w", err)
		}
		s.logger.Info("Comment restored", "id", id)
		return nil

	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

// PermanentlyDelete permanently deletes an item (only if retention period has passed)
func (s *Service) PermanentlyDelete(ctx context.Context, entityType EntityType, id uuid.UUID) error {
	pgID := uuidToPgtype(id)

	// First, check if the item exists and get its deletion timestamp
	var deletedAt time.Time
	var isDeleted bool

	switch entityType {
	case EntityTypeWorkItem:
		wi, err := s.workItemsRepo.GetWorkItemByIDIncludingDeleted(ctx, pgID)
		if err != nil {
			return fmt.Errorf("failed to get work item: %w", err)
		}
		isDeleted = wi.DeletedAt.Valid
		if isDeleted {
			deletedAt = wi.DeletedAt.Time
		}

	case EntityTypeSprint:
		sp, err := s.sprintsRepo.GetSprintByIDIncludingDeleted(ctx, pgID)
		if err != nil {
			return fmt.Errorf("failed to get sprint: %w", err)
		}
		isDeleted = sp.DeletedAt.Valid
		if isDeleted {
			deletedAt = sp.DeletedAt.Time
		}

	case EntityTypeComment:
		cm, err := s.commentsRepo.GetCommentByIDIncludingDeleted(ctx, pgID)
		if err != nil {
			return fmt.Errorf("failed to get comment: %w", err)
		}
		isDeleted = cm.DeletedAt.Valid
		if isDeleted {
			deletedAt = cm.DeletedAt.Time
		}

	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	// Check if item is soft-deleted
	if !isDeleted {
		return fmt.Errorf("item is not soft-deleted")
	}

	// Check retention period
	if err := s.checkRetentionPeriod(deletedAt); err != nil {
		return err
	}

	// Permanently delete the item
	switch entityType {
	case EntityTypeWorkItem:
		if err := s.workItemsRepo.PermanentlyDeleteWorkItem(ctx, pgID); err != nil {
			return fmt.Errorf("failed to permanently delete work item: %w", err)
		}
		s.logger.Info("Work item permanently deleted", "id", id)

	case EntityTypeSprint:
		if err := s.sprintsRepo.PermanentlyDeleteSprint(ctx, pgID); err != nil {
			return fmt.Errorf("failed to permanently delete sprint: %w", err)
		}
		s.logger.Info("Sprint permanently deleted", "id", id)

	case EntityTypeComment:
		if err := s.commentsRepo.PermanentlyDeleteComment(ctx, pgID); err != nil {
			return fmt.Errorf("failed to permanently delete comment: %w", err)
		}
		s.logger.Info("Comment permanently deleted", "id", id)
	}

	return nil
}

// checkRetentionPeriod validates that the retention period has passed
func (s *Service) checkRetentionPeriod(deletedAt time.Time) error {
	retentionDays := s.config.SoftDelete.RetentionDays
	if retentionDays <= 0 {
		// If retention is 0 or negative, allow immediate permanent deletion
		return nil
	}

	retentionPeriod := time.Duration(retentionDays) * 24 * time.Hour
	timeSinceDeletion := time.Since(deletedAt)

	if timeSinceDeletion < retentionPeriod {
		remainingTime := retentionPeriod - timeSinceDeletion
		return fmt.Errorf("retention period not met: %v remaining", remainingTime.Round(time.Hour))
	}

	return nil
}

// GetRetentionPeriod returns the configured retention period in days
func (s *Service) GetRetentionPeriod() int {
	return s.config.SoftDelete.RetentionDays
}

// CanPermanentlyDelete checks if an item can be permanently deleted based on retention period
func (s *Service) CanPermanentlyDelete(deletedAt time.Time) bool {
	return s.checkRetentionPeriod(deletedAt) == nil
}
