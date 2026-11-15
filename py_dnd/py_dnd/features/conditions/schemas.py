"""Condition schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow, LevelRow


class ConditionFields(CatalogFields):
    """Every field a client writes."""

    levels: list[LevelRow] = Field(default_factory=list, title="Levels", description="Exhaustion's levels.")
    effects: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Effects",
        description="Typed modifiers for the rules engine (the ETL's effect atoms).",
    )


class ConditionSchema(ConditionFields, CatalogRow):
    """A condition as the API returns it."""


class ConditionCreate(ConditionFields, CatalogCreateRow):
    """A new condition row (POST and the bulk upload)."""
