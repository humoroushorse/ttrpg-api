"""Spell repository."""

from __future__ import annotations

import uuid
from collections.abc import Sequence

import loguru
from pydantic import BaseModel
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.exceptions import handle_sqlalchemy_errors_decorator
from py_dnd.features.core.repository import RepositoryBase
from py_dnd.features.sources.models import Source
from py_dnd.features.spells.models import Spell
from py_dnd.features.spells.schemas import (
    SpellCreate,
    SpellSchema,
    SpellSchemaBase,
    SpellUpdate,
)


class SpellRepository(RepositoryBase[Spell, SpellSchema, SpellSchemaBase, SpellCreate, SpellUpdate]):
    """Spell Repository.

    Args:
        RepositoryBase (_type_): _description_
    """

    def __init__(self, session: AsyncSession, logger: loguru.Logger | None = None):
        super().__init__(session=session, model=Spell, schema=SpellSchema, schema_base=SpellSchemaBase)
        self.logger.trace("{} created!", self.__class__.__name__)

    @handle_sqlalchemy_errors_decorator
    async def read_by_natural_key(self, source_id: uuid.UUID, slug: str) -> Spell | None:
        """The spell with this (source_id, slug), soft-deleted or not (the caller decides)."""
        return await self.session.scalar(select(Spell).where(Spell.source_id == source_id, Spell.slug == slug))

    @handle_sqlalchemy_errors_decorator
    async def read_by_slugs(
        self, source_slug: str, slug: str, fields: Sequence[str] | None = None
    ) -> BaseModel | None:
        """The live spell with this slug in the source with this slug (the URL key: phb_2024/acid-splash)."""
        source_id = select(Source.id).where(Source.slug == source_slug).scalar_subquery()
        spell = await self.session.scalar(self._select(fields).where(Spell.source_id == source_id, Spell.slug == slug))
        return None if spell is None else self._out(spell, fields)


# def get_spell_repository(session: AsyncSession) -> SpellRepository:
#     # This function will return an instance of the SpellRepository class
#     return SpellRepository(session)
