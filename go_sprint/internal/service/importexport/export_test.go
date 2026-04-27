package importexport

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
)

// MockSprintRepository is a mock implementation of SprintRepository
type MockSprintRepository struct {
	mock.Mock
}

func (m *MockSprintRepository) GetSprintByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Sprint), args.Error(1)
}

func (m *MockSprintRepository) ListSprints(ctx context.Context, filters SprintFilters) ([]*models.Sprint, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Sprint), args.Error(1)
}

func TestExportWorkItemsToCSV_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeEpic,
			Title:       "User Authentication",
			Description: "Implement auth system",
			Status:      models.WorkItemStatusTodo,
			Priority:    models.PriorityHigh,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeStory,
			Title:       "Login Page",
			Description: "Create login page",
			Status:      models.WorkItemStatusInProgress,
			Priority:    models.PriorityMedium,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToCSV(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse CSV to verify content
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 3) // Header + 2 data rows

	// Verify header
	assert.Equal(t, "id", records[0][0])
	assert.Equal(t, "type", records[0][1])
	assert.Equal(t, "title", records[0][2])

	// Verify first data row
	assert.Equal(t, workItems[0].ID.String(), records[1][0])
	assert.Equal(t, string(workItems[0].Type), records[1][1])
	assert.Equal(t, workItems[0].Title, records[1][2])
}

func TestExportWorkItemsToJSON_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	storyPoints := 5
	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeStory,
			Title:       "Login Page",
			Description: "Create login page",
			Status:      models.WorkItemStatusInProgress,
			Priority:    models.PriorityMedium,
			StoryPoints: &storyPoints,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToJSON(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse JSON to verify content
	var result []*models.WorkItem
	err = json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, workItems[0].ID, result[0].ID)
	assert.Equal(t, workItems[0].Title, result[0].Title)
	assert.Equal(t, workItems[0].Type, result[0].Type)
	assert.NotNil(t, result[0].StoryPoints)
	assert.Equal(t, 5, *result[0].StoryPoints)
}

func TestExportWorkItemsToExcel_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeEpic,
			Title:       "User Authentication",
			Description: "Implement auth system",
			Status:      models.WorkItemStatusTodo,
			Priority:    models.PriorityHigh,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToExcel(context.Background(), &buf, workItems)

	assert.NoError(t, err)
	assert.Greater(t, buf.Len(), 0)

	// Verify Excel file can be opened
	f, err := excelize.OpenReader(&buf)
	assert.NoError(t, err)
	defer f.Close()

	// Verify sheet exists
	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "Work Items")

	// Verify header row
	headerCell, err := f.GetCellValue("Work Items", "A1")
	assert.NoError(t, err)
	assert.Equal(t, "ID", headerCell)

	// Verify data row
	idCell, err := f.GetCellValue("Work Items", "A2")
	assert.NoError(t, err)
	assert.Equal(t, workItems[0].ID.String(), idCell)

	titleCell, err := f.GetCellValue("Work Items", "C2")
	assert.NoError(t, err)
	assert.Equal(t, workItems[0].Title, titleCell)
}

func TestExportSprintsToCSV_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	capacityPoints := 50
	sprints := []*models.Sprint{
		{
			ID:              uuid.New(),
			Name:            "Sprint 1",
			Description:     "First sprint",
			Status:          models.SprintStatusActive,
			StartDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			CapacityPoints:  &capacityPoints,
			CommittedPoints: 45,
			CompletedPoints: 30,
			CreatedBy:       uuid.New(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportSprintsToCSV(context.Background(), &buf, sprints)

	assert.NoError(t, err)

	// Parse CSV to verify content
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 2) // Header + 1 data row

	// Verify header
	assert.Equal(t, "id", records[0][0])
	assert.Equal(t, "name", records[0][1])
	assert.Equal(t, "status", records[0][3])

	// Verify data row
	assert.Equal(t, sprints[0].ID.String(), records[1][0])
	assert.Equal(t, sprints[0].Name, records[1][1])
	assert.Equal(t, string(sprints[0].Status), records[1][3])
}

func TestExportSprintsToJSON_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	capacityPoints := 50
	sprints := []*models.Sprint{
		{
			ID:              uuid.New(),
			Name:            "Sprint 1",
			Description:     "First sprint",
			Status:          models.SprintStatusActive,
			StartDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			CapacityPoints:  &capacityPoints,
			CommittedPoints: 45,
			CompletedPoints: 30,
			CreatedBy:       uuid.New(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportSprintsToJSON(context.Background(), &buf, sprints)

	assert.NoError(t, err)

	// Parse JSON to verify content
	var result []*models.Sprint
	err = json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, sprints[0].ID, result[0].ID)
	assert.Equal(t, sprints[0].Name, result[0].Name)
	assert.Equal(t, sprints[0].Status, result[0].Status)
	assert.NotNil(t, result[0].CapacityPoints)
	assert.Equal(t, 50, *result[0].CapacityPoints)
}

func TestExportSprintsToExcel_Success(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	sprints := []*models.Sprint{
		{
			ID:              uuid.New(),
			Name:            "Sprint 1",
			Description:     "First sprint",
			Status:          models.SprintStatusActive,
			StartDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			CommittedPoints: 45,
			CompletedPoints: 30,
			CreatedBy:       uuid.New(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportSprintsToExcel(context.Background(), &buf, sprints)

	assert.NoError(t, err)
	assert.Greater(t, buf.Len(), 0)

	// Verify Excel file can be opened
	f, err := excelize.OpenReader(&buf)
	assert.NoError(t, err)
	defer f.Close()

	// Verify sheet exists
	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "Sprints")

	// Verify header row
	headerCell, err := f.GetCellValue("Sprints", "A1")
	assert.NoError(t, err)
	assert.Equal(t, "ID", headerCell)

	// Verify data row
	idCell, err := f.GetCellValue("Sprints", "A2")
	assert.NoError(t, err)
	assert.Equal(t, sprints[0].ID.String(), idCell)

	nameCell, err := f.GetCellValue("Sprints", "B2")
	assert.NoError(t, err)
	assert.Equal(t, sprints[0].Name, nameCell)
}

func TestWorkItemToCSVRecord(t *testing.T) {
	service := NewExportService(nil, nil, nil)

	storyPoints := 5
	assigneeID := uuid.New()
	sprintID := uuid.New()

	workItem := &models.WorkItem{
		ID:          uuid.New(),
		Type:        models.WorkItemTypeStory,
		Title:       "Test Story",
		Description: "Test Description",
		Status:      models.WorkItemStatusInProgress,
		Priority:    models.PriorityMedium,
		StoryPoints: &storyPoints,
		AssigneeID:  &assigneeID,
		ReporterID:  uuid.New(),
		SprintID:    &sprintID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	record := service.workItemToCSVRecord(workItem)

	assert.Len(t, record, 13)
	assert.Equal(t, workItem.ID.String(), record[0])
	assert.Equal(t, string(workItem.Type), record[1])
	assert.Equal(t, workItem.Title, record[2])
	assert.Equal(t, workItem.Description, record[3])
	assert.Equal(t, string(workItem.Status), record[4])
	assert.Equal(t, string(workItem.Priority), record[5])
	assert.Equal(t, "5", record[6])
	assert.Equal(t, assigneeID.String(), record[7])
	assert.Equal(t, sprintID.String(), record[10])
}

func TestSprintToCSVRecord(t *testing.T) {
	service := NewExportService(nil, nil, nil)

	capacityPoints := 50
	sprint := &models.Sprint{
		ID:              uuid.New(),
		Name:            "Sprint 1",
		Description:     "First sprint",
		Status:          models.SprintStatusActive,
		StartDate:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
		CapacityPoints:  &capacityPoints,
		CommittedPoints: 45,
		CompletedPoints: 30,
		CreatedBy:       uuid.New(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	record := service.sprintToCSVRecord(sprint)

	assert.Len(t, record, 12)
	assert.Equal(t, sprint.ID.String(), record[0])
	assert.Equal(t, sprint.Name, record[1])
	assert.Equal(t, sprint.Description, record[2])
	assert.Equal(t, string(sprint.Status), record[3])
	assert.Equal(t, "2024-01-01", record[4])
	assert.Equal(t, "2024-01-14", record[5])
	assert.Equal(t, "50", record[6])
	assert.Equal(t, "45", record[7])
	assert.Equal(t, "30", record[8])
}

func TestExportWorkItemsToCSV_EmptyList(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	var workItems []*models.WorkItem

	var buf bytes.Buffer
	err := service.ExportWorkItemsToCSV(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse CSV to verify only header exists
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 1) // Only header
}

func TestExportWorkItemsToJSON_EmptyList(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	var workItems []*models.WorkItem

	var buf bytes.Buffer
	err := service.ExportWorkItemsToJSON(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse JSON to verify empty array
	var result []*models.WorkItem
	err = json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestExportWorkItemsWithOptionalFields(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	// Work item with minimal fields (no optional fields)
	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeEpic,
			Title:       "Minimal Epic",
			Description: "Description",
			Status:      models.WorkItemStatusTodo,
			Priority:    models.PriorityLow,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToCSV(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse CSV to verify content
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 2) // Header + 1 data row

	// Verify optional fields are empty
	assert.Equal(t, "", records[1][6])  // story_points
	assert.Equal(t, "", records[1][7])  // assignee_id
	assert.Equal(t, "", records[1][9])  // parent_id
	assert.Equal(t, "", records[1][10]) // sprint_id
}

func TestExportWorkItemsToCSV_SpecialCharacters(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeStory,
			Title:       "Story with, comma",
			Description: "Description with \"quotes\" and\nnewlines",
			Status:      models.WorkItemStatusTodo,
			Priority:    models.PriorityMedium,
			ReporterID:  uuid.New(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToCSV(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Parse CSV to verify special characters are handled correctly
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	assert.NoError(t, err)
	assert.Len(t, records, 2)

	// CSV library should handle special characters correctly
	assert.Contains(t, records[1][2], "comma")
	assert.Contains(t, records[1][3], "quotes")
}

func TestExportWorkItemsToJSON_CompleteData(t *testing.T) {
	mockWorkItemRepo := new(MockWorkItemRepository)
	mockSprintRepo := new(MockSprintRepository)
	service := NewExportService(mockWorkItemRepo, mockSprintRepo, nil)

	storyPoints := 8
	assigneeID := uuid.New()
	parentID := uuid.New()
	sprintID := uuid.New()

	workItems := []*models.WorkItem{
		{
			ID:          uuid.New(),
			Type:        models.WorkItemTypeStory,
			Title:       "Complete Story",
			Description: "Full description",
			Status:      models.WorkItemStatusInProgress,
			Priority:    models.PriorityHigh,
			StoryPoints: &storyPoints,
			AssigneeID:  &assigneeID,
			ReporterID:  uuid.New(),
			ParentID:    &parentID,
			SprintID:    &sprintID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	var buf bytes.Buffer
	err := service.ExportWorkItemsToJSON(context.Background(), &buf, workItems)

	assert.NoError(t, err)

	// Verify JSON is properly formatted
	jsonStr := buf.String()
	assert.True(t, strings.Contains(jsonStr, "\n")) // Should have indentation
	assert.True(t, strings.Contains(jsonStr, "  ")) // Should have 2-space indent

	// Parse and verify all fields
	var result []*models.WorkItem
	err = json.Unmarshal(buf.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.NotNil(t, result[0].StoryPoints)
	assert.NotNil(t, result[0].AssigneeID)
	assert.NotNil(t, result[0].ParentID)
	assert.NotNil(t, result[0].SprintID)
}
