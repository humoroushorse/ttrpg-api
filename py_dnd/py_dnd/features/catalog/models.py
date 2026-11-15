"""Columns every catalog table shares."""

import uuid
from typing import Any

from sqlalchemy import ForeignKey, UniqueConstraint, text
from sqlalchemy.orm import Mapped, declared_attr, mapped_column, relationship

from py_dnd.features.sources.models import Source
from py_dnd.shared.models import soft_delete_index


class MixinCatalogEntry:
    """A book entry: its source, natural key and text. Use with ``catalog_table_args``."""

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    source_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(Source.id), nullable=False)
    # natural key with source_id: names change (punctuation fixes), slugs don't
    slug: Mapped[str] = mapped_column(nullable=False)
    name: Mapped[str] = mapped_column(nullable=False)
    dnd_version: Mapped[str] = mapped_column(nullable=False)
    dnd_version_year: Mapped[int] = mapped_column(nullable=False)
    is_unearthed_arcana: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    is_homebrew: Mapped[bool] = mapped_column(server_default="false", nullable=False)
    # data/manual/<entity>/marks.json in the ETL
    marks: Mapped[list[str]] = mapped_column(server_default=text("'{}'"), nullable=False)
    # HTML: the entry's text without a card heading (the ETL's description_html)
    description: Mapped[str] = mapped_column(nullable=False)

    @declared_attr
    def source(cls) -> Mapped[Source]:  # pylint: disable=no-self-argument
        """Joined: every read returns the source, and an async session can't lazy-load it later."""
        return relationship(Source, lazy="joined", innerjoin=True)


def catalog_table_args(table: str) -> tuple[Any, ...]:
    """``__table_args__`` for a catalog table: the (source, slug) key and the soft-delete index."""
    return (
        UniqueConstraint("source_id", "slug", name=f"uq_{table}_source_id_slug"),
        soft_delete_index(table),
        {"schema": "dnd"},
    )


# jsonb list columns default to an empty list
EMPTY_JSON_LIST = text("'[]'::jsonb")
