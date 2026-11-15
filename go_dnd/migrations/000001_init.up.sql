-- Mirror of py_dnd Alembic revision 42fc01dee024 (init), from `scripts/dnd_schema.sh dump`.
-- Never applied to the shared database: Alembic owns it. See README.md here.

CREATE SCHEMA IF NOT EXISTS dnd;

CREATE TABLE dnd.background (
    ability_score_bonuses character varying DEFAULT ''::character varying NOT NULL,
    skill_proficiencies text[] DEFAULT '{}'::text[] NOT NULL,
    tool_proficiencies text[] DEFAULT '{}'::text[] NOT NULL,
    feat character varying DEFAULT ''::character varying NOT NULL,
    equipment_text character varying DEFAULT ''::character varying NOT NULL,
    equipment_options jsonb DEFAULT '[]'::jsonb NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.condition (
    levels jsonb DEFAULT '[]'::jsonb NOT NULL,
    effects jsonb DEFAULT '[]'::jsonb NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.dnd_class (
    hit_die character varying NOT NULL,
    hit_die_size integer,
    primary_ability character varying DEFAULT ''::character varying NOT NULL,
    proficiencies jsonb DEFAULT '{}'::jsonb NOT NULL,
    starting_equipment character varying DEFAULT ''::character varying NOT NULL,
    features jsonb DEFAULT '[]'::jsonb NOT NULL,
    subclass_slugs text[] DEFAULT '{}'::text[] NOT NULL,
    spell_list character varying,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.feat (
    category character varying,
    prerequisites jsonb,
    effects jsonb DEFAULT '[]'::jsonb NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.glossary_entry (
    category character varying DEFAULT 'general'::character varying NOT NULL,
    levels jsonb DEFAULT '[]'::jsonb NOT NULL,
    effects jsonb DEFAULT '[]'::jsonb NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.item (
    category character varying,
    subcategory character varying,
    cost character varying,
    weight character varying,
    properties jsonb DEFAULT '{}'::jsonb NOT NULL,
    effects jsonb DEFAULT '[]'::jsonb NOT NULL,
    is_magic boolean DEFAULT false NOT NULL,
    magic_item_type character varying,
    base_item character varying,
    rarity character varying,
    rarity_text character varying,
    requires_attunement boolean DEFAULT false NOT NULL,
    attunement_requirement character varying,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.source (
    id uuid DEFAULT uuidv7() NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    name_short character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    publish_year integer,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.species (
    size character varying NOT NULL,
    speed character varying NOT NULL,
    creature_type character varying NOT NULL,
    traits jsonb DEFAULT '[]'::jsonb NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd.spell (
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    source_page integer,
    level integer NOT NULL,
    school character varying NOT NULL,
    is_ritual boolean NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    casting_time character varying NOT NULL,
    range character varying NOT NULL,
    has_verbal_component boolean NOT NULL,
    has_somatic_component boolean NOT NULL,
    has_material_component boolean NOT NULL,
    materials character varying,
    has_spell_cost boolean NOT NULL,
    are_materials_consumed boolean NOT NULL,
    duration character varying NOT NULL,
    is_concentration boolean NOT NULL,
    description character varying NOT NULL,
    has_saving_throw boolean NOT NULL,
    difficulty_class_saving_throw_override integer,
    damage_type character varying,
    at_higher_levels character varying,
    difficulty_class_saving_throw character varying,
    difficulty_class_type character varying,
    stat_blocks jsonb,
    is_homebrew boolean DEFAULT false NOT NULL,
    vtt jsonb NOT NULL,
    activation_cost character varying GENERATED ALWAYS AS ((vtt #>> '{modes,0,activation,cost}'::text[])) STORED,
    activation_amount integer GENERATED ALWAYS AS (COALESCE(((vtt #>> '{modes,0,activation,amount}'::text[]))::integer, 1)) STORED,
    range_kind character varying GENERATED ALWAYS AS ((vtt #>> '{range,kind}'::text[])) STORED,
    range_feet integer GENERATED ALWAYS AS (((vtt #>> '{range,feet}'::text[]))::integer) STORED,
    duration_kind character varying GENERATED ALWAYS AS ((vtt #>> '{duration,kind}'::text[])) STORED,
    duration_amount integer GENERATED ALWAYS AS (((vtt #>> '{duration,amount}'::text[]))::integer) STORED,
    duration_unit character varying GENERATED ALWAYS AS ((vtt #>> '{duration,unit}'::text[])) STORED,
    sustain character varying GENERATED ALWAYS AS ((vtt ->> 'sustain'::text)) STORED,
    has_attack_roll boolean GENERATED ALWAYS AS (((vtt @? '$."modes"[*]."rolls"[*]?(@."roll" == "attack" || @."roll" == "weapon_attack")'::jsonpath) OR (vtt @? '$."modes"[*]."ongoing"[*]."rolls"[*]?(@."roll" == "attack" || @."roll" == "weapon_attack")'::jsonpath))) STORED,
    has_save boolean GENERATED ALWAYS AS (((vtt @? '$."modes"[*]."rolls"[*]?(@."roll" == "save")'::jsonpath) OR (vtt @? '$."modes"[*]."ongoing"[*]."rolls"[*]?(@."roll" == "save")'::jsonpath))) STORED,
    has_summon boolean GENERATED ALWAYS AS ((vtt @? '$."modes"[*]."summons"[*]'::jsonpath)) STORED,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid,
    CONSTRAINT ck_spell_vtt_v1 CHECK (((jsonb_typeof(vtt) = 'object'::text) AND ((vtt ->> 'schema_version'::text) = 'v1'::text)))
);

CREATE TABLE dnd.spell_class (
    spell_id uuid NOT NULL,
    class_id uuid NOT NULL,
    qualifier character varying,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid
);

CREATE TABLE dnd.subclass (
    parent_class_slug character varying NOT NULL,
    level_gained integer DEFAULT 0 NOT NULL,
    features jsonb DEFAULT '[]'::jsonb NOT NULL,
    spell_list_additions text[] DEFAULT '{}'::text[] NOT NULL,
    id uuid DEFAULT uuidv7() NOT NULL,
    source_id uuid NOT NULL,
    slug character varying NOT NULL,
    name character varying NOT NULL,
    dnd_version character varying NOT NULL,
    dnd_version_year integer NOT NULL,
    is_unearthed_arcana boolean DEFAULT false NOT NULL,
    is_homebrew boolean DEFAULT false NOT NULL,
    marks text[] DEFAULT '{}'::text[] NOT NULL,
    description character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

CREATE TABLE dnd."user" (
    id uuid DEFAULT uuidv7() NOT NULL,
    keycloak_id uuid NOT NULL,
    username character varying,
    profile_picture_url character varying,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    deleted_at timestamp with time zone,
    deleted_by uuid
);

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT pk_background PRIMARY KEY (id);

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT pk_condition PRIMARY KEY (id);

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT pk_dnd_class PRIMARY KEY (id);

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT pk_feat PRIMARY KEY (id);

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT pk_glossary_entry PRIMARY KEY (id);

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT pk_item PRIMARY KEY (id);

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT pk_source PRIMARY KEY (id);

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT pk_species PRIMARY KEY (id);

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT pk_spell PRIMARY KEY (id);

ALTER TABLE ONLY dnd.spell_class
    ADD CONSTRAINT pk_spell_class PRIMARY KEY (spell_id, class_id);

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT pk_subclass PRIMARY KEY (id);

ALTER TABLE ONLY dnd."user"
    ADD CONSTRAINT pk_user PRIMARY KEY (id);

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT uq_background_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT uq_condition_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT uq_dnd_class_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT uq_feat_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT uq_glossary_entry_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT uq_item_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT uq_source_name_short_version UNIQUE (name_short, dnd_version, dnd_version_year);

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT uq_source_name_version UNIQUE (name, dnd_version, dnd_version_year);

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT uq_source_slug UNIQUE (slug);

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT uq_species_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT uq_spell_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT uq_subclass_source_id_slug UNIQUE (source_id, slug);

ALTER TABLE ONLY dnd."user"
    ADD CONSTRAINT uq_user_keycloak_id UNIQUE (keycloak_id);

CREATE INDEX ix_dnd_background_deleted_at ON dnd.background USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_background_updated_at ON dnd.background USING btree (updated_at);

CREATE INDEX ix_dnd_condition_deleted_at ON dnd.condition USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_condition_updated_at ON dnd.condition USING btree (updated_at);

CREATE INDEX ix_dnd_dnd_class_deleted_at ON dnd.dnd_class USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_dnd_class_updated_at ON dnd.dnd_class USING btree (updated_at);

CREATE INDEX ix_dnd_feat_deleted_at ON dnd.feat USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_feat_updated_at ON dnd.feat USING btree (updated_at);

CREATE INDEX ix_dnd_glossary_entry_deleted_at ON dnd.glossary_entry USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_glossary_entry_updated_at ON dnd.glossary_entry USING btree (updated_at);

CREATE INDEX ix_dnd_item_deleted_at ON dnd.item USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_item_updated_at ON dnd.item USING btree (updated_at);

CREATE INDEX ix_dnd_source_deleted_at ON dnd.source USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_source_updated_at ON dnd.source USING btree (updated_at);

CREATE INDEX ix_dnd_species_deleted_at ON dnd.species USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_species_updated_at ON dnd.species USING btree (updated_at);

CREATE INDEX ix_dnd_spell_class_class_id ON dnd.spell_class USING btree (class_id);

CREATE INDEX ix_dnd_spell_class_updated_at ON dnd.spell_class USING btree (updated_at);

CREATE INDEX ix_dnd_spell_deleted_at ON dnd.spell USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_spell_updated_at ON dnd.spell USING btree (updated_at);

CREATE INDEX ix_dnd_subclass_deleted_at ON dnd.subclass USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_subclass_updated_at ON dnd.subclass USING btree (updated_at);

CREATE INDEX ix_dnd_user_deleted_at ON dnd."user" USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

CREATE INDEX ix_dnd_user_updated_at ON dnd."user" USING btree (updated_at);

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT fk_background_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT fk_background_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT fk_background_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.background
    ADD CONSTRAINT fk_background_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT fk_condition_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT fk_condition_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT fk_condition_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.condition
    ADD CONSTRAINT fk_condition_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT fk_dnd_class_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT fk_dnd_class_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT fk_dnd_class_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.dnd_class
    ADD CONSTRAINT fk_dnd_class_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT fk_feat_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT fk_feat_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT fk_feat_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.feat
    ADD CONSTRAINT fk_feat_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT fk_glossary_entry_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT fk_glossary_entry_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT fk_glossary_entry_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.glossary_entry
    ADD CONSTRAINT fk_glossary_entry_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT fk_item_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT fk_item_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT fk_item_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.item
    ADD CONSTRAINT fk_item_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT fk_source_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT fk_source_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.source
    ADD CONSTRAINT fk_source_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT fk_species_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT fk_species_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT fk_species_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.species
    ADD CONSTRAINT fk_species_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.spell_class
    ADD CONSTRAINT fk_spell_class_class_id_dnd_class FOREIGN KEY (class_id) REFERENCES dnd.dnd_class(id) ON DELETE CASCADE;

ALTER TABLE ONLY dnd.spell_class
    ADD CONSTRAINT fk_spell_class_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.spell_class
    ADD CONSTRAINT fk_spell_class_spell_id_spell FOREIGN KEY (spell_id) REFERENCES dnd.spell(id) ON DELETE CASCADE;

ALTER TABLE ONLY dnd.spell_class
    ADD CONSTRAINT fk_spell_class_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT fk_spell_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT fk_spell_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT fk_spell_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.spell
    ADD CONSTRAINT fk_spell_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT fk_subclass_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT fk_subclass_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT fk_subclass_source_id_source FOREIGN KEY (source_id) REFERENCES dnd.source(id);

ALTER TABLE ONLY dnd.subclass
    ADD CONSTRAINT fk_subclass_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd."user"
    ADD CONSTRAINT fk_user_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd."user"
    ADD CONSTRAINT fk_user_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

ALTER TABLE ONLY dnd."user"
    ADD CONSTRAINT fk_user_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL;

