package burndown

import (
	"context"
	"log/slog"
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

// Mock repositories for testing
type mockSprintRepo struct {
	sprints          map[uuid.UUID]sprints.SprintManagementSprint
	completedSprints []sprints.SprintManagementSprint
}

func (m *mockSprintRepo) GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error) {
	sprintID := pgtypeToUUID(id)
	if sprint, ok := m.sprints[sprintID]; ok {
		return sprint, nil
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepo) ListCompletedSprints(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error) {
	if int(limit) > len(m.completedSprints) {
		return m.completedSprints, nil
	}
	return m.completedSprints[:limit], nil
}

type mockWorkItemRepo struct {
	workItems map[uuid.UUID][]workitems.SprintManagementWorkItem
}

func (m *mockWorkItemRepo) ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error) {
	sid := pgtypeToUUID(sprintID)
	if items, ok := m.workItems[sid]; ok {
		return items, nil
	}
	return []workitems.SprintManagementWorkItem{}, nil
}

// Test Property 20: Burndown Calculation Accuracy
// Feature: go-sprint-management, Property 20: For any active sprint,
// the burndown chart data should accurately reflect remaining work over time based on completed work items.
// Validates: Requirements 29.3
func TestProperty20_BurndownCalculationAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("burndown accurately reflects remaining work", prop.ForAll(
		func(totalPoints int, completedPoints int, daysInSprint int) bool {
			if totalPoints <= 0 || completedPoints < 0 || completedPoints > totalPoints || daysInSprint <= 0 {
				return true // Skip invalid cases
			}

			// Create an active sprint
			sprintID := uuid.New()
			startDate := time.Now().AddDate(0, 0, -daysInSprint/2) // Sprint is halfway through
			endDate := startDate.AddDate(0, 0, daysInSprint)

			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Test Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(startDate),
				EndDate:   timeToPgDate(endDate),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			// Create work items
			items := []workitems.SprintManagementWorkItem{}
			remainingToAllocate := totalPoints
			completedToAllocate := completedPoints

			// Create completed items
			for completedToAllocate > 0 {
				points := 1
				if completedToAllocate >= 5 {
					points = 5
				} else {
					points = completedToAllocate
				}

				sp := int32(points)
				items = append(items, workitems.SprintManagementWorkItem{
					ID:          uuidToPgtype(uuid.New()),
					Type:        workitems.SprintManagementWorkItemTypeStory,
					Title:       "Completed Item",
					Status:      workitems.SprintManagementWorkItemStatusDone,
					Priority:    workitems.SprintManagementPriorityLevelMedium,
					StoryPoints: &sp,
					SprintID:    uuidToPgtype(sprintID),
					ReporterID:  uuidToPgtype(uuid.New()),
					CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				})

				completedToAllocate -= points
				remainingToAllocate -= points
			}

			// Create remaining items
			for remainingToAllocate > 0 {
				points := 1
				if remainingToAllocate >= 5 {
					points = 5
				} else {
					points = remainingToAllocate
				}

				sp := int32(points)
				items = append(items, workitems.SprintManagementWorkItem{
					ID:          uuidToPgtype(uuid.New()),
					Type:        workitems.SprintManagementWorkItemTypeStory,
					Title:       "Remaining Item",
					Status:      workitems.SprintManagementWorkItemStatusTodo,
					Priority:    workitems.SprintManagementPriorityLevelMedium,
					StoryPoints: &sp,
					SprintID:    uuidToPgtype(sprintID),
					ReporterID:  uuidToPgtype(uuid.New()),
					CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				})

				remainingToAllocate -= points
			}

			// Create service with mocks
			sprintRepo := &mockSprintRepo{
				sprints: map[uuid.UUID]sprints.SprintManagementSprint{
					sprintID: sprint,
				},
			}
			workItemRepo := &mockWorkItemRepo{
				workItems: map[uuid.UUID][]workitems.SprintManagementWorkItem{
					sprintID: items,
				},
			}

			service := NewService(sprintRepo, workItemRepo, slog.Default())

			// Get burndown chart
			ctx := context.Background()
			chart, err := service.GetBurndownChart(ctx, sprintID)
			if err != nil {
				t.Logf("Error getting burndown chart: %v", err)
				return false
			}

			// Verify total points
			if chart.TotalPoints != totalPoints {
				t.Logf("Expected total points %d, got %d", totalPoints, chart.TotalPoints)
				return false
			}

			// Verify completed points
			if chart.CompletedPoints != completedPoints {
				t.Logf("Expected completed points %d, got %d", completedPoints, chart.CompletedPoints)
				return false
			}

			// Verify current (remaining) points
			expectedRemaining := totalPoints - completedPoints
			if chart.CurrentPoints != expectedRemaining {
				t.Logf("Expected remaining points %d, got %d", expectedRemaining, chart.CurrentPoints)
				return false
			}

			// Verify data points exist
			if len(chart.DataPoints) == 0 {
				t.Log("No data points generated")
				return false
			}

			// Verify first data point has total points remaining
			firstPoint := chart.DataPoints[0]
			if firstPoint.IdealRemaining != totalPoints {
				t.Logf("Expected first ideal remaining %d, got %d", totalPoints, firstPoint.IdealRemaining)
				return false
			}

			// Verify last data point has zero ideal remaining
			lastPoint := chart.DataPoints[len(chart.DataPoints)-1]
			if lastPoint.IdealRemaining != 0 {
				t.Logf("Expected last ideal remaining 0, got %d", lastPoint.IdealRemaining)
				return false
			}

			// Verify ideal burndown decreases monotonically
			for i := 1; i < len(chart.DataPoints); i++ {
				if chart.DataPoints[i].IdealRemaining > chart.DataPoints[i-1].IdealRemaining {
					t.Logf("Ideal remaining increased from %d to %d",
						chart.DataPoints[i-1].IdealRemaining,
						chart.DataPoints[i].IdealRemaining)
					return false
				}
			}

			return true
		},
		gen.IntRange(10, 100), // total points
		gen.IntRange(0, 50),   // completed points (will be capped by total)
		gen.IntRange(7, 21),   // days in sprint (1-3 weeks)
	))

	properties.TestingRun(t)
}

// Test burndown calculation with edge cases
func TestBurndownCalculation_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		totalPoints     int
		completedPoints int
		daysInSprint    int
		wantError       bool
	}{
		{
			name:            "all work completed",
			totalPoints:     50,
			completedPoints: 50,
			daysInSprint:    14,
			wantError:       false,
		},
		{
			name:            "no work completed",
			totalPoints:     50,
			completedPoints: 0,
			daysInSprint:    14,
			wantError:       false,
		},
		{
			name:            "single day sprint",
			totalPoints:     10,
			completedPoints: 5,
			daysInSprint:    1,
			wantError:       false,
		},
		{
			name:            "zero points sprint",
			totalPoints:     0,
			completedPoints: 0,
			daysInSprint:    14,
			wantError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an active sprint
			sprintID := uuid.New()
			startDate := time.Now().AddDate(0, 0, -tt.daysInSprint/2)
			endDate := startDate.AddDate(0, 0, tt.daysInSprint)

			sprint := sprints.SprintManagementSprint{
				ID:        uuidToPgtype(sprintID),
				Name:      "Test Sprint",
				Status:    sprints.SprintManagementSprintStatusActive,
				StartDate: timeToPgDate(startDate),
				EndDate:   timeToPgDate(endDate),
				CreatedBy: uuidToPgtype(uuid.New()),
				CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			// Create work items
			items := []workitems.SprintManagementWorkItem{}
			if tt.totalPoints > 0 {
				// Add completed items
				if tt.completedPoints > 0 {
					sp := int32(tt.completedPoints)
					items = append(items, workitems.SprintManagementWorkItem{
						ID:          uuidToPgtype(uuid.New()),
						Type:        workitems.SprintManagementWorkItemTypeStory,
						Title:       "Completed Item",
						Status:      workitems.SprintManagementWorkItemStatusDone,
						Priority:    workitems.SprintManagementPriorityLevelMedium,
						StoryPoints: &sp,
						SprintID:    uuidToPgtype(sprintID),
						ReporterID:  uuidToPgtype(uuid.New()),
						CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
						UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					})
				}

				// Add remaining items
				remaining := tt.totalPoints - tt.completedPoints
				if remaining > 0 {
					sp := int32(remaining)
					items = append(items, workitems.SprintManagementWorkItem{
						ID:          uuidToPgtype(uuid.New()),
						Type:        workitems.SprintManagementWorkItemTypeStory,
						Title:       "Remaining Item",
						Status:      workitems.SprintManagementWorkItemStatusTodo,
						Priority:    workitems.SprintManagementPriorityLevelMedium,
						StoryPoints: &sp,
						SprintID:    uuidToPgtype(sprintID),
						ReporterID:  uuidToPgtype(uuid.New()),
						CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
						UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					})
				}
			}

			// Create service with mocks
			sprintRepo := &mockSprintRepo{
				sprints: map[uuid.UUID]sprints.SprintManagementSprint{
					sprintID: sprint,
				},
			}
			workItemRepo := &mockWorkItemRepo{
				workItems: map[uuid.UUID][]workitems.SprintManagementWorkItem{
					sprintID: items,
				},
			}

			service := NewService(sprintRepo, workItemRepo, slog.Default())

			// Get burndown chart
			ctx := context.Background()
			chart, err := service.GetBurndownChart(ctx, sprintID)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify basic properties
			if chart.TotalPoints != tt.totalPoints {
				t.Errorf("Expected total points %d, got %d", tt.totalPoints, chart.TotalPoints)
			}

			if chart.CompletedPoints != tt.completedPoints {
				t.Errorf("Expected completed points %d, got %d", tt.completedPoints, chart.CompletedPoints)
			}

			expectedRemaining := tt.totalPoints - tt.completedPoints
			if chart.CurrentPoints != expectedRemaining {
				t.Errorf("Expected remaining points %d, got %d", expectedRemaining, chart.CurrentPoints)
			}

			// Verify data points
			if len(chart.DataPoints) == 0 {
				t.Error("No data points generated")
			}
		})
	}
}

// Helper functions
func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: !t.IsZero(),
	}
}
