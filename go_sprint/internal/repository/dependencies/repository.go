package dependencies

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository wraps the generated queries with database connection management
type Repository struct {
	master  *pgxpool.Pool
	replica *pgxpool.Pool
}

// NewRepository creates a new dependency repository
func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		master:  master,
		replica: replica,
	}
}

// CreateDependency creates a new work item dependency (write operation)
func (r *Repository) CreateDependency(ctx context.Context, params CreateDependencyParams) (SprintManagementWorkItemDependency, error) {
	queries := New(r.master)
	return queries.CreateDependency(ctx, params)
}

// GetDependencyByID retrieves a dependency by ID (read operation)
func (r *Repository) GetDependencyByID(ctx context.Context, id interface{}) (SprintManagementWorkItemDependency, error) {
	queries := New(r.replica)
	return queries.GetDependencyByID(ctx, id.(pgtype.UUID))
}

// GetDependenciesBySourceID retrieves all dependencies where the work item is the source (read operation)
func (r *Repository) GetDependenciesBySourceID(ctx context.Context, sourceID interface{}) ([]SprintManagementWorkItemDependency, error) {
	queries := New(r.replica)
	return queries.GetDependenciesBySourceID(ctx, sourceID.(pgtype.UUID))
}

// GetDependenciesByTargetID retrieves all dependencies where the work item is the target (read operation)
func (r *Repository) GetDependenciesByTargetID(ctx context.Context, targetID interface{}) ([]SprintManagementWorkItemDependency, error) {
	queries := New(r.replica)
	return queries.GetDependenciesByTargetID(ctx, targetID.(pgtype.UUID))
}

// GetAllDependenciesForWorkItem retrieves all dependencies for a work item (read operation)
func (r *Repository) GetAllDependenciesForWorkItem(ctx context.Context, workItemID interface{}) ([]SprintManagementWorkItemDependency, error) {
	queries := New(r.replica)
	return queries.GetAllDependenciesForWorkItem(ctx, workItemID.(pgtype.UUID))
}

// DeleteDependency deletes a dependency by ID (write operation)
func (r *Repository) DeleteDependency(ctx context.Context, id interface{}) error {
	queries := New(r.master)
	return queries.DeleteDependency(ctx, id.(pgtype.UUID))
}

// DeleteDependenciesByWorkItemID deletes all dependencies for a work item (write operation)
func (r *Repository) DeleteDependenciesByWorkItemID(ctx context.Context, workItemID interface{}) error {
	queries := New(r.master)
	return queries.DeleteDependenciesByWorkItemID(ctx, workItemID.(pgtype.UUID))
}

// HasBlockingDependencies checks if a work item has unresolved blocking dependencies (read operation)
func (r *Repository) HasBlockingDependencies(ctx context.Context, targetID interface{}) (bool, error) {
	queries := New(r.replica)
	return queries.HasBlockingDependencies(ctx, targetID.(pgtype.UUID))
}

// GetBlockingDependencies retrieves all unresolved blocking dependencies for a work item (read operation)
func (r *Repository) GetBlockingDependencies(ctx context.Context, targetID interface{}) ([]GetBlockingDependenciesRow, error) {
	queries := New(r.replica)
	return queries.GetBlockingDependencies(ctx, targetID.(pgtype.UUID))
}

// DependencyExists checks if a specific dependency already exists (read operation)
func (r *Repository) DependencyExists(ctx context.Context, params DependencyExistsParams) (bool, error) {
	queries := New(r.replica)
	return queries.DependencyExists(ctx, params)
}
