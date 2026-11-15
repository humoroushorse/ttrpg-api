-- Drops what 000001_tabletop.up.sql creates, children first. The schema stays: it holds this
-- directory's version table (dnd_vtt.schema_migrations).
DROP TABLE dnd_vtt.event;
DROP TABLE dnd_vtt.scene_token;
DROP TABLE dnd_vtt.campaign_state;
DROP TABLE dnd_vtt.scene;
