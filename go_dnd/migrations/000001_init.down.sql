-- Drops the tables 000001_init.up.sql creates, children first. No CASCADE, so anything else that
-- points at them (Go-only VTT tables in dnd_vtt) makes this fail instead of being dropped with them.
DROP TABLE dnd.spell_class;
DROP TABLE dnd.background;
DROP TABLE dnd.condition;
DROP TABLE dnd.dnd_class;
DROP TABLE dnd.feat;
DROP TABLE dnd.glossary_entry;
DROP TABLE dnd.item;
DROP TABLE dnd.species;
DROP TABLE dnd.subclass;
DROP TABLE dnd.spell;
DROP TABLE dnd.source;
DROP TABLE dnd."user";
