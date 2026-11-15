-- Mirror of py_dnd Alembic revision 986a9d86d5a1 (catalog entry view). Never applied to the shared
-- database: Alembic owns it. See README.md here.

CREATE VIEW dnd.catalog_entry AS
SELECT 'class'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.dnd_class
UNION ALL
SELECT 'subclass'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.subclass
UNION ALL
SELECT 'species'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.species
UNION ALL
SELECT 'background'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.background
UNION ALL
SELECT 'feat'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.feat
UNION ALL
SELECT 'spell'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.spell
UNION ALL
SELECT 'item'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id FROM dnd.item;
