"""SQLAlchemy Table: class definition."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class DndClass(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A class (Wizard, Fighter, ...). Table dnd_class: class is a Python keyword."""

    __tablename__ = "dnd_class"
    __table_args__ = catalog_table_args("dnd_class")

    # as printed: "1d10 per fighter level"
    hit_die: Mapped[str] = mapped_column(nullable=False)
    # the die's sides (10)
    hit_die_size: Mapped[int | None] = mapped_column(nullable=True)
    # "Strength or Dexterity"
    primary_ability: Mapped[str] = mapped_column(server_default="", nullable=False)
    # armor, weapons, tools, saving_throws, skills {choose, from}
    proficiencies: Mapped[dict[str, Any]] = mapped_column(server_default=text("'{}'::jsonb"), nullable=False)
    # as printed (text until the 5e.tools pass)
    starting_equipment: Mapped[str] = mapped_column(server_default="", nullable=False)
    # name, level, levels, description (+ json / markdown / html), effects
    features: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # its subclasses' slugs (artificer_alchemist)
    subclass_slugs: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
    # unused by the ETL so far
    spell_list: Mapped[str | None] = mapped_column(nullable=True)
    # from 5e.tools: int, wis, cha; null for a class that doesn't cast
    spellcasting_ability: Mapped[str | None] = mapped_column(nullable=True)
    # full, half, half_up (2024 Paladin), third, pact
    caster_progression: Mapped[str | None] = mapped_column(nullable=True)
    # levels 1-20: {level, cantrips, prepared, slots[9], pact_slots, pact_slot_level, columns}
    level_table: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # from 5e.tools: {change: long_rest | level_up, spellbook: [spells added per level], spell_list};
    # null when it doesn't cast
    spell_preparation: Mapped[dict[str, Any] | None] = mapped_column(nullable=True)
    # from 5e.tools (2024): the A / B packages [{label, items, coins, grants, picks, custom}]
    starting_equipment_options: Mapped[list[dict[str, Any]]] = mapped_column(
        server_default=EMPTY_JSON_LIST, nullable=False
    )
