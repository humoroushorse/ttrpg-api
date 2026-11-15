"""Species schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class SpeciesFields(CatalogFields):
    """Every field a client writes."""

    size: str = Field(title="Size", description='"Medium", "Small", "Medium or Small"')
    speed: str = Field(title="Speed", description='walking speed, "30 feet"')
    creature_type: str = Field(title="Creature Type", description='"Humanoid", "Fey", ...')
    traits: list[dict[str, Any]] = Field(
        default_factory=list,
        title="Traits",
        description="name, description (+ markdown / html), level, choices, effects",
    )


class SpeciesSchema(SpeciesFields, CatalogRow):
    """A species as the API returns it."""


class SpeciesCreate(SpeciesFields, CatalogCreateRow):
    """A new species row (POST and the bulk upload)."""
