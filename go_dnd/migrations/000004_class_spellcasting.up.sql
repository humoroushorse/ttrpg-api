-- Mirror of py_dnd Alembic revision 90555ce2fa37 (class spellcasting and level table). Never applied
-- to the shared database: Alembic owns it. See README.md here.
ALTER TABLE dnd.dnd_class ADD COLUMN spellcasting_ability character varying;
ALTER TABLE dnd.dnd_class ADD COLUMN caster_progression character varying;
ALTER TABLE dnd.dnd_class ADD COLUMN level_table jsonb DEFAULT '[]'::jsonb NOT NULL;
ALTER TABLE dnd.subclass ADD COLUMN spellcasting_ability character varying;
ALTER TABLE dnd.subclass ADD COLUMN caster_progression character varying;
ALTER TABLE dnd.subclass ADD COLUMN level_table jsonb DEFAULT '[]'::jsonb NOT NULL;
