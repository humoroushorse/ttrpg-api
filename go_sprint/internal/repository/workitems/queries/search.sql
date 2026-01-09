-- Search queries for work items with full-text search support

-- name: SearchWorkItems :many
SELECT 
    w.*,
    ts_rank(
        to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')),
        plainto_tsquery('english', $1)
    ) as rank
FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')) @@ plainto_tsquery('english', $1)
ORDER BY rank DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchWorkItemsWithFilters :many
SELECT 
    w.*,
    ts_rank(
        to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')),
        plainto_tsquery('english', $1)
    ) as rank
FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')) @@ plainto_tsquery('english', $1)
    AND ($2::sprint_management.work_item_type IS NULL OR w.type = $2)
    AND ($3::sprint_management.work_item_status IS NULL OR w.status = $3)
    AND ($4::sprint_management.priority_level IS NULL OR w.priority = $4)
    AND ($5::uuid IS NULL OR w.assignee_id = $5)
    AND ($6::uuid IS NULL OR w.sprint_id = $6)
ORDER BY rank DESC, created_at DESC
LIMIT $7 OFFSET $8;

-- name: SearchWorkItemsByField :many
-- Field-specific search supporting title, description, or both
SELECT 
    w.*,
    CASE 
        WHEN $2 = 'title' THEN ts_rank(to_tsvector('english', w.title), plainto_tsquery('english', $1))
        WHEN $2 = 'description' THEN ts_rank(to_tsvector('english', COALESCE(w.description, '')), plainto_tsquery('english', $1))
        ELSE ts_rank(to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')), plainto_tsquery('english', $1))
    END as rank
FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND (
        CASE 
            WHEN $2 = 'title' THEN to_tsvector('english', w.title) @@ plainto_tsquery('english', $1)
            WHEN $2 = 'description' THEN to_tsvector('english', COALESCE(w.description, '')) @@ plainto_tsquery('english', $1)
            ELSE to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')) @@ plainto_tsquery('english', $1)
        END
    )
ORDER BY rank DESC, created_at DESC
LIMIT $3 OFFSET $4;

-- name: SearchWorkItemsBoolean :many
-- Boolean search with AND/OR operators using websearch_to_tsquery
SELECT 
    w.*,
    ts_rank(
        to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')),
        websearch_to_tsquery('english', $1)
    ) as rank
FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')) @@ websearch_to_tsquery('english', $1)
ORDER BY rank DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: FilterWorkItems :many
-- Advanced filtering without search text
SELECT * FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND ($1::sprint_management.work_item_type IS NULL OR w.type = $1)
    AND ($2::sprint_management.work_item_status IS NULL OR w.status = $2)
    AND ($3::sprint_management.priority_level IS NULL OR w.priority = $3)
    AND ($4::uuid IS NULL OR w.assignee_id = $4)
    AND ($5::uuid IS NULL OR w.sprint_id = $5)
    AND ($6::uuid IS NULL OR w.reporter_id = $6)
    AND ($7::uuid IS NULL OR w.parent_id = $7)
ORDER BY created_at DESC, id
LIMIT $8 OFFSET $9;

-- name: CountSearchResults :one
-- Count total search results for pagination
SELECT COUNT(*) FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND to_tsvector('english', w.title || ' ' || COALESCE(w.description, '')) @@ plainto_tsquery('english', $1);

-- name: CountFilteredWorkItems :one
-- Count filtered results without search
SELECT COUNT(*) FROM sprint_management.work_items w
WHERE 
    deleted_at IS NULL
    AND ($1::sprint_management.work_item_type IS NULL OR w.type = $1)
    AND ($2::sprint_management.work_item_status IS NULL OR w.status = $2)
    AND ($3::sprint_management.priority_level IS NULL OR w.priority = $3)
    AND ($4::uuid IS NULL OR w.assignee_id = $4)
    AND ($5::uuid IS NULL OR w.sprint_id = $5)
    AND ($6::uuid IS NULL OR w.reporter_id = $6)
    AND ($7::uuid IS NULL OR w.parent_id = $7);
