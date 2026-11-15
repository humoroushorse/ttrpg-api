"""Database."""

from collections.abc import AsyncGenerator
from typing import Annotated

from fastapi import Depends
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from .session import master_sessionmanager, replica_sessionmanager


async def get_master_db() -> AsyncGenerator[AsyncSession]:
    """Get master database session (async)."""
    async with master_sessionmanager.session() as session:
        yield session


AsyncMasterSessionDependency = Annotated[async_sessionmaker, Depends(get_master_db)]


async def get_replica_db() -> AsyncGenerator[AsyncSession]:
    """Get replica session (async)."""
    async with replica_sessionmanager.session() as session:
        yield session


AsyncReplicaSessionDependency = Annotated[async_sessionmaker, Depends(get_replica_db)]
