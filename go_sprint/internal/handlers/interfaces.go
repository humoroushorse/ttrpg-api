package handlers

import (
	"context"

	"github.com/google/uuid"
	burndownService "github.com/humoroushorse/go_sprint/internal/service/burndown"
	dependenciesService "github.com/humoroushorse/go_sprint/internal/service/dependencies"
	estimationService "github.com/humoroushorse/go_sprint/internal/service/estimation"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

// BurndownService defines the interface for burndown operations
type BurndownService interface {
	GetBurndownChart(ctx context.Context, sprintID uuid.UUID) (*burndownService.BurndownChart, error)
}

// EstimationService defines the interface for estimation operations
type EstimationService interface {
	CalculateVelocity(ctx context.Context, sprintCount int) (*estimationService.VelocityMetrics, error)
	GetSprintCapacity(ctx context.Context, sprintID uuid.UUID) (*estimationService.SprintCapacity, error)
	ForecastCompletion(ctx context.Context, req estimationService.ForecastRequest) (*estimationService.ForecastResult, error)
}

// DependenciesService defines the interface for dependency operations
type DependenciesService interface {
	GetDependenciesForWorkItem(ctx context.Context, workItemID uuid.UUID) ([]*models.WorkItemDependency, error)
	CreateDependency(ctx context.Context, req dependenciesService.CreateDependencyRequest) (*models.WorkItemDependency, error)
	DeleteDependency(ctx context.Context, id uuid.UUID) error
}
