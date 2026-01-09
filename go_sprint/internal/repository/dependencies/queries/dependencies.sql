-- name: CreateDependency :one
INSERT INTO sprint_management.work_item_dependencies (
    source_id,
    target_id,
    dependency_type,
    created_by
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetDependencyByID :one
SELECT * FROM sprint_management.work_item_dependencies
WHERE id = $1;

-- name: GetDependenciesBySourceID :many
SELECT * FROM sprint_management.work_item_dependencies
WHERE source_id = $1
ORDER BY created_at DESC;

-- name: GetDependenciesByTargetID :many
SELECT * FROM sprint_management.work_item_dependencies
WHERE target_id = $1
ORDER BY created_at DESC;

-- name: GetAllDependenciesForWorkItem :many
SELECT * FROM sprint_management.work_item_dependencies
WHERE source_id = $1 OR target_id = $1
ORDER BY created_at DESC;

-- name: DeleteDependency :exec
DELETE FROM sprint_management.work_item_dependencies
WHERE id = $1;

-- name: DeleteDependenciesByWorkItemID :exec
DELETE FROM sprint_management.work_item_dependencies
WHERE source_id = $1 OR target_id = $1;

-- name: HasBlockingDependencies :one
SELECT EXISTS(
    SELECT 1 FROM sprint_management.work_item_dependencies
    WHERE target_id = $1 
    AND dependency_type = 'blocks'
    AND source_id IN (
        SELECT id FROM sprint_management.work_items
        WHERE status != 'done' AND deleted_at IS NULL
    )
) as has_blocking;

-- name: GetBlockingDependencies :many
SELECT d.*, wi.status as source_status
FROM sprint_management.work_item_dependencies d
JOIN sprint_management.work_items wi ON d.source_id = wi.id
WHERE d.target_id = $1 
AND d.dependency_type = 'blocks'
AND wi.status != 'done' 
AND wi.deleted_at IS NULL
ORDER BY d.created_at DESC;

-- name: DependencyExists :one
SELECT EXISTS(
    SELECT 1 FROM sprint_management.work_item_dependencies
    WHERE source_id = $1 
    AND target_id = $2 
    AND dependency_type = $3
) as exists;
