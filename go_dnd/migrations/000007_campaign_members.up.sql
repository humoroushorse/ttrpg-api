-- Mirror of py_dnd Alembic revision 1bd38fdceea4 (campaign members, invites, character editors and
-- transfers, DM gifts). Never applied to the shared database: Alembic owns it. See README.md here.
CREATE TABLE dnd.campaign_member (
    campaign_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying DEFAULT 'player' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_campaign_member PRIMARY KEY (campaign_id, user_id),
    CONSTRAINT ck_campaign_member_role CHECK (role IN ('dm', 'player')),
    CONSTRAINT fk_campaign_member_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_member_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_member_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_member_user_id_user FOREIGN KEY (user_id) REFERENCES dnd."user"(id) ON DELETE CASCADE
);
CREATE INDEX ix_dnd_campaign_member_updated_at ON dnd.campaign_member USING btree (updated_at);
CREATE INDEX ix_dnd_campaign_member_user_id ON dnd.campaign_member USING btree (user_id);

CREATE TABLE dnd.campaign_invite (
    id uuid DEFAULT uuidv7() NOT NULL,
    campaign_id uuid NOT NULL,
    token_hash character varying,
    user_id uuid,
    expires_at timestamp with time zone NOT NULL,
    max_uses integer,
    uses integer DEFAULT 0 NOT NULL,
    revoked_at timestamp with time zone,
    revoked_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_campaign_invite PRIMARY KEY (id),
    CONSTRAINT uq_campaign_invite_token_hash UNIQUE (token_hash),
    CONSTRAINT ck_campaign_invite_link_or_user CHECK ((token_hash IS NULL) <> (user_id IS NULL)),
    CONSTRAINT ck_campaign_invite_max_uses CHECK (max_uses IS NULL OR max_uses > 0),
    CONSTRAINT fk_campaign_invite_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_invite_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_invite_revoked_by_user FOREIGN KEY (revoked_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_invite_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_invite_user_id_user FOREIGN KEY (user_id) REFERENCES dnd."user"(id) ON DELETE CASCADE
);
CREATE INDEX ix_dnd_campaign_invite_campaign_id ON dnd.campaign_invite USING btree (campaign_id);
CREATE INDEX ix_dnd_campaign_invite_updated_at ON dnd.campaign_invite USING btree (updated_at);
CREATE INDEX ix_dnd_campaign_invite_user_id ON dnd.campaign_invite USING btree (user_id);

CREATE TABLE dnd.character_editor (
    character_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_editor PRIMARY KEY (character_id, user_id),
    CONSTRAINT fk_character_editor_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_editor_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_editor_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_editor_user_id_user FOREIGN KEY (user_id) REFERENCES dnd."user"(id) ON DELETE CASCADE
);
CREATE INDEX ix_dnd_character_editor_updated_at ON dnd.character_editor USING btree (updated_at);
CREATE INDEX ix_dnd_character_editor_user_id ON dnd.character_editor USING btree (user_id);

CREATE TABLE dnd.character_transfer (
    character_id uuid NOT NULL,
    to_user_id uuid NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_transfer PRIMARY KEY (character_id),
    CONSTRAINT fk_character_transfer_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_transfer_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_transfer_to_user_id_user FOREIGN KEY (to_user_id) REFERENCES dnd."user"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_transfer_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_transfer_to_user_id ON dnd.character_transfer USING btree (to_user_id);
CREATE INDEX ix_dnd_character_transfer_updated_at ON dnd.character_transfer USING btree (updated_at);

ALTER TABLE dnd.character_item ADD COLUMN given_by uuid;
ALTER TABLE dnd.character_item ADD COLUMN given_campaign_id uuid;
ALTER TABLE dnd.character_item
    ADD CONSTRAINT fk_character_item_given_by_user FOREIGN KEY (given_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;
ALTER TABLE dnd.character_item
    ADD CONSTRAINT fk_character_item_given_campaign_id_campaign FOREIGN KEY (given_campaign_id) REFERENCES dnd.campaign(id) ON DELETE SET NULL;

ALTER TABLE dnd.character_modifier ADD COLUMN campaign_id uuid;
ALTER TABLE dnd.character_modifier
    ADD CONSTRAINT fk_character_modifier_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE;

-- each live campaign's owner is a DM
INSERT INTO dnd.campaign_member (campaign_id, user_id, role, created_by, updated_by)
SELECT id, owner_id, 'dm', owner_id, owner_id FROM dnd.campaign WHERE deleted_at IS NULL;
