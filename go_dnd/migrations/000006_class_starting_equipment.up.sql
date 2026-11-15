-- Mirror of py_dnd Alembic revision 5c1e8a7d3b20 (class starting equipment options). Never applied to
-- the shared database: Alembic owns it. See README.md here.
ALTER TABLE dnd.dnd_class ADD COLUMN starting_equipment_options jsonb DEFAULT '[]'::jsonb NOT NULL;
