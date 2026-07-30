-- name: QuerySources :many
SELECT * FROM dnd.source
WHERE (
    sqlc.narg('dnd_version')::text IS NULL
    OR dnd_version = sqlc.narg('dnd_version')::text
)
ORDER BY name ASC;

-- name: BulkCreateSource :one
INSERT INTO dnd.source (
    id,
    name,
    name_short,
    publish_year,
    dnd_version,
    dnd_version_year,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: SourceExistsByName :one
SELECT EXISTS(
    SELECT 1 FROM dnd.source WHERE name = $1
) AS exists;

-- name: GetSourceByNameVersion :one
SELECT * FROM dnd.source
WHERE name = $1
  AND dnd_version = $2
  AND dnd_version_year = $3
LIMIT 1;

-- name: GetSourceByNameShortVersion :one
SELECT * FROM dnd.source
WHERE LOWER(name_short) = LOWER($1)
  AND dnd_version = $2
  AND dnd_version_year = $3
LIMIT 1;
