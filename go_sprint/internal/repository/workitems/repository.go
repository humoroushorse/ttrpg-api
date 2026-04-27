package workitems

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides read/write separation for work item operations
type Repository struct {
	master  *pgxpool.Pool // Write operations
	replica *pgxpool.Pool // Read operations
}

// NewRepository creates a new work item repository with read/write separation
func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		master:  master,
		replica: replica,
	}
}

// Write operations (use master database)

func (r *Repository) CreateWorkItem(ctx context.Context, params CreateWorkItemParams) (SprintManagementWorkItem, error) {
	queries := New(r.master)
	return queries.CreateWorkItem(ctx, params)
}

func (r *Repository) UpdateWorkItem(ctx context.Context, params UpdateWorkItemParams) (SprintManagementWorkItem, error) {
	queries := New(r.master)
	return queries.UpdateWorkItem(ctx, params)
}

func (r *Repository) SoftDeleteWorkItem(ctx context.Context, params SoftDeleteWorkItemParams) error {
	queries := New(r.master)
	return queries.SoftDeleteWorkItem(ctx, params)
}

func (r *Repository) RestoreWorkItem(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.RestoreWorkItem(ctx, id)
}

func (r *Repository) PermanentlyDeleteWorkItem(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.PermanentlyDeleteWorkItem(ctx, id)
}

// Read operations (use replica database)

func (r *Repository) GetWorkItemByID(ctx context.Context, id pgtype.UUID) (GetWorkItemByIDRow, error) {
	queries := New(r.replica)
	return queries.GetWorkItemByID(ctx, id)
}

func (r *Repository) GetWorkItemByIDIncludingDeleted(ctx context.Context, id pgtype.UUID) (SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.GetWorkItemByIDIncludingDeleted(ctx, id)
}

func (r *Repository) ListWorkItems(ctx context.Context, params ListWorkItemsParams) ([]ListWorkItemsRow, error) {
	queries := New(r.replica)
	return queries.ListWorkItems(ctx, params)
}

func (r *Repository) ListWorkItemsByType(ctx context.Context, params ListWorkItemsByTypeParams) ([]SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.ListWorkItemsByType(ctx, params)
}

func (r *Repository) ListWorkItemsByStatus(ctx context.Context, params ListWorkItemsByStatusParams) ([]SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.ListWorkItemsByStatus(ctx, params)
}

func (r *Repository) ListWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) ([]ListWorkItemsBySprintRow, error) {
	queries := New(r.replica)
	return queries.ListWorkItemsBySprint(ctx, sprintID)
}

func (r *Repository) ListWorkItemsByParent(ctx context.Context, parentID pgtype.UUID) ([]SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.ListWorkItemsByParent(ctx, parentID)
}

func (r *Repository) ListWorkItemsByAssignee(ctx context.Context, params ListWorkItemsByAssigneeParams) ([]SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.ListWorkItemsByAssignee(ctx, params)
}

func (r *Repository) ListSoftDeletedWorkItems(ctx context.Context, params ListSoftDeletedWorkItemsParams) ([]SprintManagementWorkItem, error) {
	queries := New(r.replica)
	return queries.ListSoftDeletedWorkItems(ctx, params)
}

func (r *Repository) CountWorkItemsBySprint(ctx context.Context, sprintID pgtype.UUID) (int64, error) {
	queries := New(r.replica)
	return queries.CountWorkItemsBySprint(ctx, sprintID)
}

func (r *Repository) CountWorkItemsByStatus(ctx context.Context, status SprintManagementWorkItemStatus) (int64, error) {
	queries := New(r.replica)
	return queries.CountWorkItemsByStatus(ctx, status)
}

func (r *Repository) SumStoryPointsBySprint(ctx context.Context, sprintID pgtype.UUID) (int64, error) {
	queries := New(r.replica)
	result, err := queries.SumStoryPointsBySprint(ctx, sprintID)
	if err != nil {
		return 0, err
	}

	// Convert interface{} to int64
	switch v := result.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for story points sum: %T", result)
	}
}

func (r *Repository) SumCompletedStoryPointsBySprint(ctx context.Context, sprintID pgtype.UUID) (int64, error) {
	queries := New(r.replica)
	result, err := queries.SumCompletedStoryPointsBySprint(ctx, sprintID)
	if err != nil {
		return 0, err
	}

	// Convert interface{} to int64
	switch v := result.(type) {
	case int64:
		return v, nil
	case int32:
		return int64(v), nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected type for completed story points sum: %T", result)
	}
}

func (r *Repository) HasDependencies(ctx context.Context, workItemID pgtype.UUID) (bool, error) {
	queries := New(r.replica)
	return queries.HasDependencies(ctx, workItemID)
}

func (r *Repository) HasChildren(ctx context.Context, parentID pgtype.UUID) (bool, error) {
	queries := New(r.replica)
	return queries.HasChildren(ctx, parentID)
}
