package comments

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides read/write separation for comments and activity log operations
type Repository struct {
	master  *pgxpool.Pool // Write operations
	replica *pgxpool.Pool // Read operations
}

// NewRepository creates a new comments repository with read/write separation
func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{
		master:  master,
		replica: replica,
	}
}

// Comment Write operations (use master database)

func (r *Repository) CreateComment(ctx context.Context, params CreateCommentParams) (SprintManagementComment, error) {
	queries := New(r.master)
	return queries.CreateComment(ctx, params)
}

func (r *Repository) UpdateComment(ctx context.Context, params UpdateCommentParams) (SprintManagementComment, error) {
	queries := New(r.master)
	return queries.UpdateComment(ctx, params)
}

func (r *Repository) SoftDeleteComment(ctx context.Context, params SoftDeleteCommentParams) error {
	queries := New(r.master)
	return queries.SoftDeleteComment(ctx, params)
}

func (r *Repository) RestoreComment(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.RestoreComment(ctx, id)
}

func (r *Repository) PermanentlyDeleteComment(ctx context.Context, id pgtype.UUID) error {
	queries := New(r.master)
	return queries.PermanentlyDeleteComment(ctx, id)
}

// Comment Read operations (use replica database)

func (r *Repository) GetCommentByID(ctx context.Context, id pgtype.UUID) (SprintManagementComment, error) {
	queries := New(r.replica)
	return queries.GetCommentByID(ctx, id)
}

func (r *Repository) GetCommentByIDIncludingDeleted(ctx context.Context, id pgtype.UUID) (SprintManagementComment, error) {
	queries := New(r.replica)
	return queries.GetCommentByIDIncludingDeleted(ctx, id)
}

func (r *Repository) ListCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) ([]SprintManagementComment, error) {
	queries := New(r.replica)
	return queries.ListCommentsByWorkItem(ctx, workItemID)
}

func (r *Repository) ListCommentsByAuthor(ctx context.Context, params ListCommentsByAuthorParams) ([]SprintManagementComment, error) {
	queries := New(r.replica)
	return queries.ListCommentsByAuthor(ctx, params)
}

func (r *Repository) ListSoftDeletedComments(ctx context.Context, params ListSoftDeletedCommentsParams) ([]SprintManagementComment, error) {
	queries := New(r.replica)
	return queries.ListSoftDeletedComments(ctx, params)
}

func (r *Repository) CountCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) (int64, error) {
	queries := New(r.replica)
	return queries.CountCommentsByWorkItem(ctx, workItemID)
}

// Activity Log Write operations (use master database)

func (r *Repository) CreateActivityLog(ctx context.Context, params CreateActivityLogParams) (SprintManagementActivityLog, error) {
	queries := New(r.master)
	return queries.CreateActivityLog(ctx, params)
}

// Activity Log Read operations (use replica database)

func (r *Repository) GetActivityLogByID(ctx context.Context, id pgtype.UUID) (SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.GetActivityLogByID(ctx, id)
}

func (r *Repository) ListActivityLogsByEntity(ctx context.Context, params ListActivityLogsByEntityParams) ([]SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.ListActivityLogsByEntity(ctx, params)
}

func (r *Repository) ListActivityLogsByUser(ctx context.Context, params ListActivityLogsByUserParams) ([]SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.ListActivityLogsByUser(ctx, params)
}

func (r *Repository) ListActivityLogsByTraceID(ctx context.Context, traceID pgtype.UUID) ([]SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.ListActivityLogsByTraceID(ctx, traceID)
}

func (r *Repository) ListActivityLogsByAction(ctx context.Context, params ListActivityLogsByActionParams) ([]SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.ListActivityLogsByAction(ctx, params)
}

func (r *Repository) ListRecentActivityLogs(ctx context.Context, params ListRecentActivityLogsParams) ([]SprintManagementActivityLog, error) {
	queries := New(r.replica)
	return queries.ListRecentActivityLogs(ctx, params)
}

func (r *Repository) CountActivityLogsByEntity(ctx context.Context, params CountActivityLogsByEntityParams) (int64, error) {
	queries := New(r.replica)
	return queries.CountActivityLogsByEntity(ctx, params)
}

func (r *Repository) CountActivityLogsByUser(ctx context.Context, userID pgtype.UUID) (int64, error) {
	queries := New(r.replica)
	return queries.CountActivityLogsByUser(ctx, userID)
}
