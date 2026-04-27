-- name: CreateWorkItem :one
INSERT INTO sprint_management.work_items (
    type,
    title,
    description,
    status,
    priority,
    story_points,
    assignee_id,
    reporter_id,
    parent_id,
    sprint_id,
    project_id,
    ticket_number
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetWorkItemByID :one
SELECT 
    wi.*,
    p.key as project_key
FROM sprint_management.work_items wi
LEFT JOIN sprint_management.projects p ON wi.project_id = p.id
WHERE wi.id = $1 AND wi.deleted_at IS NULL;

-- name: GetWorkItemByIDIncludingDeleted :one
SELECT * FROM sprint_management.work_items
WHERE id = $1;

-- name: UpdateWorkItem :one
UPDATE sprint_management.work_items
SET
    type = COALESCE(sqlc.narg('type'), type),
    title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    status = COALESCE(sqlc.narg('status'), status),
    priority = COALESCE(sqlc.narg('priority'), priority),
    story_points = COALESCE(sqlc.narg('story_points'), story_points),
    assignee_id = COALESCE(sqlc.narg('assignee_id'), assignee_id),
    parent_id = COALESCE(sqlc.narg('parent_id'), parent_id),
    sprint_id = COALESCE(sqlc.narg('sprint_id'), sprint_id)
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteWorkItem :exec
UPDATE sprint_management.work_items
SET
    deleted_at = NOW() AT TIME ZONE 'UTC',
    deleted_by = $2
WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreWorkItem :exec
UPDATE sprint_management.work_items
SET
    deleted_at = NULL,
    deleted_by = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: PermanentlyDeleteWorkItem :exec
DELETE FROM sprint_management.work_items
WHERE id = $1;

-- name: ListWorkItems :many
SELECT 
    wi.*,
    p.key as project_key
FROM sprint_management.work_items wi
LEFT JOIN sprint_management.projects p ON wi.project_id = p.id
WHERE wi.deleted_at IS NULL
  AND (
    sqlc.narg('cursor_timestamp')::timestamptz IS NULL
    OR (wi.created_at, wi.id) > (sqlc.narg('cursor_timestamp')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY wi.created_at ASC, wi.id ASC
LIMIT $1;

-- name: ListWorkItemsByType :many
SELECT * FROM sprint_management.work_items
WHERE type = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id
LIMIT $2 OFFSET $3;

-- name: ListWorkItemsByStatus :many
SELECT * FROM sprint_management.work_items
WHERE status = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id
LIMIT $2 OFFSET $3;

-- name: ListWorkItemsBySprint :many
SELECT 
    wi.*,
    p.key as project_key
FROM sprint_management.work_items wi
LEFT JOIN sprint_management.projects p ON wi.project_id = p.id
WHERE wi.sprint_id = $1 AND wi.deleted_at IS NULL
ORDER BY wi.created_at DESC, wi.id;

-- name: ListWorkItemsByParent :many
SELECT * FROM sprint_management.work_items
WHERE parent_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: ListWorkItemsByAssignee :many
SELECT * FROM sprint_management.work_items
WHERE assignee_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id
LIMIT $2 OFFSET $3;

-- name: ListSoftDeletedWorkItems :many
SELECT * FROM sprint_management.work_items
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id
LIMIT $1 OFFSET $2;

-- name: CountWorkItemsBySprint :one
SELECT COUNT(*) FROM sprint_management.work_items
WHERE sprint_id = $1 AND deleted_at IS NULL;

-- name: CountWorkItemsByStatus :one
SELECT COUNT(*) FROM sprint_management.work_items
WHERE status = $1 AND deleted_at IS NULL;

-- name: SumStoryPointsBySprint :one
SELECT COALESCE(SUM(story_points), 0) as total
FROM sprint_management.work_items
WHERE sprint_id = $1 AND deleted_at IS NULL;

-- name: SumCompletedStoryPointsBySprint :one
SELECT COALESCE(SUM(story_points), 0) as total
FROM sprint_management.work_items
WHERE sprint_id = $1 AND status = 'done' AND deleted_at IS NULL;

-- name: HasDependencies :one
SELECT EXISTS(
    SELECT 1 FROM sprint_management.work_item_dependencies
    WHERE source_id = $1 OR target_id = $1
) as has_dependencies;

-- name: HasChildren :one
SELECT EXISTS(
    SELECT 1 FROM sprint_management.work_items
    WHERE parent_id = $1 AND deleted_at IS NULL
) as has_children;
