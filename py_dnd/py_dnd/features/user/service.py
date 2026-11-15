"""User service: map the signed-in Keycloak user to its dnd.user row."""

import uuid
from typing import Annotated

from fastapi import Depends
from loguru import logger
from sqlalchemy import select
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.db import AsyncMasterSessionDependency
from py_dnd.features.auth.schemas import AuthUserToken
from py_dnd.features.auth.service import UserAuth
from py_dnd.features.user.models import User


async def ensure_user_id(session: AsyncSession, token: AuthUserToken) -> uuid.UUID:
    """The dnd.user id for a Keycloak user, creating the row if it's missing.

    Users made straight in Keycloak (the bulk-upload admin) have no row until their first write.
    Commits only when it creates one.
    """
    keycloak_id = uuid.UUID(token.sub)
    found = await session.scalar(select(User.id).where(User.keycloak_id == keycloak_id, User.deleted_at.is_(None)))
    if found:
        return found
    created = await session.scalar(
        insert(User)
        .values(keycloak_id=keycloak_id, username=token.preferred_username)
        .on_conflict_do_nothing(index_elements=[User.keycloak_id])
        .returning(User.id)
    )
    if created is None:  # a concurrent request made it first, or the row is soft-deleted
        created = await session.scalar(select(User.id).where(User.keycloak_id == keycloak_id))
    else:
        logger.info("Created dnd.user {} for Keycloak user {}", created, keycloak_id)
    await session.commit()
    if created is None:
        raise RuntimeError(f"no dnd.user row for Keycloak user {keycloak_id}")
    return created


async def get_current_user_id(current_user: UserAuth, db: AsyncMasterSessionDependency) -> uuid.UUID:
    """FastAPI dependency: the signed-in user's dnd.user id (for created_by, updated_by, ...)."""
    return await ensure_user_id(db, current_user)


CurrentUserId = Annotated[uuid.UUID, Depends(get_current_user_id)]
