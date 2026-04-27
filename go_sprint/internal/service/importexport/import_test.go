package importexport

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockWorkItemRepository is a mock implementation of WorkItemRepository
type MockWorkItemRepository struct {
	mock.Mock
}

func (m *MockWorkItemRepository) CreateWorkItem(ctx context.Context, req CreateWorkItemRequest) (*models.WorkItem, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkItem), args.Error(1)
}

func (m *MockWorkItemRepository) GetWorkItemByID(ctx context.Context, id uuid.UUID) (*models.WorkItem, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkItem), args.Error(1)
}

func (m *MockWorkItemRepository) UpdateWorkItem(ctx context.Context, id uuid.UUID, req UpdateWorkItemRequest) (*models.WorkItem, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WorkItem), args.Error(1)
}

func TestImportWorkItemsFromCSV_Success(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	reporterID := uuid.New()
	assigneeID := uuid.New()

	csvData := `type,title,description,priority,reporter_id,assignee_id,story_points
epic,User Authentication,Implement auth system,high,` + reporterID.String() + `,` + assigneeID.String() + `,13
story,Login Page,Create login page,medium,` + reporterID.String() + `,,5`

	// Mock successful creation
	mockRepo.On("GetWorkItemByID", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	mockRepo.On("CreateWorkItem", mock.Anything, mock.Anything).Return(&models.WorkItem{
		ID:       uuid.New(),
		Type:     models.WorkItemTypeEpic,
		Title:    "User Authentication",
		Priority: models.PriorityHigh,
	}, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalRecords)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.ErrorCount)
	assert.Equal(t, 0, result.SkippedCount)
}

func TestImportWorkItemsFromCSV_ValidationErrors(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	// CSV with validation errors
	csvData := `type,title,description,priority,reporter_id
invalid_type,Test,Description,high,` + uuid.New().String() + `
epic,,Description,high,` + uuid.New().String() + `
epic,Test,,high,` + uuid.New().String() + `
epic,Test,Description,invalid_priority,` + uuid.New().String() + `
epic,Test,Description,high,invalid_uuid`

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 5, result.TotalRecords)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 5, result.ErrorCount)
	assert.Len(t, result.Errors, 5)
}

func TestImportWorkItemsFromCSV_ConflictSkip(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	existingID := uuid.New()
	reporterID := uuid.New()

	csvData := `id,type,title,description,priority,reporter_id
` + existingID.String() + `,epic,Existing Item,Description,high,` + reporterID.String()

	// Mock existing work item
	mockRepo.On("GetWorkItemByID", mock.Anything, existingID).Return(&models.WorkItem{
		ID:    existingID,
		Type:  models.WorkItemTypeEpic,
		Title: "Existing Item",
	}, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalRecords)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 1, result.SkippedCount)
	assert.Equal(t, 0, result.ErrorCount)
}

func TestImportWorkItemsFromCSV_ConflictUpdate(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	existingID := uuid.New()
	reporterID := uuid.New()

	csvData := `id,type,title,description,priority,reporter_id
` + existingID.String() + `,epic,Updated Title,Updated Description,high,` + reporterID.String()

	// Mock existing work item
	existingItem := &models.WorkItem{
		ID:          existingID,
		Type:        models.WorkItemTypeEpic,
		Title:       "Old Title",
		Description: "Old Description",
		Priority:    models.PriorityMedium,
	}
	mockRepo.On("GetWorkItemByID", mock.Anything, existingID).Return(existingItem, nil)

	// Mock update
	updatedItem := &models.WorkItem{
		ID:          existingID,
		Type:        models.WorkItemTypeEpic,
		Title:       "Updated Title",
		Description: "Updated Description",
		Priority:    models.PriorityHigh,
	}
	mockRepo.On("UpdateWorkItem", mock.Anything, existingID, mock.Anything).Return(updatedItem, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategyUpdate)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalRecords)
	assert.Equal(t, 1, result.SuccessCount)
	assert.Equal(t, 0, result.SkippedCount)
	assert.Equal(t, 0, result.ErrorCount)
	mockRepo.AssertCalled(t, "UpdateWorkItem", mock.Anything, existingID, mock.Anything)
}

func TestImportWorkItemsFromCSV_ConflictError(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	existingID := uuid.New()
	reporterID := uuid.New()

	csvData := `id,type,title,description,priority,reporter_id
` + existingID.String() + `,epic,Existing Item,Description,high,` + reporterID.String()

	// Mock existing work item
	mockRepo.On("GetWorkItemByID", mock.Anything, existingID).Return(&models.WorkItem{
		ID:    existingID,
		Type:  models.WorkItemTypeEpic,
		Title: "Existing Item",
	}, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategyError)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalRecords)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.SkippedCount)
	assert.Equal(t, 1, result.ErrorCount)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "already exists")
}

func TestImportWorkItemsFromJSON_Success(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	reporterID := uuid.New()

	jsonData := `[
		{
			"type": "epic",
			"title": "User Authentication",
			"description": "Implement auth system",
			"priority": "high",
			"reporter_id": "` + reporterID.String() + `",
			"story_points": "13"
		},
		{
			"type": "story",
			"title": "Login Page",
			"description": "Create login page",
			"priority": "medium",
			"reporter_id": "` + reporterID.String() + `",
			"story_points": "5"
		}
	]`

	// Mock successful creation
	mockRepo.On("GetWorkItemByID", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	mockRepo.On("CreateWorkItem", mock.Anything, mock.Anything).Return(&models.WorkItem{
		ID:       uuid.New(),
		Type:     models.WorkItemTypeEpic,
		Title:    "User Authentication",
		Priority: models.PriorityHigh,
	}, nil)

	reader := strings.NewReader(jsonData)
	result, err := service.ImportWorkItemsFromJSON(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.TotalRecords)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 0, result.ErrorCount)
}

func TestImportWorkItemsFromJSON_InvalidJSON(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	jsonData := `invalid json`

	reader := strings.NewReader(jsonData)
	result, err := service.ImportWorkItemsFromJSON(context.Background(), reader, ConflictStrategySkip)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to decode JSON")
}

func TestValidateImportRecord_RequiredFields(t *testing.T) {
	service := NewImportService(nil, nil)

	tests := []struct {
		name        string
		record      *WorkItemImportRecord
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid record",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
				ReporterID:  uuid.New().String(),
			},
			expectError: false,
		},
		{
			name: "missing type",
			record: &WorkItemImportRecord{
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
				ReporterID:  uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "type is required",
		},
		{
			name: "missing title",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Description: "Description",
				Priority:    "high",
				ReporterID:  uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "title is required",
		},
		{
			name: "missing description",
			record: &WorkItemImportRecord{
				Type:       "epic",
				Title:      "Test",
				Priority:   "high",
				ReporterID: uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "description is required",
		},
		{
			name: "missing priority",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				ReporterID:  uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "priority is required",
		},
		{
			name: "missing reporter_id",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
			},
			expectError: true,
			errorMsg:    "reporter_id is required",
		},
		{
			name: "invalid type",
			record: &WorkItemImportRecord{
				Type:        "invalid",
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
				ReporterID:  uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "invalid type",
		},
		{
			name: "invalid priority",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				Priority:    "invalid",
				ReporterID:  uuid.New().String(),
			},
			expectError: true,
			errorMsg:    "invalid priority",
		},
		{
			name: "invalid reporter_id format",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
				ReporterID:  "invalid-uuid",
			},
			expectError: true,
			errorMsg:    "invalid reporter_id format",
		},
		{
			name: "invalid story_points format",
			record: &WorkItemImportRecord{
				Type:        "epic",
				Title:       "Test",
				Description: "Description",
				Priority:    "high",
				ReporterID:  uuid.New().String(),
				StoryPoints: "not-a-number",
			},
			expectError: true,
			errorMsg:    "invalid story_points format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateImportRecord(tt.record)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCSVHeader(t *testing.T) {
	service := NewImportService(nil, nil)

	tests := []struct {
		name        string
		header      []string
		expectError bool
	}{
		{
			name:        "valid header with all required fields",
			header:      []string{"type", "title", "description", "priority", "reporter_id"},
			expectError: false,
		},
		{
			name:        "valid header with optional fields",
			header:      []string{"id", "type", "title", "description", "priority", "reporter_id", "story_points"},
			expectError: false,
		},
		{
			name:        "missing type",
			header:      []string{"title", "description", "priority", "reporter_id"},
			expectError: true,
		},
		{
			name:        "missing title",
			header:      []string{"type", "description", "priority", "reporter_id"},
			expectError: true,
		},
		{
			name:        "case insensitive",
			header:      []string{"TYPE", "TITLE", "DESCRIPTION", "PRIORITY", "REPORTER_ID"},
			expectError: false,
		},
		{
			name:        "with whitespace",
			header:      []string{" type ", " title ", " description ", " priority ", " reporter_id "},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateCSVHeader(tt.header)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseCSVRecord(t *testing.T) {
	service := NewImportService(nil, nil)

	header := []string{"id", "type", "title", "description", "priority", "reporter_id", "story_points"}
	record := []string{
		uuid.New().String(),
		"epic",
		"Test Title",
		"Test Description",
		"high",
		uuid.New().String(),
		"13",
	}

	result, err := service.parseCSVRecord(header, record)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "epic", result.Type)
	assert.Equal(t, "Test Title", result.Title)
	assert.Equal(t, "Test Description", result.Description)
	assert.Equal(t, "high", result.Priority)
	assert.Equal(t, "13", result.StoryPoints)
}

func TestParseCSVRecord_LengthMismatch(t *testing.T) {
	service := NewImportService(nil, nil)

	header := []string{"type", "title", "description"}
	record := []string{"epic", "Test Title"} // Missing one field

	result, err := service.parseCSVRecord(header, record)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "length mismatch")
}

func TestRecordToCreateRequest(t *testing.T) {
	service := NewImportService(nil, nil)

	reporterID := uuid.New()
	assigneeID := uuid.New()

	record := &WorkItemImportRecord{
		Type:        "story",
		Title:       "Test Story",
		Description: "Test Description",
		Priority:    "medium",
		StoryPoints: "5",
		AssigneeID:  assigneeID.String(),
		ReporterID:  reporterID.String(),
	}

	req, err := service.recordToCreateRequest(record)

	assert.NoError(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, models.WorkItemTypeStory, req.Type)
	assert.Equal(t, "Test Story", req.Title)
	assert.Equal(t, "Test Description", req.Description)
	assert.Equal(t, models.PriorityMedium, req.Priority)
	assert.NotNil(t, req.StoryPoints)
	assert.Equal(t, 5, *req.StoryPoints)
	assert.NotNil(t, req.AssigneeID)
	assert.Equal(t, assigneeID, *req.AssigneeID)
	assert.Equal(t, reporterID, req.ReporterID)
}

func TestImportWorkItemsFromCSV_MalformedCSV(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	// CSV with inconsistent columns
	csvData := `type,title,description,priority,reporter_id
epic,Test,Description,high,` + uuid.New().String() + `
story,Test2,Description2,medium` // Missing reporter_id

	// Mock for the first valid row
	mockRepo.On("GetWorkItemByID", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	mockRepo.On("CreateWorkItem", mock.Anything, mock.Anything).Return(&models.WorkItem{
		ID:       uuid.New(),
		Type:     models.WorkItemTypeEpic,
		Title:    "Test",
		Priority: models.PriorityHigh,
	}, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ErrorCount)
}

func TestImportWorkItemsFromCSV_EmptyFile(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	csvData := `type,title,description,priority,reporter_id`

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.TotalRecords)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.ErrorCount)
}

func TestImportWorkItemsFromCSV_PartialSuccess(t *testing.T) {
	mockRepo := new(MockWorkItemRepository)
	service := NewImportService(mockRepo, nil)

	reporterID := uuid.New()

	csvData := `type,title,description,priority,reporter_id
epic,Valid Item,Description,high,` + reporterID.String() + `
invalid_type,Invalid Item,Description,high,` + reporterID.String() + `
story,Another Valid,Description,medium,` + reporterID.String()

	// Mock successful creation for valid items
	mockRepo.On("GetWorkItemByID", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	mockRepo.On("CreateWorkItem", mock.Anything, mock.Anything).Return(&models.WorkItem{
		ID:       uuid.New(),
		Type:     models.WorkItemTypeEpic,
		Title:    "Valid Item",
		Priority: models.PriorityHigh,
	}, nil)

	reader := strings.NewReader(csvData)
	result, err := service.ImportWorkItemsFromCSV(context.Background(), reader, ConflictStrategySkip)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.TotalRecords)
	assert.Equal(t, 2, result.SuccessCount)
	assert.Equal(t, 1, result.ErrorCount)
	assert.Len(t, result.Errors, 1)
}
