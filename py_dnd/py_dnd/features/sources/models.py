"""SQLAlchemy Table: source definition."""

import uuid

from sqlalchemy import UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import MixinBookeeping


class Source(MixinBookeeping, DndSchemaBase):
    """SQLAlchemy source model."""

    __tablename__ = "source"
    __table_args__ = (
        UniqueConstraint("name", "dnd_version", "dnd_version_year", name="uq_source_name_version"),
        UniqueConstraint("name_short", "dnd_version", "dnd_version_year", name="uq_source_name_short_version"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, index=True)
    # fields
    name: Mapped[str] = mapped_column(nullable=False)
    name_short: Mapped[str] = mapped_column(nullable=False)
    dnd_version: Mapped[str] = mapped_column(nullable=False)
    dnd_version_year: Mapped[int] = mapped_column(default=None)
    publish_year: Mapped[int | None] = mapped_column(default=None)
