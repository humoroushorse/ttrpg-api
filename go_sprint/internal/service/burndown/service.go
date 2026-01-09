package burndown

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
	// ErrSprintNotActive is returned when trying to get burndown for non-active sprint
	ErrSprintNotActive = errors.New("sprint is not active")
)

// SprintRepository defines the interface for sprint data access
type SprintRepository interface {
	GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error)
	ListCompletedSprints(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error)
}

// WorkItemRepository defines the interface for work item data access
type WorkItemRepository interface {
	ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error)
}

// Service provides business logic for burndown chart calculations
type Service struct {
	sprintRepo   SprintRepository
	workItemRepo WorkItemRepository
	logger       *slog.Logger
}

// NewService creates a new burndown service
func NewService(sprintRepo SprintRepository, workItemRepo WorkItemRepository, logger *slog.Logger) *Service {
	return &Service{
		sprintRepo:   sprintRepo,
		workItemRepo: workItemRepo,
		logger:       logger,
	}
}

// BurndownDataPoint represents a single point in the burndown chart
type BurndownDataPoint struct {
	Date            time.Time `json:"date"`
	RemainingPoints int       `json:"remaining_points"`
	CompletedPoints int       `json:"completed_points"`
	IdealRemaining  int       `json:"ideal_remaining"` // Ideal burndown line
}

// BurndownChart represents the complete burndown chart data
type BurndownChart struct {
	SprintID        uuid.UUID           `json:"sprint_id"`
	SprintName      string              `json:"sprint_name"`
	StartDate       time.Time           `json:"start_date"`
	EndDate         time.Time           `json:"end_date"`
	TotalPoints     int                 `json:"total_points"`
	CurrentPoints   int                 `json:"current_points"`
	CompletedPoints int                 `json:"completed_points"`
	DataPoints      []BurndownDataPoint `json:"data_points"`
	IsOnTrack       bool                `json:"is_on_track"` // Whether sprint is on track to complete
}

// GetBurndownChart calculates burndown chart data for an active sprint
func (s *Service) GetBurndownChart(ctx context.Context, sprintID uuid.UUID) (*BurndownChart, error) {
	logger := s.getLogger(ctx)

	// Get sprint
	sprint, err := s.sprintRepo.GetSprintByID(ctx, uuidToPgtype(sprintID))
	if err != nil {
		logger.Error("sprint not found", slog.String("id", sprintID.String()))
		return nil, ErrSprintNotFound
	}

	// Check if sprint is active
	if sprint.Status != sprints.SprintManagementSprintStatusActive {
		logger.Error("sprint is not active",
			slog.String("id", sprintID.String()),
			slog.String("status", string(sprint.Status)))
		return nil, ErrSprintNotActive
	}

	// Get work items
	workItems, err := s.workItemRepo.ListWorkItemsBySprint(ctx, uuidToPgtype(sprintID))
	if err != nil {
		logger.Error("failed to list work items", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list work items: %w", err)
	}

	// Calculate total and completed points
	totalPoints := 0
	completedPoints := 0

	for _, wi := range workItems {
		if wi.StoryPoints != nil {
			points := int(*wi.StoryPoints)
			totalPoints += points

			if wi.Status == workitems.SprintManagementWorkItemStatusDone {
				completedPoints += points
			}
		}
	}

	currentPoints := totalPoints - completedPoints

	// Generate burndown data points
	startDate := pgDateToTime(sprint.StartDate)
	endDate := pgDateToTime(sprint.EndDate)
	dataPoints := s.generateBurndownPoints(startDate, endDate, totalPoints, completedPoints)

	// Determine if sprint is on track
	isOnTrack := s.isSprintOnTrack(startDate, endDate, totalPoints, currentPoints)

	chart := &BurndownChart{
		SprintID:        pgtypeToUUID(sprint.ID),
		SprintName:      sprint.Name,
		StartDate:       startDate,
		EndDate:         endDate,
		TotalPoints:     totalPoints,
		CurrentPoints:   currentPoints,
		CompletedPoints: completedPoints,
		DataPoints:      dataPoints,
		IsOnTrack:       isOnTrack,
	}

	logger.Info("burndown chart calculated",
		slog.String("sprint_id", sprintID.String()),
		slog.Int("total_points", totalPoints),
		slog.Int("completed_points", completedPoints),
		slog.Bool("on_track", isOnTrack))

	return chart, nil
}

// generateBurndownPoints generates data points for the burndown chart
func (s *Service) generateBurndownPoints(startDate, endDate time.Time, totalPoints, completedPoints int) []BurndownDataPoint {
	dataPoints := []BurndownDataPoint{}

	// Calculate sprint duration in days
	duration := endDate.Sub(startDate)
	totalDays := int(duration.Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}

	// Calculate ideal burndown per day
	idealBurnPerDay := float64(totalPoints) / float64(totalDays)

	// Generate data points for each day
	currentDate := startDate
	now := time.Now()

	for day := 0; day <= totalDays; day++ {
		// Calculate ideal remaining points (use float for precision, then round)
		idealRemainingFloat := float64(totalPoints) - (float64(day) * idealBurnPerDay)
		idealRemaining := int(idealRemainingFloat + 0.5) // Round to nearest int
		if idealRemaining < 0 {
			idealRemaining = 0
		}

		// For past days, we would ideally have actual data
		// For now, we'll use current completion rate
		var remainingPoints int
		var completed int

		if currentDate.After(now) {
			// Future dates - use projection
			remainingPoints = totalPoints - completedPoints
			completed = completedPoints
		} else if day == totalDays || currentDate.Equal(endDate) {
			// End date
			remainingPoints = totalPoints - completedPoints
			completed = completedPoints
		} else {
			// Past/current dates - use actual progress
			// In a real system, we'd query historical data
			remainingPoints = totalPoints - completedPoints
			completed = completedPoints
		}

		dataPoint := BurndownDataPoint{
			Date:            currentDate,
			RemainingPoints: remainingPoints,
			CompletedPoints: completed,
			IdealRemaining:  idealRemaining,
		}

		dataPoints = append(dataPoints, dataPoint)
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return dataPoints
}

// isSprintOnTrack determines if the sprint is on track to complete
func (s *Service) isSprintOnTrack(startDate, endDate time.Time, totalPoints, currentPoints int) bool {
	now := time.Now()

	// If sprint hasn't started, it's on track
	if now.Before(startDate) {
		return true
	}

	// If sprint has ended, check if all work is done
	if now.After(endDate) {
		return currentPoints == 0
	}

	// Calculate progress
	totalDuration := endDate.Sub(startDate)
	elapsed := now.Sub(startDate)
	progressRatio := float64(elapsed) / float64(totalDuration)

	// Calculate ideal remaining points at this time
	idealRemaining := float64(totalPoints) * (1 - progressRatio)

	// Sprint is on track if current remaining is less than or equal to ideal
	// Allow 10% buffer
	return float64(currentPoints) <= idealRemaining*1.1
}

// HistoricalBurndown represents historical burndown data for a completed sprint
type HistoricalBurndown struct {
	SprintID        uuid.UUID `json:"sprint_id"`
	SprintName      string    `json:"sprint_name"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	TotalPoints     int       `json:"total_points"`
	CompletedPoints int       `json:"completed_points"`
	CompletionRate  float64   `json:"completion_rate"`
}

// GetHistoricalBurndowns retrieves historical burndown data for completed sprints
func (s *Service) GetHistoricalBurndowns(ctx context.Context, limit int) (*[]HistoricalBurndown, error) {
	logger := s.getLogger(ctx)

	if limit <= 0 {
		limit = 10 // Default to last 10 sprints
	}

	// Get completed sprints
	completedSprints, err := s.sprintRepo.ListCompletedSprints(ctx, int32(limit))
	if err != nil {
		logger.Error("failed to list completed sprints", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list completed sprints: %w", err)
	}

	historicalData := make([]HistoricalBurndown, 0, len(completedSprints))

	for _, sprint := range completedSprints {
		totalPoints := 0
		completedPoints := 0

		if sprint.CommittedPoints != nil {
			totalPoints = int(*sprint.CommittedPoints)
		}
		if sprint.CompletedPoints != nil {
			completedPoints = int(*sprint.CompletedPoints)
		}

		completionRate := 0.0
		if totalPoints > 0 {
			completionRate = float64(completedPoints) / float64(totalPoints) * 100
		}

		historical := HistoricalBurndown{
			SprintID:        pgtypeToUUID(sprint.ID),
			SprintName:      sprint.Name,
			StartDate:       pgDateToTime(sprint.StartDate),
			EndDate:         pgDateToTime(sprint.EndDate),
			TotalPoints:     totalPoints,
			CompletedPoints: completedPoints,
			CompletionRate:  completionRate,
		}

		historicalData = append(historicalData, historical)
	}

	logger.Info("historical burndowns retrieved", slog.Int("count", len(historicalData)))

	return &historicalData, nil
}

// ForecastedCompletion represents forecasted sprint completion
type ForecastedCompletion struct {
	SprintID                uuid.UUID `json:"sprint_id"`
	CurrentRemainingPoints  int       `json:"current_remaining_points"`
	DaysRemaining           int       `json:"days_remaining"`
	AverageDailyVelocity    float64   `json:"average_daily_velocity"`
	ProjectedCompletion     float64   `json:"projected_completion"` // Percentage
	EstimatedCompletionDate time.Time `json:"estimated_completion_date"`
	IsLikelyToComplete      bool      `json:"is_likely_to_complete"`
}

// ForecastSprintCompletion forecasts sprint completion based on current velocity
func (s *Service) ForecastSprintCompletion(ctx context.Context, sprintID uuid.UUID) (*ForecastedCompletion, error) {
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

	// Calculate points
	totalPoints := 0
	completedPoints := 0

	for _, wi := range workItems {
		if wi.StoryPoints != nil {
			points := int(*wi.StoryPoints)
			totalPoints += points

			if wi.Status == workitems.SprintManagementWorkItemStatusDone {
				completedPoints += points
			}
		}
	}

	remainingPoints := totalPoints - completedPoints

	// Calculate time metrics
	startDate := pgDateToTime(sprint.StartDate)
	endDate := pgDateToTime(sprint.EndDate)
	now := time.Now()

	elapsed := now.Sub(startDate)
	remaining := endDate.Sub(now)

	elapsedDays := int(elapsed.Hours() / 24)
	remainingDays := int(remaining.Hours() / 24)

	if elapsedDays < 1 {
		elapsedDays = 1 // Avoid division by zero
	}

	// Calculate average daily velocity
	avgDailyVelocity := float64(completedPoints) / float64(elapsedDays)

	// Project completion
	projectedCompletion := 0.0
	if totalPoints > 0 {
		if remainingDays > 0 {
			projectedPoints := completedPoints + int(avgDailyVelocity*float64(remainingDays))
			projectedCompletion = float64(projectedPoints) / float64(totalPoints) * 100
		} else {
			projectedCompletion = float64(completedPoints) / float64(totalPoints) * 100
		}
	}

	// Estimate completion date
	estimatedCompletionDate := endDate
	if avgDailyVelocity > 0 && remainingPoints > 0 {
		daysNeeded := int(float64(remainingPoints) / avgDailyVelocity)
		estimatedCompletionDate = now.AddDate(0, 0, daysNeeded)
	}

	// Determine if likely to complete
	isLikelyToComplete := projectedCompletion >= 95.0 // 95% or more

	forecast := &ForecastedCompletion{
		SprintID:                pgtypeToUUID(sprint.ID),
		CurrentRemainingPoints:  remainingPoints,
		DaysRemaining:           remainingDays,
		AverageDailyVelocity:    avgDailyVelocity,
		ProjectedCompletion:     projectedCompletion,
		EstimatedCompletionDate: estimatedCompletionDate,
		IsLikelyToComplete:      isLikelyToComplete,
	}

	logger.Info("sprint completion forecasted",
		slog.String("sprint_id", sprintID.String()),
		slog.Float64("projected_completion", projectedCompletion),
		slog.Bool("likely_to_complete", isLikelyToComplete))

	return forecast, nil
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
