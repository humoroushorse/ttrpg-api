"""User repository."""

from __future__ import annotations

import uuid

import loguru
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.features.core.repository import RepositoryBase
from py_dnd.features.user.models import User
from py_dnd.features.user.schemas import UserCreate, UserUpdate


class UserRepository(RepositoryBase[User, UserCreate, UserUpdate]):
    """User Repository.

    Args:
        RepositoryBase (_type_): _description_
    """

    def __init__(self, session: AsyncSession, logger: loguru.Logger | None = None):
        super().__init__(session=session, model=User)
        self.logger.trace("{} created!", self.__class__.__name__)

    async def delete(self, entity_id: uuid.UUID | str) -> uuid.UUID:
        """Delete user by UUID.

        Args:
            entity_id: UUID or string representation of UUID

        Returns:
            uuid.UUID: ID of deleted entity
        """
        if isinstance(entity_id, str):
            entity_id = uuid.UUID(entity_id)
        user = await self.read_by_id(entity_id)
        if user:
            await self.session.delete(user)
            return user.id
        raise ValueError(f"User with id {entity_id} not found")
