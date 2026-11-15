"""SQLAlchemy Table: feat definition."""

from typing import Any

from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.catalog.models import EMPTY_JSON_LIST, MixinCatalogEntry, catalog_table_args
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete


class Feat(MixinCatalogEntry, MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A feat (Alert, Magic Initiate, ...)."""

    __tablename__ = "feat"
    __table_args__ = catalog_table_args("feat")

    # "Origin", "General", "Epic Boon"; often missing (TODO)
    category: Mapped[str | None] = mapped_column(nullable=True)
    # text, min_level, ability_minimums, feats, features, species, proficiencies
    prerequisites: Mapped[dict[str, Any] | None] = mapped_column(nullable=True)
    # typed modifiers (the ETL's effect atoms)
    effects: Mapped[list[dict[str, Any]]] = mapped_column(server_default=EMPTY_JSON_LIST, nullable=False)
