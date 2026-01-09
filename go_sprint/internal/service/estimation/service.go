package estimation

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
	// ErrInvalidStoryPoints is returned when story points are not in the configured scale
	ErrInvalidStoryPoints = errors.New("story points must be from the configured scale")
	// ErrSprintNotFound is returned when a sprint is not found
	ErrSprintNotFound = errors.New("sprint not found")
	// ErrNoCompletedSprints is returned when there are no completed sprints for velocity calculation
	ErrNoCompletedSprints = errors.New("no completed sprints found for velocity calculation")
)

// DefaultStoryPointScale is the default Fibonacci-based story point scale
var DefaultStoryPointScale = []int{0, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

// SprintRepository defines the interface for sprint data access
type SprintRepository interface {
	GetSprintByID(ctx context.Context, id pgtype.UUID) (sprints.SprintManagementSprint, error)
	ListCompletedSprints(ctx context.Context, limit int32) ([]sprints.SprintManagementSprint, error)
}

// WorkItemRepository defines the interface for work item data access
type WorkItemRepository interface {
	ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]workitems.SprintManagementWorkItem, error)
}

// Config holds estimation service configuration
type Config struct {
	StoryPointScale []int // Allowed story point values
}

// Service provides business logic for estimation and planning
type Service struct {
	sprintRepo   SprintRepository
	workItemRepo WorkItemRepository
	config       Config
	logger       *slog.Logger
}

// NewService creates a new estimation service
func NewService(sprintRepo SprintRepository, workItemRepo WorkItemRepository, config Config, logger *slog.Logger) *Service {
	// Use default scale if none provided
	if len(config.StoryPointScale) == 0 {
		config.StoryPointScale = DefaultStoryPointScale
	}

	return &Service{
		sprintRepo:   sprintRepo,
		workItemRepo: workItemRepo,
		config:       config,
		logger:       logger,
	}
}

// ValidateStoryPoints validates that story points are in the configured scale
func (s *Service) ValidateStoryPoints(ctx context.Context, points int) error {
	logger := s.getLogger(ctx)

	// Check if points are in the configured scale
	for _, validPoint := range s.config.StoryPointScale {
		if points == validPoint {
			logger.Debug("story points validated", slog.Int("points", points))
			return nil
		}
	}

	logger.Error("invalid story points",
		slog.Int("points", points),
		slog.Any("valid_scale", s.config.StoryPointScale))
	return fmt.Errorf("%w: %d not in scale %v", ErrInvalidStoryPoints, points, s.config.StoryPointScale)
}

// GetStoryPointScale returns the configured story point scale
func (s *Service) GetStoryPointScale(ctx context.Context) []int {
	return s.config.StoryPointScale
}

// SprintCapacity represents sprint capacity information
type SprintCapacity struct {
	SprintID        uuid.UUID `json:"sprint_id"`
	SprintName      string    `json:"sprint_name"`
	CapacityPoints  *int      `json:"capacity_points,omitempty"`
	CommittedPoints int       `json:"committed_points"`
	CompletedPoints int       `json:"completed_points"`
	RemainingPoints int       `json:"remaining_points"`
	UtilizationRate float64   `json:"utilization_rate"` // Committed / Capacity * 100
	CompletionRate  float64   `json:"completion_rate"`  // Completed / Committed * 100
}

// GetSprintCapacity retrieves capacity information for a sprint
func (s *Service) GetSprintCapacity(ctx context.Context, sprintID uuid.UUID) (*SprintCapacity, error) {
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

	// Calculate committed and completed points
	committedPoints := 0
	completedPoints := 0

	for _, wi := range workItems {
		if wi.StoryPoints != nil {
			points := int(*wi.StoryPoints)
			committedPoints += points

			if wi.Status == workitems.SprintManagementWorkItemStatusDone {
				completedPoints += points
			}
		}
	}

	capacity := &SprintCapacity{
		SprintID:        pgtypeToUUID(sprint.ID),
		SprintName:      sprint.Name,
		CommittedPoints: committedPoints,
		CompletedPoints: completedPoints,
		RemainingPoints: committedPoints - completedPoints,
	}

	if sprint.CapacityPoints != nil {
		cp := int(*sprint.CapacityPoints)
		capacity.CapacityPoints = &cp

		// Calculate utilization rate
		if cp > 0 {
			capacity.UtilizationRate = float64(committedPoints) / float64(cp) * 100
		}
	}

	// Calculate completion rate
	if committedPoints > 0 {
		capacity.CompletionRate = float64(completedPoints) / float64(committedPoints) * 100
	}

	logger.Info("sprint capacity calculated",
		slog.String("sprint_id", sprintID.String()),
		slog.Int("committed", committedPoints),
		slog.Int("completed", completedPoints))

	return capacity, nil
}

// VelocityMetrics represents team velocity metrics
type VelocityMetrics struct {
	TotalSprints     int              `json:"total_sprints"`
	TotalPoints      int              `json:"total_points"`
	AverageVelocity  float64          `json:"average_velocity"`
	MinVelocity      int              `json:"min_velocity"`
	MaxVelocity      int              `json:"max_velocity"`
	SprintVelocities []SprintVelocity `json:"sprint_velocities"`
}

// SprintVelocity represents velocity for a single sprint
type SprintVelocity struct {
	SprintID        uuid.UUID `json:"sprint_id"`
	SprintName      string    `json:"sprint_name"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	CompletedPoints int       `json:"completed_points"`
}

// CalculateVelocity calculates team velocity based on completed sprints
func (s *Service) CalculateVelocity(ctx context.Context, sprintCount int) (*VelocityMetrics, error) {
	logger := s.getLogger(ctx)

	if sprintCount <= 0 {
		sprintCount = 5 // Default to last 5 sprints
	}

	// Get completed sprints
	completedSprints, err := s.sprintRepo.ListCompletedSprints(ctx, int32(sprintCount))
	if err != nil {
		logger.Error("failed to list completed sprints", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to list completed sprints: %w", err)
	}

	if len(completedSprints) == 0 {
		logger.Warn("no completed sprints found")
		return nil, ErrNoCompletedSprints
	}

	metrics := &VelocityMetrics{
		TotalSprints:     len(completedSprints),
		SprintVelocities: make([]SprintVelocity, 0, len(completedSprints)),
		MinVelocity:      int(^uint(0) >> 1), // Max int
	}

	// Calculate velocity for each sprint
	for _, sprint := range completedSprints {
		completedPoints := 0
		if sprint.CompletedPoints != nil {
			completedPoints = int(*sprint.CompletedPoints)
		}

		velocity := SprintVelocity{
			SprintID:        pgtypeToUUID(sprint.ID),
			SprintName:      sprint.Name,
			StartDate:       pgDateToTime(sprint.StartDate),
			EndDate:         pgDateToTime(sprint.EndDate),
			CompletedPoints: completedPoints,
		}

		metrics.SprintVelocities = append(metrics.SprintVelocities, velocity)
		metrics.TotalPoints += completedPoints

		// Track min/max
		if completedPoints < metrics.MinVelocity {
			metrics.MinVelocity = completedPoints
		}
		if completedPoints > metrics.MaxVelocity {
			metrics.MaxVelocity = completedPoints
		}
	}

	// Calculate average velocity
	if metrics.TotalSprints > 0 {
		metrics.AverageVelocity = float64(metrics.TotalPoints) / float64(metrics.TotalSprints)
	}

	logger.Info("velocity calculated",
		slog.Int("sprints", metrics.TotalSprints),
		slog.Float64("average_velocity", metrics.AverageVelocity))

	return metrics, nil
}

// ForecastRequest represents a request for sprint forecasting
type ForecastRequest struct {
	RemainingPoints int // Total story points remaining in backlog
	SprintCount     int // Number of historical sprints to use for velocity
}

// ForecastResult represents forecasting results
type ForecastResult struct {
	AverageVelocity     float64 `json:"average_velocity"`
	RemainingPoints     int     `json:"remaining_points"`
	EstimatedSprints    int     `json:"estimated_sprints"`
	EstimatedSprintsMin int     `json:"estimated_sprints_min"` // Based on max velocity
	EstimatedSprintsMax int     `json:"estimated_sprints_max"` // Based on min velocity
	ConfidenceLevel     string  `json:"confidence_level"`      // "high", "medium", "low"
}

// ForecastCompletion forecasts sprint completion based on historical velocity
func (s *Service) ForecastCompletion(ctx context.Context, req ForecastRequest) (*ForecastResult, error) {
	logger := s.getLogger(ctx)

	// Calculate velocity
	velocity, err := s.CalculateVelocity(ctx, req.SprintCount)
	if err != nil {
		return nil, err
	}

	result := &ForecastResult{
		AverageVelocity: velocity.AverageVelocity,
		RemainingPoints: req.RemainingPoints,
	}

	// Calculate estimated sprints based on average velocity
	if velocity.AverageVelocity > 0 {
		result.EstimatedSprints = int(float64(req.RemainingPoints) / velocity.AverageVelocity)
		if float64(req.RemainingPoints) > velocity.AverageVelocity*float64(result.EstimatedSprints) {
			result.EstimatedSprints++ // Round up
		}
	}

	// Calculate min/max estimates
	if velocity.MaxVelocity > 0 {
		result.EstimatedSprintsMin = req.RemainingPoints / velocity.MaxVelocity
		if req.RemainingPoints%velocity.MaxVelocity > 0 {
			result.EstimatedSprintsMin++
		}
	}

	if velocity.MinVelocity > 0 {
		result.EstimatedSprintsMax = req.RemainingPoints / velocity.MinVelocity
		if req.RemainingPoints%velocity.MinVelocity > 0 {
			result.EstimatedSprintsMax++
		}
	}

	// Determine confidence level based on velocity variance
	if velocity.TotalSprints >= 5 {
		velocityRange := velocity.MaxVelocity - velocity.MinVelocity
		if velocityRange <= int(velocity.AverageVelocity*0.2) {
			result.ConfidenceLevel = "high"
		} else if velocityRange <= int(velocity.AverageVelocity*0.5) {
			result.ConfidenceLevel = "medium"
		} else {
			result.ConfidenceLevel = "low"
		}
	} else {
		result.ConfidenceLevel = "low" // Not enough data
	}

	logger.Info("forecast calculated",
		slog.Int("remaining_points", req.RemainingPoints),
		slog.Int("estimated_sprints", result.EstimatedSprints),
		slog.String("confidence", result.ConfidenceLevel))

	return result, nil
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
