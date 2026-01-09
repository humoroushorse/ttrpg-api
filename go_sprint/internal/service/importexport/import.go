package importexport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

var (
	// ErrInvalidFormat is returned when the import format is invalid
	ErrInvalidFormat = errors.New("invalid import format")
	// ErrValidationFailed is returned when validation fails
	ErrValidationFailed = errors.New("validation failed")
	// ErrConflict is returned when a conflict is detected
	ErrConflict = errors.New("conflict detected")
)

// ConflictResolutionStrategy defines how to handle conflicts during import
type ConflictResolutionStrategy string

const (
	// ConflictStrategySkip skips conflicting records
	ConflictStrategySkip ConflictResolutionStrategy = "skip"
	// ConflictStrategyUpdate updates existing records with new data
	ConflictStrategyUpdate ConflictResolutionStrategy = "update"
	// ConflictStrategyError returns an error on conflict
	ConflictStrategyError ConflictResolutionStrategy = "error"
)

// WorkItemRepository defines the interface for work item operations
type WorkItemRepository interface {
	CreateWorkItem(ctx context.Context, req CreateWorkItemRequest) (*models.WorkItem, error)
	GetWorkItemByID(ctx context.Context, id uuid.UUID) (*models.WorkItem, error)
	UpdateWorkItem(ctx context.Context, id uuid.UUID, req UpdateWorkItemRequest) (*models.WorkItem, error)
}

// CreateWorkItemRequest represents a request to create a work item
type CreateWorkItemRequest struct {
	Type        models.WorkItemType
	Title       string
	Description string
	Priority    models.PriorityLevel
	StoryPoints *int
	AssigneeID  *uuid.UUID
	ReporterID  uuid.UUID
	ParentID    *uuid.UUID
	SprintID    *uuid.UUID
}

// UpdateWorkItemRequest represents a request to update a work item
type UpdateWorkItemRequest struct {
	Title       *string
	Description *string
	Status      *models.WorkItemStatus
	Priority    *models.PriorityLevel
	StoryPoints *int
	AssigneeID  *uuid.UUID
	SprintID    *uuid.UUID
}

// ImportService handles data import operations
type ImportService struct {
	workItemRepo WorkItemRepository
	logger       *slog.Logger
}

// NewImportService creates a new import service
func NewImportService(workItemRepo WorkItemRepository, logger *slog.Logger) *ImportService {
	return &ImportService{
		workItemRepo: workItemRepo,
		logger:       logger,
	}
}

// ImportResult represents the result of an import operation
type ImportResult struct {
	TotalRecords  int                `json:"total_records"`
	SuccessCount  int                `json:"success_count"`
	SkippedCount  int                `json:"skipped_count"`
	ErrorCount    int                `json:"error_count"`
	Errors        []ImportError      `json:"errors,omitempty"`
	ImportedItems []*models.WorkItem `json:"imported_items,omitempty"`
}

// ImportError represents an error during import
type ImportError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// WorkItemImportRecord represents a work item record for import
type WorkItemImportRecord struct {
	ID          string `json:"id" csv:"id"`
	Type        string `json:"type" csv:"type"`
	Title       string `json:"title" csv:"title"`
	Description string `json:"description" csv:"description"`
	Status      string `json:"status" csv:"status"`
	Priority    string `json:"priority" csv:"priority"`
	StoryPoints string `json:"story_points" csv:"story_points"`
	AssigneeID  string `json:"assignee_id" csv:"assignee_id"`
	ReporterID  string `json:"reporter_id" csv:"reporter_id"`
	ParentID    string `json:"parent_id" csv:"parent_id"`
	SprintID    string `json:"sprint_id" csv:"sprint_id"`
}

// ImportWorkItemsFromCSV imports work items from CSV format
func (s *ImportService) ImportWorkItemsFromCSV(ctx context.Context, reader io.Reader, strategy ConflictResolutionStrategy) (*ImportResult, error) {
	logger := s.getLogger(ctx)
	logger.Info("starting CSV import", slog.String("strategy", string(strategy)))

	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Validate header
	if err := s.validateCSVHeader(header); err != nil {
		return nil, fmt.Errorf("invalid CSV header: %w", err)
	}

	result := &ImportResult{
		Errors:        make([]ImportError, 0),
		ImportedItems: make([]*models.WorkItem, 0),
	}

	rowNum := 1 // Start at 1 (header is row 0)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     rowNum,
				Message: fmt.Sprintf("failed to read row: %v", err),
			})
			result.ErrorCount++
			rowNum++
			continue
		}

		result.TotalRecords++

		// Parse CSV record
		importRecord, err := s.parseCSVRecord(header, record)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:     rowNum,
				Message: fmt.Sprintf("failed to parse record: %v", err),
			})
			result.ErrorCount++
			rowNum++
			continue
		}

		// Import the record
		workItem, err := s.importWorkItemRecord(ctx, importRecord, strategy, rowNum)
		if err != nil {
			if errors.Is(err, ErrConflict) && strategy == ConflictStrategySkip {
				result.SkippedCount++
				logger.Debug("skipped conflicting record", slog.Int("row", rowNum))
			} else {
				result.Errors = append(result.Errors, ImportError{
					Row:     rowNum,
					Message: err.Error(),
				})
				result.ErrorCount++
			}
			rowNum++
			continue
		}

		result.ImportedItems = append(result.ImportedItems, workItem)
		result.SuccessCount++
		rowNum++
	}

	logger.Info("CSV import completed",
		slog.Int("total", result.TotalRecords),
		slog.Int("success", result.SuccessCount),
		slog.Int("skipped", result.SkippedCount),
		slog.Int("errors", result.ErrorCount),
	)

	return result, nil
}

// ImportWorkItemsFromJSON imports work items from JSON format
func (s *ImportService) ImportWorkItemsFromJSON(ctx context.Context, reader io.Reader, strategy ConflictResolutionStrategy) (*ImportResult, error) {
	logger := s.getLogger(ctx)
	logger.Info("starting JSON import", slog.String("strategy", string(strategy)))

	var records []WorkItemImportRecord
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	result := &ImportResult{
		TotalRecords:  len(records),
		Errors:        make([]ImportError, 0),
		ImportedItems: make([]*models.WorkItem, 0),
	}

	for i, record := range records {
		rowNum := i + 1

		// Import the record
		workItem, err := s.importWorkItemRecord(ctx, &record, strategy, rowNum)
		if err != nil {
			if errors.Is(err, ErrConflict) && strategy == ConflictStrategySkip {
				result.SkippedCount++
				logger.Debug("skipped conflicting record", slog.Int("row", rowNum))
			} else {
				result.Errors = append(result.Errors, ImportError{
					Row:     rowNum,
					Message: err.Error(),
				})
				result.ErrorCount++
			}
			continue
		}

		result.ImportedItems = append(result.ImportedItems, workItem)
		result.SuccessCount++
	}

	logger.Info("JSON import completed",
		slog.Int("total", result.TotalRecords),
		slog.Int("success", result.SuccessCount),
		slog.Int("skipped", result.SkippedCount),
		slog.Int("errors", result.ErrorCount),
	)

	return result, nil
}

// validateCSVHeader validates the CSV header
func (s *ImportService) validateCSVHeader(header []string) error {
	requiredFields := []string{"type", "title", "description", "priority", "reporter_id"}

	headerMap := make(map[string]bool)
	for _, field := range header {
		headerMap[strings.ToLower(strings.TrimSpace(field))] = true
	}

	for _, required := range requiredFields {
		if !headerMap[required] {
			return fmt.Errorf("missing required field: %s", required)
		}
	}

	return nil
}

// parseCSVRecord parses a CSV record into an import record
func (s *ImportService) parseCSVRecord(header []string, record []string) (*WorkItemImportRecord, error) {
	if len(header) != len(record) {
		return nil, fmt.Errorf("header and record length mismatch")
	}

	importRecord := &WorkItemImportRecord{}

	for i, field := range header {
		field = strings.ToLower(strings.TrimSpace(field))
		value := strings.TrimSpace(record[i])

		switch field {
		case "id":
			importRecord.ID = value
		case "type":
			importRecord.Type = value
		case "title":
			importRecord.Title = value
		case "description":
			importRecord.Description = value
		case "status":
			importRecord.Status = value
		case "priority":
			importRecord.Priority = value
		case "story_points":
			importRecord.StoryPoints = value
		case "assignee_id":
			importRecord.AssigneeID = value
		case "reporter_id":
			importRecord.ReporterID = value
		case "parent_id":
			importRecord.ParentID = value
		case "sprint_id":
			importRecord.SprintID = value
		}
	}

	return importRecord, nil
}

// importWorkItemRecord imports a single work item record
func (s *ImportService) importWorkItemRecord(ctx context.Context, record *WorkItemImportRecord, strategy ConflictResolutionStrategy, rowNum int) (*models.WorkItem, error) {
	// Validate the record
	if err := s.validateImportRecord(record); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if work item already exists (if ID is provided)
	if record.ID != "" {
		id, err := uuid.Parse(record.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid ID format: %w", err)
		}

		// Check if it exists
		existing, err := s.workItemRepo.GetWorkItemByID(ctx, id)
		if err == nil && existing != nil {
			// Conflict detected
			return s.handleConflict(ctx, existing, record, strategy)
		}
	}

	// Parse and create work item
	req, err := s.recordToCreateRequest(record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record: %w", err)
	}

	// Create work item
	workItem, err := s.workItemRepo.CreateWorkItem(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("failed to create work item: %w", err)
	}

	return workItem, nil
}

// validateImportRecord validates an import record
func (s *ImportService) validateImportRecord(record *WorkItemImportRecord) error {
	// Validate required fields
	if record.Type == "" {
		return fmt.Errorf("type is required")
	}
	if record.Title == "" {
		return fmt.Errorf("title is required")
	}
	if record.Description == "" {
		return fmt.Errorf("description is required")
	}
	if record.Priority == "" {
		return fmt.Errorf("priority is required")
	}
	if record.ReporterID == "" {
		return fmt.Errorf("reporter_id is required")
	}

	// Validate type
	validTypes := map[string]bool{
		"epic":   true,
		"story":  true,
		"defect": true,
	}
	if !validTypes[strings.ToLower(record.Type)] {
		return fmt.Errorf("invalid type: %s (must be epic, story, or defect)", record.Type)
	}

	// Validate priority
	validPriorities := map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	}
	if !validPriorities[strings.ToLower(record.Priority)] {
		return fmt.Errorf("invalid priority: %s (must be low, medium, high, or critical)", record.Priority)
	}

	// Validate UUIDs if provided
	if record.ReporterID != "" {
		if _, err := uuid.Parse(record.ReporterID); err != nil {
			return fmt.Errorf("invalid reporter_id format: %w", err)
		}
	}
	if record.AssigneeID != "" {
		if _, err := uuid.Parse(record.AssigneeID); err != nil {
			return fmt.Errorf("invalid assignee_id format: %w", err)
		}
	}
	if record.ParentID != "" {
		if _, err := uuid.Parse(record.ParentID); err != nil {
			return fmt.Errorf("invalid parent_id format: %w", err)
		}
	}
	if record.SprintID != "" {
		if _, err := uuid.Parse(record.SprintID); err != nil {
			return fmt.Errorf("invalid sprint_id format: %w", err)
		}
	}

	// Validate story points if provided
	if record.StoryPoints != "" {
		if _, err := strconv.Atoi(record.StoryPoints); err != nil {
			return fmt.Errorf("invalid story_points format: must be an integer")
		}
	}

	return nil
}

// recordToCreateRequest converts an import record to a create request
func (s *ImportService) recordToCreateRequest(record *WorkItemImportRecord) (*CreateWorkItemRequest, error) {
	req := &CreateWorkItemRequest{
		Type:        models.WorkItemType(strings.ToLower(record.Type)),
		Title:       record.Title,
		Description: record.Description,
		Priority:    models.PriorityLevel(strings.ToLower(record.Priority)),
	}

	// Parse reporter ID (required)
	reporterID, err := uuid.Parse(record.ReporterID)
	if err != nil {
		return nil, fmt.Errorf("invalid reporter_id: %w", err)
	}
	req.ReporterID = reporterID

	// Parse optional fields
	if record.StoryPoints != "" {
		sp, err := strconv.Atoi(record.StoryPoints)
		if err != nil {
			return nil, fmt.Errorf("invalid story_points: %w", err)
		}
		req.StoryPoints = &sp
	}

	if record.AssigneeID != "" {
		assigneeID, err := uuid.Parse(record.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("invalid assignee_id: %w", err)
		}
		req.AssigneeID = &assigneeID
	}

	if record.ParentID != "" {
		parentID, err := uuid.Parse(record.ParentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent_id: %w", err)
		}
		req.ParentID = &parentID
	}

	if record.SprintID != "" {
		sprintID, err := uuid.Parse(record.SprintID)
		if err != nil {
			return nil, fmt.Errorf("invalid sprint_id: %w", err)
		}
		req.SprintID = &sprintID
	}

	return req, nil
}

// handleConflict handles conflicts based on the strategy
func (s *ImportService) handleConflict(ctx context.Context, existing *models.WorkItem, record *WorkItemImportRecord, strategy ConflictResolutionStrategy) (*models.WorkItem, error) {
	switch strategy {
	case ConflictStrategySkip:
		return nil, ErrConflict
	case ConflictStrategyError:
		return nil, fmt.Errorf("%w: work item with ID %s already exists", ErrConflict, existing.ID)
	case ConflictStrategyUpdate:
		// Update existing work item
		updateReq := &UpdateWorkItemRequest{
			Title:       &record.Title,
			Description: &record.Description,
		}

		// Parse optional fields for update
		if record.Priority != "" {
			priority := models.PriorityLevel(strings.ToLower(record.Priority))
			updateReq.Priority = &priority
		}

		if record.Status != "" {
			status := models.WorkItemStatus(strings.ToLower(record.Status))
			updateReq.Status = &status
		}

		if record.StoryPoints != "" {
			sp, err := strconv.Atoi(record.StoryPoints)
			if err == nil {
				updateReq.StoryPoints = &sp
			}
		}

		if record.AssigneeID != "" {
			assigneeID, err := uuid.Parse(record.AssigneeID)
			if err == nil {
				updateReq.AssigneeID = &assigneeID
			}
		}

		if record.SprintID != "" {
			sprintID, err := uuid.Parse(record.SprintID)
			if err == nil {
				updateReq.SprintID = &sprintID
			}
		}

		return s.workItemRepo.UpdateWorkItem(ctx, existing.ID, *updateReq)
	default:
		return nil, fmt.Errorf("unknown conflict resolution strategy: %s", strategy)
	}
}

// getLogger gets logger from context or returns default
func (s *ImportService) getLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}
