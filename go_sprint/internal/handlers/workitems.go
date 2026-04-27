package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	api "github.com/humoroushorse/go_sprint/api/generated"
	"github.com/humoroushorse/go_sprint/internal/middleware"
	dependenciesService "github.com/humoroushorse/go_sprint/internal/service/dependencies"
	"github.com/humoroushorse/go_sprint/internal/service/workitems"
	"github.com/humoroushorse/go_sprint/pkg/cache"
	"github.com/humoroushorse/go_sprint/pkg/metrics"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/humoroushorse/go_sprint/pkg/pagination"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// WorkItemService defines the interface for work item business logic
type WorkItemService interface {
	CreateWorkItem(ctx context.Context, req workitems.CreateWorkItemRequest) (*models.WorkItem, error)
	GetWorkItem(ctx context.Context, id uuid.UUID) (*models.WorkItem, error)
	UpdateWorkItem(ctx context.Context, id uuid.UUID, req workitems.UpdateWorkItemRequest) (*models.WorkItem, error)
	ListWorkItems(ctx context.Context, typeFilter *models.WorkItemType, statusFilter *models.WorkItemStatus, sprintIDFilter *uuid.UUID, cursor *pagination.Cursor, limit int32) ([]*models.WorkItem, error)
}

// WorkItemHandler implements the ServerInterface for work item endpoints
type WorkItemHandler struct {
	service             WorkItemService
	dependenciesService DependenciesService
	logger              *slog.Logger
	cache               *cache.Cache
}

// NewWorkItemHandler creates a new work item handler
func NewWorkItemHandler(service WorkItemService, dependenciesService DependenciesService, logger *slog.Logger, cache *cache.Cache) *WorkItemHandler {
	return &WorkItemHandler{
		service:             service,
		dependenciesService: dependenciesService,
		logger:              logger,
		cache:               cache,
	}
}

// CreateWorkItem handles POST /api/v1/workitems
func (h *WorkItemHandler) CreateWorkItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// Get user from context
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		RespondUnauthorized(w, r, "User not found in context")
		return
	}

	// Parse request body
	var req api.CreateWorkItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Validate required fields
	validationErrors := h.validateCreateWorkItemRequest(req)
	if len(validationErrors) > 0 {
		RespondWithValidationError(w, r, validationErrors)
		return
	}

	// Sanitize inputs
	req.Title = middleware.SanitizeInput(req.Title)
	req.Description = middleware.SanitizeInput(req.Description)

	// Convert API request to service request
	serviceReq := workitems.CreateWorkItemRequest{
		Type:        models.WorkItemType(req.Type),
		Title:       req.Title,
		Description: req.Description,
		Priority:    models.PriorityLevel(req.Priority),
		StoryPoints: req.StoryPoints,
		AssigneeID:  convertUUIDPtr(req.AssigneeId),
		ReporterID:  user.ID,
		ParentID:    convertUUIDPtr(req.ParentId),
		SprintID:    convertUUIDPtr(req.SprintId),
		ProjectID:   &req.ProjectId,
	}

	// Create work item
	workItem, err := h.service.CreateWorkItem(ctx, serviceReq)
	if err != nil {
		logger.Error("failed to create work item",
			slog.Any("error", err),
			slog.String("type", string(serviceReq.Type)),
		)
		RespondInternalError(w, r, err)
		return
	}

	// Convert to API response
	response := convertWorkItemToAPI(workItem)

	logger.Info("work item created",
		slog.String("id", workItem.ID.String()),
		slog.String("type", string(workItem.Type)),
	)

	RespondWithSuccess(w, r, http.StatusCreated, response)
}

// GetWorkItem handles GET /api/v1/workitems/{id}
func (h *WorkItemHandler) GetWorkItem(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)
	timer := metrics.NewTimer()

	workItemID := uuid.UUID(id)
	cacheKey := fmt.Sprintf("workitem:%s", workItemID.String())

	// Try cache first
	if cached, exists := h.cache.Get(cacheKey); exists {
		metrics.RecordCacheHit("workitem")
		logger.Debug("work item retrieved from cache", slog.String("id", workItemID.String()))

		timer.ObserveAPIResponse(r.Method, r.URL.Path, "200")
		RespondWithSuccess(w, r, http.StatusOK, cached)
		return
	}

	metrics.RecordCacheMiss("workitem")

	// Get work item
	workItem, err := h.service.GetWorkItem(ctx, workItemID)
	if err != nil {
		logger.Warn("work item not found",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		timer.ObserveAPIResponse(r.Method, r.URL.Path, "404")
		RespondNotFound(w, r, "Work item")
		return
	}

	// Convert to API response
	response := convertWorkItemToAPI(workItem)

	// Cache the response
	h.cache.Set(cacheKey, response)

	logger.Debug("work item retrieved",
		slog.String("id", workItemID.String()),
	)

	timer.ObserveAPIResponse(r.Method, r.URL.Path, "200")
	RespondWithSuccess(w, r, http.StatusOK, response)
}

// ListWorkItems handles GET /api/v1/workitems
func (h *WorkItemHandler) ListWorkItems(w http.ResponseWriter, r *http.Request, params api.ListWorkItemsParams) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)
	timer := metrics.NewTimer()

	// WORKAROUND: oapi-codegen doesn't bind query params in chi, so parse manually
	queryParams := r.URL.Query()
	filtersParam := queryParams.Get("filters")

	// Get pagination config
	paginationConfig := pagination.DefaultConfig()

	// Validate and normalize page size
	var limitPtr *int32
	if params.Limit != nil {
		limit32 := int32(*params.Limit)
		limitPtr = &limit32
	}
	limit := pagination.ValidatePageSize(limitPtr, paginationConfig)

	// Record pagination metrics
	hasCursor := params.Cursor != nil && *params.Cursor != ""
	metrics.RecordPaginationCursorUsage("/api/v1/workitems", hasCursor)

	// Decode cursor if provided
	var cursor *pagination.Cursor
	if hasCursor {
		decodedCursor, err := pagination.DecodeCursor(*params.Cursor)
		if err != nil {
			logger.Warn("invalid cursor provided",
				slog.String("cursor", *params.Cursor),
				slog.Any("error", err),
			)
			timer.ObserveAPIResponse(r.Method, r.URL.Path, "400")
			RespondBadRequest(w, r, "Invalid cursor format")
			return
		}
		cursor = &decodedCursor
	}

	// Extract filters
	var typeFilter *models.WorkItemType
	if params.Type != nil {
		t := models.WorkItemType(*params.Type)
		typeFilter = &t
	}

	var statusFilter *models.WorkItemStatus
	if params.Status != nil {
		s := models.WorkItemStatus(*params.Status)
		statusFilter = &s
	}

	// Parse sprint_id filter from filters JSON parameter
	var sprintIDFilter *uuid.UUID
	if filtersParam != "" {
		var filters []map[string]interface{}
		if err := json.Unmarshal([]byte(filtersParam), &filters); err == nil {
			for _, filter := range filters {
				if field, ok := filter["field"].(string); ok && field == "sprint_id" {
					if value, ok := filter["value"].(string); ok && value != "" {
						if sprintID, err := uuid.Parse(value); err == nil {
							sprintIDFilter = &sprintID
						}
					}
				}
			}
		}
	}

	// List work items
	items, err := h.service.ListWorkItems(ctx, typeFilter, statusFilter, sprintIDFilter, cursor, limit)
	if err != nil {
		logger.Error("failed to list work items",
			slog.Any("error", err),
		)
		timer.ObserveAPIResponse(r.Method, r.URL.Path, "500")
		RespondInternalError(w, r, err)
		return
	}

	// Record metrics
	metrics.RecordPaginationPageSize("/api/v1/workitems", len(items))
	metrics.RecordQueryRowsReturned("list", "work_items", len(items))

	// Convert to API response
	apiItems := make([]api.WorkItemSummary, len(items))
	for i, item := range items {
		apiItems[i] = convertWorkItemToSummary(item)
	}

	// Build pagination response
	var lastCursor *pagination.Cursor
	if len(items) > 0 {
		lastItem := items[len(items)-1]
		lastCursor = &pagination.Cursor{
			Timestamp: lastItem.CreatedAt,
			ID:        lastItem.ID.String(),
		}
	}

	paginationResp, err := pagination.NewResponse(len(items), limit, lastCursor)
	if err != nil {
		logger.Error("failed to create pagination response",
			slog.Any("error", err),
		)
		timer.ObserveAPIResponse(r.Method, r.URL.Path, "500")
		RespondInternalError(w, r, err)
		return
	}

	response := api.WorkItemListResponse{
		Items: apiItems,
		Pagination: api.PaginationInfo{
			HasMore:    paginationResp.HasMore,
			NextCursor: paginationResp.NextCursor,
			TotalCount: nil, // Total count is expensive, only compute if requested
		},
	}

	logger.Debug("work items listed",
		slog.Int("count", len(apiItems)),
		slog.Bool("has_more", paginationResp.HasMore),
	)

	timer.ObserveAPIResponse(r.Method, r.URL.Path, "200")
	RespondWithSuccess(w, r, http.StatusOK, response)
}

// UpdateWorkItem handles PUT /api/v1/workitems/{id}
func (h *WorkItemHandler) UpdateWorkItem(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID := uuid.UUID(id)

	// Parse request body
	var req api.UpdateWorkItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Sanitize inputs
	if req.Title != nil {
		sanitized := middleware.SanitizeInput(*req.Title)
		req.Title = &sanitized
	}
	if req.Description != nil {
		sanitized := middleware.SanitizeInput(*req.Description)
		req.Description = &sanitized
	}

	// Convert API request to service request
	serviceReq := workitems.UpdateWorkItemRequest{
		Title:       req.Title,
		Description: req.Description,
		StoryPoints: req.StoryPoints,
		AssigneeID:  convertUUIDPtr(req.AssigneeId),
		SprintID:    convertUUIDPtr(req.SprintId),
	}

	if req.Status != nil {
		s := models.WorkItemStatus(*req.Status)
		serviceReq.Status = &s
	}

	if req.Priority != nil {
		p := models.PriorityLevel(*req.Priority)
		serviceReq.Priority = &p
	}

	// Update work item
	workItem, err := h.service.UpdateWorkItem(ctx, workItemID, serviceReq)
	if err != nil {
		logger.Error("failed to update work item",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		RespondInternalError(w, r, err)
		return
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("workitem:%s", workItemID.String())
	h.cache.Delete(cacheKey)

	// Convert to API response
	response := convertWorkItemToAPI(workItem)

	logger.Info("work item updated",
		slog.String("id", workItem.ID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// validateCreateWorkItemRequest validates the create work item request
func (h *WorkItemHandler) validateCreateWorkItemRequest(req api.CreateWorkItemRequest) []ValidationError {
	var errors []ValidationError

	// Validate project_id (required)
	// ProjectId is not a pointer in the generated code, so it's always present
	// Just validate it's not a zero UUID
	if req.ProjectId == uuid.Nil {
		errors = append(errors, ValidationError{
			Field:   "project_id",
			Code:    "REQUIRED",
			Message: "Project ID is required",
		})
	}

	// Validate title
	if req.Title == "" {
		errors = append(errors, ValidationError{
			Field:   "title",
			Code:    "REQUIRED",
			Message: "Title is required",
		})
	} else if len(req.Title) > 255 {
		errors = append(errors, ValidationError{
			Field:   "title",
			Code:    "TOO_LONG",
			Message: "Title cannot exceed 255 characters",
		})
	}

	// Validate description
	if req.Description == "" {
		errors = append(errors, ValidationError{
			Field:   "description",
			Code:    "REQUIRED",
			Message: "Description is required",
		})
	}

	// Validate type
	validTypes := map[api.WorkItemType]bool{
		api.WorkItemTypeEpic:   true,
		api.WorkItemTypeStory:  true,
		api.WorkItemTypeDefect: true,
	}
	if !validTypes[req.Type] {
		errors = append(errors, ValidationError{
			Field:   "type",
			Code:    "INVALID",
			Message: "Type must be one of: epic, story, defect",
		})
	}

	// Validate priority
	validPriorities := map[api.PriorityLevel]bool{
		api.PriorityLevelLow:      true,
		api.PriorityLevelMedium:   true,
		api.PriorityLevelHigh:     true,
		api.PriorityLevelCritical: true,
	}
	if !validPriorities[req.Priority] {
		errors = append(errors, ValidationError{
			Field:   "priority",
			Code:    "INVALID",
			Message: "Priority must be one of: low, medium, high, critical",
		})
	}

	// Validate story points (if provided)
	if req.StoryPoints != nil && *req.StoryPoints < 0 {
		errors = append(errors, ValidationError{
			Field:   "story_points",
			Code:    "INVALID",
			Message: "Story points cannot be negative",
		})
	}

	// Validate parent_id based on work item type
	// Epic: parent_id must be nil
	// Story/Defect: parent_id must not be nil
	if req.Type == api.WorkItemTypeEpic {
		if req.ParentId != nil {
			errors = append(errors, ValidationError{
				Field:   "parent_id",
				Code:    "INVALID",
				Message: "Epic cannot have a parent work item",
			})
		}
	} else if req.Type == api.WorkItemTypeStory || req.Type == api.WorkItemTypeDefect {
		if req.ParentId == nil {
			errors = append(errors, ValidationError{
				Field:   "parent_id",
				Code:    "REQUIRED",
				Message: fmt.Sprintf("%s must have a parent Epic", req.Type),
			})
		}
	}

	return errors
}

// convertWorkItemToAPI converts a service work item to API work item
func convertWorkItemToAPI(item *models.WorkItem) api.WorkItem {
	apiItem := api.WorkItem{
		Id:          openapi_types.UUID(item.ID),
		Type:        api.WorkItemType(item.Type),
		Title:       item.Title,
		Description: &item.Description,
		Status:      api.WorkItemStatus(item.Status),
		Priority:    api.PriorityLevel(item.Priority),
		StoryPoints: item.StoryPoints,
		ReporterId:  openapi_types.UUID(item.ReporterID),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}

	if item.AssigneeID != nil {
		assigneeUUID := openapi_types.UUID(*item.AssigneeID)
		apiItem.AssigneeId = &assigneeUUID
	}

	if item.ParentID != nil {
		parentUUID := openapi_types.UUID(*item.ParentID)
		apiItem.ParentId = &parentUUID
	}

	if item.SprintID != nil {
		sprintUUID := openapi_types.UUID(*item.SprintID)
		apiItem.SprintId = &sprintUUID
	}

	return apiItem
}

// convertUUIDPtr converts an openapi_types.UUID pointer to a uuid.UUID pointer
func convertUUIDPtr(openapiUUID *openapi_types.UUID) *uuid.UUID {
	if openapiUUID == nil {
		return nil
	}
	u := uuid.UUID(*openapiUUID)
	return &u
}

// convertUUIDPtrToOpenAPI converts a uuid.UUID pointer to an openapi_types.UUID pointer
func convertUUIDPtrToOpenAPI(u *uuid.UUID) *openapi_types.UUID {
	if u == nil {
		return nil
	}
	openapiUUID := openapi_types.UUID(*u)
	return &openapiUUID
}

// convertWorkItemToSummary converts a service work item to API work item summary
func convertWorkItemToSummary(item *models.WorkItem) api.WorkItemSummary {
	summary := api.WorkItemSummary{
		Id:          openapi_types.UUID(item.ID),
		Type:        api.WorkItemSummaryType(item.Type),
		Title:       item.Title,
		Status:      api.WorkItemSummaryStatus(item.Status),
		Priority:    api.WorkItemSummaryPriority(item.Priority),
		StoryPoints: item.StoryPoints,
		SprintId:    convertUUIDPtrToOpenAPI(item.SprintID),
	}

	if item.TicketNumber != nil {
		tn := *item.TicketNumber
		summary.TicketNumber = &tn
	}

	if item.ProjectKey != nil {
		summary.ProjectKey = item.ProjectKey
	}

	return summary
}

// GetWorkItemDependencies handles GET /api/v1/workitems/{id}/dependencies
func (h *WorkItemHandler) GetWorkItemDependencies(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID := uuid.UUID(id)

	// First verify work item exists
	_, err := h.service.GetWorkItem(ctx, workItemID)
	if err != nil {
		logger.Warn("work item not found",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Work item")
		return
	}

	// Get dependencies
	dependencies, err := h.dependenciesService.GetDependenciesForWorkItem(ctx, workItemID)
	if err != nil {
		logger.Error("failed to get dependencies",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		RespondInternalError(w, r, err)
		return
	}

	logger.Debug("work item dependencies retrieved",
		slog.String("id", workItemID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, dependencies)
}

// CreateWorkItemDependency handles POST /api/v1/workitems/{id}/dependencies
func (h *WorkItemHandler) CreateWorkItemDependency(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID := uuid.UUID(id)

	// First verify work item exists
	_, err := h.service.GetWorkItem(ctx, workItemID)
	if err != nil {
		logger.Warn("work item not found",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Work item")
		return
	}

	// Parse request body
	var req api.CreateDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Get user from context
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		RespondUnauthorized(w, r, "User not found in context")
		return
	}

	// Convert to service request
	serviceReq := dependenciesService.CreateDependencyRequest{
		SourceID:       workItemID,
		TargetID:       uuid.UUID(req.TargetId),
		DependencyType: models.DependencyType(req.DependencyType),
		CreatedBy:      user.ID,
	}

	// Create dependency
	dependency, err := h.dependenciesService.CreateDependency(ctx, serviceReq)
	if err != nil {
		logger.Error("failed to create dependency",
			slog.String("source_id", workItemID.String()),
			slog.Any("error", err),
		)
		if err.Error() == "work item cannot depend on itself" {
			RespondBadRequest(w, r, err.Error())
			return
		}
		if err.Error() == "dependency already exists" {
			RespondConflict(w, r, err.Error(), nil)
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	logger.Info("dependency created",
		slog.String("source_id", workItemID.String()),
	)

	RespondWithSuccess(w, r, http.StatusCreated, dependency)
}

// DeleteWorkItemDependency handles DELETE /api/v1/workitems/{id}/dependencies/{dependency_id}
func (h *WorkItemHandler) DeleteWorkItemDependency(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, dependencyID openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID := uuid.UUID(id)
	depID := uuid.UUID(dependencyID)

	// Delete dependency
	err := h.dependenciesService.DeleteDependency(ctx, depID)
	if err != nil {
		logger.Error("failed to delete dependency",
			slog.String("work_item_id", workItemID.String()),
			slog.String("dependency_id", depID.String()),
			slog.Any("error", err),
		)
		if err.Error() == "dependency not found" {
			RespondNotFound(w, r, "Dependency")
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	logger.Info("dependency deleted",
		slog.String("work_item_id", workItemID.String()),
		slog.String("dependency_id", depID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetChildWorkItems handles GET /api/v1/workitems/{id}/children
func (h *WorkItemHandler) GetChildWorkItems(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID := uuid.UUID(id)

	// First verify work item exists
	_, err := h.service.GetWorkItem(ctx, workItemID)
	if err != nil {
		logger.Warn("work item not found",
			slog.String("id", workItemID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Work item")
		return
	}

	// TODO: Implement GetChildWorkItems in service
	// For now, return empty list
	response := api.WorkItemListResponse{
		Items: []api.WorkItemSummary{},
		Pagination: api.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
			TotalCount: intToPtr(0),
		},
	}

	logger.Debug("child work items retrieved",
		slog.String("id", workItemID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// GetWorkItemComments handles GET /api/v1/workitems/{id}/comments
// TODO: Implement comments functionality
func (h *WorkItemHandler) GetWorkItemComments(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	workItemID, err := uuid.Parse(id.String())
	if err != nil {
		logger.Error("invalid work item ID",
			slog.String("id", id.String()),
			slog.String("error", err.Error()),
		)
		RespondBadRequest(w, r, "Invalid work item ID")
		return
	}

	// Verify work item exists
	_, err = h.service.GetWorkItem(ctx, workItemID)
	if err != nil {
		logger.Error("failed to get work item",
			slog.String("id", workItemID.String()),
			slog.String("error", err.Error()),
		)
		RespondNotFound(w, r, "Work item")
		return
	}

	// TODO: Implement comments service
	// For now, return empty list
	response := []interface{}{}

	logger.Debug("work item comments retrieved (stub)",
		slog.String("id", workItemID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// GetAuditLogs handles GET /api/v1/audit-logs
// TODO: Implement audit logs functionality
func (h *WorkItemHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// TODO: Implement audit logs service
	// For now, return empty list
	response := []interface{}{}

	logger.Debug("audit logs retrieved (stub)")

	RespondWithSuccess(w, r, http.StatusOK, response)
}
