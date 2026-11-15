-- Who the caller is to a campaign or a character, for internal/access to decide what they may do.
-- sqlc generates these into the campaigns and characters packages (sqlc.yaml), so each handler asks
-- inside its own transaction.

-- name: CampaignAccess :one
-- A live campaign, and the caller's role in it (null when they aren't a member).
SELECT g.owner_id, g.settings, m.role
FROM dnd.campaign g
LEFT JOIN dnd.campaign_member m ON m.campaign_id = g.id AND m.user_id = @user_id
WHERE g.id = @campaign_id AND g.deleted_at IS NULL;

-- name: CharacterAccess :one
-- A live character: its owner, whether the caller may edit it, and the caller's role in its campaign.
SELECT c.owner_id,
       EXISTS (SELECT 1 FROM dnd.character_editor e WHERE e.character_id = c.id AND e.user_id = @user_id) AS editor,
       g.id AS campaign_id, g.owner_id AS campaign_owner_id, g.settings AS campaign_settings,
       m.role AS campaign_role
FROM dnd."character" c
LEFT JOIN LATERAL (SELECT campaign_id FROM dnd.campaign_character WHERE character_id = c.id LIMIT 1) cc ON TRUE
LEFT JOIN dnd.campaign g ON g.id = cc.campaign_id AND g.deleted_at IS NULL
LEFT JOIN dnd.campaign_member m ON m.campaign_id = g.id AND m.user_id = @user_id
WHERE c.id = @character_id AND c.deleted_at IS NULL;
