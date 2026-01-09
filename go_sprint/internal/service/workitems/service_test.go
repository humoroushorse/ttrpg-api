package workitems

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"

	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

// Feature: go-sprint-management, Property 3: Parent-Child Relationship Validation
// Validates: Requirements 1.3
func TestProperty_ParentChildRelationshipValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("stories can only have epic parents", prop.ForAll(
		func() bool {
			epicID := uuid.New()
			storyParentID := uuid.New()

			// Setup: Create mock repository with an epic and a story
			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create an epic
			epic := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(epicID),
				Type:       workitems.SprintManagementWorkItemTypeEpic,
				Title:      "Test Epic",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[epicID] = epic

			// Create a story (potential parent for our test story)
			story := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(storyParentID),
				Type:       workitems.SprintManagementWorkItemTypeStory,
				Title:      "Test Story Parent",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[storyParentID] = story

			// Test: Story with epic parent should succeed
			err := service.validateParentChildRelationship(ctx, models.WorkItemTypeStory, epicID)
			epicParentValid := err == nil

			// Test: Story with story parent should fail
			err = service.validateParentChildRelationship(ctx, models.WorkItemTypeStory, storyParentID)
			storyParentInvalid := err != nil

			return epicParentValid && storyParentInvalid
		},
	))

	properties.Property("epics cannot have parents", prop.ForAll(
		func() bool {
			parentID := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create a parent work item (any type)
			parent := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(parentID),
				Type:       workitems.SprintManagementWorkItemTypeEpic,
				Title:      "Test Parent",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[parentID] = parent

			// Test: Epic with any parent should fail
			err := service.validateParentChildRelationship(ctx, models.WorkItemTypeEpic, parentID)
			return err != nil
		},
	))

	properties.Property("defects can have epic or story parents", prop.ForAll(
		func() bool {
			epicID := uuid.New()
			storyID := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create an epic
			epic := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(epicID),
				Type:       workitems.SprintManagementWorkItemTypeEpic,
				Title:      "Test Epic",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[epicID] = epic

			// Create a story
			story := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(storyID),
				Type:       workitems.SprintManagementWorkItemTypeStory,
				Title:      "Test Story",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[storyID] = story

			// Test: Defect with epic parent should succeed
			err := service.validateParentChildRelationship(ctx, models.WorkItemTypeDefect, epicID)
			epicParentValid := err == nil

			// Test: Defect with story parent should succeed
			err = service.validateParentChildRelationship(ctx, models.WorkItemTypeDefect, storyID)
			storyParentValid := err == nil

			return epicParentValid && storyParentValid
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property 5: Status Transition Validation
// Validates: Requirements 1.5
func TestProperty_StatusTransitionValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("valid status transitions are allowed", prop.ForAll(
		func() bool {
			workItemID := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Test valid transitions
			validTransitions := []struct {
				from models.WorkItemStatus
				to   models.WorkItemStatus
			}{
				{models.WorkItemStatusTodo, models.WorkItemStatusInProgress},
				{models.WorkItemStatusInProgress, models.WorkItemStatusInReview},
				{models.WorkItemStatusInReview, models.WorkItemStatusDone},
				{models.WorkItemStatusBlocked, models.WorkItemStatusTodo},
				{models.WorkItemStatusDone, models.WorkItemStatusInProgress}, // Reopening
			}

			for _, transition := range validTransitions {
				workItem := workitems.SprintManagementWorkItem{
					ID:         uuidToPgtype(workItemID),
					Type:       workitems.SprintManagementWorkItemTypeStory,
					Title:      "Test Work Item",
					Status:     workitems.SprintManagementWorkItemStatus(transition.from),
					Priority:   workitems.SprintManagementPriorityLevelMedium,
					ReporterID: uuidToPgtype(uuid.New()),
					CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}

				err := service.validateStatusTransition(ctx, workItem, transition.to)
				if err != nil {
					return false
				}
			}

			return true
		},
	))

	properties.Property("invalid status transitions are rejected", prop.ForAll(
		func() bool {
			workItemID := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Test invalid transitions
			invalidTransitions := []struct {
				from models.WorkItemStatus
				to   models.WorkItemStatus
			}{
				{models.WorkItemStatusTodo, models.WorkItemStatusDone},       // Skip steps
				{models.WorkItemStatusInProgress, models.WorkItemStatusDone}, // Skip review
				{models.WorkItemStatusDone, models.WorkItemStatusTodo},       // Invalid backward
			}

			for _, transition := range invalidTransitions {
				workItem := workitems.SprintManagementWorkItem{
					ID:         uuidToPgtype(workItemID),
					Type:       workitems.SprintManagementWorkItemTypeStory,
					Title:      "Test Work Item",
					Status:     workitems.SprintManagementWorkItemStatus(transition.from),
					Priority:   workitems.SprintManagementPriorityLevelMedium,
					ReporterID: uuidToPgtype(uuid.New()),
					CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}

				err := service.validateStatusTransition(ctx, workItem, transition.to)
				if err == nil {
					return false // Should have failed
				}
			}

			return true
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property 6: Dependency Prevention on Deletion
// Validates: Requirements 1.6
func TestProperty_DependencyPreventionOnDeletion(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("work items with dependencies cannot be deleted", prop.ForAll(
		func() bool {
			workItemID := uuid.New()
			deletedBy := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create a work item with dependencies
			workItem := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(workItemID),
				Type:       workitems.SprintManagementWorkItemTypeStory,
				Title:      "Test Work Item",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[workItemID] = workItem
			repo.hasDependencies[workItemID] = true

			// Attempt to delete should fail
			err := service.DeleteWorkItem(ctx, workItemID, deletedBy)
			return err == ErrWorkItemHasDependencies
		},
	))

	properties.Property("work items with children cannot be deleted", prop.ForAll(
		func() bool {
			workItemID := uuid.New()
			deletedBy := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create a work item with children
			workItem := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(workItemID),
				Type:       workitems.SprintManagementWorkItemTypeEpic,
				Title:      "Test Epic",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[workItemID] = workItem
			repo.hasChildren[workItemID] = true

			// Attempt to delete should fail
			err := service.DeleteWorkItem(ctx, workItemID, deletedBy)
			return err == ErrWorkItemHasDependencies
		},
	))

	properties.Property("work items without dependencies can be deleted", prop.ForAll(
		func() bool {
			workItemID := uuid.New()
			deletedBy := uuid.New()

			repo := newMockRepository()
			service := NewService(repo, nil)
			ctx := context.Background()

			// Create a work item without dependencies or children
			workItem := workitems.SprintManagementWorkItem{
				ID:         uuidToPgtype(workItemID),
				Type:       workitems.SprintManagementWorkItemTypeStory,
				Title:      "Test Work Item",
				Status:     workitems.SprintManagementWorkItemStatusTodo,
				Priority:   workitems.SprintManagementPriorityLevelMedium,
				ReporterID: uuidToPgtype(uuid.New()),
				CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.workItems[workItemID] = workItem
			repo.hasDependencies[workItemID] = false
			repo.hasChildren[workItemID] = false

			// Deletion should succeed
			err := service.DeleteWorkItem(ctx, workItemID, deletedBy)
			return err == nil
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Mock repository for testing
type mockRepository struct {
	workItems       map[uuid.UUID]workitems.SprintManagementWorkItem
	hasDependencies map[uuid.UUID]bool
	hasChildren     map[uuid.UUID]bool
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		workItems:       make(map[uuid.UUID]workitems.SprintManagementWorkItem),
		hasDependencies: make(map[uuid.UUID]bool),
		hasChildren:     make(map[uuid.UUID]bool),
	}
}

func (m *mockRepository) GetWorkItemByID(ctx context.Context, id pgtype.UUID) (workitems.SprintManagementWorkItem, error) {
	workItemID := pgtypeToUUID(id)
	if wi, ok := m.workItems[workItemID]; ok {
		return wi, nil
	}
	return workitems.SprintManagementWorkItem{}, ErrWorkItemNotFound
}

func (m *mockRepository) CreateWorkItem(ctx context.Context, params workitems.CreateWorkItemParams) (workitems.SprintManagementWorkItem, error) {
	// Not needed for these tests
	return workitems.SprintManagementWorkItem{}, nil
}

func (m *mockRepository) UpdateWorkItem(ctx context.Context, params workitems.UpdateWorkItemParams) (workitems.SprintManagementWorkItem, error) {
	// Not needed for these tests
	return workitems.SprintManagementWorkItem{}, nil
}

func (m *mockRepository) ListWorkItems(ctx context.Context, params workitems.ListWorkItemsParams) ([]workitems.SprintManagementWorkItem, error) {
	// Not needed for these tests
	return nil, nil
}

func (m *mockRepository) HasDependencies(ctx context.Context, id pgtype.UUID) (bool, error) {
	workItemID := pgtypeToUUID(id)
	if hasDeps, ok := m.hasDependencies[workItemID]; ok {
		return hasDeps, nil
	}
	return false, nil
}

func (m *mockRepository) HasChildren(ctx context.Context, id pgtype.UUID) (bool, error) {
	workItemID := pgtypeToUUID(id)
	if hasChild, ok := m.hasChildren[workItemID]; ok {
		return hasChild, nil
	}
	return false, nil
}

func (m *mockRepository) SoftDeleteWorkItem(ctx context.Context, params workitems.SoftDeleteWorkItemParams) error {
	workItemID := pgtypeToUUID(params.ID)
	if wi, ok := m.workItems[workItemID]; ok {
		wi.DeletedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
		wi.DeletedBy = params.DeletedBy
		m.workItems[workItemID] = wi
		return nil
	}
	return ErrWorkItemNotFound
}
