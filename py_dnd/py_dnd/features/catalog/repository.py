"""Repository for catalog entities."""

import uuid
from collections.abc import Sequence
from typing import Any

import loguru
from pydantic import BaseModel
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.exceptions import handle_sqlalchemy_errors_decorator
from py_dnd.features.core.repository import RepositoryBase
from py_dnd.features.sources.models import Source


class CatalogRepository(RepositoryBase[Any, Any, Any, Any, Any]):
    """A catalog table's repository: the base CRUD plus its (source, slug) lookups."""

    def __init__(
        self,
        session: AsyncSession,
        model: type[Any],
        schema: type[BaseModel],
        logger: loguru.Logger | None = None,
    ):
        super().__init__(session=session, model=model, schema=schema, schema_base=schema, logger=logger)

    @handle_sqlalchemy_errors_decorator
    async def read_by_natural_key(self, source_id: uuid.UUID, slug: str) -> Any | None:
        """The row with this (source_id, slug), soft-deleted or not (the caller decides)."""
        return await self.session.scalar(
            select(self.model).where(self.model.source_id == source_id, self.model.slug == slug)
        )

    @handle_sqlalchemy_errors_decorator
    async def read_by_slugs(
        self, source_slug: str, slug: str, fields: Sequence[str] | None = None
    ) -> BaseModel | None:
        """The live row with this slug in the source with this slug (the URL key: phb_2024/blinded)."""
        source_id = select(Source.id).where(Source.slug == source_slug).scalar_subquery()
        row = await self.session.scalar(
            self._select(fields).where(self.model.source_id == source_id, self.model.slug == slug)
        )
        return None if row is None else self._out(row, fields)
