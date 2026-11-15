"""SQLAlchemy Table: user definition."""

import uuid

from sqlalchemy import text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import MixinBookeeping, MixinSoftDelete, soft_delete_index


class User(MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """SQLAlchemy user model: the app's side of a Keycloak user.

    ``deleted_at`` is when the user asked to be deleted; the row is purged 14 days later.
    """

    __tablename__ = "user"
    __table_args__ = (soft_delete_index("user"), {"schema": "dnd"})

    # keys
    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    keycloak_id: Mapped[uuid.UUID] = mapped_column(unique=True, nullable=False)  # the token's `sub`
    # fields
    username: Mapped[str | None] = mapped_column(default=None, nullable=True)
    profile_picture_url: Mapped[str | None] = mapped_column(default=None, nullable=True)
