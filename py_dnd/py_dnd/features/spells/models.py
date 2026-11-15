"""SQLAlchemy Table: spell definition."""

import uuid
from typing import Any

from sqlalchemy import CheckConstraint, Computed, ForeignKey, UniqueConstraint, text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.sources.models import Source
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete, soft_delete_index


def _from_vtt(expression: str) -> Computed:
    """A STORED generated column (Postgres 18 defaults to VIRTUAL, which can't be indexed)."""
    return Computed(expression, persisted=True)


def _any_roll(condition: str) -> str:
    """Any roll in the base modes, cast or zone trigger, matching a jsonpath filter."""
    return f"vtt @? '$.modes[*].rolls[*] ? ({condition})' OR vtt @? '$.modes[*].ongoing[*].rolls[*] ? ({condition})'"


# The database only checks the version; the API validates the whole block on every write
VTT_V1_CHECK = "jsonb_typeof(vtt) = 'object' AND vtt ->> 'schema_version' = 'v1'"


class MixinVttSpell:
    """The VTT block and the columns generated from it, for any system's spell table (VTT_ROADMAP 4.2).

    Add ``CheckConstraint(VTT_V1_CHECK, name="vtt_v1")`` to the table's args. ``vtt`` follows
    ``ttrpg-api/schemas/vtt_spell.v1.schema.json``. The generated columns describe the cast
    (``modes[0]``) for filtering and sorting, like the ETL's ``VttSpellFields.columns()``.
    """

    vtt: Mapped[dict[str, Any]] = mapped_column(nullable=False)
    activation_cost: Mapped[str | None] = mapped_column(_from_vtt("vtt #>> '{modes,0,activation,cost}'"))
    activation_amount: Mapped[int | None] = mapped_column(
        _from_vtt("COALESCE((vtt #>> '{modes,0,activation,amount}')::integer, 1)")
    )
    range_kind: Mapped[str | None] = mapped_column(_from_vtt("vtt #>> '{range,kind}'"))
    range_feet: Mapped[int | None] = mapped_column(_from_vtt("(vtt #>> '{range,feet}')::integer"))
    duration_kind: Mapped[str | None] = mapped_column(_from_vtt("vtt #>> '{duration,kind}'"))
    duration_amount: Mapped[int | None] = mapped_column(_from_vtt("(vtt #>> '{duration,amount}')::integer"))
    duration_unit: Mapped[str | None] = mapped_column(_from_vtt("vtt #>> '{duration,unit}'"))
    sustain: Mapped[str | None] = mapped_column(_from_vtt("vtt ->> 'sustain'"))
    has_attack_roll: Mapped[bool | None] = mapped_column(
        _from_vtt(_any_roll('@.roll == "attack" || @.roll == "weapon_attack"'))
    )
    has_save: Mapped[bool | None] = mapped_column(_from_vtt(_any_roll('@.roll == "save"')))
    has_summon: Mapped[bool | None] = mapped_column(_from_vtt("vtt @? '$.modes[*].summons[*]'"))


class Spell(MixinVttSpell, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """SQLAlchemy spell model."""

    __tablename__ = "spell"

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    source_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(Source.id), nullable=False)
    # natural key with source_id: names change (punctuation fixes), slugs don't
    slug: Mapped[str] = mapped_column(nullable=False)
    # fields
    name: Mapped[str] = mapped_column(nullable=False)
    dnd_version: Mapped[str] = mapped_column(nullable=False)
    dnd_version_year: Mapped[int] = mapped_column(nullable=False)
    source_page: Mapped[int | None] = mapped_column(default=None, nullable=True)
    level: Mapped[int] = mapped_column(nullable=False)
    school: Mapped[str] = mapped_column(nullable=False)
    is_ritual: Mapped[bool] = mapped_column(nullable=False)
    is_unearthed_arcana: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    casting_time: Mapped[str] = mapped_column(nullable=False)
    range: Mapped[str] = mapped_column(nullable=False)
    has_verbal_component: Mapped[bool] = mapped_column(nullable=False)
    has_somatic_component: Mapped[bool] = mapped_column(nullable=False)
    has_material_component: Mapped[bool] = mapped_column(nullable=False)
    materials: Mapped[str | None] = mapped_column(default=None, nullable=True)
    has_spell_cost: Mapped[bool] = mapped_column(nullable=False)
    are_materials_consumed: Mapped[bool] = mapped_column(nullable=False)
    duration: Mapped[str] = mapped_column(nullable=False)
    is_concentration: Mapped[bool] = mapped_column(nullable=False)
    description: Mapped[str] = mapped_column(nullable=False)
    has_saving_throw: Mapped[bool] = mapped_column(nullable=False)
    difficulty_class_saving_throw_override: Mapped[int | None] = mapped_column(default=None, nullable=True)
    damage_type: Mapped[str | None] = mapped_column(default=None, nullable=True)
    at_higher_levels: Mapped[str | None] = mapped_column(default=None, nullable=True)
    difficulty_class_saving_throw: Mapped[str | None] = mapped_column(default=None, nullable=True)
    difficulty_class_type: Mapped[str | None] = mapped_column(default=None, nullable=True)
    stat_blocks: Mapped[list[dict[str, Any]] | None] = mapped_column(default=None)
    is_homebrew: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    # relationships
    # joined: every read returns the source, and an async session can't lazy-load it later
    source: Mapped[Source] = relationship(Source, lazy="joined", innerjoin=True)

    __table_args__ = (
        UniqueConstraint("source_id", "slug", name="uq_spell_source_id_slug"),
        CheckConstraint(VTT_V1_CHECK, name="vtt_v1"),
        soft_delete_index("spell"),
        {"schema": "dnd"},
    )
