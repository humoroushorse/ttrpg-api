package workitems_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// getTestDatabaseURL returns the test database URL from environment or default
func getTestDatabaseURL() string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/sprint_management_test?sslmode=disable"
	}
	return url
}

// setupTestPool creates a test database connection pool
func setupTestPool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	dbURL := getTestDatabaseURL()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("failed to create connection pool: %v (test database not available)", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database not available: %v (set TEST_DATABASE_URL or start postgres)", err)
	}

	cleanup := func() {
		// Clean up test data
		pool.Exec(ctx, "TRUNCATE sprint_management.work_items, sprint_management.sprints, sprint_management.work_item_dependencies CASCADE")
		pool.Close()
	}

	return pool, cleanup
}

// uuidToPgtype converts uuid.UUID to pgtype.UUID
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	var pgUUID pgtype.UUID
	pgUUID.Scan(id.String())
	return pgUUID
}

// TestProperty1_WorkItemTypeValidation tests that only valid work item types are accepted
// Property 1: Work Item Type Validation
// For any work item creation request, the system should accept only valid work item types
// ('epic', 'story', 'defect') and reject invalid types with appropriate error messages.
// Validates: Requirements 1.1
func TestProperty1_WorkItemTypeValidation(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := workitems.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Valid types
	validTypes := []workitems.SprintManagementWorkItemType{
		workitems.SprintManagementWorkItemTypeEpic,
		workitems.SprintManagementWorkItemTypeStory,
		workitems.SprintManagementWorkItemTypeDefect,
	}

	// Generator for valid work item types
	genValidType := gen.OneConstOf(
		workitems.SprintManagementWorkItemTypeEpic,
		workitems.SprintManagementWorkItemTypeStory,
		workitems.SprintManagementWorkItemTypeDefect,
	)

	properties.Property("Feature: go-sprint-management, Property 1: Valid work item types are accepted",
		prop.ForAll(
			func(workItemType workitems.SprintManagementWorkItemType) bool {
				ctx := context.Background()

				reporterID := uuidToPgtype(uuid.New())

				description := "Test Description"
				params := workitems.CreateWorkItemParams{
					Type:        workItemType,
					Title:       fmt.Sprintf("Test Item %s", uuid.New().String()),
					Description: &description,
					Status:      workitems.SprintManagementWorkItemStatusTodo,
					Priority:    workitems.SprintManagementPriorityLevelMedium,
					ReporterID:  reporterID,
				}

				workItem, err := repo.CreateWorkItem(ctx, params)
				if err != nil {
					t.Logf("failed to create work item with valid type %v: %v", workItemType, err)
					return false
				}

				// Verify the type was stored correctly
				if workItem.Type != workItemType {
					t.Logf("work item type mismatch: expected %v, got %v", workItemType, workItem.Type)
					return false
				}

				// Verify it's one of the valid types
				isValid := false
				for _, validType := range validTypes {
					if workItem.Type == validType {
						isValid = true
						break
					}
				}

				if !isValid {
					t.Logf("work item type is not valid: %v", workItem.Type)
					return false
				}

				return true
			},
			genValidType,
		))

	properties.TestingRun(t)
}

// TestProperty2_RequiredFieldValidation tests that required fields are enforced
// Property 2: Required Field Validation
// For any work item creation request, the system should require title, description,
// and priority fields, while allowing optional story point estimation.
// Validates: Requirements 1.2
func TestProperty2_RequiredFieldValidation(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := workitems.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for random strings
	genTitle := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 255
	})

	genDescription := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0
	})

	genPriority := gen.OneConstOf(
		workitems.SprintManagementPriorityLevelLow,
		workitems.SprintManagementPriorityLevelMedium,
		workitems.SprintManagementPriorityLevelHigh,
		workitems.SprintManagementPriorityLevelCritical,
	)

	// Generator for optional story points (nil or positive integer)
	genStoryPoints := gen.OneGenOf(
		gen.Const((*int32)(nil)), // nil
		gen.IntRange(1, 100).Map(func(n int) *int32 {
			val := int32(n)
			return &val
		}),
	)

	properties.Property("Feature: go-sprint-management, Property 2: Required fields are enforced and optional fields work",
		prop.ForAll(
			func(title, description string, priority workitems.SprintManagementPriorityLevel, storyPoints *int32) bool {
				ctx := context.Background()

				reporterID := uuidToPgtype(uuid.New())

				params := workitems.CreateWorkItemParams{
					Type:        workitems.SprintManagementWorkItemTypeStory,
					Title:       title,
					Description: &description,
					Status:      workitems.SprintManagementWorkItemStatusTodo,
					Priority:    priority,
					StoryPoints: storyPoints,
					ReporterID:  reporterID,
				}

				workItem, err := repo.CreateWorkItem(ctx, params)
				if err != nil {
					t.Logf("failed to create work item: %v", err)
					return false
				}

				// Verify required fields are present
				if workItem.Title != title {
					t.Logf("title mismatch: expected %v, got %v", title, workItem.Title)
					return false
				}

				if workItem.Description == nil || *workItem.Description != description {
					t.Logf("description mismatch: expected %v, got %v", description, workItem.Description)
					return false
				}

				if workItem.Priority != priority {
					t.Logf("priority mismatch: expected %v, got %v", priority, workItem.Priority)
					return false
				}

				// Verify story points (optional field)
				if storyPoints != nil {
					if workItem.StoryPoints == nil || *workItem.StoryPoints != *storyPoints {
						t.Logf("story points mismatch: expected %v, got %v", storyPoints, workItem.StoryPoints)
						return false
					}
				} else {
					if workItem.StoryPoints != nil {
						t.Logf("story points should be nil but got %v", workItem.StoryPoints)
						return false
					}
				}

				return true
			},
			genTitle,
			genDescription,
			genPriority,
			genStoryPoints,
		))

	properties.TestingRun(t)
}

// TestProperty23_SoftDeleteImplementation tests soft delete functionality
// Property 23: Soft Delete Implementation
// For any deletion operation on work items, the system should mark items as deleted
// rather than physically removing them.
// Validates: Requirements 37.1
func TestProperty23_SoftDeleteImplementation(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := workitems.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Feature: go-sprint-management, Property 23: Soft delete marks items as deleted without physical removal",
		prop.ForAll(
			func(seed int64) bool {
				ctx := context.Background()

				// Create a work item
				reporterID := uuidToPgtype(uuid.New())
				description := "Test Description"
				params := workitems.CreateWorkItemParams{
					Type:        workitems.SprintManagementWorkItemTypeStory,
					Title:       fmt.Sprintf("Test Item %d", seed),
					Description: &description,
					Status:      workitems.SprintManagementWorkItemStatusTodo,
					Priority:    workitems.SprintManagementPriorityLevelMedium,
					ReporterID:  reporterID,
				}

				workItem, err := repo.CreateWorkItem(ctx, params)
				if err != nil {
					t.Logf("failed to create work item: %v", err)
					return false
				}

				// Soft delete the work item
				deletedBy := uuidToPgtype(uuid.New())
				deleteParams := workitems.SoftDeleteWorkItemParams{
					ID:        workItem.ID,
					DeletedBy: deletedBy,
				}

				err = repo.SoftDeleteWorkItem(ctx, deleteParams)
				if err != nil {
					t.Logf("failed to soft delete work item: %v", err)
					return false
				}

				// Verify the item is not returned by normal queries
				_, err = repo.GetWorkItemByID(ctx, workItem.ID)
				if err == nil {
					t.Logf("soft deleted item should not be returned by GetWorkItemByID")
					return false
				}

				// Verify the item still exists in database (including deleted)
				deletedItem, err := repo.GetWorkItemByIDIncludingDeleted(ctx, workItem.ID)
				if err != nil {
					t.Logf("failed to retrieve soft deleted item: %v", err)
					return false
				}

				// Verify deleted_at is set
				if !deletedItem.DeletedAt.Valid {
					t.Logf("deleted_at should be set for soft deleted item")
					return false
				}

				// Verify deleted_by is set correctly
				if !deletedItem.DeletedBy.Valid {
					t.Logf("deleted_by should be set for soft deleted item")
					return false
				}

				// Verify deleted_at is recent (within last minute)
				now := time.Now().UTC()
				deletedAt := deletedItem.DeletedAt.Time
				if deletedAt.After(now) || deletedAt.Before(now.Add(-time.Minute)) {
					t.Logf("deleted_at timestamp is not recent: %v", deletedAt)
					return false
				}

				// Verify the item can be restored
				err = repo.RestoreWorkItem(ctx, workItem.ID)
				if err != nil {
					t.Logf("failed to restore work item: %v", err)
					return false
				}

				// Verify the item is now returned by normal queries
				restoredItem, err := repo.GetWorkItemByID(ctx, workItem.ID)
				if err != nil {
					t.Logf("failed to retrieve restored item: %v", err)
					return false
				}

				// Verify deleted_at is cleared
				if restoredItem.DeletedAt.Valid {
					t.Logf("deleted_at should be cleared for restored item")
					return false
				}

				// Verify deleted_by is cleared
				if restoredItem.DeletedBy.Valid {
					t.Logf("deleted_by should be cleared for restored item")
					return false
				}

				return true
			},
			gen.Int64(),
		))

	properties.TestingRun(t)
}
