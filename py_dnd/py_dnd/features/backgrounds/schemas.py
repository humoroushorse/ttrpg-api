"""Background schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class BackgroundFields(CatalogFields):
    """Every field a client writes."""

    ability_score_bonuses: str = Field(
        default="", title="Ability Score Bonuses", description='2024, as printed: "Strength, Constitution, Charisma"'
    )
    skill_proficiencies: list[str] = Field(default_factory=list, title="Skill Proficiencies")
    tool_proficiencies: list[str] = Field(default_factory=list, title="Tool Proficiencies")
    feat: str = Field(default="", title="Feat", description='2024, as printed: "Magic Initiate (Cleric)"')
    equipment_text: str = Field(default="", title="Equipment Text", description="the wiki's Equipment line")
    equipment_options: list[dict[str, Any]] = Field(
        default_factory=list, title="Equipment Options", description="[{label, items, coins}] (A / B in 2024)"
    )


class BackgroundSchema(BackgroundFields, CatalogRow):
    """A background as the API returns it."""


class BackgroundCreate(BackgroundFields, CatalogCreateRow):
    """A new background row (POST and the bulk upload)."""
