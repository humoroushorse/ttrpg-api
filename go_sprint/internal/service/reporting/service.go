package reporting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
)

var (
	// ErrSprintNotFound is returned when a sprint is not found
	ErrSprintNotFound = errors.New("sprint not found")
	// ErrNoDataAvailable is returned when there is no data for the requested report
	ErrNoDataAvailable = errors.New("no data available for the requested report")
	// ErrInvalidDateRange is returned when the date range is invalid
	ErrInvalidDateRange = errors.New("invalid date range: end date must be after start date")
)

// SprintRepository defines the interface for sprint data access
type SprintRepository interface {
	GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error)
	ListSprints(ctx context.Context, params sprints.ListSprintsParams) ([]sprints.SprintManagementSprint, error)
	ListCompletedSprints(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error)
	ListSprintsByStatus(ctx context.Context, params sprints.ListSprintsByStatusParams) ([]sprints.SprintManagementSprint, error)
}

// WorkItemRepository defines the interface for work item data access
type WorkItemRepository interface {
	ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error)
	ListWorkItems(ctx context.Context, params workitems.ListWorkItemsParams) ([]workitems.SprintManagementWorkItem, error)
	GetWorkItemByID(ctx context.Context, id pgtype.UUID) (workitems.SprintManagementWorkItem, error)
}

// Service provides business logic for reporting and analytics
type Service struct {
	sprintRepo   SprintRepository
	workItemRepo WorkItemRepository
	logger       *slog.Logger
}

// NewService creates a new reporting service
func NewService(sprintRepo SprintRepository, workItemRepo WorkItemRepository, logger *slog.Logger) *Service {
	return &Service{
		sprintRepo:   sprintRepo,
		workItemRepo: workItemRepo,
		logger:       logger,
	}
}

// SprintReport represents a comprehensive sprint report
type SprintReport struct {
	SprintID            uuid.UUID      `json:"sprint_id"`
	SprintName          string         `json:"sprint_name"`
	Status              string         `json:"status"`
	StartDate           time.Time      `json:"start_date"`
	EndDate             time.Time      `json:"end_date"`
	DurationDays        int            `json:"duration_days"`
	TotalWorkItems      int            `json:"total_work_items"`
	CompletedItems      int            `json:"completed_items"`
	InProgressItems     int            `json:"in_progress_items"`
	TodoItems           int            `json:"todo_items"`
	BlockedItems        int            `json:"blocked_items"`
	CompletionRate      float64        `json:"completion_rate"`
	CommittedPoints     int            `json:"committed_points"`
	CompletedPoints     int            `json:"completed_points"`
	CapacityPoints      *int           `json:"capacity_points,omitempty"`
	Velocity            int            `json:"velocity"`
	UtilizationRate     float64        `json:"utilization_rate"`
	WorkItemsByType     map[string]int `json:"work_items_by_type"`
	WorkItemsByPriority map[string]int `json:"work_items_by_priority"`
}

// GenerateSprintReport generates a comprehensive report for a sprint
func (s *Service) GenerateSprintReport(ctx context.Context, sprintID uuid.UUID) (*SprintReport, error) {
	logger := s.getLogger(ctx)

	// Get sprint
	sprint, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(sprintID))
	if err != nil {
		logger.Error("sprint not found", slog.String("id", sprintID.String()))
		return nil, ErrSprintNotFound
	}

	// Get work items
	workItems, err := s.workItemRepo.ListWorkItemsBySprint(ctx, uuidToPgtype(sprintID))
	if err != nil {
		logger.Error("failed to list work items", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	report := &SprintReport{
		SprintID:            pgtypeToUUID(sprint.ID),
		SprintName:          sprint.Name,
		Status:              string(sprint.Status),
		StartDate:           pgDateToTime(sprint.StartDate),
		EndDate:             pgDateToTime(sprint.EndDate),
		TotalWorkItems:      len(workItems),
		WorkItemsByType:     make(map[string]int),
		WorkItemsByPriority: make(map[string]int),
	}

	// Calculate duration
	report.DurationDays = int(report.EndDate.Sub(report.StartDate).Hours() / 24)

	// Set capacity if available
	if sprint.CapacityPoints != nil {
		cp := int(*sprint.CapacityPoints)
		report.CapacityPoints = &cp
	}

	// Analyze work items
	committedPoints := 0
	completedPoints := 0

	for _, wi := range workItems {
		// Count by status
		switch wi.Status {
		case workitems.SprintManagementWorkItemStatusDone:
			report.CompletedItems++
		case workitems.SprintManagementWorkItemStatusInProgress, workitems.SprintManagementWorkItemStatusInReview:
			report.InProgressItems++
		case workitems.SprintManagementWorkItemStatusTodo:
			report.TodoItems++
		case workitems.SprintManagementWorkItemStatusBlocked:
			report.BlockedItems++
		}

		// Count by type
		report.WorkItemsByType[string(wi.Type)]++

		// Count by priority
		report.WorkItemsByPriority[string(wi.Priority)]++

		// Calculate points
		if wi.StoryPoints != nil {
			points := int(*wi.StoryPoints)
			committedPoints += points

			if wi.Status == workitems.SprintManagementWorkItemStatusDone {
				completedPoints += points
			}
		}
	}

	report.CommittedPoints = committedPoints
	report.CompletedPoints = completedPoints
	report.Velocity = completedPoints

	// Calculate completion rate
	if report.TotalWorkItems > 0 {
		report.CompletionRate = float64(report.CompletedItems) / float64(report.TotalWorkItems) * 100
	}

	// Calculate utilization rate
	if report.CapacityPoints != nil && *report.CapacityPoints > 0 {
		report.UtilizationRate = float64(committedPoints) / float64(*report.CapacityPoints) * 100
	}

	logger.Info("sprint report generated",
		slog.String("sprint_id", sprintID.String()),
		slog.Int("total_items", report.TotalWorkItems),
		slog.Float64("completion_rate", report.CompletionRate))

	return report, nil
}

// TeamPerformanceReport represents team performance analytics
type TeamPerformanceReport struct {
	TotalSprints          int                 `json:"total_sprints"`
	CompletedSprints      int                 `json:"completed_sprints"`
	TotalWorkItems        int                 `json:"total_work_items"`
	CompletedWorkItems    int                 `json:"completed_work_items"`
	TotalStoryPoints      int                 `json:"total_story_points"`
	CompletedStoryPoints  int                 `json:"completed_story_points"`
	AverageVelocity       float64             `json:"average_velocity"`
	AverageCompletionRate float64             `json:"average_completion_rate"`
	SprintReports         []SprintReport      `json:"sprint_reports"`
	VelocityTrend         []VelocityDataPoint `json:"velocity_trend"`
	WorkItemDistribution  map[string]int      `json:"work_item_distribution"`
}

// VelocityDataPoint represents a single point in velocity trend
type VelocityDataPoint struct {
	SprintName string    `json:"sprint_name"`
	Date       time.Time `json:"date"`
	Velocity   int       `json:"velocity"`
}

// DateRangeFilter represents a date range for filtering
type DateRangeFilter struct {
	StartDate *time.Time
	EndDate   *time.Time
}

// GenerateTeamPerformanceReport generates team performance analytics across multiple sprints
func (s *Service) GenerateTeamPerformanceReport(ctx context.Context, dateRange DateRangeFilter, limit int) (*TeamPerformanceReport, error) {
	logger := s.getLogger(ctx)

	// Validate date range
	if dateRange.StartDate != nil && dateRange.EndDate != nil {
		if dateRange.EndDate.Before(*dateRange.StartDate) {
			return nil, ErrInvalidDateRange
		}
	}

	// Get sprints within date range
	var allSprints []sprints.SprintManagementSprint
	var err error

	if limit > 0 {
		// Get limited number of completed sprints
		allSprints, err = s.sprintRepo.ListCompletedSprints(ctx, int32(limit))
	} else {
		// Get all sprints
		allSprints, err = s.sprintRepo.ListSprints(ctx, sprints.ListSprintsParams{
			Limit:           1000, // Large limit to get all
			CursorTimestamp: pgtype.Timestamptz{Valid: false},
			CursorID:        pgtype.UUID{Valid: false},
		})
	}

	if err != nil {
		logger.Error("failed to list sprints", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list sprints: %w", err)
	}

	// Filter by date range if provided
	var filteredSprints []sprints.SprintManagementSprint
	for _, sprint := range allSprints {
		sprintStart := pgDateToTime(sprint.StartDate)
		sprintEnd := pgDateToTime(sprint.EndDate)

		// Check if sprint falls within date range
		if dateRange.StartDate != nil && sprintEnd.Before(*dateRange.StartDate) {
			continue
		}
		if dateRange.EndDate != nil && sprintStart.After(*dateRange.EndDate) {
			continue
		}

		filteredSprints = append(filteredSprints, sprint)
	}

	if len(filteredSprints) == 0 {
		return nil, ErrNoDataAvailable
	}

	report := &TeamPerformanceReport{
		TotalSprints:         len(filteredSprints),
		SprintReports:        make([]SprintReport, 0, len(filteredSprints)),
		VelocityTrend:        make([]VelocityDataPoint, 0, len(filteredSprints)),
		WorkItemDistribution: make(map[string]int),
	}

	// Generate report for each sprint
	totalVelocity := 0
	totalCompletionRate := 0.0

	for _, sprint := range filteredSprints {
		sprintReport, err := s.GenerateSprintReport(ctx, pgtypeToUUID(sprint.ID))
		if err != nil {
			logger.Warn("failed to generate sprint report",
				slog.String("sprint_id", pgtypeToUUID(sprint.ID).String()),
				slog.String("error", err.Error()))
			continue
		}

		report.SprintReports = append(report.SprintReports, *sprintReport)

		// Aggregate metrics
		if sprint.Status == sprints.SprintManagementSprintStatusCompleted {
			report.CompletedSprints++
		}

		report.TotalWorkItems += sprintReport.TotalWorkItems
		report.CompletedWorkItems += sprintReport.CompletedItems
		report.TotalStoryPoints += sprintReport.CommittedPoints
		report.CompletedStoryPoints += sprintReport.CompletedPoints

		totalVelocity += sprintReport.Velocity
		totalCompletionRate += sprintReport.CompletionRate

		// Add to velocity trend
		report.VelocityTrend = append(report.VelocityTrend, VelocityDataPoint{
			SprintName: sprintReport.SprintName,
			Date:       sprintReport.EndDate,
			Velocity:   sprintReport.Velocity,
		})

		// Aggregate work item distribution
		for wiType, count := range sprintReport.WorkItemsByType {
			report.WorkItemDistribution[wiType] += count
		}
	}

	// Calculate averages
	if len(report.SprintReports) > 0 {
		report.AverageVelocity = float64(totalVelocity) / float64(len(report.SprintReports))
		report.AverageCompletionRate = totalCompletionRate / float64(len(report.SprintReports))
	}

	logger.Info("team performance report generated",
		slog.Int("total_sprints", report.TotalSprints),
		slog.Float64("average_velocity", report.AverageVelocity))

	return report, nil
}

// CycleTimeMetrics represents cycle time and lead time metrics
type CycleTimeMetrics struct {
	WorkItemID     uuid.UUID  `json:"work_item_id"`
	WorkItemTitle  string     `json:"work_item_title"`
	WorkItemType   string     `json:"work_item_type"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	LeadTimeHours  *float64   `json:"lead_time_hours,omitempty"`
	CycleTimeHours *float64   `json:"cycle_time_hours,omitempty"`
	Status         string     `json:"status"`
}

// CycleTimeSummary represents aggregated cycle time statistics
type CycleTimeSummary struct {
	TotalWorkItems     int                    `json:"total_work_items"`
	CompletedWorkItems int                    `json:"completed_work_items"`
	AverageLeadTime    float64                `json:"average_lead_time_hours"`
	AverageCycleTime   float64                `json:"average_cycle_time_hours"`
	MedianLeadTime     float64                `json:"median_lead_time_hours"`
	MedianCycleTime    float64                `json:"median_cycle_time_hours"`
	MinLeadTime        float64                `json:"min_lead_time_hours"`
	MaxLeadTime        float64                `json:"max_lead_time_hours"`
	MinCycleTime       float64                `json:"min_cycle_time_hours"`
	MaxCycleTime       float64                `json:"max_cycle_time_hours"`
	MetricsByType      map[string]TypeMetrics `json:"metrics_by_type"`
	WorkItems          []CycleTimeMetrics     `json:"work_items"`
}

// TypeMetrics represents metrics for a specific work item type
type TypeMetrics struct {
	Count            int     `json:"count"`
	AverageLeadTime  float64 `json:"average_lead_time_hours"`
	AverageCycleTime float64 `json:"average_cycle_time_hours"`
}

// CalculateCycleTime calculates cycle time and lead time for work items
func (s *Service) CalculateCycleTime(ctx context.Context, dateRange DateRangeFilter, workItemType *string) (*CycleTimeSummary, error) {
	logger := s.getLogger(ctx)

	// Validate date range
	if dateRange.StartDate != nil && dateRange.EndDate != nil {
		if dateRange.EndDate.Before(*dateRange.StartDate) {
			return nil, ErrInvalidDateRange
		}
	}

	// Get work items
	params := workitems.ListWorkItemsParams{
		Limit:           1000, // Large limit
		CursorTimestamp: pgtype.Timestamptz{Valid: false},
		CursorID:        pgtype.UUID{Valid: false},
	}

	allWorkItems, err := s.workItemRepo.ListWorkItems(ctx, params)
	if err != nil {
		logger.Error("failed to list work items", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	summary := &CycleTimeSummary{
		MetricsByType: make(map[string]TypeMetrics),
		WorkItems:     make([]CycleTimeMetrics, 0),
	}

	var leadTimes []float64
	var cycleTimes []float64
	typeMetrics := make(map[string]*typeMetricsAccumulator)

	for _, wi := range allWorkItems {
		// Filter by type if specified
		if workItemType != nil && string(wi.Type) != *workItemType {
			continue
		}

		// Filter by date range
		createdAt := wi.CreatedAt.Time
		if dateRange.StartDate != nil && createdAt.Before(*dateRange.StartDate) {
			continue
		}
		if dateRange.EndDate != nil && createdAt.After(*dateRange.EndDate) {
			continue
		}

		metric := CycleTimeMetrics{
			WorkItemID:    pgtypeToUUID(wi.ID),
			WorkItemTitle: wi.Title,
			WorkItemType:  string(wi.Type),
			CreatedAt:     createdAt,
			Status:        string(wi.Status),
		}

		summary.TotalWorkItems++

		// Calculate lead time (creation to completion)
		if wi.Status == workitems.SprintManagementWorkItemStatusDone {
			summary.CompletedWorkItems++

			// Use updated_at as completion time (in real system, track status change time)
			completedAt := wi.UpdatedAt.Time
			metric.CompletedAt = &completedAt

			leadTime := completedAt.Sub(createdAt).Hours()
			metric.LeadTimeHours = &leadTime
			leadTimes = append(leadTimes, leadTime)

			// For cycle time, we'd ideally track when work started
			// For now, use lead time as approximation
			cycleTime := leadTime
			metric.CycleTimeHours = &cycleTime
			cycleTimes = append(cycleTimes, cycleTime)

			// Accumulate by type
			if _, exists := typeMetrics[string(wi.Type)]; !exists {
				typeMetrics[string(wi.Type)] = &typeMetricsAccumulator{}
			}
			typeMetrics[string(wi.Type)].count++
			typeMetrics[string(wi.Type)].totalLeadTime += leadTime
			typeMetrics[string(wi.Type)].totalCycleTime += cycleTime
		}

		summary.WorkItems = append(summary.WorkItems, metric)
	}

	if len(leadTimes) == 0 {
		return summary, nil // Return empty summary if no completed items
	}

	// Calculate statistics
	summary.AverageLeadTime = average(leadTimes)
	summary.AverageCycleTime = average(cycleTimes)
	summary.MedianLeadTime = median(leadTimes)
	summary.MedianCycleTime = median(cycleTimes)
	summary.MinLeadTime = min(leadTimes)
	summary.MaxLeadTime = max(leadTimes)
	summary.MinCycleTime = min(cycleTimes)
	summary.MaxCycleTime = max(cycleTimes)

	// Calculate metrics by type
	for wiType, acc := range typeMetrics {
		summary.MetricsByType[wiType] = TypeMetrics{
			Count:            acc.count,
			AverageLeadTime:  acc.totalLeadTime / float64(acc.count),
			AverageCycleTime: acc.totalCycleTime / float64(acc.count),
		}
	}

	logger.Info("cycle time calculated",
		slog.Int("total_items", summary.TotalWorkItems),
		slog.Int("completed_items", summary.CompletedWorkItems),
		slog.Float64("average_lead_time", summary.AverageLeadTime))

	return summary, nil
}

// typeMetricsAccumulator is a helper for accumulating metrics by type
type typeMetricsAccumulator struct {
	count          int
	totalLeadTime  float64
	totalCycleTime float64
}

// getLogger gets logger from context or returns default
func (s *Service) getLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("logger").(*slog.Logger); ok {
		return logger
	}
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// Helper functions for UUID and date conversion
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: id != uuid.Nil,
	}
}

func pgtypeToUUID(pg pgtype.UUID) uuid.UUID {
	if !pg.Valid {
		return uuid.Nil
	}
	return pg.Bytes
}

func pgDateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}

// Statistical helper functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Sort values
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func min(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maxVal := values[0]
	for _, v := range values[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// DashboardMetrics represents customizable KPI metrics for dashboards
type DashboardMetrics struct {
	ActiveSprints       int               `json:"active_sprints"`
	TotalWorkItems      int               `json:"total_work_items"`
	CompletedWorkItems  int               `json:"completed_work_items"`
	InProgressWorkItems int               `json:"in_progress_work_items"`
	BlockedWorkItems    int               `json:"blocked_work_items"`
	CurrentVelocity     float64           `json:"current_velocity"`
	AverageVelocity     float64           `json:"average_velocity"`
	CompletionRate      float64           `json:"completion_rate"`
	AverageCycleTime    float64           `json:"average_cycle_time_hours"`
	WorkItemsByType     map[string]int    `json:"work_items_by_type"`
	WorkItemsByPriority map[string]int    `json:"work_items_by_priority"`
	RecentActivity      []ActivitySummary `json:"recent_activity"`
	UpcomingDeadlines   []DeadlineSummary `json:"upcoming_deadlines"`
}

// ActivitySummary represents recent activity
type ActivitySummary struct {
	Date           time.Time `json:"date"`
	ItemsCreated   int       `json:"items_created"`
	ItemsCompleted int       `json:"items_completed"`
	ItemsUpdated   int       `json:"items_updated"`
}

// DeadlineSummary represents upcoming sprint deadlines
type DeadlineSummary struct {
	SprintID   uuid.UUID `json:"sprint_id"`
	SprintName string    `json:"sprint_name"`
	EndDate    time.Time `json:"end_date"`
	DaysLeft   int       `json:"days_left"`
	Status     string    `json:"status"`
}

// GetDashboardMetrics retrieves customizable KPI metrics for dashboards
func (s *Service) GetDashboardMetrics(ctx context.Context, dateRange DateRangeFilter) (*DashboardMetrics, error) {
	logger := s.getLogger(ctx)

	metrics := &DashboardMetrics{
		WorkItemsByType:     make(map[string]int),
		WorkItemsByPriority: make(map[string]int),
		RecentActivity:      make([]ActivitySummary, 0),
		UpcomingDeadlines:   make([]DeadlineSummary, 0),
	}

	// Get active sprints
	activeSprints, err := s.sprintRepo.ListSprintsByStatus(ctx, sprints.ListSprintsByStatusParams{
		Status: sprints.SprintManagementSprintStatusActive,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		logger.Error("failed to list active sprints", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list active sprints: %w", err)
	}

	metrics.ActiveSprints = len(activeSprints)

	// Get all work items
	allWorkItems, err := s.workItemRepo.ListWorkItems(ctx, workitems.ListWorkItemsParams{
		Limit:           10000,
		CursorTimestamp: pgtype.Timestamptz{Valid: false},
		CursorID:        pgtype.UUID{Valid: false},
	})
	if err != nil {
		logger.Error("failed to list work items", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	// Analyze work items
	for _, wi := range allWorkItems {
		metrics.TotalWorkItems++

		// Count by status
		switch wi.Status {
		case workitems.SprintManagementWorkItemStatusDone:
			metrics.CompletedWorkItems++
		case workitems.SprintManagementWorkItemStatusInProgress, workitems.SprintManagementWorkItemStatusInReview:
			metrics.InProgressWorkItems++
		case workitems.SprintManagementWorkItemStatusBlocked:
			metrics.BlockedWorkItems++
		}

		// Count by type
		metrics.WorkItemsByType[string(wi.Type)]++

		// Count by priority
		metrics.WorkItemsByPriority[string(wi.Priority)]++
	}

	// Calculate completion rate
	if metrics.TotalWorkItems > 0 {
		metrics.CompletionRate = float64(metrics.CompletedWorkItems) / float64(metrics.TotalWorkItems) * 100
	}

	// Get velocity metrics
	velocityMetrics, err := s.getVelocityForDashboard(ctx, 5)
	if err == nil {
		metrics.AverageVelocity = velocityMetrics.AverageVelocity
		if len(velocityMetrics.SprintVelocities) > 0 {
			metrics.CurrentVelocity = float64(velocityMetrics.SprintVelocities[0].CompletedPoints)
		}
	}

	// Get cycle time
	cycleTime, err := s.CalculateCycleTime(ctx, dateRange, nil)
	if err == nil && cycleTime.CompletedWorkItems > 0 {
		metrics.AverageCycleTime = cycleTime.AverageCycleTime
	}

	// Get upcoming deadlines
	now := time.Now()
	for _, sprint := range activeSprints {
		endDate := pgDateToTime(sprint.EndDate)
		daysLeft := int(endDate.Sub(now).Hours() / 24)

		if daysLeft >= 0 && daysLeft <= 30 { // Next 30 days
			metrics.UpcomingDeadlines = append(metrics.UpcomingDeadlines, DeadlineSummary{
				SprintID:   pgtypeToUUID(sprint.ID),
				SprintName: sprint.Name,
				EndDate:    endDate,
				DaysLeft:   daysLeft,
				Status:     string(sprint.Status),
			})
		}
	}

	logger.Info("dashboard metrics retrieved",
		slog.Int("active_sprints", metrics.ActiveSprints),
		slog.Int("total_work_items", metrics.TotalWorkItems))

	return metrics, nil
}

// VelocityMetrics represents team velocity metrics (simplified for dashboard)
type VelocityMetrics struct {
	TotalSprints     int              `json:"total_sprints"`
	TotalPoints      int              `json:"total_points"`
	AverageVelocity  float64          `json:"average_velocity"`
	SprintVelocities []SprintVelocity `json:"sprint_velocities"`
}

// SprintVelocity represents velocity for a single sprint
type SprintVelocity struct {
	SprintID        uuid.UUID `json:"sprint_id"`
	SprintName      string    `json:"sprint_name"`
	CompletedPoints int       `json:"completed_points"`
}

// getVelocityForDashboard is a helper to get velocity metrics
func (s *Service) getVelocityForDashboard(ctx context.Context, sprintCount int) (*VelocityMetrics, error) {
	completedSprints, err := s.sprintRepo.ListCompletedSprints(ctx, int32(sprintCount))
	if err != nil {
		return nil, err
	}

	if len(completedSprints) == 0 {
		return &VelocityMetrics{}, nil
	}

	metrics := &VelocityMetrics{
		TotalSprints:     len(completedSprints),
		SprintVelocities: make([]SprintVelocity, 0, len(completedSprints)),
	}

	for _, sprint := range completedSprints {
		completedPoints := 0
		if sprint.CompletedPoints != nil {
			completedPoints = int(*sprint.CompletedPoints)
		}

		metrics.SprintVelocities = append(metrics.SprintVelocities, SprintVelocity{
			SprintID:        pgtypeToUUID(sprint.ID),
			SprintName:      sprint.Name,
			CompletedPoints: completedPoints,
		})

		metrics.TotalPoints += completedPoints
	}

	if metrics.TotalSprints > 0 {
		metrics.AverageVelocity = float64(metrics.TotalPoints) / float64(metrics.TotalSprints)
	}

	return metrics, nil
}

// ExportFormat represents the format for report export
type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)

// ExportSprintReport exports a sprint report in the specified format
func (s *Service) ExportSprintReport(ctx context.Context, sprintID uuid.UUID, format ExportFormat) ([]byte, error) {
	report, err := s.GenerateSprintReport(ctx, sprintID)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatCSV:
		return s.exportSprintReportCSV(report)
	case ExportFormatJSON:
		return s.exportSprintReportJSON(report)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportSprintReportCSV exports sprint report as CSV
func (s *Service) exportSprintReportCSV(report *SprintReport) ([]byte, error) {
	var buf []byte

	// Header
	header := "Sprint ID,Sprint Name,Status,Start Date,End Date,Duration Days,Total Work Items,Completed Items,In Progress Items,Todo Items,Blocked Items,Completion Rate,Committed Points,Completed Points,Velocity,Utilization Rate\n"
	buf = append(buf, []byte(header)...)

	// Data row
	row := fmt.Sprintf("%s,%s,%s,%s,%s,%d,%d,%d,%d,%d,%d,%.2f,%d,%d,%d,%.2f\n",
		report.SprintID.String(),
		report.SprintName,
		report.Status,
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02"),
		report.DurationDays,
		report.TotalWorkItems,
		report.CompletedItems,
		report.InProgressItems,
		report.TodoItems,
		report.BlockedItems,
		report.CompletionRate,
		report.CommittedPoints,
		report.CompletedPoints,
		report.Velocity,
		report.UtilizationRate,
	)
	buf = append(buf, []byte(row)...)

	return buf, nil
}

// exportSprintReportJSON exports sprint report as JSON
func (s *Service) exportSprintReportJSON(report *SprintReport) ([]byte, error) {
	// Use standard JSON encoding
	data := map[string]interface{}{
		"sprint_id":              report.SprintID.String(),
		"sprint_name":            report.SprintName,
		"status":                 report.Status,
		"start_date":             report.StartDate.Format("2006-01-02"),
		"end_date":               report.EndDate.Format("2006-01-02"),
		"duration_days":          report.DurationDays,
		"total_work_items":       report.TotalWorkItems,
		"completed_items":        report.CompletedItems,
		"in_progress_items":      report.InProgressItems,
		"todo_items":             report.TodoItems,
		"blocked_items":          report.BlockedItems,
		"completion_rate":        report.CompletionRate,
		"committed_points":       report.CommittedPoints,
		"completed_points":       report.CompletedPoints,
		"velocity":               report.Velocity,
		"utilization_rate":       report.UtilizationRate,
		"work_items_by_type":     report.WorkItemsByType,
		"work_items_by_priority": report.WorkItemsByPriority,
	}

	// Simple JSON marshaling
	var buf []byte
	buf = append(buf, '{')
	first := true
	for key, value := range data {
		if !first {
			buf = append(buf, ',')
		}
		first = false
		buf = append(buf, []byte(fmt.Sprintf("\"%s\":", key))...)

		switch v := value.(type) {
		case string:
			buf = append(buf, []byte(fmt.Sprintf("\"%s\"", v))...)
		case int:
			buf = append(buf, []byte(fmt.Sprintf("%d", v))...)
		case float64:
			buf = append(buf, []byte(fmt.Sprintf("%.2f", v))...)
		case map[string]int:
			buf = append(buf, '{')
			firstMap := true
			for k, val := range v {
				if !firstMap {
					buf = append(buf, ',')
				}
				firstMap = false
				buf = append(buf, []byte(fmt.Sprintf("\"%s\":%d", k, val))...)
			}
			buf = append(buf, '}')
		}
	}
	buf = append(buf, '}')

	return buf, nil
}

// ExportTeamPerformanceReport exports team performance report in the specified format
func (s *Service) ExportTeamPerformanceReport(ctx context.Context, dateRange DateRangeFilter, limit int, format ExportFormat) ([]byte, error) {
	report, err := s.GenerateTeamPerformanceReport(ctx, dateRange, limit)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatCSV:
		return s.exportTeamPerformanceReportCSV(report)
	case ExportFormatJSON:
		return s.exportTeamPerformanceReportJSON(report)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportTeamPerformanceReportCSV exports team performance report as CSV
func (s *Service) exportTeamPerformanceReportCSV(report *TeamPerformanceReport) ([]byte, error) {
	var buf []byte

	// Summary header
	buf = append(buf, []byte("Team Performance Summary\n")...)
	buf = append(buf, []byte(fmt.Sprintf("Total Sprints,%d\n", report.TotalSprints))...)
	buf = append(buf, []byte(fmt.Sprintf("Completed Sprints,%d\n", report.CompletedSprints))...)
	buf = append(buf, []byte(fmt.Sprintf("Total Work Items,%d\n", report.TotalWorkItems))...)
	buf = append(buf, []byte(fmt.Sprintf("Completed Work Items,%d\n", report.CompletedWorkItems))...)
	buf = append(buf, []byte(fmt.Sprintf("Average Velocity,%.2f\n", report.AverageVelocity))...)
	buf = append(buf, []byte(fmt.Sprintf("Average Completion Rate,%.2f\n\n", report.AverageCompletionRate))...)

	// Sprint details header
	buf = append(buf, []byte("Sprint Name,Velocity,Completion Rate\n")...)

	// Sprint data
	for _, sprint := range report.SprintReports {
		row := fmt.Sprintf("%s,%d,%.2f\n",
			sprint.SprintName,
			sprint.Velocity,
			sprint.CompletionRate,
		)
		buf = append(buf, []byte(row)...)
	}

	return buf, nil
}

// exportTeamPerformanceReportJSON exports team performance report as JSON
func (s *Service) exportTeamPerformanceReportJSON(report *TeamPerformanceReport) ([]byte, error) {
	// Simple JSON representation
	var buf []byte
	buf = append(buf, []byte(fmt.Sprintf(`{
		"total_sprints": %d,
		"completed_sprints": %d,
		"total_work_items": %d,
		"completed_work_items": %d,
		"total_story_points": %d,
		"completed_story_points": %d,
		"average_velocity": %.2f,
		"average_completion_rate": %.2f,
		"sprint_count": %d
	}`,
		report.TotalSprints,
		report.CompletedSprints,
		report.TotalWorkItems,
		report.CompletedWorkItems,
		report.TotalStoryPoints,
		report.CompletedStoryPoints,
		report.AverageVelocity,
		report.AverageCompletionRate,
		len(report.SprintReports),
	))...)

	return buf, nil
}

// ExportCycleTimeReport exports cycle time report in the specified format
func (s *Service) ExportCycleTimeReport(ctx context.Context, dateRange DateRangeFilter, workItemType *string, format ExportFormat) ([]byte, error) {
	report, err := s.CalculateCycleTime(ctx, dateRange, workItemType)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatCSV:
		return s.exportCycleTimeReportCSV(report)
	case ExportFormatJSON:
		return s.exportCycleTimeReportJSON(report)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportCycleTimeReportCSV exports cycle time report as CSV
func (s *Service) exportCycleTimeReportCSV(report *CycleTimeSummary) ([]byte, error) {
	var buf []byte

	// Summary header
	buf = append(buf, []byte("Cycle Time Summary\n")...)
	buf = append(buf, []byte(fmt.Sprintf("Total Work Items,%d\n", report.TotalWorkItems))...)
	buf = append(buf, []byte(fmt.Sprintf("Completed Work Items,%d\n", report.CompletedWorkItems))...)
	buf = append(buf, []byte(fmt.Sprintf("Average Lead Time (hours),%.2f\n", report.AverageLeadTime))...)
	buf = append(buf, []byte(fmt.Sprintf("Average Cycle Time (hours),%.2f\n\n", report.AverageCycleTime))...)

	// Work item details header
	buf = append(buf, []byte("Work Item ID,Title,Type,Status,Created At,Lead Time (hours)\n")...)

	// Work item data
	for _, wi := range report.WorkItems {
		leadTime := ""
		if wi.LeadTimeHours != nil {
			leadTime = fmt.Sprintf("%.2f", *wi.LeadTimeHours)
		}

		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
			wi.WorkItemID.String(),
			wi.WorkItemTitle,
			wi.WorkItemType,
			wi.Status,
			wi.CreatedAt.Format("2006-01-02 15:04:05"),
			leadTime,
		)
		buf = append(buf, []byte(row)...)
	}

	return buf, nil
}

// exportCycleTimeReportJSON exports cycle time report as JSON
func (s *Service) exportCycleTimeReportJSON(report *CycleTimeSummary) ([]byte, error) {
	// Simple JSON representation
	var buf []byte
	buf = append(buf, []byte(fmt.Sprintf(`{
		"total_work_items": %d,
		"completed_work_items": %d,
		"average_lead_time_hours": %.2f,
		"average_cycle_time_hours": %.2f,
		"median_lead_time_hours": %.2f,
		"median_cycle_time_hours": %.2f,
		"min_lead_time_hours": %.2f,
		"max_lead_time_hours": %.2f
	}`,
		report.TotalWorkItems,
		report.CompletedWorkItems,
		report.AverageLeadTime,
		report.AverageCycleTime,
		report.MedianLeadTime,
		report.MedianCycleTime,
		report.MinLeadTime,
		report.MaxLeadTime,
	))...)

	return buf, nil
}
