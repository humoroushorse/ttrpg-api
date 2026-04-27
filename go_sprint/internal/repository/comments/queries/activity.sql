-- name: CreateActivityLog :one
INSERT INTO sprint_management.activity_logs (
    entity_type,
    entity_id,
    action,
    user_id,
    changes,
    trace_id
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetActivityLogByID :one
SELECT * FROM sprint_management.activity_logs
WHERE id = $1;

-- name: ListActivityLogsByEntity :many
SELECT * FROM sprint_management.activity_logs
WHERE entity_type = $1 AND entity_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListActivityLogsByUser :many
SELECT * FROM sprint_management.activity_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActivityLogsByTraceID :many
SELECT * FROM sprint_management.activity_logs
WHERE trace_id = $1
ORDER BY created_at ASC;

-- name: ListActivityLogsByAction :many
SELECT * FROM sprint_management.activity_logs
WHERE action = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRecentActivityLogs :many
SELECT * FROM sprint_management.activity_logs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountActivityLogsByEntity :one
SELECT COUNT(*) FROM sprint_management.activity_logs
WHERE entity_type = $1 AND entity_id = $2;

-- name: CountActivityLogsByUser :one
SELECT COUNT(*) FROM sprint_management.activity_logs
WHERE user_id = $1;
