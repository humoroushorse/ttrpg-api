"""SQLAlchemy Table: dnd_events definition."""

from __future__ import annotations

import uuid

from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase


class User(DndSchemaBase):
    """SQLAlchemy user model."""

    __tablename__ = "user"

    # keys
    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, index=True, default=uuid.uuid4)
    # fields
    username: Mapped[str | None] = mapped_column(default=None, nullable=True)
    profile_picture_url: Mapped[str | None] = mapped_column(default=None, nullable=True)
