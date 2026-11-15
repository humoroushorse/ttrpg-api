"""SQLAlchemy Table: source definition."""

import uuid

from sqlalchemy import UniqueConstraint, text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete, soft_delete_index


class Source(MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """SQLAlchemy source model."""

    __tablename__ = "source"
    __table_args__ = (
        UniqueConstraint("name", "dnd_version", "dnd_version_year", name="uq_source_name_version"),
        UniqueConstraint("name_short", "dnd_version", "dnd_version_year", name="uq_source_name_short_version"),
        soft_delete_index("source"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    # natural key: the ETL's source_id ("phb_2024"), unique across editions
    slug: Mapped[str] = mapped_column(unique=True, nullable=False)
    # fields
    name: Mapped[str] = mapped_column(nullable=False)
    name_short: Mapped[str] = mapped_column(nullable=False)
    dnd_version: Mapped[str] = mapped_column(nullable=False)
    dnd_version_year: Mapped[int] = mapped_column(nullable=False)
    publish_year: Mapped[int | None] = mapped_column(default=None)
    # data/manual/sources/marks.json in the ETL ("third_party")
    marks: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
