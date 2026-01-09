# API Usage Example

This document demonstrates how to use the generated OpenAPI code in your Go application.

## Generated Code Structure

The generated code (`api/generated/api.gen.go`) provides:

1. **Type Definitions**: All request/response models as Go structs
2. **Server Interface**: Interface that you must implement
3. **Chi Router Integration**: Helper functions to wire up your handlers

## Implementing the Server Interface

### Step 1: Create Handler Struct

```go
// internal/handlers/api_handler.go
package handlers

import (
    "encoding/json"
    "net/http"
    
    "github.com/humoroushorse/go_sprint/api/generated"
    "github.com/humoroushorse/go_sprint/internal/service"
)

type APIHandler struct {
    workItemService *service.WorkItemService
    sprintService   *service.SprintService
}

func NewAPIHandler(
    workItemService *service.WorkItemService,
    sprintService *service.SprintService,
) *APIHandler {
    return &APIHandler{
        workItemService: workItemService,
        sprintService:   sprintService,
    }
}
```

### Step 2: Implement Interface Methods

```go
// ListWorkItems implements the generated ServerInterface
func (h *APIHandler) ListWorkItems(w http.ResponseWriter, r *http.Request, params generated.ListWorkItemsParams) {
    ctx := r.Context()
    
    // Extract query parameters
    filters := service.WorkItemFilters{
        Type:       params.Type,
        Status:     params.Status,
        Cursor:     params.Cursor,
        Limit:      params.Limit,
    }
    
    // Call service layer
    result, err := h.workItemService.ListWorkItems(ctx, filters)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
        return
    }
    
    // Convert to API response type
    response := generated.WorkItemListResponse{
        Items: convertWorkItems(result.Items),
        Pagination: generated.PaginationInfo{
            NextCursor:  result.NextCursor,
            HasMore:     result.HasMore,
            TotalCount:  result.TotalCount,
        },
    }
    
    writeJSON(w, http.StatusOK, response)
}

// CreateWorkItem implements the generated ServerInterface
func (h *APIHandler) CreateWorkItem(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Decode request body
    var req generated.CreateWorkItemRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON")
        return
    }
    
    // Validate request
    if err := validateCreateWorkItemRequest(req); err != nil {
        writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
        return
    }
    
    // Call service layer
    workItem, err := h.workItemService.CreateWorkItem(ctx, convertCreateRequest(req))
    if err != nil {
        writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
        return
    }
    
    // Convert to API response type
    response := convertWorkItem(workItem)
    
    writeJSON(w, http.StatusCreated, response)
}

// GetWorkItem implements the generated ServerInterface
func (h *APIHandler) GetWorkItem(w http.ResponseWriter, r *http.Request, id string) {
    ctx := r.Context()
    
    // Parse UUID
    workItemID, err := uuid.Parse(id)
    if err != nil {
        writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid work item ID")
        return
    }
    
    // Call service layer
    workItem, err := h.workItemService.GetWorkItem(ctx, workItemID)
    if err != nil {
        if errors.Is(err, service.ErrNotFound) {
            writeError(w, http.StatusNotFound, "NOT_FOUND", "Work item not found")
            return
        }
        writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
        return
    }
    
    // Convert to API response type
    response := convertWorkItem(workItem)
    
    writeJSON(w, http.StatusOK, response)
}

// Implement other methods...
```

### Step 3: Helper Functions

```go
// Helper function to write JSON responses
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

// Helper function to write error responses
func writeError(w http.ResponseWriter, status int, code, message string) {
    response := generated.ErrorResponse{
        Error: generated.ErrorDetail{
            Code:    code,
            Message: message,
        },
        TraceId:   getTraceID(w),
        Timestamp: time.Now(),
    }
    writeJSON(w, status, response)
}

// Convert domain models to API types
func convertWorkItem(wi *domain.WorkItem) generated.WorkItem {
    return generated.WorkItem{
        Id:          wi.ID.String(),
        Type:        generated.WorkItemType(wi.Type),
        Title:       wi.Title,
        Description: wi.Description,
        Status:      generated.WorkItemStatus(wi.Status),
        Priority:    generated.PriorityLevel(wi.Priority),
        StoryPoints: wi.StoryPoints,
        AssigneeId:  ptrToString(wi.AssigneeID),
        ReporterId:  wi.ReporterID.String(),
        ParentId:    ptrToString(wi.ParentID),
        SprintId:    ptrToString(wi.SprintID),
        CreatedAt:   wi.CreatedAt,
        UpdatedAt:   wi.UpdatedAt,
    }
}

func convertWorkItems(items []*domain.WorkItem) []generated.WorkItem {
    result := make([]generated.WorkItem, len(items))
    for i, item := range items {
        result[i] = convertWorkItem(item)
    }
    return result
}
```

### Step 4: Wire Up Router

```go
// cmd/server/main.go
package main

import (
    "log"
    "net/http"
    
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    
    "github.com/humoroushorse/go_sprint/api/generated"
    "github.com/humoroushorse/go_sprint/internal/handlers"
    "github.com/humoroushorse/go_sprint/internal/service"
)

func main() {
    // Initialize services
    workItemService := service.NewWorkItemService(/* dependencies */)
    sprintService := service.NewSprintService(/* dependencies */)
    
    // Create API handler
    apiHandler := handlers.NewAPIHandler(workItemService, sprintService)
    
    // Create Chi router
    r := chi.NewRouter()
    
    // Add middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.RequestID)
    
    // Register generated handlers
    generated.HandlerFromMux(apiHandler, r)
    
    // Start server
    log.Println("Starting server on :8082")
    if err := http.ListenAndServe(":8082", r); err != nil {
        log.Fatal(err)
    }
}
```

## Using Generated Types

### Request Validation

```go
func validateCreateWorkItemRequest(req generated.CreateWorkItemRequest) error {
    if req.Title == "" {
        return errors.New("title is required")
    }
    if len(req.Title) > 255 {
        return errors.New("title must be 255 characters or less")
    }
    if req.Description == "" {
        return errors.New("description is required")
    }
    
    // Validate enum values
    validTypes := map[generated.WorkItemType]bool{
        generated.Epic:   true,
        generated.Story:  true,
        generated.Defect: true,
    }
    if !validTypes[req.Type] {
        return errors.New("invalid work item type")
    }
    
    return nil
}
```

### Type Conversions

```go
// Convert API types to domain types
func convertCreateRequest(req generated.CreateWorkItemRequest) *domain.CreateWorkItemRequest {
    return &domain.CreateWorkItemRequest{
        Type:        domain.WorkItemType(req.Type),
        Title:       req.Title,
        Description: req.Description,
        Priority:    domain.PriorityLevel(req.Priority),
        StoryPoints: req.StoryPoints,
        AssigneeID:  stringToPtr(req.AssigneeId),
        ParentID:    stringToPtr(req.ParentId),
        SprintID:    stringToPtr(req.SprintId),
    }
}

// Helper functions for pointer conversions
func ptrToString(p *uuid.UUID) *string {
    if p == nil {
        return nil
    }
    s := p.String()
    return &s
}

func stringToPtr(s *string) *uuid.UUID {
    if s == nil || *s == "" {
        return nil
    }
    id, err := uuid.Parse(*s)
    if err != nil {
        return nil
    }
    return &id
}
```

## Testing with Generated Types

```go
// internal/handlers/api_handler_test.go
package handlers_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/humoroushorse/go_sprint/api/generated"
    "github.com/humoroushorse/go_sprint/internal/handlers"
)

func TestCreateWorkItem(t *testing.T) {
    // Setup
    handler := setupTestHandler(t)
    
    // Create request
    req := generated.CreateWorkItemRequest{
        Type:        generated.Story,
        Title:       "Test Story",
        Description: "Test Description",
        Priority:    generated.High,
    }
    
    body, err := json.Marshal(req)
    require.NoError(t, err)
    
    // Make request
    httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/workitems", bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    
    handler.CreateWorkItem(w, httpReq)
    
    // Assert response
    assert.Equal(t, http.StatusCreated, w.Code)
    
    var response generated.WorkItem
    err = json.NewDecoder(w.Body).Decode(&response)
    require.NoError(t, err)
    
    assert.Equal(t, req.Title, response.Title)
    assert.Equal(t, req.Type, response.Type)
}
```

## Complete Example

See the full implementation in:
- `internal/handlers/` - Handler implementations
- `internal/service/` - Business logic
- `internal/repository/` - Data access
- `cmd/server/main.go` - Server setup

## Next Steps

1. Implement all interface methods
2. Add middleware for authentication, logging, etc.
3. Add comprehensive error handling
4. Write unit and integration tests
5. Add API documentation
6. Deploy to production

## Additional Resources

- [Generated API Code](../api/generated/api.gen.go)
- [OpenAPI Specification](../api/openapi/combined.yaml)
- [Frontend Integration Guide](./frontend-integration.md)
