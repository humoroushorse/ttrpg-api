"""Shared models."""

import datetime

from sqlalchemy import func
from sqlalchemy.orm import Mapped, mapped_column


class MixinBookeeping:
    """Generic bookeeping methods applied to most tables."""

    created_at: Mapped[datetime.datetime] = mapped_column(server_default=func.now(), nullable=False)
    created_by: Mapped[str] = mapped_column(nullable=False)
    updated_at: Mapped[datetime.datetime | None] = mapped_column(
        index=True,
        onupdate=func.now(),
        nullable=True,
    )
    updated_by: Mapped[str | None] = mapped_column(nullable=True)
