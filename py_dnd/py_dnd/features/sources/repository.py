"""Sources repository."""

from __future__ import annotations

from collections.abc import Sequence

import loguru
from pydantic import BaseModel
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.exceptions import handle_sqlalchemy_errors_decorator
from py_dnd.features.core.repository import RepositoryBase
from py_dnd.features.sources.models import Source
from py_dnd.features.sources.schemas import (
    SourceCreate,
    SourceSchema,
    SourceSchemaBase,
    SourceUpdate,
)


class SourceRepository(RepositoryBase[Source, SourceSchema, SourceSchemaBase, SourceCreate, SourceUpdate]):
    """Source Repository."""

    def __init__(self, session: AsyncSession, logger: loguru.Logger | None = None):
        super().__init__(session=session, model=Source, schema=SourceSchema, schema_base=SourceSchemaBase)
        self.logger.trace("{} created!", self.__class__.__name__)

    @handle_sqlalchemy_errors_decorator
    async def read_by_natural_key(self, slug: str) -> Source | None:
        """The source with this slug, soft-deleted or not (the caller decides)."""
        return await self.session.scalar(select(Source).where(Source.slug == slug))

    @handle_sqlalchemy_errors_decorator
    async def read_by_slug(self, slug: str, fields: Sequence[str] | None = None) -> BaseModel | None:
        """The live source with this slug (phb_2024)."""
        source = await self.session.scalar(self._select(fields).where(Source.slug == slug))
        return None if source is None else self._out(source, fields)
