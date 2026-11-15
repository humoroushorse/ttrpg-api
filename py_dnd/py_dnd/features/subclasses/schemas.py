"""Subclass schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class SubclassFields(CatalogFields):
    """Every field a client writes."""

    parent_class_slug: str = Field(
        title="Parent Class Slug", description="the class's slug (wizard); same edition as the subclass"
    )
    level_gained: int = Field(default=0, title="Level Gained", description="the class level it's chosen at")
    features: list[dict[str, Any]] = Field(
        default_factory=list, title="Features", description="same shape as a class's features"
    )
    spell_list_additions: list[str] = Field(
        default_factory=list, title="Spell List Additions", description="spell names (not linked yet)"
    )
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
        description="same shape as a class's: change (long_rest | level_up), spellbook, spell_list (the class "
        "list it picks from: Eldritch Knight's is Wizard)",
    )


class SubclassSchema(SubclassFields, CatalogRow):
    """A subclass as the API returns it."""


class SubclassCreate(SubclassFields, CatalogCreateRow):
    """A new subclass row (POST and the bulk upload)."""
