-- name: ClassIsLive :one
SELECT EXISTS (SELECT 1 FROM dnd.dnd_class WHERE id = $1 AND deleted_at IS NULL);

-- name: SpellIsLive :one
SELECT EXISTS (SELECT 1 FROM dnd.spell WHERE id = $1 AND deleted_at IS NULL);

-- name: ListClassSpells :many
-- A class's links to live spells, by spell name then newest edition.
SELECT sc.spell_id, sc.class_id, sc.qualifier, sc.created_at, sc.created_by
FROM dnd.spell_class sc
JOIN dnd.spell s ON s.id = sc.spell_id
WHERE sc.class_id = $1 AND s.deleted_at IS NULL
ORDER BY s.name, s.dnd_version_year DESC;

-- name: GetLink :one
SELECT spell_id, class_id, qualifier, created_at, created_by
FROM dnd.spell_class
WHERE spell_id = $1 AND class_id = $2;

-- name: AddLink :one
-- No row when the link already exists (GetLink returns it unchanged).
INSERT INTO dnd.spell_class (spell_id, class_id, qualifier, created_by, updated_by)
VALUES (@spell_id, @class_id, @qualifier, @user_id, @user_id)
ON CONFLICT (spell_id, class_id) DO NOTHING
RETURNING spell_id, class_id, qualifier, created_at, created_by;

-- name: RemoveLink :one
DELETE FROM dnd.spell_class
WHERE spell_id = $1 AND class_id = $2
RETURNING spell_id;

-- name: SpellsByKeys :many
-- The spells (deleted ones too) for a bulk file's (source_id, slug) pairs.
SELECT s.id, s.source_id, s.slug, s.dnd_version_year, s.deleted_at IS NOT NULL AS deleted
FROM dnd.spell s
JOIN (SELECT unnest(@source_ids::uuid[]) AS source_id, unnest(@slugs::text[]) AS slug) k
  ON s.source_id = k.source_id AND s.slug = k.slug;

-- name: LiveClasses :many
SELECT id, lower(name)::text AS name, dnd_version_year
FROM dnd.dnd_class
WHERE deleted_at IS NULL;

-- name: LinksOfSpells :many
SELECT spell_id, class_id FROM dnd.spell_class WHERE spell_id = ANY(@spell_ids::uuid[]);

-- name: AddLinks :exec
-- One row per (spell, class, qualifier); an empty qualifier is NULL (the class's own list).
INSERT INTO dnd.spell_class (spell_id, class_id, qualifier, created_by, updated_by)
SELECT l.spell_id, l.class_id, NULLIF(l.qualifier, ''), @user_id::uuid, @user_id::uuid
FROM (SELECT unnest(@spell_ids::uuid[]) AS spell_id, unnest(@class_ids::uuid[]) AS class_id,
             unnest(@qualifiers::text[]) AS qualifier) l;
