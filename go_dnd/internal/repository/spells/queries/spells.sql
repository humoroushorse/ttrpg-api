-- name: CountAllSpells :one
SELECT COUNT(*) FROM dnd.spell;

-- name: ListSpells :many
SELECT * FROM dnd.spell
ORDER BY name ASC
LIMIT $1 OFFSET $2;

-- name: CountQuerySpells :one
SELECT COUNT(*) FROM dnd.spell
WHERE (
    sqlc.narg('name')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('name')::text || '%'
)
AND (
    sqlc.narg('level')::int IS NULL
    OR level = sqlc.narg('level')::int
)
AND (
    sqlc.narg('school')::text IS NULL
    OR school = sqlc.narg('school')::text
);

-- name: QuerySpells :many
SELECT * FROM dnd.spell
WHERE (
    sqlc.narg('name')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('name')::text || '%'
)
AND (
    sqlc.narg('level')::int IS NULL
    OR level = sqlc.narg('level')::int
)
AND (
    sqlc.narg('school')::text IS NULL
    OR school = sqlc.narg('school')::text
)
ORDER BY name ASC
LIMIT $1 OFFSET $2;

-- name: CreateSpell :one
INSERT INTO dnd.spell (
    id,
    source_id,
    name,
    slug,
    dnd_version,
    dnd_version_year,
    source_page,
    level,
    school,
    is_ritual,
    is_unearthed_arcana,
    casting_time,
    range,
    has_verbal_component,
    has_somatic_component,
    has_material_component,
    materials,
    has_spell_cost,
    are_materials_consumed,
    duration,
    is_concentration,
    description,
    has_saving_throw,
    difficulty_class_saving_throw_override,
    damage_type,
    at_higher_levels,
    difficulty_class_saving_throw,
    difficulty_class_type,
    stat_blocks,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33
) RETURNING *;

-- name: SpellExistsByNameVersion :one
SELECT EXISTS(
    SELECT 1 FROM dnd.spell WHERE name = $1 AND dnd_version = $2 AND dnd_version_year = $3 AND source_id = $4
) AS exists;

-- name: GetSpellByUniqueKey :one
SELECT * FROM dnd.spell
WHERE name = $1 AND dnd_version = $2 AND dnd_version_year = $3 AND source_id = $4
LIMIT 1;
