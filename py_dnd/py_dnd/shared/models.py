"""Shared models."""

import datetime
import uuid

from sqlalchemy import DateTime, ForeignKey, Index, func, text
from sqlalchemy.orm import Mapped, mapped_column

# Audit and soft-delete columns point at dnd.user (auth.users only exists in go_sprint). They're
# SET NULL, so deleting the user who imported the catalog doesn't delete the catalog.
USER_ID_FK = "dnd.user.id"


class MixinBookeeping:
    """Generic bookeeping methods applied to most tables."""

    created_at: Mapped[datetime.datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), nullable=False
    )
    created_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)
    updated_at: Mapped[datetime.datetime] = mapped_column(
        DateTime(timezone=True),
        index=True,
        server_default=func.now(),
        onupdate=func.now(),
        nullable=False,
    )
    updated_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)


class MixinSoftDelete:
    """Soft delete: reads skip rows with ``deleted_at`` set (see RepositoryBase)."""

    deleted_at: Mapped[datetime.datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    deleted_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)


def soft_delete_index(table: str) -> Index:
    """Index for finding deleted rows (purge jobs), per the schema standards."""
    return Index(f"ix_dnd_{table}_deleted_at", "deleted_at", postgresql_where=text("deleted_at IS NOT NULL"))
