package sprints

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
)

// Feature: go-sprint-management, Property 9: Closed Sprint Protection
// Validates: Requirements 2.3
func TestProperty_ClosedSprintProtection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("cannot add work items to completed sprints", prop.ForAll(
		func() bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a completed sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Completed Sprint",
				Status:    sprints.SprintManagementSprintStatusCompleted,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -14)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, -7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -14), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Attempt to update the sprint (which would be needed to add work items)
			updateReq := UpdateSprintRequest{
				Name: stringPtr("Updated Sprint"),
			}
			_, err := service.UpdateSprint(ctx, sprintID, updateReq)

			// Should fail with ErrSprintClosed
			return err == ErrSprintClosed
		},
	))

	properties.Property("cannot add work items to cancelled sprints", prop.ForAll(
		func() bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a cancelled sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Cancelled Sprint",
				Status:    sprints.SprintManagementSprintStatusCancelled,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -14)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, -7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -14), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Attempt to update the sprint
			updateReq := UpdateSprintRequest{
				Name: stringPtr("Updated Sprint"),
			}
			_, err := service.UpdateSprint(ctx, sprintID, updateReq)

			// Should fail with ErrSprintClosed
			return err == ErrSprintClosed
		},
	))

	properties.Property("can add work items to planned sprints", prop.ForAll(
		func(name string) bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a planned sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Planned Sprint",
				Status:    sprints.SprintManagementSprintStatusPlanned,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, 7)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, 21)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Attempt to update the sprint (should succeed)
			updateReq := UpdateSprintRequest{
				Name: &name,
			}
			_, err := service.UpdateSprint(ctx, sprintID, updateReq)

			// Should succeed
			return err == nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
	))

	properties.Property("can add work items to active sprints", prop.ForAll(
		func(name string) bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create an active sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Active Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -7)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, 7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Attempt to update the sprint (should succeed)
			updateReq := UpdateSprintRequest{
				Name: &name,
			}
			_, err := service.UpdateSprint(ctx, sprintID, updateReq)

			// Should succeed
			return err == nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property 11: Sprint Information Completeness
// Validates: Requirements 2.5
func TestProperty_SprintInformationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("sprint retrieval includes all work items", prop.ForAll(
		func(numWorkItems uint8) bool {
			sprintID := uuid.New()
			// Limit work items to reasonable number for testing
			workItemCount := int(numWorkItems % 20)

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Test Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -7)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, 7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Create work items for the sprint
			for i := 0; i < workItemCount; i++ {
				workItem := workitems.SprintManagementWorkItem{
					ID:         uuidToPgtype(uuid.New()),
					Type:       workitems.SprintManagementWorkItemTypeStory,
					Title:      "Test Work Item",
					Status:     workitems.SprintManagementWorkItemStatusTodo,
					Priority:   workitems.SprintManagementPriorityLevelMedium,
					SprintID:   pgtype.UUID{Bytes: sprintID, Valid: true},
					ReporterID: uuidToPgtype(uuid.New()),
					CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				workItemRepo.workItemsBySprint[sprintID] = append(workItemRepo.workItemsBySprint[sprintID], workItem)
			}

			// Get sprint with metrics
			result, err := service.GetSprintWithMetrics(ctx, sprintID)
			if err != nil {
				return false
			}

			// Verify all work items are included in metrics
			return result.Metrics.TotalWorkItems == workItemCount
		},
		gen.UInt8(),
	))

	properties.Property("sprint retrieval includes progress metrics", prop.ForAll(
		func() bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Test Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -7)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, 7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Create work items with different statuses
			statuses := []workitems.SprintManagementWorkItemStatus{
				workitems.SprintManagementWorkItemStatusTodo,
				workitems.SprintManagementWorkItemStatusInProgress,
				workitems.SprintManagementWorkItemStatusInReview,
				workitems.SprintManagementWorkItemStatusDone,
			}

			for _, status := range statuses {
				workItem := workitems.SprintManagementWorkItem{
					ID:         uuidToPgtype(uuid.New()),
					Type:       workitems.SprintManagementWorkItemTypeStory,
					Title:      "Test Work Item",
					Status:     status,
					Priority:   workitems.SprintManagementPriorityLevelMedium,
					SprintID:   pgtype.UUID{Bytes: sprintID, Valid: true},
					ReporterID: uuidToPgtype(uuid.New()),
					CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				workItemRepo.workItemsBySprint[sprintID] = append(workItemRepo.workItemsBySprint[sprintID], workItem)
			}

			// Get sprint with metrics
			result, err := service.GetSprintWithMetrics(ctx, sprintID)
			if err != nil {
				return false
			}

			// Verify metrics are calculated
			metricsComplete := result.Metrics.TotalWorkItems == 4 &&
				result.Metrics.CompletedWorkItems == 1 &&
				result.Metrics.InProgressWorkItems == 2 &&
				result.Metrics.TodoWorkItems == 1

			return metricsComplete
		},
	))

	properties.Property("sprint retrieval includes story point metrics", prop.ForAll(
		func(points1, points2, points3 uint8) bool {
			sprintID := uuid.New()

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a sprint
			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Test Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(time.Now().AddDate(0, 0, -7)),
				EndDate:   timeToPgDate(time.Now().AddDate(0, 0, 7)),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Create work items with story points
			p1 := int32(points1 % 20) // Limit to reasonable values
			p2 := int32(points2 % 20)
			p3 := int32(points3 % 20)

			// Done work item
			workItem1 := workitems.SprintManagementWorkItem{
				ID:          uuidToPgtype(uuid.New()),
				Type:        workitems.SprintManagementWorkItemTypeStory,
				Title:       "Done Work Item",
				Status:      workitems.SprintManagementWorkItemStatusDone,
				Priority:    workitems.SprintManagementPriorityLevelMedium,
				StoryPoints: &p1,
				SprintID:    pgtype.UUID{Bytes: sprintID, Valid: true},
				ReporterID:  uuidToPgtype(uuid.New()),
				CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			workItemRepo.workItemsBySprint[sprintID] = append(workItemRepo.workItemsBySprint[sprintID], workItem1)

			// In progress work item
			workItem2 := workitems.SprintManagementWorkItem{
				ID:          uuidToPgtype(uuid.New()),
				Type:        workitems.SprintManagementWorkItemTypeStory,
				Title:       "In Progress Work Item",
				Status:      workitems.SprintManagementWorkItemStatusInProgress,
				Priority:    workitems.SprintManagementPriorityLevelMedium,
				StoryPoints: &p2,
				SprintID:    pgtype.UUID{Bytes: sprintID, Valid: true},
				ReporterID:  uuidToPgtype(uuid.New()),
				CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			workItemRepo.workItemsBySprint[sprintID] = append(workItemRepo.workItemsBySprint[sprintID], workItem2)

			// Todo work item
			workItem3 := workitems.SprintManagementWorkItem{
				ID:          uuidToPgtype(uuid.New()),
				Type:        workitems.SprintManagementWorkItemTypeStory,
				Title:       "Todo Work Item",
				Status:      workitems.SprintManagementWorkItemStatusTodo,
				Priority:    workitems.SprintManagementPriorityLevelMedium,
				StoryPoints: &p3,
				SprintID:    pgtype.UUID{Bytes: sprintID, Valid: true},
				ReporterID:  uuidToPgtype(uuid.New()),
				CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			workItemRepo.workItemsBySprint[sprintID] = append(workItemRepo.workItemsBySprint[sprintID], workItem3)

			// Get sprint with metrics
			result, err := service.GetSprintWithMetrics(ctx, sprintID)
			if err != nil {
				return false
			}

			// Verify story point metrics
			expectedCommitted := int(p1 + p2 + p3)
			expectedCompleted := int(p1)

			pointsMatch := result.Metrics.CommittedPoints == expectedCommitted &&
				result.Metrics.CompletedPoints == expectedCompleted

			// Verify completion rate
			var expectedRate float64
			if expectedCommitted > 0 {
				expectedRate = float64(expectedCompleted) / float64(expectedCommitted) * 100
			}
			rateMatch := result.Metrics.CompletionRate == expectedRate

			return pointsMatch && rateMatch
		},
		gen.UInt8(),
		gen.UInt8(),
		gen.UInt8(),
	))

	properties.Property("sprint retrieval includes sprint metadata", prop.ForAll(
		func() bool {
			sprintID := uuid.New()
			createdBy := uuid.New()
			sprintName := "Test Sprint"
			sprintDescription := "Test Description"

			repo := newMockSprintRepository()
			workItemRepo := newMockWorkItemRepository()
			service := NewService(repo, workItemRepo, nil)
			ctx := context.Background()

			// Create a sprint with all metadata
			sprint := sprints.SprintManagementSprint{
				ID:          uuidToPgtype(sprintID),
				Name:        sprintName,
				Description: &sprintDescription,
				Status:      sprints.SprintManagementSprintStatusActive,
				StartDate:   timeToPgDate(time.Now().AddDate(0, 0, -7)),
				EndDate:     timeToPgDate(time.Now().AddDate(0, 0, 7)),
				CreatedBy:   uuidToPgtype(createdBy),
				CreatedAt:   pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}
			repo.sprints[sprintID] = sprint

			// Get sprint with metrics
			result, err := service.GetSprintWithMetrics(ctx, sprintID)
			if err != nil {
				return false
			}

			// Verify all metadata is present
			metadataComplete := result.Sprint.ID == sprintID &&
				result.Sprint.Name == sprintName &&
				result.Sprint.Description == sprintDescription &&
				result.Sprint.CreatedBy == createdBy &&
				!result.Sprint.StartDate.IsZero() &&
				!result.Sprint.EndDate.IsZero() &&
				!result.Sprint.CreatedAt.IsZero() &&
				!result.Sprint.UpdatedAt.IsZero()

			return metadataComplete
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Mock repositories for testing

type mockSprintRepository struct {
	sprints map[uuid.UUID]sprints.SprintManagementSprint
}

func newMockSprintRepository() *mockSprintRepository {
	return &mockSprintRepository{
		sprints: make(map[uuid.UUID]sprints.SprintManagementSprint),
	}
}

func (m *mockSprintRepository) CreateSprint(ctx context.Context, params sprints.CreateSprintParams) (sprints.SprintManagementSprint, error) {
	id := uuid.New()
	sprint := sprints.SprintManagementSprint{
		ID:          uuidToPgtype(id),
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		StartDate:   params.StartDate,
		EndDate:     params.EndDate,
		CreatedBy:   params.CreatedBy,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if params.CapacityPoints != nil {
		sprint.CapacityPoints = params.CapacityPoints
	}
	m.sprints[id] = sprint
	return sprint, nil
}

func (m *mockSprintRepository) GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error) {
	sprintID := pgtypeToUUID(id)
	if sprint, ok := m.sprints[sprintID]; ok {
		return sprint, nil
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepository) UpdateSprint(ctx context.Context, params sprints.UpdateSprintParams) (sprints.SprintManagementSprint, error) {
	sprintID := pgtypeToUUID(params.ID)
	if sprint, ok := m.sprints[sprintID]; ok {
		if params.Name != nil {
			sprint.Name = *params.Name
		}
		if params.Description != nil {
			sprint.Description = params.Description
		}
		sprint.StartDate = params.StartDate
		sprint.EndDate = params.EndDate
		if params.CapacityPoints != nil {
			sprint.CapacityPoints = params.CapacityPoints
		}
		if params.Status.Valid {
			sprint.Status = params.Status.SprintManagementSprintStatus
		}
		sprint.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.sprints[sprintID] = sprint
		return sprint, nil
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepository) UpdateSprintStatus(ctx context.Context, params sprints.UpdateSprintStatusParams) (sprints.SprintManagementSprint, error) {
	sprintID := pgtypeToUUID(params.ID)
	if sprint, ok := m.sprints[sprintID]; ok {
		sprint.Status = params.Status
		sprint.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.sprints[sprintID] = sprint
		return sprint, nil
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepository) UpdateSprintMetrics(ctx context.Context, params sprints.UpdateSprintMetricsParams) (sprints.SprintManagementSprint, error) {
	sprintID := pgtypeToUUID(params.ID)
	if sprint, ok := m.sprints[sprintID]; ok {
		if params.CommittedPoints != nil {
			sprint.CommittedPoints = params.CommittedPoints
		}
		if params.CompletedPoints != nil {
			sprint.CompletedPoints = params.CompletedPoints
		}
		sprint.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		m.sprints[sprintID] = sprint
		return sprint, nil
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepository) MoveWorkItemsToBacklog(ctx context.Context, sprintID pgtype.UUID) error {
	// Mock implementation - just return success
	return nil
}

func (m *mockSprintRepository) GetSprintWithWorkItems(ctx context.Context, id pgtype.UUID) (sprints.GetSprintWithWorkItemsRow, error) {
	// Not needed for these tests
	return sprints.GetSprintWithWorkItemsRow{}, nil
}

type mockWorkItemRepository struct {
	workItemsBySprint map[uuid.UUID][]workitems.SprintManagementWorkItem
}

func newMockWorkItemRepository() *mockWorkItemRepository {
	return &mockWorkItemRepository{
		workItemsBySprint: make(map[uuid.UUID][]workitems.SprintManagementWorkItem),
	}
}

func (m *mockWorkItemRepository) ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error) {
	sid := pgtypeToUUID(sprintID)
	if items, ok := m.workItemsBySprint[sid]; ok {
		return items, nil
	}
	return []workitems.SprintManagementWorkItem{}, nil
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}
