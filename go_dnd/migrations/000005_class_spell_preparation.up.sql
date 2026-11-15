-- Mirror of py_dnd Alembic revision 27b78fda2df2 (class spell preparation). Never applied to the
-- shared database: Alembic owns it. See README.md here.
ALTER TABLE dnd.dnd_class ADD COLUMN spell_preparation jsonb;
ALTER TABLE dnd.subclass ADD COLUMN spell_preparation jsonb;
