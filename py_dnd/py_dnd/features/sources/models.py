"""SQLAlchemy Table: source definition."""

import uuid
import uuid6

from sqlalchemy import UniqueConstraint
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from py_dnd import shared
from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import MixinBookeeping


class Source(MixinBookeeping, DndSchemaBase):
    """SQLAlchemy source model."""
    __tablename__ = "source"
    __table_args__ = (
        UniqueConstraint("name", "dnd_version", "dnd_version_year", name="uq_source_name_version_year"),
        UniqueConstraint("name_short", "dnd_version", "dnd_version_year", name="uq_source_name_short_version_year"),
        {"schema": shared.enums.DbSchemaEnum.DND.value},
    )

    # keys
    id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True),
        primary_key=True, 
        index=True,
        default=uuid6.uuid7
    )
    # fields
    name: Mapped[str] = mapped_column(nullable=False, unique=True)
    name_short: Mapped[str] = mapped_column(nullable=False, unique=True)
    dnd_version: Mapped[str] = mapped_column(nullable=False)
    dnd_version_year: Mapped[int] = mapped_column(default=None)
    publish_year: Mapped[int | None] = mapped_column(default=None)
