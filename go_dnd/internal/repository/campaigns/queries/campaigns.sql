-- Reads are scoped to the campaign's members. Writes run after internal/access allows them (the
-- handler asks with CampaignAccess first, in the same transaction).

-- name: ListCampaigns :many
-- The live campaigns the user is a member of, by name, with their role. A limit of 0 returns every one.
SELECT sqlc.embed(c), m.role, count(*) OVER () AS total
FROM dnd.campaign c
JOIN dnd.campaign_member m ON m.campaign_id = c.id AND m.user_id = @user_id
WHERE c.deleted_at IS NULL
ORDER BY c.name, c.id
LIMIT NULLIF(@max_rows::int, 0) OFFSET @skip_rows::int;

-- name: GetCampaign :one
SELECT * FROM dnd.campaign
WHERE id = @id AND deleted_at IS NULL;

-- name: GetCampaignForUpdate :one
SELECT * FROM dnd.campaign
WHERE id = @id AND deleted_at IS NULL
FOR UPDATE;

-- name: CreateCampaign :one
INSERT INTO dnd.campaign (owner_id, name, description, rules, settings, created_by, updated_by)
VALUES (@owner_id, @name, @description, @rules, @settings, @owner_id, @owner_id)
RETURNING *;

-- name: UpdateCampaign :one
UPDATE dnd.campaign
SET name = @name, description = @description, rules = @rules, settings = @settings, updated_at = now(),
    updated_by = @user_id
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCampaign :one
-- The owner only, also checked here.
UPDATE dnd.campaign
SET deleted_at = now(), deleted_by = @owner_id
WHERE id = @id AND owner_id = @owner_id AND deleted_at IS NULL
RETURNING id;

-- name: AddMember :exec
-- Already a member: nothing changes (a second invite doesn't demote a DM).
INSERT INTO dnd.campaign_member (campaign_id, user_id, role, created_by, updated_by)
VALUES (@campaign_id, @user_id, @role, @by, @by)
ON CONFLICT (campaign_id, user_id) DO NOTHING;

-- name: ListMembers :many
-- DMs first, then by name.
SELECT m.user_id, u.username, u.profile_picture_url, m.role, m.created_at
FROM dnd.campaign_member m
JOIN dnd."user" u ON u.id = m.user_id
WHERE m.campaign_id = @campaign_id
ORDER BY (m.role = 'dm') DESC, lower(u.username), m.user_id;

-- name: MemberRole :one
SELECT role FROM dnd.campaign_member WHERE campaign_id = @campaign_id AND user_id = @user_id;

-- name: SetMemberRole :exec
UPDATE dnd.campaign_member SET role = @role, updated_at = now(), updated_by = @by
WHERE campaign_id = @campaign_id AND user_id = @user_id;

-- name: RemoveMember :exec
DELETE FROM dnd.campaign_member WHERE campaign_id = @campaign_id AND user_id = @user_id;

-- name: UnlinkMemberCharacters :many
-- The member's characters leave the campaign with them (back to their own rules).
DELETE FROM dnd.campaign_character cc
USING dnd."character" c
WHERE cc.campaign_id = @campaign_id AND cc.character_id = c.id AND c.owner_id = @user_id
RETURNING cc.character_id;

-- name: LogCampaignLeave :exec
-- A character that left with its owner: a version bump (its rules changed) and a change log row.
WITH bumped AS (
    UPDATE dnd."character" SET version = version + 1, updated_at = now(), updated_by = @by WHERE id = @character_id
)
INSERT INTO dnd.character_change (character_id, created_by, action, entity, entity_id)
VALUES (@character_id, @by, 'campaign.leave', 'campaign', @campaign_id::uuid);

-- name: UserByUsername :one
SELECT id, username FROM dnd."user" WHERE lower(username) = lower(@username) AND deleted_at IS NULL;

-- name: CreateInvite :one
INSERT INTO dnd.campaign_invite (campaign_id, token_hash, user_id, expires_at, max_uses, created_by, updated_by)
VALUES (@campaign_id, sqlc.narg(token_hash), sqlc.narg(user_id)::uuid, @expires_at, sqlc.narg(max_uses)::int, @by, @by)
RETURNING id;

-- name: ListInvites :many
-- A campaign's invites still open: not revoked, not expired, not used up.
SELECT i.id, i.user_id, u.username AS invited_username, i.expires_at, i.max_uses, i.uses, i.created_at,
       b.username AS invited_by_username, (i.token_hash IS NOT NULL)::bool AS is_link
FROM dnd.campaign_invite i
LEFT JOIN dnd."user" u ON u.id = i.user_id
LEFT JOIN dnd."user" b ON b.id = i.created_by
WHERE i.campaign_id = @campaign_id AND i.revoked_at IS NULL AND i.expires_at > now()
  AND (i.max_uses IS NULL OR i.uses < i.max_uses)
ORDER BY i.created_at DESC;

-- name: PendingUserInvite :one
-- An open invite for this user to this campaign, to avoid a second one.
SELECT id FROM dnd.campaign_invite
WHERE campaign_id = @campaign_id AND user_id = @user_id AND revoked_at IS NULL AND uses = 0 AND expires_at > now()
LIMIT 1;

-- name: RevokeInvite :execrows
UPDATE dnd.campaign_invite SET revoked_at = now(), revoked_by = @by, updated_at = now(), updated_by = @by
WHERE id = @id AND campaign_id = @campaign_id AND revoked_at IS NULL;

-- name: InviteByToken :one
-- A link's invite and its campaign, locked for an accept.
SELECT i.id, i.campaign_id, i.expires_at, i.max_uses, i.uses, i.revoked_at, g.name AS campaign_name,
       (g.deleted_at IS NOT NULL)::bool AS campaign_deleted, b.username AS invited_by_username
FROM dnd.campaign_invite i
JOIN dnd.campaign g ON g.id = i.campaign_id
LEFT JOIN dnd."user" b ON b.id = i.created_by
WHERE i.token_hash = @token_hash
FOR UPDATE OF i;

-- name: UserInvite :one
-- One of the user's own invites, locked for an accept or decline.
SELECT i.id, i.campaign_id, i.expires_at, i.max_uses, i.uses, i.revoked_at, g.name AS campaign_name,
       (g.deleted_at IS NOT NULL)::bool AS campaign_deleted, b.username AS invited_by_username
FROM dnd.campaign_invite i
JOIN dnd.campaign g ON g.id = i.campaign_id
LEFT JOIN dnd."user" b ON b.id = i.created_by
WHERE i.id = @id AND i.user_id = @user_id
FOR UPDATE OF i;

-- name: UseInvite :exec
UPDATE dnd.campaign_invite SET uses = uses + 1, updated_at = now(), updated_by = @by WHERE id = @id;

-- name: DeclineInvite :exec
UPDATE dnd.campaign_invite SET revoked_at = now(), revoked_by = @by, updated_at = now(), updated_by = @by
WHERE id = @id;

-- name: MyPendingInvites :many
-- The user's open invites to campaigns they aren't in yet.
SELECT i.id, i.campaign_id, g.name AS campaign_name, i.expires_at, i.created_at, b.username AS invited_by_username
FROM dnd.campaign_invite i
JOIN dnd.campaign g ON g.id = i.campaign_id AND g.deleted_at IS NULL
LEFT JOIN dnd."user" b ON b.id = i.created_by
WHERE i.user_id = @user_id AND i.revoked_at IS NULL AND i.uses = 0 AND i.expires_at > now()
  AND NOT EXISTS (SELECT 1 FROM dnd.campaign_member m WHERE m.campaign_id = i.campaign_id AND m.user_id = @user_id)
ORDER BY i.created_at DESC;
