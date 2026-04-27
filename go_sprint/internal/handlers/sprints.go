package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	api "github.com/humoroushorse/go_sprint/api/generated"
	"github.com/humoroushorse/go_sprint/internal/middleware"
	"github.com/humoroushorse/go_sprint/internal/service/estimation"
	"github.com/humoroushorse/go_sprint/internal/service/sprints"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/humoroushorse/go_sprint/pkg/pagination"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// SprintService defines the interface for sprint business logic
type SprintService interface {
	CreateSprint(ctx context.Context, req sprints.CreateSprintRequest) (*models.Sprint, error)
	GetSprint(ctx context.Context, id uuid.UUID) (*models.Sprint, error)
	UpdateSprint(ctx context.Context, id uuid.UUID, req sprints.UpdateSprintRequest) (*models.Sprint, error)
	ListSprints(ctx context.Context, cursor *pagination.Cursor, limit int32, searchQuery *string, statusFilter []string) ([]*models.Sprint, error)
	GetSprintMetrics(ctx context.Context, id uuid.UUID) (*sprints.SprintMetrics, error)
	CloseSprint(ctx context.Context, id uuid.UUID) (*models.Sprint, error)
}

// SprintHandler implements the ServerInterface for sprint endpoints
type SprintHandler struct {
	service           SprintService
	burndownService   BurndownService
	estimationService EstimationService
	logger            *slog.Logger
}

// NewSprintHandler creates a new sprint handler
func NewSprintHandler(service SprintService, burndownService BurndownService, estimationService EstimationService, logger *slog.Logger) *SprintHandler {
	return &SprintHandler{
		service:           service,
		burndownService:   burndownService,
		estimationService: estimationService,
		logger:            logger,
	}
}

// CreateSprint handles POST /api/v1/sprints
func (h *SprintHandler) CreateSprint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// Get user from context
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok {
		RespondUnauthorized(w, r, "User not found in context")
		return
	}

	// Parse request body
	var req api.CreateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Validate required fields
	validationErrors := h.validateCreateSprintRequest(req)
	if len(validationErrors) > 0 {
		RespondWithValidationError(w, r, validationErrors)
		return
	}

	// Sanitize inputs
	req.Name = middleware.SanitizeInput(req.Name)
	if req.Description != nil {
		desc := middleware.SanitizeInput(*req.Description)
		req.Description = &desc
	}

	// Get user timezone from header (optional, defaults to UTC)
	userTimezone := r.Header.Get("X-User-Timezone")
	if userTimezone == "" {
		userTimezone = "UTC"
	}

	// Convert API request to service request
	serviceReq := sprints.CreateSprintRequest{
		Name:           req.Name,
		Description:    getStringValue(req.Description),
		StartDate:      req.StartDate.Time,
		EndDate:        req.EndDate.Time,
		CapacityPoints: req.CapacityPoints,
		CreatedBy:      user.ID,
		UserTimezone:   userTimezone,
	}

	// Create sprint
	sprint, err := h.service.CreateSprint(ctx, serviceReq)
	if err != nil {
		logger.Error("failed to create sprint",
			slog.Any("error", err),
			slog.String("name", serviceReq.Name),
		)
		// Check if it's a validation error
		if errors.Is(err, sprints.ErrInvalidDateRange) ||
			errors.Is(err, sprints.ErrRequiredFieldMissing) ||
			err.Error() == "start date cannot be in the past" {
			RespondBadRequest(w, r, err.Error())
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	// Convert to API response
	response := convertSprintToAPI(sprint)

	logger.Info("sprint created",
		slog.String("id", sprint.ID.String()),
		slog.String("name", sprint.Name),
	)

	RespondWithSuccess(w, r, http.StatusCreated, response)
}

// GetSprint handles GET /api/v1/sprints/{id}
func (h *SprintHandler) GetSprint(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// Get sprint
	sprint, err := h.service.GetSprint(ctx, sprintID)
	if err != nil {
		logger.Warn("sprint not found",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Sprint")
		return
	}

	// Convert to API response
	response := convertSprintToAPI(sprint)

	logger.Debug("sprint retrieved",
		slog.String("id", sprintID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// UpdateSprint handles PUT /api/v1/sprints/{id}
func (h *SprintHandler) UpdateSprint(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// Parse request body
	var req api.UpdateSprintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Sanitize inputs
	if req.Name != nil {
		name := middleware.SanitizeInput(*req.Name)
		req.Name = &name
	}
	if req.Description != nil {
		desc := middleware.SanitizeInput(*req.Description)
		req.Description = &desc
	}

	// Convert API request to service request
	serviceReq := sprints.UpdateSprintRequest{
		Name:           req.Name,
		Description:    req.Description,
		CapacityPoints: req.CapacityPoints,
	}

	// Handle dates if provided
	if req.StartDate != nil {
		serviceReq.StartDate = &req.StartDate.Time
	}
	if req.EndDate != nil {
		serviceReq.EndDate = &req.EndDate.Time
	}

	// Handle status if provided
	if req.Status != nil {
		status := string(*req.Status)
		serviceReq.Status = &status
	}

	// Update sprint
	sprint, err := h.service.UpdateSprint(ctx, sprintID, serviceReq)
	if err != nil {
		logger.Error("failed to update sprint",
			slog.Any("error", err),
			slog.String("id", sprintID.String()),
		)
		// Check if it's a validation error
		if errors.Is(err, sprints.ErrInvalidDateRange) ||
			errors.Is(err, sprints.ErrRequiredFieldMissing) ||
			errors.Is(err, sprints.ErrSprintNotFound) {
			RespondBadRequest(w, r, err.Error())
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	// Convert to API response
	response := convertSprintToAPI(sprint)

	logger.Info("sprint updated",
		slog.String("id", sprint.ID.String()),
		slog.String("name", sprint.Name),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// DeleteSprint handles DELETE /api/v1/sprints/{id}
func (h *SprintHandler) DeleteSprint(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// TODO: Implement DeleteSprint in service
	logger.Warn("delete sprint not yet implemented",
		slog.String("id", sprintID.String()),
	)

	http.Error(w, "Delete sprint not yet implemented", http.StatusNotImplemented)
}

// ListSprints handles GET /api/v1/sprints
func (h *SprintHandler) ListSprints(w http.ResponseWriter, r *http.Request, params api.ListSprintsParams) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// WORKAROUND: oapi-codegen doesn't bind query params in chi, so parse manually
	queryParams := r.URL.Query()
	if filtersParam := queryParams.Get("filters"); filtersParam != "" {
		params.Filters = &filtersParam
	}
	if cursorParam := queryParams.Get("cursor"); cursorParam != "" {
		params.Cursor = &cursorParam
	}
	if limitParam := queryParams.Get("limit"); limitParam != "" {
		if limitInt, err := strconv.Atoi(limitParam); err == nil {
			params.Limit = &limitInt
		}
	}

	// Get pagination config
	paginationConfig := pagination.DefaultConfig()

	// Validate and normalize page size
	var limitPtr *int32
	if params.Limit != nil {
		limit32 := int32(*params.Limit)
		limitPtr = &limit32
	}
	limit := pagination.ValidatePageSize(limitPtr, paginationConfig)

	// Decode cursor if provided
	var cursor *pagination.Cursor
	if params.Cursor != nil && *params.Cursor != "" {
		decodedCursor, err := pagination.DecodeCursor(*params.Cursor)
		if err != nil {
			logger.Warn("invalid cursor provided",
				slog.String("cursor", *params.Cursor),
				slog.Any("error", err),
			)
			RespondBadRequest(w, r, "Invalid cursor format")
			return
		}
		cursor = &decodedCursor
	}

	// Parse filters from query parameter
	var searchQuery *string
	var statusFilter []string

	if params.Filters != nil && *params.Filters != "" {
		var filters []map[string]interface{}
		if err := json.Unmarshal([]byte(*params.Filters), &filters); err != nil {
			logger.Warn("invalid filters format",
				slog.String("filters", *params.Filters),
				slog.Any("error", err),
			)
			RespondBadRequest(w, r, "Invalid filters format")
			return
		}

		// Process each filter
		for _, filter := range filters {
			field, ok := filter["field"].(string)
			if !ok {
				continue
			}

			switch field {
			case "search":
				if value, ok := filter["value"].(string); ok && value != "" {
					searchQuery = &value
				}
			case "status":
				if value, ok := filter["value"].([]interface{}); ok {
					for _, v := range value {
						if status, ok := v.(string); ok {
							statusFilter = append(statusFilter, status)
						}
					}
				} else if value, ok := filter["value"].(string); ok && value != "" {
					statusFilter = append(statusFilter, value)
				}
			}
		}
	}

	// List sprints with filters
	items, err := h.service.ListSprints(ctx, cursor, limit, searchQuery, statusFilter)
	if err != nil {
		logger.Error("failed to list sprints",
			slog.Any("error", err),
		)
		RespondInternalError(w, r, err)
		return
	}

	// Convert to API response
	apiItems := make([]api.Sprint, len(items))
	for i, item := range items {
		apiItems[i] = convertSprintToAPI(item)
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
		RespondInternalError(w, r, err)
		return
	}

	response := api.SprintListResponse{
		Items: apiItems,
		Pagination: api.PaginationInfo{
			HasMore:    paginationResp.HasMore,
			NextCursor: paginationResp.NextCursor,
			TotalCount: nil, // Total count is expensive, only compute if requested
		},
	}

	logger.Debug("sprints listed",
		slog.Int("count", len(apiItems)),
		slog.Bool("has_more", paginationResp.HasMore),
		slog.Any("search_query", searchQuery),
		slog.Any("status_filter", statusFilter),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// validateCreateSprintRequest validates the create sprint request
func (h *SprintHandler) validateCreateSprintRequest(req api.CreateSprintRequest) []ValidationError {
	var errors []ValidationError

	// Validate name
	if req.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Code:    "REQUIRED",
			Message: "Name is required",
		})
	} else if len(req.Name) > 255 {
		errors = append(errors, ValidationError{
			Field:   "name",
			Code:    "TOO_LONG",
			Message: "Name cannot exceed 255 characters",
		})
	}

	// Validate dates
	if req.StartDate.Time.IsZero() {
		errors = append(errors, ValidationError{
			Field:   "start_date",
			Code:    "REQUIRED",
			Message: "Start date is required",
		})
	}

	if req.EndDate.Time.IsZero() {
		errors = append(errors, ValidationError{
			Field:   "end_date",
			Code:    "REQUIRED",
			Message: "End date is required",
		})
	}

	// Validate date range
	if !req.StartDate.Time.IsZero() && !req.EndDate.Time.IsZero() {
		if !req.EndDate.Time.After(req.StartDate.Time) {
			errors = append(errors, ValidationError{
				Field:   "end_date",
				Code:    "INVALID",
				Message: "End date must be after start date",
			})
		}
	}

	// Validate capacity points (if provided)
	if req.CapacityPoints != nil && *req.CapacityPoints < 0 {
		errors = append(errors, ValidationError{
			Field:   "capacity_points",
			Code:    "INVALID",
			Message: "Capacity points cannot be negative",
		})
	}

	return errors
}

// convertSprintToAPI converts a service sprint to API sprint
func convertSprintToAPI(item *models.Sprint) api.Sprint {
	apiItem := api.Sprint{
		Id:              openapi_types.UUID(item.ID),
		Name:            item.Name,
		Description:     stringToPtr(item.Description),
		Status:          api.SprintStatus(item.Status),
		StartDate:       openapi_types.Date{Time: item.StartDate},
		EndDate:         openapi_types.Date{Time: item.EndDate},
		CapacityPoints:  item.CapacityPoints,
		CommittedPoints: intToPtr(item.CommittedPoints),
		CompletedPoints: intToPtr(item.CompletedPoints),
		CreatedBy:       openapi_types.UUID(item.CreatedBy),
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}

	return apiItem
}

// Helper functions

func stringToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intToPtr(i int) *int {
	return &i
}

func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetSprintMetrics handles GET /api/v1/sprints/{id}/metrics
func (h *SprintHandler) GetSprintMetrics(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// Get sprint metrics
	metrics, err := h.service.GetSprintMetrics(ctx, sprintID)
	if err != nil {
		logger.Warn("failed to get sprint metrics",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		if err.Error() == "sprint not found" {
			RespondNotFound(w, r, "Sprint")
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	// Convert to API response
	response := api.SprintMetrics{
		TotalItems:           metrics.TotalWorkItems,
		CompletedItems:       metrics.CompletedWorkItems,
		InProgressItems:      metrics.InProgressWorkItems,
		TodoItems:            metrics.TodoWorkItems,
		BlockedItems:         intToPtr(0), // TODO: Add blocked items tracking
		CommittedPoints:      metrics.CommittedPoints,
		CompletedPoints:      metrics.CompletedPoints,
		RemainingPoints:      intToPtr(metrics.CommittedPoints - metrics.CompletedPoints),
		CompletionPercentage: float32(metrics.CompletionRate),
		Velocity:             nil, // TODO: Add velocity calculation
		DaysRemaining:        nil, // TODO: Add days remaining calculation
	}

	logger.Debug("sprint metrics retrieved",
		slog.String("id", sprintID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// GetSprintWorkItems handles GET /api/v1/sprints/{id}/workitems
func (h *SprintHandler) GetSprintWorkItems(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, params api.GetSprintWorkItemsParams) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// First verify sprint exists
	_, err := h.service.GetSprint(ctx, sprintID)
	if err != nil {
		logger.Warn("sprint not found",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Sprint")
		return
	}

	// TODO: Implement GetWorkItemsBySprint in service
	// For now, return empty list
	response := api.WorkItemListResponse{
		Items: []api.WorkItemSummary{},
		Pagination: api.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
			TotalCount: intToPtr(0),
		},
	}

	logger.Debug("sprint work items retrieved",
		slog.String("id", sprintID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// CloseSprint handles POST /api/v1/sprints/{id}/close
func (h *SprintHandler) CloseSprint(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// Close sprint
	sprint, err := h.service.CloseSprint(ctx, sprintID)
	if err != nil {
		logger.Error("failed to close sprint",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		if err.Error() == "sprint not found" {
			RespondNotFound(w, r, "Sprint")
			return
		}
		if err.Error() == "cannot modify closed sprint" {
			RespondConflict(w, r, "Sprint is already closed", nil)
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	// Get final metrics
	metrics, err := h.service.GetSprintMetrics(ctx, sprintID)
	if err != nil {
		logger.Warn("failed to get final metrics", slog.Any("error", err))
		metrics = &sprints.SprintMetrics{} // Use empty metrics if fetch fails
	}

	// Convert to API response
	response := api.SprintCloseResult{
		SprintId:       openapi_types.UUID(sprint.ID),
		CompletedItems: metrics.CompletedWorkItems,
		MovedToBacklog: metrics.TodoWorkItems + metrics.InProgressWorkItems,
		FinalMetrics: api.SprintMetrics{
			TotalItems:           metrics.TotalWorkItems,
			CompletedItems:       metrics.CompletedWorkItems,
			InProgressItems:      metrics.InProgressWorkItems,
			TodoItems:            metrics.TodoWorkItems,
			BlockedItems:         intToPtr(0),
			CommittedPoints:      metrics.CommittedPoints,
			CompletedPoints:      metrics.CompletedPoints,
			RemainingPoints:      intToPtr(metrics.CommittedPoints - metrics.CompletedPoints),
			CompletionPercentage: float32(metrics.CompletionRate),
			Velocity:             nil,
			DaysRemaining:        nil,
		},
	}

	logger.Info("sprint closed",
		slog.String("id", sprintID.String()),
		slog.Int("completed", metrics.CompletedWorkItems),
		slog.Int("moved_to_backlog", metrics.TodoWorkItems+metrics.InProgressWorkItems),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// GetSprintBurndown handles GET /api/v1/sprints/{id}/burndown
func (h *SprintHandler) GetSprintBurndown(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// Get burndown chart
	burndown, err := h.burndownService.GetBurndownChart(ctx, sprintID)
	if err != nil {
		logger.Warn("failed to get burndown chart",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		if err.Error() == "sprint not found" {
			RespondNotFound(w, r, "Sprint")
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	logger.Debug("burndown chart retrieved",
		slog.String("id", sprintID.String()),
	)

	RespondWithSuccess(w, r, http.StatusOK, burndown)
}

// GetSprintVelocity handles GET /api/v1/sprints/{id}/velocity
func (h *SprintHandler) GetSprintVelocity(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	sprintID := uuid.UUID(id)

	// First verify sprint exists and is completed
	sprint, err := h.service.GetSprint(ctx, sprintID)
	if err != nil {
		logger.Warn("sprint not found",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		RespondNotFound(w, r, "Sprint")
		return
	}

	if sprint.Status != "completed" {
		RespondConflict(w, r, "Velocity can only be calculated for completed sprints", nil)
		return
	}

	// Get sprint metrics for velocity
	metrics, err := h.service.GetSprintMetrics(ctx, sprintID)
	if err != nil {
		logger.Error("failed to get sprint metrics",
			slog.String("id", sprintID.String()),
			slog.Any("error", err),
		)
		RespondInternalError(w, r, err)
		return
	}

	// Calculate velocity (completed points)
	velocity := float32(metrics.CompletedPoints)
	completionRate := float32(0)
	if metrics.CommittedPoints > 0 {
		completionRate = float32(metrics.CompletedPoints) / float32(metrics.CommittedPoints) * 100
	}

	response := api.VelocityData{
		SprintId:        openapi_types.UUID(sprint.ID),
		Velocity:        velocity,
		CompletedPoints: metrics.CompletedPoints,
		CommittedPoints: metrics.CommittedPoints,
		CompletionRate:  &completionRate,
	}

	logger.Debug("sprint velocity retrieved",
		slog.String("id", sprintID.String()),
		slog.Float64("velocity", float64(velocity)),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// CalculateSprintCapacity handles POST /api/v1/sprints/planning/capacity
func (h *SprintHandler) CalculateSprintCapacity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// Parse request body
	var req api.CapacityCalculationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Default lookback sprints to 3 if not provided
	sprintCount := 3
	if req.LookbackSprints != nil {
		sprintCount = *req.LookbackSprints
	}

	// Calculate velocity metrics
	velocity, err := h.estimationService.CalculateVelocity(ctx, sprintCount)
	if err != nil {
		logger.Error("failed to calculate velocity",
			slog.Any("error", err),
			slog.Int("sprint_count", sprintCount),
		)
		if errors.Is(err, estimation.ErrNoCompletedSprints) {
			RespondBadRequest(w, r, "No completed sprints found for capacity calculation")
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	// Apply adjustment factor if provided
	adjustmentFactor := 1.0
	if req.AdjustmentFactor != nil {
		adjustmentFactor = float64(*req.AdjustmentFactor)
	}

	recommendedCapacity := int(velocity.AverageVelocity * adjustmentFactor)
	minCapacity := int(float64(velocity.MinVelocity) * adjustmentFactor)
	maxCapacity := int(float64(velocity.MaxVelocity) * adjustmentFactor)

	// Determine confidence level
	confidenceLevel := "low"
	if velocity.TotalSprints >= 5 {
		velocityRange := velocity.MaxVelocity - velocity.MinVelocity
		if velocityRange <= int(velocity.AverageVelocity*0.2) {
			confidenceLevel = "high"
		} else if velocityRange <= int(velocity.AverageVelocity*0.5) {
			confidenceLevel = "medium"
		}
	}

	// Build historical sprint data
	historicalSprints := make([]api.HistoricalSprintData, len(velocity.SprintVelocities))
	for i, sv := range velocity.SprintVelocities {
		completionRate := float32(100.0) // Assume 100% for completed sprints
		historicalSprints[i] = api.HistoricalSprintData{
			SprintId:       openapi_types.UUID(sv.SprintID),
			SprintName:     sv.SprintName,
			Velocity:       float32(sv.CompletedPoints),
			CompletionRate: completionRate,
		}
	}

	response := api.CapacityRecommendation{
		RecommendedCapacity: recommendedCapacity,
		AverageVelocity:     float32(velocity.AverageVelocity),
		MinCapacity:         &minCapacity,
		MaxCapacity:         &maxCapacity,
		ConfidenceLevel:     api.CapacityRecommendationConfidenceLevel(confidenceLevel),
		HistoricalSprints:   &historicalSprints,
	}

	logger.Info("capacity calculated",
		slog.Int("recommended_capacity", recommendedCapacity),
		slog.Float64("average_velocity", velocity.AverageVelocity),
		slog.String("confidence", confidenceLevel),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}

// ForecastSprintCompletion handles POST /api/v1/sprints/planning/forecast
func (h *SprintHandler) ForecastSprintCompletion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := middleware.LoggerFromContext(ctx)

	// Parse request body
	var req api.ForecastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("failed to decode request body", slog.Any("error", err))
		RespondBadRequest(w, r, "Invalid request body")
		return
	}

	// Validate remaining points
	if req.RemainingPoints < 0 {
		RespondBadRequest(w, r, "Remaining points cannot be negative")
		return
	}

	// Default lookback sprints to 3 if not provided
	sprintCount := 3
	if req.LookbackSprints != nil {
		sprintCount = *req.LookbackSprints
	}

	// Create forecast request for service
	forecastReq := estimation.ForecastRequest{
		RemainingPoints: req.RemainingPoints,
		SprintCount:     sprintCount,
	}

	// Calculate forecast
	forecast, err := h.estimationService.ForecastCompletion(ctx, forecastReq)
	if err != nil {
		logger.Error("failed to calculate forecast",
			slog.Any("error", err),
			slog.Int("remaining_points", req.RemainingPoints),
		)
		if errors.Is(err, estimation.ErrNoCompletedSprints) {
			RespondBadRequest(w, r, "No completed sprints found for forecasting")
			return
		}
		RespondInternalError(w, r, err)
		return
	}

	response := api.ForecastResult{
		EstimatedSprints: float32(forecast.EstimatedSprints),
		AverageVelocity:  float32(forecast.AverageVelocity),
		MinSprints:       &forecast.EstimatedSprintsMin,
		MaxSprints:       &forecast.EstimatedSprintsMax,
		ConfidenceLevel:  api.ForecastResultConfidenceLevel(forecast.ConfidenceLevel),
	}

	logger.Info("forecast calculated",
		slog.Int("remaining_points", req.RemainingPoints),
		slog.Int("estimated_sprints", forecast.EstimatedSprints),
		slog.String("confidence", forecast.ConfidenceLevel),
	)

	RespondWithSuccess(w, r, http.StatusOK, response)
}
