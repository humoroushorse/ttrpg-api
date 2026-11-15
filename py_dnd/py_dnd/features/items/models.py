"""SQLAlchemy Table: item definition."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Item(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """An item: mundane gear and magic items share the table (is_magic)."""

    __tablename__ = "item"
    __table_args__ = catalog_table_args("item")

    # "weapon", "armor", "wondrous-item", ...
    category: Mapped[str | None] = mapped_column(nullable=True)
    # "Martial Melee Weapons", "Light Armor"
    subcategory: Mapped[str | None] = mapped_column(nullable=True)
    # as printed: "25 GP" (copper int later)
    cost: Mapped[str | None] = mapped_column(nullable=True)
    # as printed: "1 lb."
    weight: Mapped[str | None] = mapped_column(nullable=True)
    # the table's other columns: Damage, Properties, Mastery, Armor Class, ...
    properties: Mapped[dict[str, Any]] = mapped_column(server_default=text("'{}'::jsonb"), nullable=False)
    # typed modifiers (the ETL's effect atoms)
    effects: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    is_magic: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    # "Weapon", "Wondrous Item", "Ring"
    magic_item_type: Mapped[str | None] = mapped_column(nullable=True)
    # "Any Melee Weapon", "any sword"
    base_item: Mapped[str | None] = mapped_column(nullable=True)
    # common ... artifact, varies, unknown
    rarity: Mapped[str | None] = mapped_column(nullable=True)
    # as printed when it isn't one value
    rarity_text: Mapped[str | None] = mapped_column(nullable=True)
    requires_attunement: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    # "by a wizard"
    attunement_requirement: Mapped[str | None] = mapped_column(nullable=True)
