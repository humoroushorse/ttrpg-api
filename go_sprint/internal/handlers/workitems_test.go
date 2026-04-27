package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_auth/pkg/auth/models"
	api "github.com/humoroushorse/go_sprint/api/generated"
	"github.com/humoroushorse/go_sprint/internal/middleware"
	"github.com/humoroushorse/go_sprint/internal/service/workitems"
	pkgmodels "github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// mockWorkItemService is a mock implementation of the work item service for testing
type mockWorkItemService struct {
	createFunc func(ctx context.Context, req workitems.CreateWorkItemRequest) (*pkgmodels.WorkItem, error)
	getFunc    func(ctx context.Context, id uuid.UUID) (*pkgmodels.WorkItem, error)
	listFunc   func(ctx context.Context, limit, offset int32) ([]*pkgmodels.WorkItem, error)
}

func (m *mockWorkItemService) CreateWorkItem(ctx context.Context, req workitems.CreateWorkItemRequest) (*pkgmodels.WorkItem, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockWorkItemService) GetWorkItem(ctx context.Context, id uuid.UUID) (*pkgmodels.WorkItem, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockWorkItemService) ListWorkItems(ctx context.Context, limit, offset int32) ([]*pkgmodels.WorkItem, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, limit, offset)
	}
	return nil, nil
}

// Test helpers

func createTestContext() context.Context {
	ctx := context.Background()

	// Add user to context
	user := &models.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
	}
	ctx = context.WithValue(ctx, middleware.UserContextKey, user)

	// Add trace ID to context
	ctx = context.WithValue(ctx, middleware.TraceIDContextKey, uuid.New().String())

	// Add logger to context
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx = context.WithValue(ctx, middleware.LoggerContextKey, logger)

	return ctx
}

// Generators for property-based testing

func genWorkItemType() gopter.Gen {
	return gen.OneConstOf(
		api.Epic,
		api.Story,
		api.Defect,
	)
}

func genPriorityLevel() gopter.Gen {
	return gen.OneConstOf(
		api.Low,
		api.Medium,
		api.High,
		api.Critical,
	)
}

func genWorkItemStatus() gopter.Gen {
	return gen.OneConstOf(
		api.Todo,
		api.InProgress,
		api.InReview,
		api.Done,
		api.Blocked,
	)
}

// Property 4: Work Item Response Completeness
// Feature: go-sprint-management, Property 4: For any work item retrieval, the response should contain all required fields including type, status, assignee, story points, and timestamps
// Validates: Requirements 1.4
func TestProperty4_WorkItemResponseCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("work item response contains all required fields", prop.ForAll(
		func(workItemType api.WorkItemType, status api.WorkItemStatus, priority api.PriorityLevel) bool {
			// Create a mock work item
			workItemID := uuid.New()
			reporterID := uuid.New()
			now := time.Now()

			mockWorkItem := &pkgmodels.WorkItem{
				ID:          workItemID,
				Type:        pkgmodels.WorkItemType(workItemType),
				Title:       "Test Work Item",
				Description: "Test Description",
				Status:      pkgmodels.WorkItemStatus(status),
				Priority:    pkgmodels.PriorityLevel(priority),
				ReporterID:  reporterID,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			// Create mock service
			mockService := &mockWorkItemService{
				getFunc: func(ctx context.Context, id uuid.UUID) (*pkgmodels.WorkItem, error) {
					return mockWorkItem, nil
				},
			}

			// Create handler
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := NewWorkItemHandler(mockService, logger)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workitems/"+workItemID.String(), nil)
			req = req.WithContext(createTestContext())
			w := httptest.NewRecorder()

			// Call handler
			handler.GetWorkItem(w, req, openapi_types.UUID(workItemID))

			// Check response
			if w.Code != http.StatusOK {
				t.Logf("Expected status 200, got %d", w.Code)
				return false
			}

			// Decode response
			var response api.WorkItem
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Logf("Failed to decode response: %v", err)
				return false
			}

			// Verify all required fields are present
			checks := []bool{
				response.Id != openapi_types.UUID(uuid.Nil),                // ID present
				response.Type == workItemType,                              // Type matches
				response.Title != "",                                       // Title present
				response.Description != nil && *response.Description != "", // Description present
				response.Status == status,                                  // Status matches
				response.Priority == priority,                              // Priority matches
				response.ReporterId != openapi_types.UUID(uuid.Nil),        // Reporter ID present
				!response.CreatedAt.IsZero(),                               // CreatedAt present
				!response.UpdatedAt.IsZero(),                               // UpdatedAt present
			}

			for _, check := range checks {
				if !check {
					return false
				}
			}

			return true
		},
		genWorkItemType(),
		genWorkItemStatus(),
		genPriorityLevel(),
	))

	properties.TestingRun(t)
}

// Property 16: Relationship Information Completeness
// Feature: go-sprint-management, Property 16: For any work item retrieval, the response should include all related work items and complete dependency information
// Validates: Requirements 28.4
func TestProperty16_RelationshipInformationCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("work item response includes relationship information when present", prop.ForAll(
		func(hasParent bool, hasAssignee bool, hasSprint bool) bool {
			// Create a mock work item with optional relationships
			workItemID := uuid.New()
			reporterID := uuid.New()
			now := time.Now()

			mockWorkItem := &pkgmodels.WorkItem{
				ID:          workItemID,
				Type:        pkgmodels.WorkItemTypeStory,
				Title:       "Test Work Item",
				Description: "Test Description",
				Status:      pkgmodels.WorkItemStatusTodo,
				Priority:    pkgmodels.PriorityMedium,
				ReporterID:  reporterID,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			// Add optional relationships
			if hasParent {
				parentID := uuid.New()
				mockWorkItem.ParentID = &parentID
			}

			if hasAssignee {
				assigneeID := uuid.New()
				mockWorkItem.AssigneeID = &assigneeID
			}

			if hasSprint {
				sprintID := uuid.New()
				mockWorkItem.SprintID = &sprintID
			}

			// Create mock service
			mockService := &mockWorkItemService{
				getFunc: func(ctx context.Context, id uuid.UUID) (*pkgmodels.WorkItem, error) {
					return mockWorkItem, nil
				},
			}

			// Create handler
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := NewWorkItemHandler(mockService, logger)

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/api/v1/workitems/"+workItemID.String(), nil)
			req = req.WithContext(createTestContext())
			w := httptest.NewRecorder()

			// Call handler
			handler.GetWorkItem(w, req, openapi_types.UUID(workItemID))

			// Check response
			if w.Code != http.StatusOK {
				t.Logf("Expected status 200, got %d", w.Code)
				return false
			}

			// Decode response
			var response api.WorkItem
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Logf("Failed to decode response: %v", err)
				return false
			}

			// Verify relationship fields match expectations
			if hasParent && response.ParentId == nil {
				t.Log("Expected parent ID to be present")
				return false
			}
			if !hasParent && response.ParentId != nil {
				t.Log("Expected parent ID to be nil")
				return false
			}

			if hasAssignee && response.AssigneeId == nil {
				t.Log("Expected assignee ID to be present")
				return false
			}
			if !hasAssignee && response.AssigneeId != nil {
				t.Log("Expected assignee ID to be nil")
				return false
			}

			if hasSprint && response.SprintId == nil {
				t.Log("Expected sprint ID to be present")
				return false
			}
			if !hasSprint && response.SprintId != nil {
				t.Log("Expected sprint ID to be nil")
				return false
			}

			return true
		},
		gen.Bool(),
		gen.Bool(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// Unit test for validation
func TestValidateCreateWorkItemRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewWorkItemHandler(nil, logger)

	tests := []struct {
		name         string
		req          api.CreateWorkItemRequest
		expectErrors bool
		errorCount   int
	}{
		{
			name: "valid request",
			req: api.CreateWorkItemRequest{
				Title:       "Valid Title",
				Description: "Valid Description",
				Type:        api.Story,
				Priority:    api.Medium,
			},
			expectErrors: false,
		},
		{
			name: "missing title",
			req: api.CreateWorkItemRequest{
				Title:       "",
				Description: "Valid Description",
				Type:        api.Story,
				Priority:    api.Medium,
			},
			expectErrors: true,
			errorCount:   1,
		},
		{
			name: "missing description",
			req: api.CreateWorkItemRequest{
				Title:       "Valid Title",
				Description: "",
				Type:        api.Story,
				Priority:    api.Medium,
			},
			expectErrors: true,
			errorCount:   1,
		},
		{
			name: "title too long",
			req: api.CreateWorkItemRequest{
				Title:       string(make([]byte, 256)),
				Description: "Valid Description",
				Type:        api.Story,
				Priority:    api.Medium,
			},
			expectErrors: true,
			errorCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := handler.validateCreateWorkItemRequest(tt.req)

			if tt.expectErrors && len(errors) == 0 {
				t.Error("Expected validation errors but got none")
			}

			if !tt.expectErrors && len(errors) > 0 {
				t.Errorf("Expected no validation errors but got %d", len(errors))
			}

			if tt.expectErrors && tt.errorCount > 0 && len(errors) != tt.errorCount {
				t.Errorf("Expected %d validation errors but got %d", tt.errorCount, len(errors))
			}
		})
	}
}
