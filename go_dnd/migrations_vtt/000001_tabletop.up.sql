-- The tabletop's first tables: scenes, tokens, the campaign's event log and its counter. Go-only, so
-- they live in schema dnd_vtt, which Alembic never sees (README.md here). They point at the shared
-- dnd tables across schemas.
CREATE SCHEMA IF NOT EXISTS dnd_vtt;

-- A map, or a group of maps, or both. Scenes form a tree per campaign (the world map, a town in it, a
-- building in the town, each floor of the building), so parent_id is just organization: any scene can
-- have a background and children. The API rejects a move that would make a cycle.
CREATE TABLE dnd_vtt.scene (
    id uuid DEFAULT uuidv7() NOT NULL,
    campaign_id uuid NOT NULL,
    parent_id uuid,
    -- order among siblings
    sort_order integer DEFAULT 0 NOT NULL,
    name character varying NOT NULL,
    -- the map image, as copies in preference order: [{url, type}], e.g. avif, then webp, then png. The
    -- client loads the first one the browser can decode. Empty: a blank grid. The API validates it
    background_sources jsonb DEFAULT '[]'::jsonb NOT NULL,
    -- the grid. Token positions are in cells, so a sharper copy of the same map only changes cell_px.
    -- square or none (gridless: the scale still applies to measuring); hex later
    grid_type character varying DEFAULT 'square' NOT NULL,
    -- the playable area
    width_cells integer NOT NULL,
    height_cells integer NOT NULL,
    -- image pixels per cell, and where the first cell starts on the image (to line up a drawn grid)
    cell_px double precision NOT NULL,
    grid_offset_x double precision DEFAULT 0 NOT NULL,
    grid_offset_y double precision DEFAULT 0 NOT NULL,
    -- what one cell is worth: 5 ft on a battle map, 1 mi on an overland map
    grid_distance double precision DEFAULT 5 NOT NULL,
    grid_units character varying DEFAULT 'ft' NOT NULL,
    -- presentation only (grid color and opacity, ...); the API validates it
    settings jsonb DEFAULT '{}'::jsonb NOT NULL,
    -- the last token number handed out here, so a removed #3 isn't reused by the next token
    last_token_number integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    -- in the trash. A subtree deleted together shares one deleted_at, so it's restored together
    deleted_at timestamp with time zone,
    deleted_by uuid,
    CONSTRAINT pk_scene PRIMARY KEY (id),
    -- the target of the same-campaign foreign keys below
    CONSTRAINT uq_scene_campaign_id_id UNIQUE (campaign_id, id),
    CONSTRAINT ck_scene_grid_type CHECK (grid_type IN ('square', 'none')),
    CONSTRAINT ck_scene_size CHECK (width_cells > 0 AND height_cells > 0),
    CONSTRAINT ck_scene_cell_px CHECK (cell_px > 0),
    CONSTRAINT ck_scene_grid_distance CHECK (grid_distance > 0),
    CONSTRAINT ck_scene_not_own_parent CHECK (parent_id <> id),
    CONSTRAINT ck_scene_last_token_number CHECK (last_token_number >= 0),
    CONSTRAINT ck_scene_background_sources CHECK (jsonb_typeof(background_sources) = 'array'),
    CONSTRAINT fk_scene_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    -- a parent in the same campaign. Only the trash purge hard-deletes, and it takes the subtree
    CONSTRAINT fk_scene_parent FOREIGN KEY (campaign_id, parent_id) REFERENCES dnd_vtt.scene(campaign_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_scene_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_scene_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_scene_deleted_by_user FOREIGN KEY (deleted_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_vtt_scene_campaign_id_parent_id ON dnd_vtt.scene USING btree (campaign_id, parent_id, sort_order);
CREATE INDEX ix_dnd_vtt_scene_deleted_at ON dnd_vtt.scene USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);

-- The campaign's tabletop state, one row per campaign: which scene players are on, and the event
-- counter. Every command's transaction locks this row and bumps last_seq, so events commit in seq
-- order with no gaps. fillfactor and no index on last_seq keep that update a HOT update.
CREATE TABLE dnd_vtt.campaign_state (
    campaign_id uuid NOT NULL,
    active_scene_id uuid,
    last_seq bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_campaign_state PRIMARY KEY (campaign_id),
    CONSTRAINT ck_campaign_state_last_seq CHECK (last_seq >= 0),
    CONSTRAINT fk_campaign_state_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    -- a scene of this campaign; the API won't trash the active scene, and a purge only clears the column
    CONSTRAINT fk_campaign_state_active_scene FOREIGN KEY (campaign_id, active_scene_id) REFERENCES dnd_vtt.scene(campaign_id, id) ON DELETE SET NULL (active_scene_id),
    CONSTRAINT fk_campaign_state_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_campaign_state_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
) WITH (fillfactor = 70);

-- Something on a scene: a character, or a marker the DM places (an NPC, a monster until monster
-- instances exist, a summon, an object). Positions and sizes are in cells and may be fractions: a
-- snapped token has whole numbers, a Tiny one is 0.5.
--
-- On a character token, a null name or image_url means "from the sheet", so renames and new portraits
-- follow. The DM can still set either (a disguise).
CREATE TABLE dnd_vtt.scene_token (
    id uuid DEFAULT uuidv7() NOT NULL,
    campaign_id uuid NOT NULL,
    scene_id uuid NOT NULL,
    -- character or marker; monster later
    kind character varying NOT NULL,
    -- the character this token is (its sheet, HP, who may move it)
    character_id uuid,
    -- a marker its player moves: a familiar, a summon, an illusory double. Whoever can edit this
    -- character may move it
    controller_character_id uuid,
    -- the true name, which DMs see
    name character varying,
    -- what players see instead of the name ("Hooded figure", "Wolf"); null: players see the name.
    -- The API never sends the name to players when this is set
    player_name character varying,
    -- "#3" on the token, so players can say "I attack #3". Unique in the scene; new ones come from
    -- scene.last_token_number
    number integer,
    -- the ring color: party, friendly, neutral, hostile
    disposition character varying DEFAULT 'neutral' NOT NULL,
    image_url character varying,
    -- top-left corner, in cells from the grid's origin
    x double precision NOT NULL,
    y double precision NOT NULL,
    -- footprint in cells: 0.5 Tiny, 1 Small or Medium, 2 Large, 3 Huge, 4 Gargantuan
    size_cells double precision DEFAULT 1 NOT NULL,
    -- tokens, or dm (players are never sent it)
    layer character varying DEFAULT 'tokens' NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    CONSTRAINT pk_scene_token PRIMARY KEY (id),
    -- a character is in one place per scene; copies of it are markers
    CONSTRAINT uq_scene_token_scene_id_character_id UNIQUE (scene_id, character_id),
    CONSTRAINT uq_scene_token_scene_id_number UNIQUE (scene_id, number),
    CONSTRAINT ck_scene_token_kind CHECK (kind IN ('character', 'marker')),
    CONSTRAINT ck_scene_token_character CHECK ((kind = 'character') = (character_id IS NOT NULL)),
    CONSTRAINT ck_scene_token_controller CHECK (controller_character_id IS NULL OR kind <> 'character'),
    CONSTRAINT ck_scene_token_name CHECK (kind = 'character' OR name IS NOT NULL),
    CONSTRAINT ck_scene_token_number CHECK (number > 0),
    CONSTRAINT ck_scene_token_disposition CHECK (disposition IN ('party', 'friendly', 'neutral', 'hostile')),
    CONSTRAINT ck_scene_token_size CHECK (size_cells > 0),
    CONSTRAINT ck_scene_token_layer CHECK (layer IN ('tokens', 'dm')),
    CONSTRAINT fk_scene_token_scene FOREIGN KEY (campaign_id, scene_id) REFERENCES dnd_vtt.scene(campaign_id, id) ON DELETE CASCADE,
    -- the character must be in this campaign; leaving it, or being deleted, removes the token. The API
    -- logs token.removed first, so open tables hear about it
    CONSTRAINT fk_scene_token_character FOREIGN KEY (campaign_id, character_id) REFERENCES dnd.campaign_character(campaign_id, character_id) ON DELETE CASCADE,
    CONSTRAINT fk_scene_token_controller_character FOREIGN KEY (campaign_id, controller_character_id) REFERENCES dnd.campaign_character(campaign_id, character_id) ON DELETE SET NULL (controller_character_id),
    CONSTRAINT fk_scene_token_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL,
    CONSTRAINT fk_scene_token_updated_by_user FOREIGN KEY (updated_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
) WITH (fillfactor = 80);
-- for the foreign keys to campaign_character (a character leaving or being deleted). Nothing indexes
-- x, y or updated_at, so a move stays a HOT update
CREATE INDEX ix_dnd_vtt_scene_token_campaign_id_character_id ON dnd_vtt.scene_token USING btree (campaign_id, character_id) WHERE (character_id IS NOT NULL);
CREATE INDEX ix_dnd_vtt_scene_token_campaign_id_controller ON dnd_vtt.scene_token USING btree (campaign_id, controller_character_id) WHERE (controller_character_id IS NOT NULL);

-- The campaign's log: one row per saved change at the table (scenes, tokens; later walls, drawings,
-- effects, initiative). Append-only, so it has created_* but no updated_*. Pings and drag previews
-- aren't saved. seq is per campaign with no gaps; tabs replay from the last seq they saw.
CREATE TABLE dnd_vtt.event (
    campaign_id uuid NOT NULL,
    seq bigint NOT NULL,
    -- the scene it happened on; null for campaign events. No foreign key: the log outlives a purged scene
    scene_id uuid,
    -- token.moved, token.added, scene.activated, ...
    type character varying NOT NULL,
    -- all or dms. Each recipient's copy is still built per viewer (a token's true name, later HP)
    audience character varying NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    -- the client's id for the command, so a retried request isn't applied twice
    command_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    -- who did it
    created_by uuid,
    CONSTRAINT pk_event PRIMARY KEY (campaign_id, seq),
    CONSTRAINT ck_event_seq CHECK (seq > 0),
    CONSTRAINT ck_event_audience CHECK (audience IN ('all', 'dms')),
    CONSTRAINT fk_event_campaign_id_campaign FOREIGN KEY (campaign_id) REFERENCES dnd.campaign(id) ON DELETE CASCADE,
    CONSTRAINT fk_event_created_by_user FOREIGN KEY (created_by) REFERENCES dnd."user"(id) ON DELETE SET NULL
);
CREATE INDEX ix_dnd_vtt_event_campaign_id_command_id ON dnd_vtt.event USING btree (campaign_id, command_id) WHERE (command_id IS NOT NULL);
-- the user purge's SET NULL would otherwise scan the whole log, and "what did this player do" uses it
CREATE INDEX ix_dnd_vtt_event_created_by ON dnd_vtt.event USING btree (created_by);
