package reporting

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
)

// Mock repositories for testing
type mockSprintRepo struct {
	sprints           []sprints.SprintManagementSprint
	getByIDFunc       func(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error)
	listFunc          func(ctx context.Context, params sprints.ListSprintsParams) ([]sprints.SprintManagementSprint, error)
	listCompletedFunc func(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error)
	listByStatusFunc  func(ctx context.Context, params sprints.ListSprintsByStatusParams) ([]sprints.SprintManagementSprint, error)
}

func (m *mockSprintRepo) GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	for _, s := range m.sprints {
		if s.ID.Bytes == id.Bytes {
			return s, nil
		}
	}
	return sprints.SprintManagementSprint{}, ErrSprintNotFound
}

func (m *mockSprintRepo) ListSprints(ctx context.Context, params sprints.ListSprintsParams) ([]sprints.SprintManagementSprint, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, params)
	}
	return m.sprints, nil
}

func (m *mockSprintRepo) ListCompletedSprints(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error) {
	if m.listCompletedFunc != nil {
		return m.listCompletedFunc(ctx, limit)
	}
	var completed []sprints.SprintManagementSprint
	for _, s := range m.sprints {
		if s.Status == sprints.SprintManagementSprintStatusCompleted {
			completed = append(completed, s)
			if len(completed) >= int(limit) {
				break
			}
		}
	}
	return completed, nil
}

func (m *mockSprintRepo) ListSprintsByStatus(ctx context.Context, params sprints.ListSprintsByStatusParams) ([]sprints.SprintManagementSprint, error) {
	if m.listByStatusFunc != nil {
		return m.listByStatusFunc(ctx, params)
	}
	var filtered []sprints.SprintManagementSprint
	for _, s := range m.sprints {
		if s.Status == params.Status {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

type mockWorkItemRepo struct {
	workItems        []workitems.SprintManagementWorkItem
	listBySprintFunc func(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error)
	listFunc         func(ctx context.Context, params workitems.ListWorkItemsParams) ([]workitems.SprintManagementWorkItem, error)
	getByIDFunc      func(ctx context.Context, id pgtype.UUID) (workitems.SprintManagementWorkItem, error)
}

func (m *mockWorkItemRepo) ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error) {
	if m.listBySprintFunc != nil {
		return m.listBySprintFunc(ctx, sprintID)
	}
	var items []workitems.SprintManagementWorkItem
	for _, wi := range m.workItems {
		if wi.SprintID.Valid && wi.SprintID.Bytes == sprintID.Bytes {
			items = append(items, wi)
		}
	}
	return items, nil
}

func (m *mockWorkItemRepo) ListWorkItems(ctx context.Context, params workitems.ListWorkItemsParams) ([]workitems.SprintManagementWorkItem, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, params)
	}
	return m.workItems, nil
}

func (m *mockWorkItemRepo) GetWorkItemByID(ctx context.Context, id pgtype.UUID) (workitems.SprintManagementWorkItem, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	for _, wi := range m.workItems {
		if wi.ID.Bytes == id.Bytes {
			return wi, nil
		}
	}
	return workitems.SprintManagementWorkItem{}, ErrSprintNotFound
}

// Helper functions for creating test data
func createTestSprint(id uuid.UUID, name string, status sprints.SprintManagementSprintStatus, startDate, endDate time.Time, committedPoints, completedPoints *int32) sprints.SprintManagementSprint {
	return sprints.SprintManagementSprint{
		ID:              uuidToPgtype(id),
		Name:            name,
		Status:          status,
		StartDate:       timeToPgDate(startDate),
		EndDate:         timeToPgDate(endDate),
		CommittedPoints: committedPoints,
		CompletedPoints: completedPoints,
		CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func createTestWorkItem(id uuid.UUID, sprintID *uuid.UUID, title string, wiType workitems.SprintManagementWorkItemType, status workitems.SprintManagementWorkItemStatus, priority workitems.SprintManagementPriorityLevel, storyPoints *int32) workitems.SprintManagementWorkItem {
	wi := workitems.SprintManagementWorkItem{
		ID:          uuidToPgtype(id),
		Title:       title,
		Type:        wiType,
		Status:      status,
		Priority:    priority,
		StoryPoints: storyPoints,
		CreatedAt:   pgtype.Timestamptz{Time: time.Now().Add(-48 * time.Hour), Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	if sprintID != nil {
		wi.SprintID = uuidToPgtype(*sprintID)
	}

	return wi
}

func timeToPgDate(t time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  t,
		Valid: true,
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}

// Test GenerateSprintReport
func TestGenerateSprintReport(t *testing.T) {
	ctx := context.Background()

	sprintID := uuid.New()
	startDate := time.Now().AddDate(0, 0, -14)
	endDate := time.Now()

	sprint := createTestSprint(
		sprintID,
		"Sprint 1",
		sprints.SprintManagementSprintStatusActive,
		startDate,
		endDate,
		int32Ptr(50),
		int32Ptr(30),
	)

	workItems := []workitems.SprintManagementWorkItem{
		createTestWorkItem(uuid.New(), &sprintID, "Story 1", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelHigh, int32Ptr(5)),
		createTestWorkItem(uuid.New(), &sprintID, "Story 2", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusInProgress, workitems.SprintManagementPriorityLevelMedium, int32Ptr(8)),
		createTestWorkItem(uuid.New(), &sprintID, "Bug 1", workitems.SprintManagementWorkItemTypeDefect, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelCritical, int32Ptr(3)),
		createTestWorkItem(uuid.New(), &sprintID, "Story 3", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusTodo, workitems.SprintManagementPriorityLevelLow, int32Ptr(13)),
	}

	sprintRepo := &mockSprintRepo{sprints: []sprints.SprintManagementSprint{sprint}}
	workItemRepo := &mockWorkItemRepo{workItems: workItems}

	service := NewService(sprintRepo, workItemRepo, nil)

	report, err := service.GenerateSprintReport(ctx, sprintID)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, sprintID, report.SprintID)
	assert.Equal(t, "Sprint 1", report.SprintName)
	assert.Equal(t, 4, report.TotalWorkItems)
	assert.Equal(t, 2, report.CompletedItems)
	assert.Equal(t, 1, report.InProgressItems)
	assert.Equal(t, 1, report.TodoItems)
	assert.Equal(t, 29, report.CommittedPoints) // 5+8+3+13
	assert.Equal(t, 8, report.CompletedPoints)  // 5+3
	assert.Equal(t, 8, report.Velocity)
	assert.Equal(t, 50.0, report.CompletionRate) // 2/4 * 100

	// Check work items by type
	assert.Equal(t, 3, report.WorkItemsByType["story"])
	assert.Equal(t, 1, report.WorkItemsByType["defect"])
}

// Test GenerateTeamPerformanceReport
func TestGenerateTeamPerformanceReport(t *testing.T) {
	ctx := context.Background()

	sprint1ID := uuid.New()
	sprint2ID := uuid.New()

	sprint1 := createTestSprint(
		sprint1ID,
		"Sprint 1",
		sprints.SprintManagementSprintStatusCompleted,
		time.Now().AddDate(0, 0, -28),
		time.Now().AddDate(0, 0, -14),
		int32Ptr(40),
		int32Ptr(35),
	)

	sprint2 := createTestSprint(
		sprint2ID,
		"Sprint 2",
		sprints.SprintManagementSprintStatusCompleted,
		time.Now().AddDate(0, 0, -14),
		time.Now(),
		int32Ptr(50),
		int32Ptr(45),
	)

	workItems := []workitems.SprintManagementWorkItem{
		// Sprint 1 items
		createTestWorkItem(uuid.New(), &sprint1ID, "Story 1", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelHigh, int32Ptr(5)),
		createTestWorkItem(uuid.New(), &sprint1ID, "Story 2", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelMedium, int32Ptr(8)),
		// Sprint 2 items
		createTestWorkItem(uuid.New(), &sprint2ID, "Story 3", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelHigh, int32Ptr(13)),
		createTestWorkItem(uuid.New(), &sprint2ID, "Story 4", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelMedium, int32Ptr(8)),
	}

	sprintRepo := &mockSprintRepo{sprints: []sprints.SprintManagementSprint{sprint1, sprint2}}
	workItemRepo := &mockWorkItemRepo{workItems: workItems}

	service := NewService(sprintRepo, workItemRepo, nil)

	report, err := service.GenerateTeamPerformanceReport(ctx, DateRangeFilter{}, 10)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, 2, report.TotalSprints)
	assert.Equal(t, 2, report.CompletedSprints)
	assert.Equal(t, 4, report.TotalWorkItems)
	assert.Equal(t, 4, report.CompletedWorkItems)
	assert.Equal(t, 34, report.TotalStoryPoints)         // 13+8+8+5
	assert.Equal(t, 34, report.CompletedStoryPoints)     // All done
	assert.InDelta(t, 17.0, report.AverageVelocity, 0.1) // (13+8+8+5)/2
	assert.Equal(t, 100.0, report.AverageCompletionRate) // All completed

	assert.Len(t, report.SprintReports, 2)
	assert.Len(t, report.VelocityTrend, 2)
}

// Test CalculateCycleTime
func TestCalculateCycleTime(t *testing.T) {
	ctx := context.Background()

	now := time.Now()

	workItems := []workitems.SprintManagementWorkItem{
		// Completed items with different cycle times
		{
			ID:          uuidToPgtype(uuid.New()),
			Title:       "Story 1",
			Type:        workitems.SprintManagementWorkItemTypeStory,
			Status:      workitems.SprintManagementWorkItemStatusDone,
			Priority:    workitems.SprintManagementPriorityLevelHigh,
			StoryPoints: int32Ptr(5),
			CreatedAt:   pgtype.Timestamptz{Time: now.Add(-48 * time.Hour), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
		{
			ID:          uuidToPgtype(uuid.New()),
			Title:       "Story 2",
			Type:        workitems.SprintManagementWorkItemTypeStory,
			Status:      workitems.SprintManagementWorkItemStatusDone,
			Priority:    workitems.SprintManagementPriorityLevelMedium,
			StoryPoints: int32Ptr(8),
			CreatedAt:   pgtype.Timestamptz{Time: now.Add(-72 * time.Hour), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
		// In-progress item (not completed)
		{
			ID:          uuidToPgtype(uuid.New()),
			Title:       "Story 3",
			Type:        workitems.SprintManagementWorkItemTypeStory,
			Status:      workitems.SprintManagementWorkItemStatusInProgress,
			Priority:    workitems.SprintManagementPriorityLevelLow,
			StoryPoints: int32Ptr(13),
			CreatedAt:   pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		},
	}

	sprintRepo := &mockSprintRepo{}
	workItemRepo := &mockWorkItemRepo{workItems: workItems}

	service := NewService(sprintRepo, workItemRepo, nil)

	summary, err := service.CalculateCycleTime(ctx, DateRangeFilter{}, nil)
	require.NoError(t, err)
	require.NotNil(t, summary)

	assert.Equal(t, 3, summary.TotalWorkItems)
	assert.Equal(t, 2, summary.CompletedWorkItems)
	assert.InDelta(t, 60.0, summary.AverageLeadTime, 1.0) // (48+72)/2
	assert.InDelta(t, 60.0, summary.AverageCycleTime, 1.0)

	// Check metrics by type
	assert.Contains(t, summary.MetricsByType, "story")
	assert.Equal(t, 2, summary.MetricsByType["story"].Count)
}

// Test GetDashboardMetrics
func TestGetDashboardMetrics(t *testing.T) {
	ctx := context.Background()

	sprintID := uuid.New()
	sprint := createTestSprint(
		sprintID,
		"Active Sprint",
		sprints.SprintManagementSprintStatusActive,
		time.Now().AddDate(0, 0, -7),
		time.Now().AddDate(0, 0, 7),
		int32Ptr(50),
		int32Ptr(20),
	)

	workItems := []workitems.SprintManagementWorkItem{
		createTestWorkItem(uuid.New(), &sprintID, "Story 1", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelHigh, int32Ptr(5)),
		createTestWorkItem(uuid.New(), &sprintID, "Story 2", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusInProgress, workitems.SprintManagementPriorityLevelMedium, int32Ptr(8)),
		createTestWorkItem(uuid.New(), &sprintID, "Bug 1", workitems.SprintManagementWorkItemTypeDefect, workitems.SprintManagementWorkItemStatusBlocked, workitems.SprintManagementPriorityLevelCritical, int32Ptr(3)),
	}

	sprintRepo := &mockSprintRepo{sprints: []sprints.SprintManagementSprint{sprint}}
	workItemRepo := &mockWorkItemRepo{workItems: workItems}

	service := NewService(sprintRepo, workItemRepo, nil)

	metrics, err := service.GetDashboardMetrics(ctx, DateRangeFilter{})
	require.NoError(t, err)
	require.NotNil(t, metrics)

	assert.Equal(t, 1, metrics.ActiveSprints)
	assert.Equal(t, 3, metrics.TotalWorkItems)
	assert.Equal(t, 1, metrics.CompletedWorkItems)
	assert.Equal(t, 1, metrics.InProgressWorkItems)
	assert.Equal(t, 1, metrics.BlockedWorkItems)
	assert.InDelta(t, 33.33, metrics.CompletionRate, 0.1) // 1/3 * 100

	// Check work items by type
	assert.Equal(t, 2, metrics.WorkItemsByType["story"])
	assert.Equal(t, 1, metrics.WorkItemsByType["defect"])

	// Check upcoming deadlines
	assert.Len(t, metrics.UpcomingDeadlines, 1)
	assert.Equal(t, sprintID, metrics.UpcomingDeadlines[0].SprintID)
}

// Test export functions
func TestExportSprintReport(t *testing.T) {
	ctx := context.Background()

	sprintID := uuid.New()
	sprint := createTestSprint(
		sprintID,
		"Sprint 1",
		sprints.SprintManagementSprintStatusCompleted,
		time.Now().AddDate(0, 0, -14),
		time.Now(),
		int32Ptr(50),
		int32Ptr(45),
	)

	workItems := []workitems.SprintManagementWorkItem{
		createTestWorkItem(uuid.New(), &sprintID, "Story 1", workitems.SprintManagementWorkItemTypeStory, workitems.SprintManagementWorkItemStatusDone, workitems.SprintManagementPriorityLevelHigh, int32Ptr(5)),
	}

	sprintRepo := &mockSprintRepo{sprints: []sprints.SprintManagementSprint{sprint}}
	workItemRepo := &mockWorkItemRepo{workItems: workItems}

	service := NewService(sprintRepo, workItemRepo, nil)

	// Test CSV export
	csvData, err := service.ExportSprintReport(ctx, sprintID, ExportFormatCSV)
	require.NoError(t, err)
	assert.NotEmpty(t, csvData)
	assert.Contains(t, string(csvData), "Sprint ID")
	assert.Contains(t, string(csvData), "Sprint 1")

	// Test JSON export
	jsonData, err := service.ExportSprintReport(ctx, sprintID, ExportFormatJSON)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)
	assert.Contains(t, string(jsonData), "sprint_id")
	assert.Contains(t, string(jsonData), "Sprint 1")
}

// Test date range filtering
func TestDateRangeFiltering(t *testing.T) {
	ctx := context.Background()

	// Create sprints with different date ranges
	sprint1 := createTestSprint(
		uuid.New(),
		"Old Sprint",
		sprints.SprintManagementSprintStatusCompleted,
		time.Now().AddDate(0, -3, 0),
		time.Now().AddDate(0, -2, -14),
		int32Ptr(40),
		int32Ptr(35),
	)

	sprint2 := createTestSprint(
		uuid.New(),
		"Recent Sprint",
		sprints.SprintManagementSprintStatusCompleted,
		time.Now().AddDate(0, 0, -14),
		time.Now(),
		int32Ptr(50),
		int32Ptr(45),
	)

	sprintRepo := &mockSprintRepo{sprints: []sprints.SprintManagementSprint{sprint1, sprint2}}
	workItemRepo := &mockWorkItemRepo{workItems: []workitems.SprintManagementWorkItem{}}

	service := NewService(sprintRepo, workItemRepo, nil)

	// Filter to only recent sprint
	startDate := time.Now().AddDate(0, -1, 0)
	report, err := service.GenerateTeamPerformanceReport(ctx, DateRangeFilter{StartDate: &startDate}, 0)
	require.NoError(t, err)

	// Should only include sprint2
	assert.Equal(t, 1, report.TotalSprints)
}

// Test invalid date range
func TestInvalidDateRange(t *testing.T) {
	ctx := context.Background()

	sprintRepo := &mockSprintRepo{}
	workItemRepo := &mockWorkItemRepo{}

	service := NewService(sprintRepo, workItemRepo, nil)

	// End date before start date
	startDate := time.Now()
	endDate := time.Now().AddDate(0, 0, -1)

	_, err := service.GenerateTeamPerformanceReport(ctx, DateRangeFilter{
		StartDate: &startDate,
		EndDate:   &endDate,
	}, 0)

	assert.ErrorIs(t, err, ErrInvalidDateRange)
}

// Test statistical functions
func TestStatisticalFunctions(t *testing.T) {
	t.Run("average", func(t *testing.T) {
		values := []float64{10, 20, 30, 40, 50}
		assert.Equal(t, 30.0, average(values))

		assert.Equal(t, 0.0, average([]float64{}))
	})

	t.Run("median", func(t *testing.T) {
		// Odd number of values
		values := []float64{10, 30, 20, 50, 40}
		assert.Equal(t, 30.0, median(values))

		// Even number of values
		values = []float64{10, 20, 30, 40}
		assert.Equal(t, 25.0, median(values))

		assert.Equal(t, 0.0, median([]float64{}))
	})

	t.Run("min", func(t *testing.T) {
		values := []float64{30, 10, 50, 20, 40}
		assert.Equal(t, 10.0, min(values))

		assert.Equal(t, 0.0, min([]float64{}))
	})

	t.Run("max", func(t *testing.T) {
		values := []float64{30, 10, 50, 20, 40}
		assert.Equal(t, 50.0, max(values))

		assert.Equal(t, 0.0, max([]float64{}))
	})
}
