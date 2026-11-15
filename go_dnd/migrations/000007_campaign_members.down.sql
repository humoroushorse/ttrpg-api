ALTER TABLE dnd.character_modifier DROP COLUMN campaign_id;
ALTER TABLE dnd.character_item DROP COLUMN given_campaign_id;
ALTER TABLE dnd.character_item DROP COLUMN given_by;
DROP TABLE dnd.character_transfer;
DROP TABLE dnd.character_editor;
DROP TABLE dnd.campaign_invite;
DROP TABLE dnd.campaign_member;
