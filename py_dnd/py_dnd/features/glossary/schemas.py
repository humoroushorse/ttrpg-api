"""Glossary entry schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any, Literal

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow, LevelRow

GlossaryCategory = Literal["action", "area_of_effect", "hazard", "attitude", "general"]
GLOSSARY_CATEGORIES: tuple[str, ...] = GlossaryCategory.__args__  # type: ignore[attr-defined]


class GlossaryEntryFields(CatalogFields):
    """Every field a client writes."""

    category: GlossaryCategory = Field(default="general", title="Category")
    levels: list[LevelRow] = Field(default_factory=list, title="Levels")
    effects: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Effects",
        description="Typed modifiers for the rules engine (the ETL's effect atoms).",
    )


class GlossaryEntrySchema(GlossaryEntryFields, CatalogRow):
    """A glossary entry as the API returns it."""


class GlossaryEntryCreate(GlossaryEntryFields, CatalogCreateRow):
    """A new glossary entry row (POST and the bulk upload)."""
