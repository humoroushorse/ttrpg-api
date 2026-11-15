-- Reads and writes run after internal/access allows them: the handler asks with CharacterAccess or
-- CampaignAccess first, in the same transaction. Deleting a character also checks the owner here.

-- name: ListCharacters :many
-- The user's live characters by name: their own, and the ones they may edit. A limit of 0 returns every one.
SELECT sqlc.embed(c), cc.campaign_id::uuid AS campaign_id, count(*) OVER () AS total
FROM dnd."character" c
LEFT JOIN LATERAL (SELECT campaign_id FROM dnd.campaign_character WHERE character_id = c.id LIMIT 1) cc ON TRUE
WHERE (c.owner_id = @user_id
       OR EXISTS (SELECT 1 FROM dnd.character_editor e WHERE e.character_id = c.id AND e.user_id = @user_id))
  AND c.deleted_at IS NULL
ORDER BY c.name, c.id
LIMIT NULLIF(@max_rows::int, 0) OFFSET @skip_rows::int;

-- name: GetCharacter :one
SELECT sqlc.embed(c), cc.campaign_id::uuid AS campaign_id
FROM dnd."character" c
LEFT JOIN LATERAL (SELECT campaign_id FROM dnd.campaign_character WHERE character_id = c.id LIMIT 1) cc ON TRUE
WHERE c.id = @id AND c.deleted_at IS NULL;

-- name: LockCharacter :one
-- Locks the live row for a write and returns its version.
SELECT version FROM dnd."character"
WHERE id = @id AND deleted_at IS NULL
FOR UPDATE;

-- name: CreateCharacter :one
INSERT INTO dnd."character" (id, owner_id, name, current_level, rules, ability_method, base_scores, currency,
                             details, created_by, updated_by)
VALUES (COALESCE(sqlc.narg(id)::uuid, uuidv7()), @owner_id, @name, @current_level, sqlc.narg(rules)::jsonb,
        @ability_method, @base_scores, @currency, @details, @owner_id, @owner_id)
RETURNING id;

-- name: UpdateCharacter :exec
UPDATE dnd."character"
SET name = @name, current_level = @current_level, rules = sqlc.narg(rules)::jsonb, currency = @currency,
    details = @details, version = version + 1, updated_at = now(), updated_by = @user_id
WHERE id = @id;

-- name: TouchCharacter :exec
-- A write to a child row: bump the version.
UPDATE dnd."character" SET version = version + 1, updated_at = now(), updated_by = @user_id WHERE id = @id;

-- name: DeleteCharacter :one
UPDATE dnd."character"
SET deleted_at = now(), deleted_by = @owner_id, version = version + 1
WHERE id = @id AND owner_id = @owner_id AND deleted_at IS NULL
RETURNING id;

-- name: CampaignRules :one
-- A live campaign's rules.
SELECT rules FROM dnd.campaign WHERE id = @id AND deleted_at IS NULL;

-- name: LinkCampaign :exec
INSERT INTO dnd.campaign_character (campaign_id, character_id, created_by, updated_by)
VALUES (@campaign_id, @character_id, @user_id, @user_id);

-- name: ListLevels :many
SELECT l.id, l.level, l.class_id, l.hp_method, l.hp_value, l.hp_rolls,
       k.slug AS class_slug, k.name AS class_name, k.dnd_version_year AS class_year, s.slug AS class_source_slug
FROM dnd.character_level l
JOIN dnd.dnd_class k ON k.id = l.class_id
JOIN dnd.source s ON s.id = k.source_id
WHERE l.character_id = @character_id
ORDER BY l.level;

-- name: GetLevel :one
SELECT * FROM dnd.character_level WHERE character_id = @character_id AND level = @level;

-- name: UpsertLevel :one
INSERT INTO dnd.character_level (character_id, level, class_id, hp_method, hp_value, created_by, updated_by)
VALUES (@character_id, @level, @class_id, @hp_method, sqlc.narg(hp_value)::int, @user_id, @user_id)
ON CONFLICT (character_id, level) DO UPDATE
SET class_id = EXCLUDED.class_id, hp_method = EXCLUDED.hp_method, hp_value = EXCLUDED.hp_value,
    updated_at = now(), updated_by = EXCLUDED.updated_by
RETURNING *;

-- name: DeleteLevel :exec
DELETE FROM dnd.character_level WHERE character_id = @character_id AND level = @level;

-- name: SetClassOnAllLevels :exec
UPDATE dnd.character_level SET class_id = @class_id, updated_at = now(), updated_by = @user_id
WHERE character_id = @character_id;

-- name: TopLevel :one
SELECT COALESCE(max(level), 0)::int FROM dnd.character_level WHERE character_id = @character_id;

-- name: OtherClass :one
-- A class other than this one on another level, if any (single class until multiclassing).
SELECT class_id FROM dnd.character_level
WHERE character_id = @character_id AND level <> @level AND class_id <> @class_id
LIMIT 1;

-- name: ClassForRules :one
-- What the rules need to know about a class: its edition, UA, source, and whether the base edition
-- has a live class with the same slug (for other_edition: if_missing).
SELECT k.dnd_version_year, k.is_unearthed_arcana, (k.deleted_at IS NOT NULL)::bool AS deleted, k.hit_die_size,
       s.slug AS source_slug,
       EXISTS (SELECT 1 FROM dnd.dnd_class b WHERE b.slug = k.slug AND b.dnd_version_year = @base_edition::int
               AND b.deleted_at IS NULL) AS base_has_slug
FROM dnd.dnd_class k JOIN dnd.source s ON s.id = k.source_id
WHERE k.id = @id;

-- name: OpApplied :one
SELECT EXISTS (SELECT 1 FROM dnd.character_change WHERE op_id = @op_id);

-- name: LogChange :exec
INSERT INTO dnd.character_change (character_id, created_by, action, entity, entity_id, before, after, op_id)
VALUES (@character_id, @user_id, @action, sqlc.narg(entity), sqlc.narg(entity_id)::uuid,
        sqlc.narg(before)::jsonb, sqlc.narg(after)::jsonb, sqlc.narg(op_id)::uuid);

-- name: CharacterClass :one
-- The character's class (level 1's; single class until multiclassing).
SELECT k.id, k.slug, k.name, k.dnd_version_year, k.hit_die_size
FROM dnd.character_level l JOIN dnd.dnd_class k ON k.id = l.class_id
WHERE l.character_id = @character_id AND l.level = 1;

-- name: EntryForRules :one
-- One catalog entry as the content rules see it, and whether the base edition has a live entry with
-- its slug (other_edition: if_missing).
SELECT e.name, e.dnd_version_year, e.is_unearthed_arcana, (e.deleted_at IS NOT NULL)::bool AS deleted,
       s.slug AS source_slug,
       (base.slug IS NOT NULL)::bool AS base_has_slug
FROM dnd.catalog_entry e
JOIN dnd.source s ON s.id = e.source_id
LEFT JOIN (SELECT DISTINCT b.category, b.slug FROM dnd.catalog_entry b
           WHERE b.dnd_version_year = @base_edition::int AND b.deleted_at IS NULL) base
       ON base.category = e.category AND base.slug = e.slug
WHERE e.category = @category::text AND e.id = @id;

-- name: CharacterPicks :many
-- Every catalog entry the character uses (classes, choices, prepared spells), as the content rules
-- see it, for checking against a rule set.
SELECT e.category, e.id AS ref_id, e.name, e.dnd_version_year, e.is_unearthed_arcana,
       (e.deleted_at IS NOT NULL)::bool AS deleted, s.slug AS source_slug,
       (base.slug IS NOT NULL)::bool AS base_has_slug
FROM (
    SELECT DISTINCT 'class'::text AS category, l.class_id AS ref_id FROM dnd.character_level l WHERE l.character_id = @character_id
    UNION SELECT 'subclass', c.subclass_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.subclass_id IS NOT NULL
    UNION SELECT 'species', c.species_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.species_id IS NOT NULL
    UNION SELECT 'background', c.background_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.background_id IS NOT NULL
    UNION SELECT 'feat', c.feat_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.feat_id IS NOT NULL
    UNION SELECT 'spell', c.spell_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.spell_id IS NOT NULL
    UNION SELECT 'item', c.item_id FROM dnd.character_choice c WHERE c.character_id = @character_id AND c.item_id IS NOT NULL
    UNION SELECT 'spell', ps.spell_id FROM dnd.character_prepared_spell ps WHERE ps.character_id = @character_id
) p
JOIN dnd.catalog_entry e ON e.category = p.category AND e.id = p.ref_id
JOIN dnd.source s ON s.id = e.source_id
LEFT JOIN (SELECT DISTINCT b.category, b.slug FROM dnd.catalog_entry b
           WHERE b.dnd_version_year = @base_edition::int AND b.deleted_at IS NULL) base
       ON base.category = e.category AND base.slug = e.slug
ORDER BY e.category, e.name;

-- name: SubclassInfo :one
SELECT parent_class_slug, level_gained, dnd_version_year FROM dnd.subclass WHERE id = @id;

-- name: ListChoices :many
SELECT sqlc.embed(c),
       COALESCE(sp.name, sc.name, bg.name, ft.name, sl.name, it.name, '')::text AS ref_name
FROM dnd.character_choice c
LEFT JOIN dnd.species sp ON sp.id = c.species_id
LEFT JOIN dnd.subclass sc ON sc.id = c.subclass_id
LEFT JOIN dnd.background bg ON bg.id = c.background_id
LEFT JOIN dnd.feat ft ON ft.id = c.feat_id
LEFT JOIN dnd.spell sl ON sl.id = c.spell_id
LEFT JOIN dnd.item it ON it.id = c.item_id
WHERE c.character_id = @character_id
ORDER BY c.level, c.created_at, c.id;

-- name: GetChoice :one
SELECT * FROM dnd.character_choice WHERE id = @id AND character_id = @character_id;

-- name: CountChoicesOfKind :one
-- Other choices of this kind (one species, one background, one subclass per character).
SELECT count(*) FROM dnd.character_choice
WHERE character_id = @character_id AND kind = @kind AND id <> @except_id;

-- name: InsertChoice :one
INSERT INTO dnd.character_choice (id, character_id, level, kind, slot, granted_by_id, species_id, subclass_id,
                                  background_id, feat_id, spell_id, item_id, value, overrides_filter, created_by, updated_by)
VALUES (COALESCE(sqlc.narg(id)::uuid, uuidv7()), @character_id, @level, @kind, @slot, sqlc.narg(granted_by_id)::uuid,
        sqlc.narg(species_id)::uuid, sqlc.narg(subclass_id)::uuid, sqlc.narg(background_id)::uuid, sqlc.narg(feat_id)::uuid,
        sqlc.narg(spell_id)::uuid, sqlc.narg(item_id)::uuid, sqlc.narg(value)::jsonb, @overrides_filter, @user_id, @user_id)
RETURNING *;

-- name: UpdateChoice :one
UPDATE dnd.character_choice
SET level = @level, kind = @kind, slot = @slot, granted_by_id = sqlc.narg(granted_by_id)::uuid,
    species_id = sqlc.narg(species_id)::uuid, subclass_id = sqlc.narg(subclass_id)::uuid,
    background_id = sqlc.narg(background_id)::uuid, feat_id = sqlc.narg(feat_id)::uuid,
    spell_id = sqlc.narg(spell_id)::uuid, item_id = sqlc.narg(item_id)::uuid, value = sqlc.narg(value)::jsonb,
    overrides_filter = @overrides_filter, updated_at = now(), updated_by = @user_id
WHERE id = @id AND character_id = @character_id
RETURNING *;

-- name: ChoiceDependents :many
-- What was granted by this choice: other choices (which block removing it) and items (which stay).
SELECT 'choice'::text AS kind, c.id FROM dnd.character_choice c WHERE c.granted_by_id = @id::uuid
UNION ALL SELECT 'item'::text, i.id FROM dnd.character_item i WHERE i.granted_by_id = @id::uuid;

-- name: DeleteChoice :exec
DELETE FROM dnd.character_choice WHERE id = @id AND character_id = @character_id;

-- name: ListItems :many
SELECT sqlc.embed(ci), it.name AS item_name, it.slug AS item_slug
FROM dnd.character_item ci LEFT JOIN dnd.item it ON it.id = ci.item_id
WHERE ci.character_id = @character_id
ORDER BY ci.created_at, ci.id;

-- name: GetItem :one
SELECT * FROM dnd.character_item WHERE id = @id AND character_id = @character_id;

-- name: CatalogItemLive :one
SELECT EXISTS (SELECT 1 FROM dnd.item WHERE id = @id AND deleted_at IS NULL);

-- name: InsertItem :one
-- given_by and given_campaign_id mark a campaign DM's gift.
INSERT INTO dnd.character_item (id, character_id, item_id, custom_name, custom_description, quantity, equipped, attuned,
                                level_received, note, granted_by_id, given_by, given_campaign_id, created_by, updated_by)
VALUES (COALESCE(sqlc.narg(id)::uuid, uuidv7()), @character_id, sqlc.narg(item_id)::uuid, sqlc.narg(custom_name)::text,
        sqlc.narg(custom_description)::text, @quantity, @equipped, @attuned, sqlc.narg(level_received)::int,
        sqlc.narg(note)::text, sqlc.narg(granted_by_id)::uuid, sqlc.narg(given_by)::uuid,
        sqlc.narg(given_campaign_id)::uuid, @user_id, @user_id)
RETURNING *;

-- name: UpdateItem :one
UPDATE dnd.character_item
SET custom_name = sqlc.narg(custom_name)::text, custom_description = sqlc.narg(custom_description)::text,
    quantity = @quantity, equipped = @equipped, attuned = @attuned, level_received = sqlc.narg(level_received)::int,
    note = sqlc.narg(note)::text, updated_at = now(), updated_by = @user_id
WHERE id = @id AND character_id = @character_id
RETURNING *;

-- name: DeleteItem :exec
DELETE FROM dnd.character_item WHERE id = @id AND character_id = @character_id;

-- name: ClassEquipmentOptions :one
SELECT k.dnd_version_year, k.starting_equipment_options AS options
FROM dnd.character_level l JOIN dnd.dnd_class k ON k.id = l.class_id
WHERE l.character_id = @character_id AND l.level = 1;

-- name: BackgroundEquipmentOptions :one
-- The character's background (its origin choice) and its options.
SELECT b.dnd_version_year, b.equipment_options AS options
FROM dnd.character_choice c JOIN dnd.background b ON b.id = c.background_id
WHERE c.character_id = @character_id AND c.kind = 'background' AND c.granted_by_id IS NULL
LIMIT 1;

-- name: ItemByName :one
-- A live catalog item by name, any case, the given edition's first.
SELECT id FROM dnd.item
WHERE lower(name) = lower(@name::text) AND deleted_at IS NULL
ORDER BY (dnd_version_year = @year::int) DESC, dnd_version_year DESC, id
LIMIT 1;

-- name: SetChoiceValue :exec
UPDATE dnd.character_choice SET value = @value WHERE id = @id AND character_id = @character_id;

-- name: SetCurrency :exec
UPDATE dnd."character" SET currency = @currency, updated_at = now(), updated_by = @user_id WHERE id = @id;

-- name: ListPreparedSpells :many
SELECT p.spell_id, p.class_id, s.name AS spell_name, s.slug AS spell_slug, s.level AS spell_level, k.slug AS class_slug
FROM dnd.character_prepared_spell p
JOIN dnd.spell s ON s.id = p.spell_id
JOIN dnd.dnd_class k ON k.id = p.class_id
WHERE p.character_id = @character_id
ORDER BY s.level, s.name;

-- name: ClassSpellPreparation :one
-- A spell's level, and when the class's spells change (long_rest, level_up; "" when the catalog doesn't say).
SELECT s.level AS spell_level, COALESCE(k.spell_preparation ->> 'change', '')::text AS change
FROM dnd.spell s, dnd.dnd_class k
WHERE s.id = @spell_id AND k.id = @class_id;

-- name: SpellOnClassList :one
SELECT EXISTS (SELECT 1 FROM dnd.spell_class WHERE spell_id = @spell_id AND class_id = @class_id);

-- name: AddPreparedSpell :execrows
INSERT INTO dnd.character_prepared_spell (character_id, spell_id, class_id, created_by, updated_by)
VALUES (@character_id, @spell_id, @class_id, @user_id, @user_id)
ON CONFLICT DO NOTHING;

-- name: RemovePreparedSpell :execrows
DELETE FROM dnd.character_prepared_spell WHERE character_id = @character_id AND spell_id = @spell_id AND class_id = @class_id;

-- name: ListModifiers :many
SELECT * FROM dnd.character_modifier WHERE character_id = @character_id ORDER BY created_at, id;

-- name: GetModifier :one
SELECT * FROM dnd.character_modifier WHERE id = @id AND character_id = @character_id;

-- name: InsertModifier :one
INSERT INTO dnd.character_modifier (id, character_id, label, note, target, op, value, from_level, active, added_as,
                                    campaign_id, created_by, updated_by)
VALUES (COALESCE(sqlc.narg(id)::uuid, uuidv7()), @character_id, @label, sqlc.narg(note)::text, @target, @op, @value,
        sqlc.narg(from_level)::int, @active, @added_as, sqlc.narg(campaign_id)::uuid, @user_id, @user_id)
RETURNING *;

-- name: UpdateModifier :one
UPDATE dnd.character_modifier
SET label = @label, note = sqlc.narg(note)::text, target = @target, op = @op, value = @value,
    from_level = sqlc.narg(from_level)::int, active = @active, added_as = @added_as, updated_at = now(), updated_by = @user_id
WHERE id = @id AND character_id = @character_id
RETURNING *;

-- name: DeleteModifier :exec
DELETE FROM dnd.character_modifier WHERE id = @id AND character_id = @character_id;

-- name: SetAbilityScores :exec
UPDATE dnd."character" SET ability_method = @ability_method, base_scores = @base_scores WHERE id = @id;

-- name: SetAbilityRolls :exec
UPDATE dnd."character" SET ability_rolls = @ability_rolls, ability_roll_count = @ability_roll_count WHERE id = @id;

-- name: SetLevelHPRoll :one
UPDATE dnd.character_level
SET hp_method = 'rolled', hp_value = @hp_value, hp_rolls = @hp_rolls, updated_at = now(), updated_by = @user_id
WHERE character_id = @character_id AND level = @level
RETURNING id;

-- name: CharacterCampaign :one
SELECT campaign_id FROM dnd.campaign_character WHERE character_id = @character_id LIMIT 1;

-- name: UnlinkCampaign :execrows
DELETE FROM dnd.campaign_character WHERE campaign_id = @campaign_id AND character_id = @character_id;

-- name: CampaignCharacters :many
-- A live campaign's live characters, with who plays each and the party list's class and species.
SELECT c.id, c.name, c.current_level, c.owner_id, u.username AS owner_username, cc.approved_exceptions,
       COALESCE((SELECT k.name FROM dnd.character_level l JOIN dnd.dnd_class k ON k.id = l.class_id
                 WHERE l.character_id = c.id ORDER BY l.level LIMIT 1), '')::text AS class_name,
       COALESCE((SELECT s.name FROM dnd.character_choice ch JOIN dnd.species s ON s.id = ch.species_id
                 WHERE ch.character_id = c.id AND ch.kind = 'species' LIMIT 1), '')::text AS species_name
FROM dnd.campaign_character cc
JOIN dnd.campaign g ON g.id = cc.campaign_id
JOIN dnd."character" c ON c.id = cc.character_id
JOIN dnd."user" u ON u.id = c.owner_id
WHERE cc.campaign_id = @campaign_id AND g.deleted_at IS NULL AND c.deleted_at IS NULL
ORDER BY c.name, c.id;

-- name: ClassForSheet :one
SELECT name, slug, dnd_version_year, hit_die_size, proficiencies, features, spellcasting_ability, caster_progression,
       level_table, spell_preparation, starting_equipment_options
FROM dnd.dnd_class WHERE id = @id;

-- name: SpeciesForSheet :one
SELECT name, slug, size, speed, traits FROM dnd.species WHERE id = @id;

-- name: BackgroundForSheet :one
SELECT name, slug, ability_score_bonuses, skill_proficiencies, tool_proficiencies, feat, equipment_options
FROM dnd.background WHERE id = @id;

-- name: SubclassForSheet :one
SELECT name, slug, features, spellcasting_ability, caster_progression, level_table, spell_preparation
FROM dnd.subclass WHERE id = @id;

-- name: FeatsForSheet :many
-- The character's feats (deleted catalog rows included) with what they do.
SELECT ch.id AS choice_id, f.name, f.slug, f.category, f.effects
FROM dnd.character_choice ch JOIN dnd.feat f ON f.id = ch.feat_id
WHERE ch.character_id = @character_id
ORDER BY ch.level, ch.created_at, ch.id;

-- name: SubclassLevelFor :one
-- When a 2014 class takes its subclass: the lowest level_gained among its subclasses.
SELECT COALESCE(min(level_gained), 3)::int FROM dnd.subclass
WHERE parent_class_slug = @class_slug AND dnd_version_year = @year::int AND deleted_at IS NULL;

-- name: ItemsForSheet :many
SELECT ci.equipped, ci.attuned, ci.level_received, ci.custom_name, it.name AS item_name, it.category, it.subcategory,
       COALESCE(it.requires_attunement, false)::bool AS requires_attunement, it.effects, it.properties
FROM dnd.character_item ci LEFT JOIN dnd.item it ON it.id = ci.item_id
WHERE ci.character_id = @character_id
ORDER BY ci.created_at, ci.id;

-- name: CampaignLinkExceptions :one
-- The character's link to a live campaign: the exceptions its DMs approved.
SELECT cc.approved_exceptions
FROM dnd.campaign_character cc JOIN dnd.campaign g ON g.id = cc.campaign_id
WHERE cc.campaign_id = @campaign_id AND cc.character_id = @character_id AND g.deleted_at IS NULL;

-- name: SetApprovedExceptions :exec
UPDATE dnd.campaign_character
SET approved_exceptions = @approved_exceptions, updated_at = now(), updated_by = @user_id
WHERE campaign_id = @campaign_id AND character_id = @character_id;

-- name: CharacterExceptions :one
-- The exceptions approved for the character in its campaign ([] when there's none).
SELECT COALESCE((SELECT cc.approved_exceptions FROM dnd.campaign_character cc
                 WHERE cc.character_id = @character_id LIMIT 1), '[]'::jsonb)::jsonb AS approved_exceptions;

-- name: OptionEntries :many
-- A category's live catalog entries as the content rules see them, with whether the base edition
-- has an entry with the same slug (other_edition: if_missing).
SELECT e.id, e.name, e.slug, e.dnd_version_year, e.is_unearthed_arcana, s.slug AS source_slug,
       (base.slug IS NOT NULL)::bool AS base_has_slug
FROM dnd.catalog_entry e
JOIN dnd.source s ON s.id = e.source_id
LEFT JOIN (SELECT DISTINCT b.slug FROM dnd.catalog_entry b
           WHERE b.category = @category::text AND b.dnd_version_year = @base_edition::int AND b.deleted_at IS NULL) base
       ON base.slug = e.slug
WHERE e.category = @category::text AND e.deleted_at IS NULL
ORDER BY e.name, e.dnd_version_year DESC, e.id;

-- name: FeatOptions :many
SELECT id, category, prerequisites FROM dnd.feat WHERE deleted_at IS NULL;

-- name: SpellOptions :many
-- Each live spell's level, school, ritual tag and the classes whose lists have it.
SELECT s.id, s.level, s.school, s.is_ritual,
       COALESCE(array_agg(DISTINCT k.name) FILTER (WHERE k.name IS NOT NULL), '{}')::text[] AS class_names,
       COALESCE(array_agg(DISTINCT sc.class_id) FILTER (WHERE sc.class_id IS NOT NULL), '{}')::uuid[] AS class_ids
FROM dnd.spell s
LEFT JOIN dnd.spell_class sc ON sc.spell_id = s.id
LEFT JOIN dnd.dnd_class k ON k.id = sc.class_id
WHERE s.deleted_at IS NULL
GROUP BY s.id;

-- name: SubclassOptions :many
SELECT id, parent_class_slug, level_gained FROM dnd.subclass WHERE deleted_at IS NULL;

-- name: UserByUsername :one
SELECT id, username FROM dnd."user" WHERE lower(username) = lower(@username) AND deleted_at IS NULL;

-- name: UserByID :one
SELECT id, username FROM dnd."user" WHERE id = @id AND deleted_at IS NULL;

-- name: ListEditors :many
SELECT e.user_id, u.username, u.profile_picture_url, e.created_at
FROM dnd.character_editor e JOIN dnd."user" u ON u.id = e.user_id
WHERE e.character_id = @character_id
ORDER BY lower(u.username), e.user_id;

-- name: AddEditor :execrows
INSERT INTO dnd.character_editor (character_id, user_id, created_by, updated_by)
VALUES (@character_id, @user_id, @by, @by)
ON CONFLICT (character_id, user_id) DO NOTHING;

-- name: RemoveEditor :execrows
DELETE FROM dnd.character_editor WHERE character_id = @character_id AND user_id = @user_id;

-- name: ClearEditors :exec
DELETE FROM dnd.character_editor WHERE character_id = @character_id;

-- name: OfferTransfer :exec
-- One offer at a time: a new one replaces it.
INSERT INTO dnd.character_transfer (character_id, to_user_id, expires_at, created_by, updated_by)
VALUES (@character_id, @to_user_id, @expires_at, @by, @by)
ON CONFLICT (character_id) DO UPDATE
SET to_user_id = EXCLUDED.to_user_id, expires_at = EXCLUDED.expires_at, created_at = now(),
    created_by = EXCLUDED.created_by, updated_at = now(), updated_by = EXCLUDED.updated_by;

-- name: GetTransfer :one
SELECT t.to_user_id, u.username AS to_username, t.expires_at, t.created_at
FROM dnd.character_transfer t JOIN dnd."user" u ON u.id = t.to_user_id
WHERE t.character_id = @character_id;

-- name: CancelTransfer :execrows
DELETE FROM dnd.character_transfer WHERE character_id = @character_id;

-- name: IncomingTransfers :many
-- Offers to the user that haven't expired, of live characters.
SELECT t.character_id, c.name AS character_name, c.current_level, o.username AS from_username, t.expires_at, t.created_at,
       g.id AS campaign_id, g.name AS campaign_name
FROM dnd.character_transfer t
JOIN dnd."character" c ON c.id = t.character_id AND c.deleted_at IS NULL
JOIN dnd."user" o ON o.id = c.owner_id
LEFT JOIN LATERAL (SELECT campaign_id FROM dnd.campaign_character WHERE character_id = c.id LIMIT 1) cc ON TRUE
LEFT JOIN dnd.campaign g ON g.id = cc.campaign_id AND g.deleted_at IS NULL
WHERE t.to_user_id = @user_id AND t.expires_at > now()
ORDER BY t.created_at DESC;

-- name: LockTransfer :one
-- The user's offer for this character, with the character locked, for an accept.
SELECT c.owner_id, t.expires_at, c.version
FROM dnd.character_transfer t
JOIN dnd."character" c ON c.id = t.character_id AND c.deleted_at IS NULL
WHERE t.character_id = @character_id AND t.to_user_id = @user_id
FOR UPDATE OF t, c;

-- name: SetOwner :exec
UPDATE dnd."character" SET owner_id = @owner_id, version = version + 1, updated_at = now(), updated_by = @owner_id
WHERE id = @id;
