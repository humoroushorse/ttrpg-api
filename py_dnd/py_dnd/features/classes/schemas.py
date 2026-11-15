"""Class schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class DndClassFields(CatalogFields):
    """Every field a client writes."""

    hit_die: str = Field(title="Hit Die", description='as printed: "1d10 per fighter level"')
    hit_die_size: int | None = Field(default=None, title="Hit Die Size", description="the die's sides (10)")
    primary_ability: str = Field(default="", title="Primary Ability", description='"Strength or Dexterity"')
    proficiencies: dict[str, Any] = Field(
        default_factory=dict,
        title="Proficiencies",
        description="armor, weapons, tools, saving_throws, skills {choose, from}",
    )
    starting_equipment: str = Field(
        default="", title="Starting Equipment", description="as printed (text until the 5e.tools pass)"
    )
    features: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Features",
        description="name, level, levels, description (+ json / markdown / html), effects",
    )
    subclass_slugs: list[str] = Field(
        default_factory=list, title="Subclass Slugs", description="its subclasses' slugs (artificer_alchemist)"
    )
    spell_list: str | None = Field(default=None, title="Spell List", description="unused by the ETL so far")
    spellcasting_ability: str | None = Field(
        default=None, title="Spellcasting Ability", description="int, wis, cha; null when it doesn't cast"
    )
    caster_progression: str | None = Field(
        default=None, title="Caster Progression", description="full, half, half_up (2024 Paladin), third, pact"
    )
    level_table: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Level Table",
        description="levels 1-20: level, cantrips, prepared, slots (spell levels 1-9), pact_slots, "
        "pact_slot_level, columns (the rest of the table as printed)",
    )
    spell_preparation: dict[str, Any] | None = Field(
        default=None,
        title="Spell Preparation",
        description="change: long_rest (prepared from the class's list) or level_up (learned at each level); "
        "spellbook: spells added at each level (Wizard); null when it doesn't cast",
    )
    starting_equipment_options: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Starting Equipment Options",
        description="the A / B packages: label, coins, grants (catalog items by name, quantity), picks (an item "
        "of a category), custom (items with no catalog entry)",
    )


class DndClassSchema(DndClassFields, CatalogRow):
    """A class as the API returns it."""


class DndClassCreate(DndClassFields, CatalogCreateRow):
    """A new class row (POST and the bulk upload)."""
