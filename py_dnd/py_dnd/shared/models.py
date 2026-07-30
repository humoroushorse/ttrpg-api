"""Shared models."""

import datetime
import uuid

from sqlalchemy import DateTime, func
from sqlalchemy.orm import Mapped, mapped_column


class MixinBookeeping:
    """Generic bookeeping methods applied to most tables."""

    created_at: Mapped[datetime.datetime] = mapped_column(
        DateTime(timezone=True), server_default=func.now(), nullable=False
    )
    created_by: Mapped[uuid.UUID] = mapped_column(nullable=False)
    updated_at: Mapped[datetime.datetime] = mapped_column(
        DateTime(timezone=True),
        index=True,
        default=func.now(),
        onupdate=func.now(),
        nullable=False,
    )
    updated_by: Mapped[uuid.UUID] = mapped_column(nullable=False)
