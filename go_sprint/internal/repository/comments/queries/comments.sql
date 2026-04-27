-- name: CreateComment :one
INSERT INTO sprint_management.comments (
    work_item_id,
    author_id,
    content
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetCommentByID :one
SELECT * FROM sprint_management.comments
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetCommentByIDIncludingDeleted :one
SELECT * FROM sprint_management.comments
WHERE id = $1;

-- name: UpdateComment :one
UPDATE sprint_management.comments
SET
    content = $2,
    updated_at = NOW() AT TIME ZONE 'UTC'
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteComment :exec
UPDATE sprint_management.comments
SET
    deleted_at = NOW() AT TIME ZONE 'UTC',
    deleted_by = $2
WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreComment :exec
UPDATE sprint_management.comments
SET
    deleted_at = NULL,
    deleted_by = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: PermanentlyDeleteComment :exec
DELETE FROM sprint_management.comments
WHERE id = $1;

-- name: ListCommentsByWorkItem :many
SELECT * FROM sprint_management.comments
WHERE work_item_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: ListCommentsByAuthor :many
SELECT * FROM sprint_management.comments
WHERE author_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSoftDeletedComments :many
SELECT * FROM sprint_management.comments
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC
LIMIT $1 OFFSET $2;

-- name: CountCommentsByWorkItem :one
SELECT COUNT(*) FROM sprint_management.comments
WHERE work_item_id = $1 AND deleted_at IS NULL;
