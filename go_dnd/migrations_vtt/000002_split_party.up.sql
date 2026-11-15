-- Split party, and what players see of a token's disposition.

-- A player's place at the table when it isn't the campaign's active scene. No row, or a null scene_id:
-- they follow campaign_state.active_scene_id. A player sees, and is sent events for, only their
-- current scene; DMs open any scene. Each scene keeps its own tokens, so a character's token stays
-- where it was left in every scene, and moving it in one scene doesn't touch the others.
CREATE TABLE dnd_vtt.member_state (
    campaign_id uuid NOT NULL,
    user_id uuid NOT NULL,
    scene_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_member_state PRIMARY KEY (campaign_id, user_id),
    -- leaving the campaign drops it
    CONSTRAINT fk_member_state_member FOREIGN KEY (campaign_id, user_id) REFERENCES dnd.campaign_member(campaign_id, user_id) ON DELETE CASCADE,
    -- a purged scene sends them back to the active one
    CONSTRAINT fk_member_state_scene FOREIGN KEY (campaign_id, scene_id) REFERENCES dnd_vtt.scene(campaign_id, id) ON DELETE SET NULL (scene_id),
    CONSTRAINT fk_member_state_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_member_state_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
-- who's in a scene, and the foreign key from scene
CREATE INDEX ix_dnd_vtt_member_state_campaign_id_scene_id ON dnd_vtt.member_state USING btree (campaign_id, scene_id) WHERE (scene_id IS NOT NULL);

-- What players see instead of the disposition, like player_name for the name: a doppelganger can look
-- friendly, a monster can look neutral. Null: players see the disposition.
ALTER TABLE dnd_vtt.scene_token ADD COLUMN player_disposition character varying;
ALTER TABLE dnd_vtt.scene_token
    ADD CONSTRAINT ck_scene_token_player_disposition CHECK (player_disposition IN ('party', 'friendly', 'neutral', 'hostile'));
