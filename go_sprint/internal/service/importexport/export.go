package importexport

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/xuri/excelize/v2"
)

// ExportFormat defines the export format
type ExportFormat string

const (
	// ExportFormatCSV exports to CSV format
	ExportFormatCSV ExportFormat = "csv"
	// ExportFormatJSON exports to JSON format
	ExportFormatJSON ExportFormat = "json"
	// ExportFormatExcel exports to Excel format
	ExportFormatExcel ExportFormat = "excel"
)

// ExportService handles data export operations
type ExportService struct {
	workItemRepo WorkItemRepository
	sprintRepo   SprintRepository
	logger       *slog.Logger
}

// SprintRepository defines the interface for sprint operations
type SprintRepository interface {
	GetSprintByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error)
	ListSprints(ctx context.Context, filters SprintFilters) ([]*models.Sprint, error)
}

// SprintFilters defines filters for sprint queries
type SprintFilters struct {
	Status    *models.SprintStatus
	StartDate *time.Time
	EndDate   *time.Time
}

// NewExportService creates a new export service
func NewExportService(workItemRepo WorkItemRepository, sprintRepo SprintRepository, logger *slog.Logger) *ExportService {
	return &ExportService{
		workItemRepo: workItemRepo,
		sprintRepo:   sprintRepo,
		logger:       logger,
	}
}

// ExportWorkItemsToCSV exports work items to CSV format
func (s *ExportService) ExportWorkItemsToCSV(ctx context.Context, writer io.Writer, workItems []*models.WorkItem) error {
	logger := s.getLogger(ctx)
	logger.Info("starting CSV export", slog.Int("count", len(workItems)))

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"id", "type", "title", "description", "status", "priority",
		"story_points", "assignee_id", "reporter_id", "parent_id",
		"sprint_id", "created_at", "updated_at",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write records
	for _, item := range workItems {
		record := s.workItemToCSVRecord(item)
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	logger.Info("CSV export completed", slog.Int("count", len(workItems)))
	return nil
}

// ExportWorkItemsToJSON exports work items to JSON format
func (s *ExportService) ExportWorkItemsToJSON(ctx context.Context, writer io.Writer, workItems []*models.WorkItem) error {
	logger := s.getLogger(ctx)
	logger.Info("starting JSON export", slog.Int("count", len(workItems)))

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(workItems); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	logger.Info("JSON export completed", slog.Int("count", len(workItems)))
	return nil
}

// ExportWorkItemsToExcel exports work items to Excel format
func (s *ExportService) ExportWorkItemsToExcel(ctx context.Context, writer io.Writer, workItems []*models.WorkItem) error {
	logger := s.getLogger(ctx)
	logger.Info("starting Excel export", slog.Int("count", len(workItems)))

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.Error("failed to close Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Work Items"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Write header
	header := []string{
		"ID", "Type", "Title", "Description", "Status", "Priority",
		"Story Points", "Assignee ID", "Reporter ID", "Parent ID",
		"Sprint ID", "Created At", "Updated At",
	}

	for i, h := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// Style header
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	if err == nil {
		f.SetCellStyle(sheetName, "A1", fmt.Sprintf("%s1", string(rune('A'+len(header)-1))), headerStyle)
	}

	// Write data
	for i, item := range workItems {
		row := i + 2 // Start from row 2 (row 1 is header)
		s.writeWorkItemToExcel(f, sheetName, row, item)
	}

	// Auto-fit columns
	for i := range header {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, col, col, 15)
	}

	// Write to writer
	if err := f.Write(writer); err != nil {
		return fmt.Errorf("failed to write Excel file: %w", err)
	}

	logger.Info("Excel export completed", slog.Int("count", len(workItems)))
	return nil
}

// ExportSprintsToCSV exports sprints to CSV format
func (s *ExportService) ExportSprintsToCSV(ctx context.Context, writer io.Writer, sprints []*models.Sprint) error {
	logger := s.getLogger(ctx)
	logger.Info("starting sprint CSV export", slog.Int("count", len(sprints)))

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"id", "name", "description", "status", "start_date", "end_date",
		"capacity_points", "committed_points", "completed_points",
		"created_by", "created_at", "updated_at",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write records
	for _, sprint := range sprints {
		record := s.sprintToCSVRecord(sprint)
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	logger.Info("sprint CSV export completed", slog.Int("count", len(sprints)))
	return nil
}

// ExportSprintsToJSON exports sprints to JSON format
func (s *ExportService) ExportSprintsToJSON(ctx context.Context, writer io.Writer, sprints []*models.Sprint) error {
	logger := s.getLogger(ctx)
	logger.Info("starting sprint JSON export", slog.Int("count", len(sprints)))

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(sprints); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	logger.Info("sprint JSON export completed", slog.Int("count", len(sprints)))
	return nil
}

// ExportSprintsToExcel exports sprints to Excel format
func (s *ExportService) ExportSprintsToExcel(ctx context.Context, writer io.Writer, sprints []*models.Sprint) error {
	logger := s.getLogger(ctx)
	logger.Info("starting sprint Excel export", slog.Int("count", len(sprints)))

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.Error("failed to close Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Sprints"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Write header
	header := []string{
		"ID", "Name", "Description", "Status", "Start Date", "End Date",
		"Capacity Points", "Committed Points", "Completed Points",
		"Created By", "Created At", "Updated At",
	}

	for i, h := range header {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// Style header
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	if err == nil {
		f.SetCellStyle(sheetName, "A1", fmt.Sprintf("%s1", string(rune('A'+len(header)-1))), headerStyle)
	}

	// Write data
	for i, sprint := range sprints {
		row := i + 2 // Start from row 2 (row 1 is header)
		s.writeSprintToExcel(f, sheetName, row, sprint)
	}

	// Auto-fit columns
	for i := range header {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, col, col, 15)
	}

	// Write to writer
	if err := f.Write(writer); err != nil {
		return fmt.Errorf("failed to write Excel file: %w", err)
	}

	logger.Info("sprint Excel export completed", slog.Int("count", len(sprints)))
	return nil
}

// workItemToCSVRecord converts a work item to a CSV record
func (s *ExportService) workItemToCSVRecord(item *models.WorkItem) []string {
	record := make([]string, 13)

	record[0] = item.ID.String()
	record[1] = string(item.Type)
	record[2] = item.Title
	record[3] = item.Description
	record[4] = string(item.Status)
	record[5] = string(item.Priority)

	if item.StoryPoints != nil {
		record[6] = strconv.Itoa(*item.StoryPoints)
	}
	if item.AssigneeID != nil {
		record[7] = item.AssigneeID.String()
	}
	record[8] = item.ReporterID.String()
	if item.ParentID != nil {
		record[9] = item.ParentID.String()
	}
	if item.SprintID != nil {
		record[10] = item.SprintID.String()
	}
	record[11] = item.CreatedAt.Format(time.RFC3339)
	record[12] = item.UpdatedAt.Format(time.RFC3339)

	return record
}

// sprintToCSVRecord converts a sprint to a CSV record
func (s *ExportService) sprintToCSVRecord(sprint *models.Sprint) []string {
	record := make([]string, 12)

	record[0] = sprint.ID.String()
	record[1] = sprint.Name
	record[2] = sprint.Description
	record[3] = string(sprint.Status)
	record[4] = sprint.StartDate.Format("2006-01-02")
	record[5] = sprint.EndDate.Format("2006-01-02")

	if sprint.CapacityPoints != nil {
		record[6] = strconv.Itoa(*sprint.CapacityPoints)
	}
	record[7] = strconv.Itoa(sprint.CommittedPoints)
	record[8] = strconv.Itoa(sprint.CompletedPoints)
	record[9] = sprint.CreatedBy.String()
	record[10] = sprint.CreatedAt.Format(time.RFC3339)
	record[11] = sprint.UpdatedAt.Format(time.RFC3339)

	return record
}

// writeWorkItemToExcel writes a work item to an Excel row
func (s *ExportService) writeWorkItemToExcel(f *excelize.File, sheetName string, row int, item *models.WorkItem) {
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), item.ID.String())
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), string(item.Type))
	f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), item.Title)
	f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), item.Description)
	f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), string(item.Status))
	f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), string(item.Priority))

	if item.StoryPoints != nil {
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), *item.StoryPoints)
	}
	if item.AssigneeID != nil {
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), item.AssigneeID.String())
	}
	f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), item.ReporterID.String())
	if item.ParentID != nil {
		f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), item.ParentID.String())
	}
	if item.SprintID != nil {
		f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), item.SprintID.String())
	}
	f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), item.CreatedAt.Format(time.RFC3339))
	f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), item.UpdatedAt.Format(time.RFC3339))
}

// writeSprintToExcel writes a sprint to an Excel row
func (s *ExportService) writeSprintToExcel(f *excelize.File, sheetName string, row int, sprint *models.Sprint) {
	f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), sprint.ID.String())
	f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), sprint.Name)
	f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), sprint.Description)
	f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), string(sprint.Status))
	f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), sprint.StartDate.Format("2006-01-02"))
	f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), sprint.EndDate.Format("2006-01-02"))

	if sprint.CapacityPoints != nil {
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), *sprint.CapacityPoints)
	}
	f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), sprint.CommittedPoints)
	f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), sprint.CompletedPoints)
	f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), sprint.CreatedBy.String())
	f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), sprint.CreatedAt.Format(time.RFC3339))
	f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), sprint.UpdatedAt.Format(time.RFC3339))
}

// getLogger gets logger from context or returns default
func (s *ExportService) getLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}
