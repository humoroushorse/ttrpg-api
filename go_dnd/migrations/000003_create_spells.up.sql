CREATE TYPE dnd.spell_school AS ENUM (
    'abjuration',
    'alteration',
    'conjuration',
    'divination',
    'enchantment',
    'evocation',
    'transmutation',
    'illusion',
    'invocation',
    'necromancy'
);

CREATE TABLE IF NOT EXISTS dnd.spell (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL REFERENCES dnd.source(id),
    name TEXT NOT NULL,
    slug TEXT,
    dnd_version TEXT NOT NULL,
    dnd_version_year INT NOT NULL,
    source_page INT,
    level INT NOT NULL CHECK (level >= 0 AND level <= 9),
    school dnd.spell_school NOT NULL,
    is_ritual BOOLEAN NOT NULL DEFAULT FALSE,
    is_unearthed_arcana BOOLEAN NOT NULL DEFAULT FALSE,
    casting_time TEXT NOT NULL,
    range TEXT NOT NULL,
    has_verbal_component BOOLEAN NOT NULL DEFAULT FALSE,
    has_somatic_component BOOLEAN NOT NULL DEFAULT FALSE,
    has_material_component BOOLEAN NOT NULL DEFAULT FALSE,
    materials TEXT,
    has_spell_cost BOOLEAN NOT NULL DEFAULT FALSE,
    are_materials_consumed BOOLEAN NOT NULL DEFAULT FALSE,
    duration TEXT NOT NULL,
    is_concentration BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL,
    has_saving_throw BOOLEAN NOT NULL DEFAULT FALSE,
    difficulty_class_saving_throw_override INT,
    damage_type TEXT,
    at_higher_levels TEXT,
    difficulty_class_saving_throw TEXT,
    difficulty_class_type TEXT,
    stat_blocks JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_spell_level ON dnd.spell(level);
CREATE INDEX IF NOT EXISTS idx_spell_school ON dnd.spell(school);
CREATE INDEX IF NOT EXISTS idx_spell_source_id ON dnd.spell(source_id);
CREATE INDEX IF NOT EXISTS idx_spell_name ON dnd.spell(name);
CREATE UNIQUE INDEX IF NOT EXISTS uq_spell_slug_version ON dnd.spell(slug, dnd_version, dnd_version_year) WHERE slug IS NOT NULL;
