"""SQLAlchemy Table: subclass definition."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Subclass(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A subclass (School of Evocation, Battle Master, ...)."""

    __tablename__ = "subclass"
    __table_args__ = catalog_table_args("subclass")

    # the class's slug (wizard); same edition as the subclass
    parent_class_slug: Mapped[str] = mapped_column(nullable=False)
    # the class level it's chosen at
    level_gained: Mapped[int] = mapped_column(server_default="0", nullable=False)
    # same shape as a class's features
    features: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # spell names (not linked yet)
    spell_list_additions: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
    # from 5e.tools: int, wis, cha; null for a subclass that doesn't add casting
    spellcasting_ability: Mapped[str | None] = mapped_column(nullable=True)
    # full, half, half_up (2024 Paladin), third, pact
    caster_progression: Mapped[str | None] = mapped_column(nullable=True)
    # levels 1-20: {level, cantrips, prepared, slots[9], pact_slots, pact_slot_level, columns}
    level_table: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # same shape as a class's; spell_list is the class list it picks from (Eldritch Knight: Wizard)
    spell_preparation: Mapped[dict[str, Any] | None] = mapped_column(nullable=True)
