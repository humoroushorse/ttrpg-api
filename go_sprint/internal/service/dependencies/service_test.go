package dependencies

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/humoroushorse/go_sprint/internal/repository/dependencies"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

// Feature: go-sprint-management, Property 13: Dependency Type Validation
// Validates: Requirements 28.1
func TestProperty_DependencyTypeValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("only valid dependency types are accepted", prop.ForAll(
		func(depType string) bool {
			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			sourceID := uuid.New()
			targetID := uuid.New()
			createdBy := uuid.New()

			req := CreateDependencyRequest{
				SourceID:       sourceID,
				TargetID:       targetID,
				DependencyType: models.DependencyType(depType),
				CreatedBy:      createdBy,
			}

			_, err := service.CreateDependency(ctx, req)

			// Check if the type is valid
			validTypes := []string{"blocks", "is_blocked_by", "relates_to", "duplicates"}
			isValid := false
			for _, vt := range validTypes {
				if depType == vt {
					isValid = true
					break
				}
			}

			if isValid {
				// Valid types should succeed (or fail for other reasons, but not type validation)
				return err == nil || err != ErrInvalidDependencyType
			} else {
				// Invalid types should fail with type validation error
				return errors.Is(err, ErrInvalidDependencyType)
			}
		},
		gen.OneConstOf("blocks", "is_blocked_by", "relates_to", "duplicates", "invalid", "wrong", ""),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property 15: Dependency Resolution Validation
// Validates: Requirements 28.3
func TestProperty_DependencyResolutionValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("work items with unresolved blocking dependencies cannot be completed", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			workItemID := uuid.New()
			blockerID := uuid.New()

			// Create a blocking dependency
			dep := dependencies.SprintManagementWorkItemDependency{
				ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
				SourceID:       pgtype.UUID{Bytes: blockerID, Valid: true},
				TargetID:       pgtype.UUID{Bytes: workItemID, Valid: true},
				DependencyType: dependencies.SprintManagementDependencyTypeBlocks,
				CreatedBy:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
				CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.dependencies[dep.ID.Bytes] = dep
			repo.hasBlocking[workItemID] = true

			// Test: Should fail validation
			err := service.ValidateDependencyResolution(ctx, workItemID)
			return errors.Is(err, ErrHasBlockingDependencies)
		},
	))

	properties.Property("work items without blocking dependencies can be completed", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			workItemID := uuid.New()
			repo.hasBlocking[workItemID] = false

			// Test: Should pass validation
			err := service.ValidateDependencyResolution(ctx, workItemID)
			return err == nil
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property 17: Dependency Cleanup on Deletion
// Validates: Requirements 28.5
func TestProperty_DependencyCleanupOnDeletion(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("deleting a work item removes all its dependencies", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			workItemID := uuid.New()
			relatedID1 := uuid.New()
			relatedID2 := uuid.New()

			// Create dependencies
			dep1 := dependencies.SprintManagementWorkItemDependency{
				ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
				SourceID:       pgtype.UUID{Bytes: workItemID, Valid: true},
				TargetID:       pgtype.UUID{Bytes: relatedID1, Valid: true},
				DependencyType: dependencies.SprintManagementDependencyTypeBlocks,
				CreatedBy:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
				CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			dep2 := dependencies.SprintManagementWorkItemDependency{
				ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
				SourceID:       pgtype.UUID{Bytes: relatedID2, Valid: true},
				TargetID:       pgtype.UUID{Bytes: workItemID, Valid: true},
				DependencyType: dependencies.SprintManagementDependencyTypeRelatesTo,
				CreatedBy:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
				CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			repo.dependencies[dep1.ID.Bytes] = dep1
			repo.dependencies[dep2.ID.Bytes] = dep2

			// Delete dependencies for work item
			err := service.DeleteDependenciesForWorkItem(ctx, workItemID)
			if err != nil {
				return false
			}

			// Verify dependencies were deleted
			return repo.deletedWorkItemID == workItemID
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit tests for specific scenarios

func TestCreateDependency_SelfDependency(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, nil)
	ctx := context.Background()

	workItemID := uuid.New()
	req := CreateDependencyRequest{
		SourceID:       workItemID,
		TargetID:       workItemID,
		DependencyType: models.DependencyTypeBlocks,
		CreatedBy:      uuid.New(),
	}

	_, err := service.CreateDependency(ctx, req)
	if !errors.Is(err, ErrSelfDependency) {
		t.Errorf("expected ErrSelfDependency, got %v", err)
	}
}

func TestCreateDependency_AlreadyExists(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, nil)
	ctx := context.Background()

	sourceID := uuid.New()
	targetID := uuid.New()

	// Mark as existing
	repo.existingDeps[sourceID.String()+targetID.String()+"blocks"] = true

	req := CreateDependencyRequest{
		SourceID:       sourceID,
		TargetID:       targetID,
		DependencyType: models.DependencyTypeBlocks,
		CreatedBy:      uuid.New(),
	}

	_, err := service.CreateDependency(ctx, req)
	if !errors.Is(err, ErrDependencyAlreadyExists) {
		t.Errorf("expected ErrDependencyAlreadyExists, got %v", err)
	}
}

func TestCreateDependency_Success(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, nil)
	ctx := context.Background()

	sourceID := uuid.New()
	targetID := uuid.New()
	createdBy := uuid.New()

	req := CreateDependencyRequest{
		SourceID:       sourceID,
		TargetID:       targetID,
		DependencyType: models.DependencyTypeBlocks,
		CreatedBy:      createdBy,
	}

	dep, err := service.CreateDependency(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dep.SourceID != sourceID {
		t.Errorf("expected source_id %v, got %v", sourceID, dep.SourceID)
	}
	if dep.TargetID != targetID {
		t.Errorf("expected target_id %v, got %v", targetID, dep.TargetID)
	}
	if dep.DependencyType != models.DependencyTypeBlocks {
		t.Errorf("expected type blocks, got %v", dep.DependencyType)
	}
}

func TestDeleteDependency_NotFound(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, nil)
	ctx := context.Background()

	err := service.DeleteDependency(ctx, uuid.New())
	if !errors.Is(err, ErrDependencyNotFound) {
		t.Errorf("expected ErrDependencyNotFound, got %v", err)
	}
}

func TestGetDependenciesForWorkItem(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, nil)
	ctx := context.Background()

	workItemID := uuid.New()
	relatedID := uuid.New()

	// Create a dependency
	dep := dependencies.SprintManagementWorkItemDependency{
		ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceID:       pgtype.UUID{Bytes: workItemID, Valid: true},
		TargetID:       pgtype.UUID{Bytes: relatedID, Valid: true},
		DependencyType: dependencies.SprintManagementDependencyTypeBlocks,
		CreatedBy:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	repo.dependencies[dep.ID.Bytes] = dep

	deps, err := service.GetDependenciesForWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(deps))
	}
}

// Mock repository for testing
type mockRepository struct {
	dependencies      map[uuid.UUID]dependencies.SprintManagementWorkItemDependency
	existingDeps      map[string]bool
	hasBlocking       map[uuid.UUID]bool
	deletedWorkItemID uuid.UUID
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		dependencies: make(map[uuid.UUID]dependencies.SprintManagementWorkItemDependency),
		existingDeps: make(map[string]bool),
		hasBlocking:  make(map[uuid.UUID]bool),
	}
}

func (m *mockRepository) CreateDependency(ctx context.Context, params dependencies.CreateDependencyParams) (dependencies.SprintManagementWorkItemDependency, error) {
	dep := dependencies.SprintManagementWorkItemDependency{
		ID:             pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceID:       params.SourceID,
		TargetID:       params.TargetID,
		DependencyType: params.DependencyType,
		CreatedBy:      params.CreatedBy,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.dependencies[dep.ID.Bytes] = dep
	return dep, nil
}

func (m *mockRepository) GetDependencyByID(ctx context.Context, id interface{}) (dependencies.SprintManagementWorkItemDependency, error) {
	pgID := id.(pgtype.UUID)
	if dep, ok := m.dependencies[pgID.Bytes]; ok {
		return dep, nil
	}
	return dependencies.SprintManagementWorkItemDependency{}, errors.New("not found")
}

func (m *mockRepository) GetDependenciesBySourceID(ctx context.Context, sourceID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error) {
	pgSourceID := sourceID.(pgtype.UUID)
	var result []dependencies.SprintManagementWorkItemDependency
	for _, dep := range m.dependencies {
		if dep.SourceID.Bytes == pgSourceID.Bytes {
			result = append(result, dep)
		}
	}
	return result, nil
}

func (m *mockRepository) GetDependenciesByTargetID(ctx context.Context, targetID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error) {
	pgTargetID := targetID.(pgtype.UUID)
	var result []dependencies.SprintManagementWorkItemDependency
	for _, dep := range m.dependencies {
		if dep.TargetID.Bytes == pgTargetID.Bytes {
			result = append(result, dep)
		}
	}
	return result, nil
}

func (m *mockRepository) GetAllDependenciesForWorkItem(ctx context.Context, workItemID interface{}) ([]dependencies.SprintManagementWorkItemDependency, error) {
	pgWorkItemID := workItemID.(pgtype.UUID)
	var result []dependencies.SprintManagementWorkItemDependency
	for _, dep := range m.dependencies {
		if dep.SourceID.Bytes == pgWorkItemID.Bytes || dep.TargetID.Bytes == pgWorkItemID.Bytes {
			result = append(result, dep)
		}
	}
	return result, nil
}

func (m *mockRepository) DeleteDependency(ctx context.Context, id interface{}) error {
	pgID := id.(pgtype.UUID)
	if _, ok := m.dependencies[pgID.Bytes]; ok {
		delete(m.dependencies, pgID.Bytes)
		return nil
	}
	return errors.New("not found")
}

func (m *mockRepository) DeleteDependenciesByWorkItemID(ctx context.Context, workItemID interface{}) error {
	pgWorkItemID := workItemID.(pgtype.UUID)
	m.deletedWorkItemID = pgWorkItemID.Bytes

	// Remove all dependencies for this work item
	for id, dep := range m.dependencies {
		if dep.SourceID.Bytes == pgWorkItemID.Bytes || dep.TargetID.Bytes == pgWorkItemID.Bytes {
			delete(m.dependencies, id)
		}
	}
	return nil
}

func (m *mockRepository) HasBlockingDependencies(ctx context.Context, targetID interface{}) (bool, error) {
	pgTargetID := targetID.(pgtype.UUID)
	if hasBlocking, ok := m.hasBlocking[pgTargetID.Bytes]; ok {
		return hasBlocking, nil
	}
	return false, nil
}

func (m *mockRepository) GetBlockingDependencies(ctx context.Context, targetID interface{}) ([]dependencies.GetBlockingDependenciesRow, error) {
	pgTargetID := targetID.(pgtype.UUID)
	var result []dependencies.GetBlockingDependenciesRow
	for _, dep := range m.dependencies {
		if dep.TargetID.Bytes == pgTargetID.Bytes && dep.DependencyType == dependencies.SprintManagementDependencyTypeBlocks {
			result = append(result, dependencies.GetBlockingDependenciesRow{
				ID:             dep.ID,
				SourceID:       dep.SourceID,
				TargetID:       dep.TargetID,
				DependencyType: dep.DependencyType,
				CreatedAt:      dep.CreatedAt,
				CreatedBy:      dep.CreatedBy,
			})
		}
	}
	return result, nil
}

func (m *mockRepository) DependencyExists(ctx context.Context, params dependencies.DependencyExistsParams) (bool, error) {
	key := uuid.UUID(params.SourceID.Bytes).String() + uuid.UUID(params.TargetID.Bytes).String() + string(params.DependencyType)
	if exists, ok := m.existingDeps[key]; ok {
		return exists, nil
	}
	return false, nil
}
