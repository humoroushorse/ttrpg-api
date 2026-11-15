-- The tabletop (schema dnd_vtt): scenes, tokens, who's on which scene, and the campaign's event log.
-- Every write starts with LockTableState inside its transaction, so a campaign's commands apply one at
-- a time and their events commit in seq order.

-- name: EnsureCampaignState :exec
-- The campaign's state row, made on the first write at its table.
INSERT INTO dnd_vtt.campaign_state (campaign_id, created_by, updated_by)
VALUES (@campaign_id::uuid, @user_id::uuid, @user_id::uuid)
ON CONFLICT (campaign_id) DO NOTHING;

-- name: LockTableState :one
-- The active scene, the last seq, and the caller's own scene (null: they follow the active one), with
-- the campaign's state row locked until the transaction ends.
SELECT s.active_scene_id, s.last_seq, m.scene_id AS member_scene_id
FROM dnd_vtt.campaign_state s
LEFT JOIN dnd_vtt.member_state m ON m.campaign_id = s.campaign_id AND m.user_id = @user_id::uuid
WHERE s.campaign_id = @campaign_id::uuid
FOR UPDATE OF s;

-- name: TableState :one
-- LockTableState without the lock, for reads. No row: nobody has set up this campaign's table yet.
SELECT s.active_scene_id, s.last_seq, m.scene_id AS member_scene_id
FROM dnd_vtt.campaign_state s
LEFT JOIN dnd_vtt.member_state m ON m.campaign_id = s.campaign_id AND m.user_id = @user_id::uuid
WHERE s.campaign_id = @campaign_id::uuid;

-- name: SetActiveScene :exec
UPDATE dnd_vtt.campaign_state
SET active_scene_id = sqlc.narg(scene_id)::uuid, updated_at = now(), updated_by = @user_id::uuid
WHERE campaign_id = @campaign_id::uuid;

-- name: AppendEvent :one
-- Takes the next seq and writes the event in one statement. LockTableState must come first in the
-- transaction, so no other command can take a seq between the two.
WITH next AS (
    UPDATE dnd_vtt.campaign_state
    SET last_seq = last_seq + 1, updated_at = now(), updated_by = @user_id::uuid
    WHERE campaign_id = @campaign_id::uuid
    RETURNING last_seq
)
INSERT INTO dnd_vtt.event (campaign_id, seq, scene_id, type, audience, payload, command_id, created_by)
SELECT @campaign_id::uuid, next.last_seq, sqlc.narg(scene_id)::uuid, @type::varchar, @audience::varchar,
       @payload::jsonb, sqlc.narg(command_id)::uuid, @user_id::uuid
FROM next
RETURNING seq, created_at;

-- name: CommandSeq :one
-- The first event a command made, when the same command_id was already applied (a retried request).
SELECT seq FROM dnd_vtt.event
WHERE campaign_id = @campaign_id::uuid AND command_id = @command_id::uuid
ORDER BY seq
LIMIT 1;

-- name: EventsAfter :many
-- Replay, oldest first. The stream filters each event by audience and scene for its viewer.
SELECT seq, scene_id, type, audience, payload, created_at, created_by
FROM dnd_vtt.event
WHERE campaign_id = @campaign_id::uuid AND seq > @after_seq::bigint
ORDER BY seq
LIMIT sqlc.arg(max_events)::int;

-- name: OldestSeq :one
-- The first seq still in the log (history past its retention is dropped). A tab asking to replay from
-- before it reloads the scene instead. 0 when the campaign has no events.
SELECT COALESCE(min(seq), 0)::bigint AS seq FROM dnd_vtt.event WHERE campaign_id = @campaign_id::uuid;

-- name: CreateEventPartitions :many
-- This month's and the next months' partitions of the log; returns the ones it made.
SELECT dnd_vtt.create_event_partitions(@months_ahead::int)::text AS partition;

-- name: DropEventPartitions :many
-- Drops the months past the log's retention; returns their names.
SELECT dnd_vtt.drop_event_partitions(make_interval(days => @keep_days::int))::text AS partition;

-- name: TryJobLock :one
-- Whether this transaction got the job's lock (released when it ends): every pod runs the daily jobs,
-- and one does the work.
SELECT pg_try_advisory_xact_lock(@key::bigint) AS locked;

-- name: SetLockTimeout :exec
-- For this transaction only. Creating or dropping a partition locks the whole log briefly; waiting
-- behind a long transaction would stall every write queued behind it.
SELECT set_config('lock_timeout', @timeout::text, true);

-- name: DefaultPartitionHasRows :one
-- Rows land in event_default only when the partition job fell behind.
SELECT EXISTS (SELECT 1 FROM dnd_vtt.event_default) AS has_rows;

-- name: MemberScenes :many
-- The players the DM moved off the active scene, and where to.
SELECT user_id, scene_id::uuid AS scene_id
FROM dnd_vtt.member_state
WHERE campaign_id = @campaign_id::uuid AND scene_id IS NOT NULL;

-- name: SetMemberScene :exec
-- Moves one member to a scene; a null scene sends them back to the active one.
INSERT INTO dnd_vtt.member_state (campaign_id, user_id, scene_id, created_by, updated_by)
VALUES (@campaign_id::uuid, @user_id::uuid, sqlc.narg(scene_id)::uuid, @by::uuid, @by::uuid)
ON CONFLICT (campaign_id, user_id)
DO UPDATE SET scene_id = EXCLUDED.scene_id, updated_at = now(), updated_by = EXCLUDED.updated_by;

-- name: ClearMemberScenes :many
-- Everyone follows the active scene again; returns who had been moved off it.
UPDATE dnd_vtt.member_state
SET scene_id = NULL, updated_at = now(), updated_by = @by::uuid
WHERE campaign_id = @campaign_id::uuid AND scene_id IS NOT NULL
RETURNING user_id;

-- name: CreateScene :one
-- A scene, last among its siblings.
INSERT INTO dnd_vtt.scene (campaign_id, parent_id, sort_order, name, background_sources, grid_type,
                           width_cells, height_cells, cell_px, grid_offset_x, grid_offset_y, grid_distance,
                           grid_units, settings, created_by, updated_by)
VALUES (@campaign_id::uuid, sqlc.narg(parent_id)::uuid,
        (SELECT COALESCE(max(sort_order) + 1, 0)::int FROM dnd_vtt.scene
         WHERE campaign_id = @campaign_id::uuid AND parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::uuid
           AND deleted_at IS NULL),
        @name::varchar, @background_sources::jsonb, @grid_type::varchar, @width_cells::int, @height_cells::int,
        @cell_px::float8, @grid_offset_x::float8, @grid_offset_y::float8, @grid_distance::float8,
        @grid_units::varchar, @settings::jsonb, @by::uuid, @by::uuid)
RETURNING *;

-- name: Scene :one
-- A live scene by id alone (the /vtt/scenes/{id} routes); the caller then checks its campaign.
SELECT * FROM dnd_vtt.scene WHERE id = @id::uuid AND deleted_at IS NULL;

-- name: Scenes :many
-- A campaign's live scenes; the client builds the tree from parent_id and sort_order.
SELECT * FROM dnd_vtt.scene
WHERE campaign_id = @campaign_id::uuid AND deleted_at IS NULL
ORDER BY sort_order, name, id;

-- name: UpdateScene :one
-- Every editable column. The route merges a PATCH into the current row first, so a null parent_id
-- means "top level", not "unchanged".
UPDATE dnd_vtt.scene
SET parent_id = sqlc.narg(parent_id)::uuid, sort_order = @sort_order::int, name = @name::varchar,
    background_sources = @background_sources::jsonb, grid_type = @grid_type::varchar,
    width_cells = @width_cells::int, height_cells = @height_cells::int, cell_px = @cell_px::float8,
    grid_offset_x = @grid_offset_x::float8, grid_offset_y = @grid_offset_y::float8,
    grid_distance = @grid_distance::float8, grid_units = @grid_units::varchar, settings = @settings::jsonb,
    updated_at = now(), updated_by = @by::uuid
WHERE id = @id::uuid AND campaign_id = @campaign_id::uuid AND deleted_at IS NULL
RETURNING *;

-- name: SceneSubtree :many
-- A live scene and every live scene under it: what a delete trashes, and where a move may not go.
-- UNION, not UNION ALL, so a cycle (the API never makes one) still ends.
WITH RECURSIVE tree AS (
    SELECT s.id FROM dnd_vtt.scene s
    WHERE s.campaign_id = @campaign_id::uuid AND s.id = @id::uuid AND s.deleted_at IS NULL
    UNION
    SELECT c.id FROM dnd_vtt.scene c JOIN tree ON c.parent_id = tree.id
    WHERE c.campaign_id = @campaign_id::uuid AND c.deleted_at IS NULL
)
SELECT id FROM tree;

-- name: TrashScenes :exec
-- One deleted_at for all of them (now() is the transaction's start), so they're restored together.
UPDATE dnd_vtt.scene
SET deleted_at = now(), deleted_by = @by::uuid, updated_at = now(), updated_by = @by::uuid
WHERE campaign_id = @campaign_id::uuid AND id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- name: NextTokenNumber :one
-- The scene's next "#n": past every number handed out before (a removed #3 isn't reused) and past any
-- the DM set by hand.
UPDATE dnd_vtt.scene s
SET last_token_number = GREATEST(
        s.last_token_number,
        COALESCE((SELECT max(t.number) FROM dnd_vtt.scene_token t WHERE t.scene_id = s.id), 0)) + 1
WHERE s.id = @scene_id::uuid AND s.campaign_id = @campaign_id::uuid
RETURNING s.last_token_number;

-- name: SceneTokens :many
-- A scene's tokens, with what a character token takes from the sheet when it doesn't set its own
-- name or image. Tokens of soft-deleted characters are left out.
SELECT t.id, t.campaign_id, t.scene_id, t.kind, t.character_id, t.controller_character_id, t.name,
       t.player_name, t.number, t.disposition, t.player_disposition, t.image_url, t.x, t.y, t.size_cells,
       t.layer, t.created_at, t.created_by, t.updated_at, t.updated_by,
       c.name AS character_name, COALESCE(c.details ->> 'portrait', '')::varchar AS character_portrait
FROM dnd_vtt.scene_token t
LEFT JOIN dnd."character" c ON c.id = t.character_id
WHERE t.scene_id = @scene_id::uuid AND (t.character_id IS NULL OR c.deleted_at IS NULL)
ORDER BY t.created_at, t.id;

-- name: Token :one
-- A token of the campaign on a live scene.
SELECT t.* FROM dnd_vtt.scene_token t
JOIN dnd_vtt.scene s ON s.id = t.scene_id AND s.deleted_at IS NULL
WHERE t.id = @id::uuid AND t.campaign_id = @campaign_id::uuid;

-- name: AddToken :one
INSERT INTO dnd_vtt.scene_token (campaign_id, scene_id, kind, character_id, controller_character_id, name,
                                 player_name, number, disposition, player_disposition, image_url, x, y,
                                 size_cells, layer, created_by, updated_by)
VALUES (@campaign_id::uuid, @scene_id::uuid, @kind::varchar, sqlc.narg(character_id)::uuid,
        sqlc.narg(controller_character_id)::uuid, sqlc.narg(name)::varchar, sqlc.narg(player_name)::varchar,
        sqlc.narg(number)::int, @disposition::varchar, sqlc.narg(player_disposition)::varchar,
        sqlc.narg(image_url)::varchar, @x::float8, @y::float8, @size_cells::float8, @layer::varchar,
        @by::uuid, @by::uuid)
RETURNING *;

-- name: MoveToken :exec
UPDATE dnd_vtt.scene_token
SET x = @x::float8, y = @y::float8, updated_at = now(), updated_by = @by::uuid
WHERE id = @id::uuid AND campaign_id = @campaign_id::uuid;

-- name: UpdateToken :one
-- Every editable column but the position. The route merges the command into the current row first, so
-- a null name on a character token means "the sheet's", not "unchanged".
UPDATE dnd_vtt.scene_token
SET controller_character_id = sqlc.narg(controller_character_id)::uuid, name = sqlc.narg(name)::varchar,
    player_name = sqlc.narg(player_name)::varchar, number = sqlc.narg(number)::int,
    disposition = @disposition::varchar, player_disposition = sqlc.narg(player_disposition)::varchar,
    image_url = sqlc.narg(image_url)::varchar, size_cells = @size_cells::float8, layer = @layer::varchar,
    updated_at = now(), updated_by = @by::uuid
WHERE id = @id::uuid AND campaign_id = @campaign_id::uuid
RETURNING *;

-- name: RemoveToken :exec
DELETE FROM dnd_vtt.scene_token WHERE id = @id::uuid AND campaign_id = @campaign_id::uuid;

-- name: SceneToken :one
-- One token as SceneTokens returns it, for a command's answer and its event.
SELECT t.id, t.campaign_id, t.scene_id, t.kind, t.character_id, t.controller_character_id, t.name,
       t.player_name, t.number, t.disposition, t.player_disposition, t.image_url, t.x, t.y, t.size_cells,
       t.layer, t.created_at, t.created_by, t.updated_at, t.updated_by,
       c.name AS character_name, COALESCE(c.details ->> 'portrait', '')::varchar AS character_portrait
FROM dnd_vtt.scene_token t
LEFT JOIN dnd."character" c ON c.id = t.character_id
WHERE t.id = @id::uuid AND t.campaign_id = @campaign_id::uuid;

-- name: OwnerCharacterIDs :many
-- The campaign's characters a user owns: what leaves the campaign with them.
SELECT cc.character_id
FROM dnd.campaign_character cc
JOIN dnd."character" c ON c.id = cc.character_id
WHERE cc.campaign_id = @campaign_id::uuid AND c.owner_id = @user_id::uuid;

-- name: TokensOfCharacters :many
-- Every token in the campaign that is one of these characters (removed when they leave) or that one of
-- them controls (it loses its controller).
SELECT id, scene_id, layer, (character_id IS NOT NULL AND character_id = ANY(@ids::uuid[]))::bool AS is_character
FROM dnd_vtt.scene_token
WHERE campaign_id = @campaign_id::uuid
  AND (character_id = ANY(@ids::uuid[]) OR controller_character_id = ANY(@ids::uuid[]));

-- name: CampaignSeqs :many
-- The last seq of each of these campaigns: a pod's check that it hasn't missed an event.
SELECT campaign_id, last_seq FROM dnd_vtt.campaign_state WHERE campaign_id = ANY(@ids::uuid[]);

-- name: CampaignCharacterEditors :many
-- The campaign's live characters, each with its owner and whether the caller is on its editor list:
-- what access.Character needs to answer EditBuild, for every token at once.
SELECT cc.character_id, c.owner_id,
       EXISTS (SELECT 1 FROM dnd.character_editor e WHERE e.character_id = c.id AND e.user_id = @user_id::uuid) AS editor
FROM dnd.campaign_character cc
JOIN dnd."character" c ON c.id = cc.character_id AND c.deleted_at IS NULL
WHERE cc.campaign_id = @campaign_id::uuid;
