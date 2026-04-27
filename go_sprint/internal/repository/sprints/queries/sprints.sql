-- name: CreateSprint :one
INSERT INTO sprint_management.sprints (
    name,
    description,
    status,
    start_date,
    end_date,
    capacity_points,
    created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetSprintByID :one
SELECT * FROM sprint_management.sprints
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetSprintByIDIncludingDeleted :one
SELECT * FROM sprint_management.sprints
WHERE id = $1;

-- name: UpdateSprint :one
UPDATE sprint_management.sprints
SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('status'), status),
    start_date = COALESCE(sqlc.narg('start_date'), start_date),
    end_date = COALESCE(sqlc.narg('end_date'), end_date),
    capacity_points = COALESCE(sqlc.narg('capacity_points'), capacity_points),
    committed_points = COALESCE(sqlc.narg('committed_points'), committed_points),
    completed_points = COALESCE(sqlc.narg('completed_points'), completed_points)
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateSprintStatus :one
UPDATE sprint_management.sprints
SET status = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateSprintMetrics :one
UPDATE sprint_management.sprints
SET
    committed_points = $2,
    completed_points = $3
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteSprint :exec
UPDATE sprint_management.sprints
SET
    deleted_at = NOW() AT TIME ZONE 'UTC',
    deleted_by = $2
WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreSprint :exec
UPDATE sprint_management.sprints
SET
    deleted_at = NULL,
    deleted_by = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: PermanentlyDeleteSprint :exec
DELETE FROM sprint_management.sprints
WHERE id = $1;

-- name: ListSprints :many
SELECT * FROM sprint_management.sprints
WHERE deleted_at IS NULL
  AND (
    sqlc.narg('cursor_timestamp')::timestamptz IS NULL
    OR (created_at, id) > (sqlc.narg('cursor_timestamp')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
  AND (
    sqlc.narg('search_query')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('search_query')::text || '%'
    OR description ILIKE '%' || sqlc.narg('search_query')::text || '%'
  )
  AND (
    sqlc.narg('status_filter')::text[] IS NULL
    OR status::text = ANY(sqlc.narg('status_filter')::text[])
  )
ORDER BY created_at ASC, id ASC
LIMIT $1;

-- name: ListSprintsByStatus :many
SELECT * FROM sprint_management.sprints
WHERE status = $1 AND deleted_at IS NULL
ORDER BY start_date DESC, id
LIMIT $2 OFFSET $3;

-- name: ListActiveSprints :many
SELECT * FROM sprint_management.sprints
WHERE status = 'active' AND deleted_at IS NULL
ORDER BY start_date DESC, id;

-- name: ListSoftDeletedSprints :many
SELECT * FROM sprint_management.sprints
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id
LIMIT $1 OFFSET $2;

-- name: GetSprintWithWorkItems :one
SELECT
    s.*,
    COUNT(wi.id) as work_item_count,
    COALESCE(SUM(wi.story_points), 0) as total_story_points,
    COALESCE(SUM(CASE WHEN wi.status = 'done' THEN wi.story_points ELSE 0 END), 0) as completed_story_points
FROM sprint_management.sprints s
LEFT JOIN sprint_management.work_items wi ON s.id = wi.sprint_id AND wi.deleted_at IS NULL
WHERE s.id = $1 AND s.deleted_at IS NULL
GROUP BY s.id;

-- name: MoveWorkItemsToBacklog :exec
UPDATE sprint_management.work_items
SET sprint_id = NULL
WHERE sprint_id = $1 AND status != 'done' AND deleted_at IS NULL;

-- name: CountSprintsByStatus :one
SELECT COUNT(*) FROM sprint_management.sprints
WHERE status = $1 AND deleted_at IS NULL;

-- name: ListCompletedSprints :many
SELECT * FROM sprint_management.sprints
WHERE status = 'completed' AND deleted_at IS NULL
ORDER BY end_date DESC, id
LIMIT $1;
