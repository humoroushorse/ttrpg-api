-- Mirror of py_dnd Alembic revision 31ef9eb3cb82 (campaigns and characters). Never applied to the
-- shared database: Alembic owns it. See README.md here. Hand-written from the revision;
-- `make schema-check` proves it builds the same schema.

CREATE TABLE dnd.campaign (
    id uuid DEFAULT uuidv7() NOT NULL,
    owner_id uuid NOT NULL,
    name character varying NOT NULL,
    description character varying DEFAULT '' NOT NULL,
    rules jsonb DEFAULT '{}'::jsonb NOT NULL,
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid,
    CONSTRAINT pk_campaign PRIMARY KEY (id),
    CONSTRAINT fk_campaign_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_owner_id_user FOREIGN KEY (owner_id) REFERENCES dnd."user"(id) ON DELETE RESTRICT,
    CONSTRAINT fk_campaign_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_campaign_deleted_at ON dnd.campaign USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);
CREATE INDEX ix_dnd_campaign_owner_id ON dnd.campaign USING btree (owner_id);
CREATE INDEX ix_dnd_campaign_updated_at ON dnd.campaign USING btree (updated_at);

CREATE TABLE dnd."character" (
    id uuid DEFAULT uuidv7() NOT NULL,
    owner_id uuid NOT NULL,
    name character varying NOT NULL,
    current_level integer DEFAULT 1 NOT NULL,
    rules jsonb,
    ability_method character varying DEFAULT 'point_buy' NOT NULL,
    base_scores jsonb DEFAULT '{}'::jsonb NOT NULL,
    ability_rolls jsonb DEFAULT '[]'::jsonb NOT NULL,
    ability_roll_count integer DEFAULT 0 NOT NULL,
    currency jsonb DEFAULT '{}'::jsonb NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    version integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid,
    CONSTRAINT pk_character PRIMARY KEY (id),
    CONSTRAINT ck_character_ability_method CHECK (ability_method IN ('point_buy', 'standard_array', 'rolled', 'manual')),
    CONSTRAINT ck_character_current_level CHECK (current_level BETWEEN 1 AND 20),
    CONSTRAINT fk_character_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_owner_id_user FOREIGN KEY (owner_id) REFERENCES dnd."user"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_deleted_at ON dnd."character" USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);
CREATE INDEX ix_dnd_character_owner_id ON dnd."character" USING btree (owner_id);
CREATE INDEX ix_dnd_character_updated_at ON dnd."character" USING btree (updated_at);

CREATE TABLE dnd.campaign_character (
    campaign_id uuid NOT NULL,
    character_id uuid NOT NULL,
    approved_exceptions jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_campaign_character PRIMARY KEY (campaign_id, character_id),
    CONSTRAINT fk_campaign_character_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_character_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_character_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_character_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_campaign_character_character_id ON dnd.campaign_character USING btree (character_id);
CREATE INDEX ix_dnd_campaign_character_updated_at ON dnd.campaign_character USING btree (updated_at);

CREATE TABLE dnd.character_change (
    id uuid DEFAULT uuidv7() NOT NULL,
    character_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    action character varying NOT NULL,
    entity character varying,
    entity_id uuid,
    "before" jsonb,
    "after" jsonb,
    note character varying,
    op_id uuid,
    CONSTRAINT pk_character_change PRIMARY KEY (id),
    CONSTRAINT uq_character_change_op_id UNIQUE (op_id),
    CONSTRAINT fk_character_change_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_change_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_change_character_id_created_at ON dnd.character_change USING btree (character_id, created_at);

CREATE TABLE dnd.character_modifier (
    id uuid DEFAULT uuidv7() NOT NULL,
    character_id uuid NOT NULL,
    label character varying NOT NULL,
    note character varying,
    target character varying NOT NULL,
    op character varying NOT NULL,
    value jsonb NOT NULL,
    from_level integer,
    active boolean DEFAULT true NOT NULL,
    added_as character varying DEFAULT 'player' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_modifier PRIMARY KEY (id),
    CONSTRAINT ck_character_modifier_added_as CHECK (added_as IN ('player', 'dm')),
    CONSTRAINT fk_character_modifier_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_modifier_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_modifier_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_modifier_character_id ON dnd.character_modifier USING btree (character_id);
CREATE INDEX ix_dnd_character_modifier_updated_at ON dnd.character_modifier USING btree (updated_at);

CREATE TABLE dnd.character_choice (
    id uuid DEFAULT uuidv7() NOT NULL,
    character_id uuid NOT NULL,
    level integer NOT NULL,
    kind character varying NOT NULL,
    slot character varying NOT NULL,
    granted_by_id uuid,
    species_id uuid,
    subclass_id uuid,
    background_id uuid,
    feat_id uuid,
    spell_id uuid,
    item_id uuid,
    value jsonb,
    overrides_filter boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_choice PRIMARY KEY (id),
    CONSTRAINT ck_character_choice_level CHECK (level BETWEEN 1 AND 20),
    CONSTRAINT ck_character_choice_one_pick CHECK (num_nonnulls(species_id, subclass_id, background_id, feat_id, spell_id, item_id) <= 1),
    CONSTRAINT fk_character_choice_background_id_background FOREIGN KEY (background_id) REFERENCES dnd.background(id),
    CONSTRAINT fk_character_choice_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_choice_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_choice_feat_id_feat FOREIGN KEY (feat_id) REFERENCES dnd.feat(id),
    CONSTRAINT fk_character_choice_granted_by_id_character_choice FOREIGN KEY (granted_by_id) REFERENCES dnd.character_choice(id),
    CONSTRAINT fk_character_choice_item_id_item FOREIGN KEY (item_id) REFERENCES dnd.item(id),
    CONSTRAINT fk_character_choice_species_id_species FOREIGN KEY (species_id) REFERENCES dnd.species(id),
    CONSTRAINT fk_character_choice_spell_id_spell FOREIGN KEY (spell_id) REFERENCES dnd.spell(id),
    CONSTRAINT fk_character_choice_subclass_id_subclass FOREIGN KEY (subclass_id) REFERENCES dnd.subclass(id),
    CONSTRAINT fk_character_choice_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_choice_character_id ON dnd.character_choice USING btree (character_id);
CREATE INDEX ix_dnd_character_choice_updated_at ON dnd.character_choice USING btree (updated_at);

CREATE TABLE dnd.character_level (
    id uuid DEFAULT uuidv7() NOT NULL,
    character_id uuid NOT NULL,
    level integer NOT NULL,
    class_id uuid NOT NULL,
    hp_method character varying DEFAULT 'average' NOT NULL,
    hp_value integer,
    hp_rolls jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_level PRIMARY KEY (id),
    CONSTRAINT uq_character_level_character_id_level UNIQUE (character_id, level),
    CONSTRAINT ck_character_level_hp_method CHECK (hp_method IN ('average', 'rolled', 'manual')),
    CONSTRAINT ck_character_level_level CHECK (level BETWEEN 1 AND 20),
    CONSTRAINT fk_character_level_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_level_class_id_dnd_class FOREIGN KEY (class_id) REFERENCES dnd.dnd_class(id),
    CONSTRAINT fk_character_level_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_level_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_level_updated_at ON dnd.character_level USING btree (updated_at);

CREATE TABLE dnd.character_prepared_spell (
    character_id uuid NOT NULL,
    spell_id uuid NOT NULL,
    class_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_prepared_spell PRIMARY KEY (character_id, spell_id, class_id),
    CONSTRAINT fk_character_prepared_spell_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_prepared_spell_class_id_dnd_class FOREIGN KEY (class_id) REFERENCES dnd.dnd_class(id),
    CONSTRAINT fk_character_prepared_spell_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_prepared_spell_spell_id_spell FOREIGN KEY (spell_id) REFERENCES dnd.spell(id),
    CONSTRAINT fk_character_prepared_spell_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_prepared_spell_updated_at ON dnd.character_prepared_spell USING btree (updated_at);

CREATE TABLE dnd.character_item (
    id uuid DEFAULT uuidv7() NOT NULL,
    character_id uuid NOT NULL,
    item_id uuid,
    custom_name character varying,
    custom_description character varying,
    quantity integer DEFAULT 1 NOT NULL,
    equipped boolean DEFAULT false NOT NULL,
    attuned boolean DEFAULT false NOT NULL,
    level_received integer,
    note character varying,
    granted_by_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_character_item PRIMARY KEY (id),
    CONSTRAINT ck_character_item_item_or_custom CHECK (item_id IS NOT NULL OR custom_name IS NOT NULL),
    CONSTRAINT ck_character_item_quantity CHECK (quantity >= 0),
    CONSTRAINT fk_character_item_character_id_character FOREIGN KEY (character_id) REFERENCES dnd."character"(id) ON DELETE CASCADE,
    CONSTRAINT fk_character_item_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_item_granted_by_id_character_choice FOREIGN KEY (granted_by_id) REFERENCES dnd.character_choice(id) ON DELETE SET NULL,
    CONSTRAINT fk_character_item_item_id_item FOREIGN KEY (item_id) REFERENCES dnd.item(id),
    CONSTRAINT fk_character_item_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_character_item_character_id ON dnd.character_item USING btree (character_id);
CREATE INDEX ix_dnd_character_item_updated_at ON dnd.character_item USING btree (updated_at);
