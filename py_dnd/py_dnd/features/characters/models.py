"""SQLAlchemy Tables: a character's build and state (CHARACTER_PLAN.md section 4).

The tables hold what the player chose and what affects the character, each tied to a level where it
matters. Nothing computed is stored: the sheet is calculated from these rows and the catalog.

Child rows are hard-deleted (the change log keeps their history) and go with the character
(ON DELETE CASCADE). Catalog references are plain foreign keys: catalog rows are soft-deleted, so
they never disappear from under a character.
"""

import datetime
import uuid
from typing import Any

from sqlalchemy import CheckConstraint, DateTime, ForeignKey, Index, UniqueConstraint, func, text
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import USER_ID_FK, MixinBookeeping, MixinSoftDelete, soft_delete_index

EMPTY_JSON_OBJECT = text("'{}'::jsonb")
EMPTY_JSON_LIST = text("'[]'::jsonb")
CHARACTER_FK = "dnd.character.id"


def _character_id() -> Mapped[uuid.UUID]:
    return mapped_column(ForeignKey(CHARACTER_FK, ondelete="CASCADE"), nullable=False)


def _id() -> Mapped[uuid.UUID]:
    # clients may send their own uuidv7 (offline creates replay without duplicating)
    return mapped_column(primary_key=True, server_default=text("uuidv7()"))


class Character(MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A character: who owns it, which level is selected, and how its ability scores were made."""

    __tablename__ = "character"
    __table_args__ = (
        CheckConstraint("current_level BETWEEN 1 AND 20", name="current_level"),
        CheckConstraint(
            "ability_method IN ('point_buy', 'standard_array', 'rolled', 'manual')", name="ability_method"
        ),
        Index("ix_dnd_character_owner_id", "owner_id"),
        soft_delete_index("character"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    # CASCADE: the user purge hard-deletes their characters
    owner_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(USER_ID_FK, ondelete="CASCADE"), nullable=False)
    name: Mapped[str] = mapped_column(nullable=False)
    # which planned levels apply; levels above it are kept
    current_level: Mapped[int] = mapped_column(server_default="1", nullable=False)
    # own rule set; null when a campaign supplies it
    rules: Mapped[dict[str, Any] | None] = mapped_column(nullable=True)
    ability_method: Mapped[str] = mapped_column(server_default="point_buy", nullable=False)
    # {str, dex, con, int, wis, cha} before any bonus
    base_scores: Mapped[dict[str, Any]] = mapped_column(server_default=EMPTY_JSON_OBJECT, nullable=False)
    # the last rules.rolled_sets_kept sets: [{dice: [[6,4,3,1], ...], at, origin: server | offline}]
    ability_rolls: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # every set ever rolled, including the ones no longer kept
    ability_roll_count: Mapped[int] = mapped_column(server_default="0", nullable=False)
    # {cp, sp, ep, gp, pp}, no history
    currency: Mapped[dict[str, Any]] = mapped_column(server_default=EMPTY_JSON_OBJECT, nullable=False)
    # alignment, appearance, backstory, personality, ideals, bonds, flaws, portrait
    details: Mapped[dict[str, Any]] = mapped_column(server_default=EMPTY_JSON_OBJECT, nullable=False)
    # +1 on every write; a write based on an older version is a conflict (offline sync)
    version: Mapped[int] = mapped_column(server_default="1", nullable=False)


class CharacterLevel(MixinBookeeping, DndSchemaBase):
    """One planned level (levels above current_level included): its class and its hit points."""

    __tablename__ = "character_level"
    __table_args__ = (
        UniqueConstraint("character_id", "level", name="uq_character_level_character_id_level"),
        CheckConstraint("level BETWEEN 1 AND 20", name="level"),
        CheckConstraint("hp_method IN ('average', 'rolled', 'manual')", name="hp_method"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    character_id: Mapped[uuid.UUID] = _character_id()
    level: Mapped[int] = mapped_column(nullable=False)
    # the same class on every row until multiclassing
    class_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.dnd_class.id"), nullable=False)
    hp_method: Mapped[str] = mapped_column(server_default="average", nullable=False)
    # this level's die result, without Con (level 1 is always the max die); null until set
    hp_value: Mapped[int | None] = mapped_column(nullable=True)
    # dice rolled for this level: [{value, at, origin: server | offline}]
    hp_rolls: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)


class CharacterChoice(MixinBookeeping, DndSchemaBase):
    """One choice the player made, at the level it was made."""

    __tablename__ = "character_choice"
    __table_args__ = (
        CheckConstraint("level BETWEEN 1 AND 20", name="level"),
        CheckConstraint(
            "num_nonnulls(species_id, subclass_id, background_id, feat_id, spell_id, item_id) <= 1",
            name="one_pick",
        ),
        Index("ix_dnd_character_choice_character_id", "character_id"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    character_id: Mapped[uuid.UUID] = _character_id()
    # 1 for origin choices (species, background, starting ability scores)
    level: Mapped[int] = mapped_column(nullable=False)
    # species, subclass, background, feat, asi, skill, expertise, tool, language, fighting_style,
    # equipment_option, spell (only a feat's or feature's own picks; class spells aren't choices),
    # saving_throw, resistance, spellcasting_ability, option (a named option: Elven Lineage, a size)
    kind: Mapped[str] = mapped_column(nullable=False)
    # the choice point it answers, as the sheet's choice_points list them (class:wizard:skills)
    slot: Mapped[str] = mapped_column(nullable=False)
    # the choice that made this one possible (background -> feat -> that feat's spells). No cascade:
    # the API removes or re-links dependents explicitly, so nothing disappears silently
    granted_by_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.character_choice.id"), nullable=True)
    # what was picked, when it's a catalog row (at most one)
    species_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.species.id"), nullable=True)
    subclass_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.subclass.id"), nullable=True)
    background_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.background.id"), nullable=True)
    feat_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.feat.id"), nullable=True)
    spell_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.spell.id"), nullable=True)
    item_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.item.id"), nullable=True)
    # picks that aren't catalog rows: {"skill": "arcana"}, {"increases": {"int": 2, "wis": 1}}
    value: Mapped[dict[str, Any] | None] = mapped_column(nullable=True)
    # picked with a default filter switched off
    overrides_filter: Mapped[bool] = mapped_column(server_default="false", nullable=False)


class CharacterItem(MixinBookeeping, DndSchemaBase):
    """An item the character carries (catalog or custom)."""

    __tablename__ = "character_item"
    __table_args__ = (
        CheckConstraint("item_id IS NOT NULL OR custom_name IS NOT NULL", name="item_or_custom"),
        CheckConstraint("quantity >= 0", name="quantity"),
        Index("ix_dnd_character_item_character_id", "character_id"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    character_id: Mapped[uuid.UUID] = _character_id()
    # null for a custom item
    item_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey("dnd.item.id"), nullable=True)
    custom_name: Mapped[str | None] = mapped_column(nullable=True)
    custom_description: Mapped[str | None] = mapped_column(nullable=True)
    quantity: Mapped[int] = mapped_column(server_default="1", nullable=False)
    equipped: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    attuned: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    # hidden while current_level is below it
    level_received: Mapped[int | None] = mapped_column(nullable=True)
    # "from Gerald, session 3"
    note: Mapped[str | None] = mapped_column(nullable=True)
    # starting equipment from a class or background option; the item stays if that choice goes
    granted_by_id: Mapped[uuid.UUID | None] = mapped_column(
        ForeignKey("dnd.character_choice.id", ondelete="SET NULL"), nullable=True
    )
    # a gift from a campaign's DM: a custom one's name and description are the DMs' to change, its
    # quantity, equipped, attuned and note the player's
    given_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)
    given_campaign_id: Mapped[uuid.UUID | None] = mapped_column(
        ForeignKey("dnd.campaign.id", ondelete="SET NULL"), nullable=True
    )


class CharacterPreparedSpell(MixinBookeeping, DndSchemaBase):
    """A class spell the character has prepared (or knows). Over the limit is a warning, not an error."""

    __tablename__ = "character_prepared_spell"
    __table_args__ = ({"schema": "dnd"},)

    character_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(CHARACTER_FK, ondelete="CASCADE"), primary_key=True)
    spell_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.spell.id"), primary_key=True)
    # which class casts it (needed once multiclassing exists)
    class_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.dnd_class.id"), primary_key=True)


class CharacterModifier(MixinBookeeping, DndSchemaBase):
    """A custom bonus or penalty: a boon, a curse, a spell cast on the character.

    In the modifier shape of VTT_ROADMAP.md 4.1. The hard limits still apply (scores stay 1-30).
    """

    __tablename__ = "character_modifier"
    __table_args__ = (
        CheckConstraint("added_as IN ('player', 'dm')", name="added_as"),
        Index("ix_dnd_character_modifier_character_id", "character_id"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    character_id: Mapped[uuid.UUID] = _character_id()
    # "Blessing of Pelor (DM)", "Mage Armor"
    label: Mapped[str] = mapped_column(nullable=False)
    note: Mapped[str | None] = mapped_column(nullable=True)
    # ability.str, ac, speed.walk, skill.stealth, save.dex, hp.max, initiative, ...
    target: Mapped[str] = mapped_column(nullable=False)
    # add, set, adv, disadv, resist, grant, ...
    op: Mapped[str] = mapped_column(nullable=False)
    # 2, -1, {"dice": "1d4"}
    value: Mapped[Any] = mapped_column(JSONB, nullable=False)
    # applies from this level up
    from_level: Mapped[int | None] = mapped_column(nullable=True)
    # a curse can be switched off without being deleted
    active: Mapped[bool] = mapped_column(server_default="true", nullable=False)
    added_as: Mapped[str] = mapped_column(server_default="player", nullable=False)
    # given by this campaign's DM: it counts only while the character is in the campaign, and only its
    # DMs change it
    campaign_id: Mapped[uuid.UUID | None] = mapped_column(
        ForeignKey("dnd.campaign.id", ondelete="CASCADE"), nullable=True
    )


class CharacterEditor(MixinBookeeping, DndSchemaBase):
    """Someone the owner lets edit the character's build: another player, the DM. created_by granted it."""

    __tablename__ = "character_editor"
    __table_args__ = (
        Index("ix_dnd_character_editor_user_id", "user_id"),
        {"schema": "dnd"},
    )

    character_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(CHARACTER_FK, ondelete="CASCADE"), primary_key=True)
    user_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(USER_ID_FK, ondelete="CASCADE"), primary_key=True)


class CharacterTransfer(MixinBookeeping, DndSchemaBase):
    """A pending hand-over of a character.

    The owner offers it to someone, who accepts or declines before it expires. One at a time per
    character; the change log keeps the history.
    """

    __tablename__ = "character_transfer"
    __table_args__ = (
        Index("ix_dnd_character_transfer_to_user_id", "to_user_id"),
        {"schema": "dnd"},
    )

    character_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(CHARACTER_FK, ondelete="CASCADE"), primary_key=True)
    to_user_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(USER_ID_FK, ondelete="CASCADE"), nullable=False)
    expires_at: Mapped[datetime.datetime] = mapped_column(DateTime(timezone=True), nullable=False)


class CharacterChange(DndSchemaBase):
    """The audit log: one row per change, written in the same transaction.

    Append-only, so it has created_* but no updated_*.
    """

    __tablename__ = "character_change"
    __table_args__ = (
        Index("ix_dnd_character_change_character_id_created_at", "character_id", "created_at"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = _id()
    character_id: Mapped[uuid.UUID] = _character_id()
    created_at: Mapped[datetime.datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), nullable=False
    )
    created_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)
    # level.set, choice.add, choice.update, choice.remove, item.add, scores.set, abilities.roll, ...
    action: Mapped[str] = mapped_column(nullable=False)
    # the changed row: character_choice + its id, ...
    entity: Mapped[str | None] = mapped_column(nullable=True)
    entity_id: Mapped[uuid.UUID | None] = mapped_column(nullable=True)
    before: Mapped[Any | None] = mapped_column(JSONB, nullable=True)
    after: Mapped[Any | None] = mapped_column(JSONB, nullable=True)
    note: Mapped[str | None] = mapped_column(nullable=True)
    # the client's operation id: a replayed offline operation is applied once
    op_id: Mapped[uuid.UUID | None] = mapped_column(unique=True, nullable=True)
