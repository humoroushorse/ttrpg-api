"""SQLAlchemy Table: condition definition."""

from typing import Any

from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Condition(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A condition (Blinded, Exhaustion, ...)."""

    __tablename__ = "condition"
    __table_args__ = catalog_table_args("condition")

    # leveled effects (Exhaustion); empty otherwise
    levels: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
    # typed modifiers for the rules engine (the ETL's effect atoms)
    effects: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
