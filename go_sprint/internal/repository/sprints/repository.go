package sprints

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides read/write separation for sprint operations
type Repository struct {
	master  *pgxpool.Pool // Write operations
	replica *pgxpool.Pool // Read operations
}

// NewRepository creates a new sprint repository with read/write separation
func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		master:  master,
		replica: replica,
	}
}

// Write operations (use master database)

func (r *Repository) CreateSprint(ctx context.Context, params CreateSprintParams) (SprintManagementSprint, error) {
	queries := New(r.master)
	return queries.CreateSprint(ctx, params)
}

func (r *Repository) UpdateSprint(ctx context.Context, params UpdateSprintParams) (SprintManagementSprint, error) {
	queries := New(r.master)
	return queries.UpdateSprint(ctx, params)
}

func (r *Repository) UpdateSprintStatus(ctx context.Context, params UpdateSprintStatusParams) (SprintManagementSprint, error) {
	queries := New(r.master)
	return queries.UpdateSprintStatus(ctx, params)
}

func (r *Repository) UpdateSprintMetrics(ctx context.Context, params UpdateSprintMetricsParams) (SprintManagementSprint, error) {
	queries := New(r.master)
	return queries.UpdateSprintMetrics(ctx, params)
}

func (r *Repository) SoftDeleteSprint(ctx context.Context, params SoftDeleteSprintParams) error {
	queries := New(r.master)
	return queries.SoftDeleteSprint(ctx, params)
}

func (r *Repository) RestoreSprint(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.RestoreSprint(ctx, id)
}

func (r *Repository) PermanentlyDeleteSprint(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.PermanentlyDeleteSprint(ctx, id)
}

func (r *Repository) MoveWorkItemsToBacklog(ctx context.Context, sprintID pgtype.UUID) error {
	queries := New(r.master)
	return queries.MoveWorkItemsToBacklog(ctx, sprintID)
}

// Read operations (use replica database)

func (r *Repository) GetSprintByID(ctx context.Context, id pgtype.UUID) (SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.GetSprintByID(ctx, id)
}

func (r *Repository) GetSprintByIDIncludingDeleted(ctx context.Context, id pgtype.UUID) (SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.GetSprintByIDIncludingDeleted(ctx, id)
}

func (r *Repository) ListSprints(ctx context.Context, params ListSprintsParams) ([]SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.ListSprints(ctx, params)
}

func (r *Repository) ListSprintsByStatus(ctx context.Context, params ListSprintsByStatusParams) ([]SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.ListSprintsByStatus(ctx, params)
}

func (r *Repository) ListActiveSprints(ctx context.Context) ([]SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.ListActiveSprints(ctx)
}

func (r *Repository) ListSoftDeletedSprints(ctx context.Context, params ListSoftDeletedSprintsParams) ([]SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.ListSoftDeletedSprints(ctx, params)
}

func (r *Repository) GetSprintWithWorkItems(ctx context.Context, id pgtype.UUID) (GetSprintWithWorkItemsRow, error) {
	queries := New(r.replica)
	return queries.GetSprintWithWorkItems(ctx, id)
}

func (r *Repository) CountSprintsByStatus(ctx context.Context, status SprintManagementSprintStatus) (int64, error) {
	queries := New(r.replica)
	return queries.CountSprintsByStatus(ctx, status)
}

func (r *Repository) ListCompletedSprints(ctx context.Context, limit int32) ([]SprintManagementSprint, error) {
	queries := New(r.replica)
	return queries.ListCompletedSprints(ctx, limit)
}
