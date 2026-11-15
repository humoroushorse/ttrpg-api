"""Feat schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class FeatFields(CatalogFields):
    """Every field a client writes."""

    category: str | None = Field(
        default=None, title="Category", description='"Origin", "General", "Epic Boon"; often missing (TODO)'
    )
    prerequisites: dict[str, Any] | None = Field(
        default=None,
        title="Prerequisites",
        description="text, min_level, ability_minimums, feats, features, species, proficiencies",
    )
    effects: list[dict[str, Any]] = Field(
        default_factory=list, title="Effects", description="typed modifiers (the ETL's effect atoms)"
    )


class FeatSchema(FeatFields, CatalogRow):
    """A feat as the API returns it."""


class FeatCreate(FeatFields, CatalogCreateRow):
    """A new feat row (POST and the bulk upload)."""
