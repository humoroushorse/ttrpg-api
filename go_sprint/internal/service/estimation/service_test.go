package estimation

import (
	"context"
	"log/slog"
	"reflect"
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

// Test Property 18: Story Point Scale Validation
// Feature: go-sprint-management, Property 18: For any work item with story point estimation,
// the system should validate points against the configured scale and reject invalid values.
// Validates: Requirements 29.1
func TestProperty18_StoryPointScaleValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Define the configured scale
	configuredScale := []int{0, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

	properties.Property("story points must be from configured scale", prop.ForAll(
		func(points int) bool {
			// Create service with configured scale
			service := NewService(
				&mockSprintRepo{sprints: make(map[uuid.UUID]sprints.SprintManagementSprint)},
				&mockWorkItemRepo{workItems: make(map[uuid.UUID][]workitems.SprintManagementWorkItem)},
				Config{StoryPointScale: configuredScale},
				slog.Default(),
			)

			ctx := context.Background()
			err := service.ValidateStoryPoints(ctx, points)

			// Check if points are in the scale
			inScale := false
			for _, validPoint := range configuredScale {
				if points == validPoint {
					inScale = true
					break
				}
			}

			// Validation should succeed if and only if points are in scale
			if inScale {
				return err == nil
			}
			return err != nil && err.Error() != ""
		},
		gen.IntRange(-10, 100),
	))

	properties.TestingRun(t)
}

// Test Property 19: Sprint Capacity Tracking
// Feature: go-sprint-management, Property 19: For any sprint with assigned work items,
// the system should accurately track committed story points against sprint capacity.
// Validates: Requirements 29.2
func TestProperty19_SprintCapacityTracking(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("sprint capacity tracking is accurate", prop.ForAll(
		func(capacityPoints int, workItemPoints []int) bool {
			// Create a sprint
			sprintID := uuid.New()
			cp := int32(capacityPoints)
			sprint := sprints.SprintManagementSprint{
				ID:             uuidToPgtype(sprintID),
				Name:           "Test Sprint",
				Status:         sprints.SprintManagementSprintStatusActive,
				CapacityPoints: &cp,
				StartDate:      timeToPgDate(time.Now()),
				EndDate:        timeToPgDate(time.Now().AddDate(0, 0, 14)),
				CreatedBy:      uuidToPgtype(uuid.New()),
				CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
				UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}

			// Create work items with story points
			items := make([]workitems.SprintManagementWorkItem, len(workItemPoints))
			expectedCommitted := 0
			expectedCompleted := 0

			for i, points := range workItemPoints {
				sp := int32(points)
				status := workitems.SprintManagementWorkItemStatusTodo
				if i%2 == 0 { // Half are completed
					status = workitems.SprintManagementWorkItemStatusDone
					expectedCompleted += points
				}
				expectedCommitted += points

				items[i] = workitems.SprintManagementWorkItem{
					ID:          uuidToPgtype(uuid.New()),
					Type:        workitems.SprintManagementWorkItemTypeStory,
					Title:       "Test Item",
					Status:      status,
					Priority:    workitems.SprintManagementPriorityLevelMedium,
					StoryPoints: &sp,
					SprintID:    uuidToPgtype(sprintID),
					ReporterID:  uuidToPgtype(uuid.New()),
					CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
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

			service := NewService(sprintRepo, workItemRepo, Config{}, slog.Default())

			// Get sprint capacity
			ctx := context.Background()
			capacity, err := service.GetSprintCapacity(ctx, sprintID)
			if err != nil {
				return false
			}

			// Verify capacity tracking
			if capacity.CommittedPoints != expectedCommitted {
				t.Logf("Expected committed %d, got %d", expectedCommitted, capacity.CommittedPoints)
				return false
			}

			if capacity.CompletedPoints != expectedCompleted {
				t.Logf("Expected completed %d, got %d", expectedCompleted, capacity.CompletedPoints)
				return false
			}

			if capacity.RemainingPoints != (expectedCommitted - expectedCompleted) {
				t.Logf("Expected remaining %d, got %d", expectedCommitted-expectedCompleted, capacity.RemainingPoints)
				return false
			}

			// Verify utilization rate calculation
			if capacityPoints > 0 {
				expectedUtilization := float64(expectedCommitted) / float64(capacityPoints) * 100
				if capacity.UtilizationRate != expectedUtilization {
					t.Logf("Expected utilization %.2f, got %.2f", expectedUtilization, capacity.UtilizationRate)
					return false
				}
			}

			// Verify completion rate calculation
			if expectedCommitted > 0 {
				expectedCompletion := float64(expectedCompleted) / float64(expectedCommitted) * 100
				if capacity.CompletionRate != expectedCompletion {
					t.Logf("Expected completion %.2f, got %.2f", expectedCompletion, capacity.CompletionRate)
					return false
				}
			}

			return true
		},
		gen.IntRange(10, 100), // capacity points
		gen.SliceOfN(5, gen.IntRange(1, 13)).SuchThat(func(v interface{}) bool {
			slice := v.([]int)
			return len(slice) > 0
		}), // work item points
	))

	properties.TestingRun(t)
}

// Test Property 21: Velocity Calculation Accuracy
// Feature: go-sprint-management, Property 21: For any completed sprint,
// the calculated team velocity should equal the sum of story points for all completed work items.
// Validates: Requirements 29.4
func TestProperty21_VelocityCalculationAccuracy(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("velocity equals sum of completed points", prop.ForAll(
		func(sprintData []struct{ completedPoints int }) bool {
			if len(sprintData) == 0 {
				return true // Skip empty case
			}

			// Create completed sprints
			completedSprints := make([]sprints.SprintManagementSprint, len(sprintData))
			expectedTotal := 0

			for i, data := range sprintData {
				cp := int32(data.completedPoints)
				expectedTotal += data.completedPoints

				completedSprints[i] = sprints.SprintManagementSprint{
					ID:              uuidToPgtype(uuid.New()),
					Name:            "Sprint " + string(rune(i+1)),
					Status:          sprints.SprintManagementSprintStatusCompleted,
					StartDate:       timeToPgDate(time.Now().AddDate(0, 0, -14*(len(sprintData)-i))),
					EndDate:         timeToPgDate(time.Now().AddDate(0, 0, -14*(len(sprintData)-i-1))),
					CompletedPoints: &cp,
					CreatedBy:       uuidToPgtype(uuid.New()),
					CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
			}

			// Create service with mocks
			sprintRepo := &mockSprintRepo{
				sprints:          make(map[uuid.UUID]sprints.SprintManagementSprint),
				completedSprints: completedSprints,
			}
			workItemRepo := &mockWorkItemRepo{
				workItems: make(map[uuid.UUID][]workitems.SprintManagementWorkItem),
			}

			service := NewService(sprintRepo, workItemRepo, Config{}, slog.Default())

			// Calculate velocity
			ctx := context.Background()
			velocity, err := service.CalculateVelocity(ctx, len(sprintData))
			if err != nil {
				t.Logf("Error calculating velocity: %v", err)
				return false
			}

			// Verify total points
			if velocity.TotalPoints != expectedTotal {
				t.Logf("Expected total %d, got %d", expectedTotal, velocity.TotalPoints)
				return false
			}

			// Verify average velocity
			expectedAverage := float64(expectedTotal) / float64(len(sprintData))
			if velocity.AverageVelocity != expectedAverage {
				t.Logf("Expected average %.2f, got %.2f", expectedAverage, velocity.AverageVelocity)
				return false
			}

			// Verify sprint count
			if velocity.TotalSprints != len(sprintData) {
				t.Logf("Expected %d sprints, got %d", len(sprintData), velocity.TotalSprints)
				return false
			}

			return true
		},
		gen.SliceOfN(5, gen.Struct(reflect.TypeOf(struct{ completedPoints int }{}), map[string]gopter.Gen{
			"completedPoints": gen.IntRange(0, 89),
		})).SuchThat(func(v interface{}) bool {
			slice := v.([]struct{ completedPoints int })
			return len(slice) > 0
		}),
	))

	properties.TestingRun(t)
}

// Test Property 22: Forecasting Calculation Consistency
// Feature: go-sprint-management, Property 22: For any forecasting request,
// the predicted sprint capacity should be based on historical velocity data using consistent calculation methods.
// Validates: Requirements 29.5
func TestProperty22_ForecastingCalculationConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("forecasting uses consistent calculation", prop.ForAll(
		func(velocities []int, remainingPoints int) bool {
			if len(velocities) == 0 || remainingPoints <= 0 {
				return true // Skip invalid cases
			}

			// Create completed sprints with given velocities
			completedSprints := make([]sprints.SprintManagementSprint, len(velocities))
			totalVelocity := 0
			minVelocity := velocities[0]
			maxVelocity := velocities[0]

			for i, vel := range velocities {
				cp := int32(vel)
				totalVelocity += vel

				if vel < minVelocity {
					minVelocity = vel
				}
				if vel > maxVelocity {
					maxVelocity = vel
				}

				completedSprints[i] = sprints.SprintManagementSprint{
					ID:              uuidToPgtype(uuid.New()),
					Name:            "Sprint " + string(rune(i+1)),
					Status:          sprints.SprintManagementSprintStatusCompleted,
					StartDate:       timeToPgDate(time.Now().AddDate(0, 0, -14*(len(velocities)-i))),
					EndDate:         timeToPgDate(time.Now().AddDate(0, 0, -14*(len(velocities)-i-1))),
					CompletedPoints: &cp,
					CreatedBy:       uuidToPgtype(uuid.New()),
					CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
			}

			// Create service with mocks
			sprintRepo := &mockSprintRepo{
				sprints:          make(map[uuid.UUID]sprints.SprintManagementSprint),
				completedSprints: completedSprints,
			}
			workItemRepo := &mockWorkItemRepo{
				workItems: make(map[uuid.UUID][]workitems.SprintManagementWorkItem),
			}

			service := NewService(sprintRepo, workItemRepo, Config{}, slog.Default())

			// Calculate forecast
			ctx := context.Background()
			forecast, err := service.ForecastCompletion(ctx, ForecastRequest{
				RemainingPoints: remainingPoints,
				SprintCount:     len(velocities),
			})
			if err != nil {
				t.Logf("Error calculating forecast: %v", err)
				return false
			}

			// Verify average velocity is used
			expectedAverage := float64(totalVelocity) / float64(len(velocities))
			if forecast.AverageVelocity != expectedAverage {
				t.Logf("Expected average velocity %.2f, got %.2f", expectedAverage, forecast.AverageVelocity)
				return false
			}

			// Verify estimated sprints calculation
			expectedSprints := int(float64(remainingPoints) / expectedAverage)
			if float64(remainingPoints) > expectedAverage*float64(expectedSprints) {
				expectedSprints++ // Round up
			}
			if forecast.EstimatedSprints != expectedSprints {
				t.Logf("Expected %d sprints, got %d", expectedSprints, forecast.EstimatedSprints)
				return false
			}

			// Verify min estimate (best case with max velocity)
			if maxVelocity > 0 {
				expectedMin := remainingPoints / maxVelocity
				if remainingPoints%maxVelocity > 0 {
					expectedMin++
				}
				if forecast.EstimatedSprintsMin != expectedMin {
					t.Logf("Expected min %d sprints, got %d", expectedMin, forecast.EstimatedSprintsMin)
					return false
				}
			}

			// Verify max estimate (worst case with min velocity)
			if minVelocity > 0 {
				expectedMax := remainingPoints / minVelocity
				if remainingPoints%minVelocity > 0 {
					expectedMax++
				}
				if forecast.EstimatedSprintsMax != expectedMax {
					t.Logf("Expected max %d sprints, got %d", expectedMax, forecast.EstimatedSprintsMax)
					return false
				}
			}

			// Verify confidence level is set
			if forecast.ConfidenceLevel == "" {
				t.Log("Confidence level not set")
				return false
			}

			return true
		},
		gen.SliceOfN(5, gen.IntRange(5, 50)).SuchThat(func(v interface{}) bool {
			slice := v.([]int)
			return len(slice) > 0
		}), // velocities
		gen.IntRange(10, 200), // remaining points
	))

	properties.TestingRun(t)
}

// Helper functions
func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: !t.IsZero(),
	}
}
