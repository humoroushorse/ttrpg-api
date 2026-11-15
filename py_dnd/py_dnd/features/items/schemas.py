"""Item schemas (see ``features/catalog/schemas.py`` for the pattern)."""

from typing import Any

from pydantic import Field

from py_dnd.features.catalog.schemas import CatalogCreateRow, CatalogFields, CatalogRow


class ItemFields(CatalogFields):
    """Every field a client writes."""

    category: str | None = Field(default=None, title="Category", description='"weapon", "armor", "wondrous-item", ...')
    subcategory: str | None = Field(
        default=None, title="Subcategory", description='"Martial Melee Weapons", "Light Armor"'
    )
    cost: str | None = Field(default=None, title="Cost", description='as printed: "25 GP" (copper int later)')
    weight: str | None = Field(default=None, title="Weight", description='as printed: "1 lb."')
    properties: dict[str, Any] = Field(
        default_factory=dict,
        title="Properties",
        description="the table's other columns: Damage, Properties, Mastery, Armor Class, ...",
    )
    effects: list[dict[str, Any]] = Field(
        default_factory=list, title="Effects", description="typed modifiers (the ETL's effect atoms)"
    )
    is_magic: bool = Field(default=False, title="Is Magic")
    magic_item_type: str | None = Field(
        default=None, title="Magic Item Type", description='"Weapon", "Wondrous Item", "Ring"'
    )
    base_item: str | None = Field(default=None, title="Base Item", description='"Any Melee Weapon", "any sword"')
    rarity: str | None = Field(default=None, title="Rarity", description="common ... artifact, varies, unknown")
    rarity_text: str | None = Field(
        default=None, title="Rarity Text", description="as printed when it isn't one value"
    )
    requires_attunement: bool = Field(default=False, title="Requires Attunement")
    attunement_requirement: str | None = Field(
        default=None, title="Attunement Requirement", description='"by a wizard"'
    )


class ItemSchema(ItemFields, CatalogRow):
    """An item as the API returns it."""


class ItemCreate(ItemFields, CatalogCreateRow):
    """A new item row (POST and the bulk upload)."""
