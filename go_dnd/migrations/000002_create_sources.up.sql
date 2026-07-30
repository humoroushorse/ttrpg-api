CREATE TABLE IF NOT EXISTS dnd.source (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    name_short TEXT NOT NULL,
    publish_year INT,
    dnd_version TEXT NOT NULL,
    dnd_version_year INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    CONSTRAINT uq_source_name_version UNIQUE (name, dnd_version, dnd_version_year),
    CONSTRAINT uq_source_name_short_version UNIQUE (name_short, dnd_version, dnd_version_year)
);

CREATE INDEX IF NOT EXISTS idx_source_dnd_version ON dnd.source(dnd_version);
