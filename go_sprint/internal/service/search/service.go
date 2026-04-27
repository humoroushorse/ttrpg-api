package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides search and filtering functionality for work items
type Service struct {
	master  *pgxpool.Pool
	replica *pgxpool.Pool
}

// NewService creates a new search service
func NewService(master, replica *pgxpool.Pool) *Service {
	return &Service{
		master:  master,
		replica: replica,
	}
}

// SearchRequest represents a search request with filters
type SearchRequest struct {
	Query      string                                    // Search query text
	Field      string                                    // Field to search: "title", "description", or "all"
	Type       *workitems.SprintManagementWorkItemType   // Filter by work item type
	Status     *workitems.SprintManagementWorkItemStatus // Filter by status
	Priority   *workitems.SprintManagementPriorityLevel  // Filter by priority
	AssigneeID *pgtype.UUID                              // Filter by assignee
	SprintID   *pgtype.UUID                              // Filter by sprint
	ReporterID *pgtype.UUID                              // Filter by reporter
	ParentID   *pgtype.UUID                              // Filter by parent
	Limit      int32                                     // Page size
	Offset     int32                                     // Pagination offset
	UseBoolean bool                                      // Use boolean search operators
}

// SearchResult represents a search result with ranking
type SearchResult struct {
	WorkItem workitems.SprintManagementWorkItem
	Rank     interface{} // Relevance rank from full-text search
}

// FilterRequest represents a filter-only request (no search text)
type FilterRequest struct {
	Type       *workitems.SprintManagementWorkItemType
	Status     *workitems.SprintManagementWorkItemStatus
	Priority   *workitems.SprintManagementPriorityLevel
	AssigneeID *pgtype.UUID
	SprintID   *pgtype.UUID
	ReporterID *pgtype.UUID
	ParentID   *pgtype.UUID
	Limit      int32
	Offset     int32
}

// Search performs full-text search with optional filters
func (s *Service) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// Validate request
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	// If no query text, use filter-only search
	if strings.TrimSpace(req.Query) == "" {
		return s.filterOnly(ctx, req)
	}

	// Determine which search method to use
	if req.UseBoolean {
		return s.searchBoolean(ctx, req)
	}

	// Check if filters are provided
	hasFilters := req.Type != nil || req.Status != nil || req.Priority != nil ||
		req.AssigneeID != nil || req.SprintID != nil

	if hasFilters {
		return s.searchWithFilters(ctx, req)
	}

	// Check if field-specific search
	if req.Field != "" && req.Field != "all" {
		return s.searchByField(ctx, req)
	}

	// Default: simple full-text search
	return s.searchSimple(ctx, req)
}

// searchSimple performs basic full-text search
func (s *Service) searchSimple(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	queries := workitems.New(s.replica)

	rows, err := queries.SearchWorkItems(ctx, workitems.SearchWorkItemsParams{
		PlaintoTsquery: req.Query,
		Limit:          req.Limit,
		Offset:         req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search work items: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			WorkItem: workitems.SprintManagementWorkItem{
				ID:          row.ID,
				Type:        row.Type,
				Title:       row.Title,
				Description: row.Description,
				Status:      row.Status,
				Priority:    row.Priority,
				StoryPoints: row.StoryPoints,
				AssigneeID:  row.AssigneeID,
				ReporterID:  row.ReporterID,
				ParentID:    row.ParentID,
				SprintID:    row.SprintID,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				DeletedAt:   row.DeletedAt,
				DeletedBy:   row.DeletedBy,
			},
			Rank: row.Rank,
		})
	}

	return results, nil
}

// searchWithFilters performs search with multiple filters
func (s *Service) searchWithFilters(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	queries := workitems.New(s.replica)

	// Convert nullable types for sqlc
	var typeParam workitems.SprintManagementWorkItemType
	if req.Type != nil {
		typeParam = *req.Type
	}

	var statusParam workitems.SprintManagementWorkItemStatus
	if req.Status != nil {
		statusParam = *req.Status
	}

	var priorityParam workitems.SprintManagementPriorityLevel
	if req.Priority != nil {
		priorityParam = *req.Priority
	}

	var assigneeParam pgtype.UUID
	if req.AssigneeID != nil {
		assigneeParam = *req.AssigneeID
	}

	var sprintParam pgtype.UUID
	if req.SprintID != nil {
		sprintParam = *req.SprintID
	}

	rows, err := queries.SearchWorkItemsWithFilters(ctx, workitems.SearchWorkItemsWithFiltersParams{
		PlaintoTsquery: req.Query,
		Column2:        typeParam,
		Column3:        statusParam,
		Column4:        priorityParam,
		Column5:        assigneeParam,
		Column6:        sprintParam,
		Limit:          req.Limit,
		Offset:         req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search work items with filters: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			WorkItem: workitems.SprintManagementWorkItem{
				ID:          row.ID,
				Type:        row.Type,
				Title:       row.Title,
				Description: row.Description,
				Status:      row.Status,
				Priority:    row.Priority,
				StoryPoints: row.StoryPoints,
				AssigneeID:  row.AssigneeID,
				ReporterID:  row.ReporterID,
				ParentID:    row.ParentID,
				SprintID:    row.SprintID,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				DeletedAt:   row.DeletedAt,
				DeletedBy:   row.DeletedBy,
			},
			Rank: row.Rank,
		})
	}

	return results, nil
}

// searchByField performs field-specific search
func (s *Service) searchByField(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	queries := workitems.New(s.replica)

	rows, err := queries.SearchWorkItemsByField(ctx, workitems.SearchWorkItemsByFieldParams{
		PlaintoTsquery: req.Query,
		Column2:        req.Field,
		Limit:          req.Limit,
		Offset:         req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search work items by field: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			WorkItem: workitems.SprintManagementWorkItem{
				ID:          row.ID,
				Type:        row.Type,
				Title:       row.Title,
				Description: row.Description,
				Status:      row.Status,
				Priority:    row.Priority,
				StoryPoints: row.StoryPoints,
				AssigneeID:  row.AssigneeID,
				ReporterID:  row.ReporterID,
				ParentID:    row.ParentID,
				SprintID:    row.SprintID,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				DeletedAt:   row.DeletedAt,
				DeletedBy:   row.DeletedBy,
			},
			Rank: row.Rank,
		})
	}

	return results, nil
}

// searchBoolean performs boolean search with AND/OR operators
func (s *Service) searchBoolean(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	queries := workitems.New(s.replica)

	rows, err := queries.SearchWorkItemsBoolean(ctx, workitems.SearchWorkItemsBooleanParams{
		WebsearchToTsquery: req.Query,
		Limit:              req.Limit,
		Offset:             req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to perform boolean search: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			WorkItem: workitems.SprintManagementWorkItem{
				ID:          row.ID,
				Type:        row.Type,
				Title:       row.Title,
				Description: row.Description,
				Status:      row.Status,
				Priority:    row.Priority,
				StoryPoints: row.StoryPoints,
				AssigneeID:  row.AssigneeID,
				ReporterID:  row.ReporterID,
				ParentID:    row.ParentID,
				SprintID:    row.SprintID,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				DeletedAt:   row.DeletedAt,
				DeletedBy:   row.DeletedBy,
			},
			Rank: row.Rank,
		})
	}

	return results, nil
}

// filterOnly performs filtering without search text
func (s *Service) filterOnly(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	queries := workitems.New(s.replica)

	// Convert nullable types for sqlc
	var typeParam workitems.SprintManagementWorkItemType
	if req.Type != nil {
		typeParam = *req.Type
	}

	var statusParam workitems.SprintManagementWorkItemStatus
	if req.Status != nil {
		statusParam = *req.Status
	}

	var priorityParam workitems.SprintManagementPriorityLevel
	if req.Priority != nil {
		priorityParam = *req.Priority
	}

	var assigneeParam pgtype.UUID
	if req.AssigneeID != nil {
		assigneeParam = *req.AssigneeID
	}

	var sprintParam pgtype.UUID
	if req.SprintID != nil {
		sprintParam = *req.SprintID
	}

	var reporterParam pgtype.UUID
	if req.ReporterID != nil {
		reporterParam = *req.ReporterID
	}

	var parentParam pgtype.UUID
	if req.ParentID != nil {
		parentParam = *req.ParentID
	}

	rows, err := queries.FilterWorkItems(ctx, workitems.FilterWorkItemsParams{
		Column1: typeParam,
		Column2: statusParam,
		Column3: priorityParam,
		Column4: assigneeParam,
		Column5: sprintParam,
		Column6: reporterParam,
		Column7: parentParam,
		Limit:   req.Limit,
		Offset:  req.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to filter work items: %w", err)
	}

	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			WorkItem: row,
			Rank:     nil, // No ranking for filter-only queries
		})
	}

	return results, nil
}

// CountSearchResults returns the total count of search results for pagination
func (s *Service) CountSearchResults(ctx context.Context, query string) (int64, error) {
	if strings.TrimSpace(query) == "" {
		return 0, fmt.Errorf("query cannot be empty")
	}

	queries := workitems.New(s.replica)
	return queries.CountSearchResults(ctx, query)
}

// CountFilteredResults returns the total count of filtered results
func (s *Service) CountFilteredResults(ctx context.Context, req FilterRequest) (int64, error) {
	queries := workitems.New(s.replica)

	// Convert nullable types for sqlc
	var typeParam workitems.SprintManagementWorkItemType
	if req.Type != nil {
		typeParam = *req.Type
	}

	var statusParam workitems.SprintManagementWorkItemStatus
	if req.Status != nil {
		statusParam = *req.Status
	}

	var priorityParam workitems.SprintManagementPriorityLevel
	if req.Priority != nil {
		priorityParam = *req.Priority
	}

	var assigneeParam pgtype.UUID
	if req.AssigneeID != nil {
		assigneeParam = *req.AssigneeID
	}

	var sprintParam pgtype.UUID
	if req.SprintID != nil {
		sprintParam = *req.SprintID
	}

	var reporterParam pgtype.UUID
	if req.ReporterID != nil {
		reporterParam = *req.ReporterID
	}

	var parentParam pgtype.UUID
	if req.ParentID != nil {
		parentParam = *req.ParentID
	}

	return queries.CountFilteredWorkItems(ctx, workitems.CountFilteredWorkItemsParams{
		Column1: typeParam,
		Column2: statusParam,
		Column3: priorityParam,
		Column4: assigneeParam,
		Column5: sprintParam,
		Column6: reporterParam,
		Column7: parentParam,
	})
}

// validateSearchRequest validates the search request parameters
func (s *Service) validateSearchRequest(req SearchRequest) error {
	if req.Limit <= 0 {
		return fmt.Errorf("limit must be greater than 0")
	}

	if req.Limit > 1000 {
		return fmt.Errorf("limit cannot exceed 1000")
	}

	if req.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}

	if req.Field != "" && req.Field != "all" && req.Field != "title" && req.Field != "description" {
		return fmt.Errorf("field must be one of: all, title, description")
	}

	return nil
}
