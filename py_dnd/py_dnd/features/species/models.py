"""SQLAlchemy Table: species definition."""

from typing import Any

from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Species(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A species (2024) or race / lineage (2014)."""

    __tablename__ = "species"
    __table_args__ = catalog_table_args("species")

    # "Medium", "Small", "Medium or Small"
    size: Mapped[str] = mapped_column(nullable=False)
    # walking speed, "30 feet"
    speed: Mapped[str] = mapped_column(nullable=False)
    # "Humanoid", "Fey", ...
    creature_type: Mapped[str] = mapped_column(nullable=False)
    # name, description (+ markdown / html), level, choices, effects
    traits: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
