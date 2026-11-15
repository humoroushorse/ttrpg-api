"""SQLAlchemy Table: background definition."""

from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Background(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A background (Acolyte, Sage, ...)."""

    __tablename__ = "background"
    __table_args__ = catalog_table_args("background")

    # 2024, as printed: "Strength, Constitution, Charisma"
    ability_score_bonuses: Mapped[str] = mapped_column(server_default="", nullable=False)
    skill_proficiencies: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
    tool_proficiencies: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
    # 2024, as printed: "Magic Initiate (Cleric)"
    feat: Mapped[str] = mapped_column(server_default="", nullable=False)
    # the wiki's Equipment line
    equipment_text: Mapped[str] = mapped_column(server_default="", nullable=False)
    # [{label, items, coins}] (A / B in 2024)
    equipment_options: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
