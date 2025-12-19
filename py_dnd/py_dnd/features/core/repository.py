"""Core repository classes/functions for building reposigtories."""

from __future__ import annotations

import uuid
from enum import Enum
from typing import Any, Generic, Sequence, TypeVar

import loguru
from fastapi.encoders import jsonable_encoder
from pydantic import BaseModel
from sqlalchemy import Result, Select, func, or_, select
from sqlalchemy.orm.attributes import InstrumentedAttribute
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.database.exceptions import handle_sqlalchemy_errors_decorator

ModelType = TypeVar("ModelType", bound=DndSchemaBase)
CreateSchemaType = TypeVar("CreateSchemaType", bound=BaseModel)
UpdateSchemaType = TypeVar("UpdateSchemaType", bound=BaseModel)


class RepositoryBase(Generic[ModelType, CreateSchemaType, UpdateSchemaType]):
    """Base repository with CRUD operations.

    Provides standard Create, Read, Update, Delete operations for SQLAlchemy models.
    
    Default ID type is UUID. Models using different ID types (int, str, etc.) should 
    override the read_by_id and related methods in their specific repository classes.

    Args:
        Generic: Type parameters for Model, CreateSchema, and UpdateSchema
    """

    def __init__(self, session: AsyncSession, model: type[ModelType], logger: loguru.Logger | None = None):
        """Initialize repository.

        Args:
            session: SQLAlchemy async session
            model: SQLAlchemy model class
            logger: Optional logger instance
        """
        self.session = session
        self.model = model
        self.logger = logger if logger else loguru.logger

    async def read_by_id(
        self,
        entity_id: uuid.UUID | str,
    ) -> ModelType | None:
        """Get an entity by UUID.

        Args:
            entity_id: UUID or string representation of UUID

        Returns:
            ModelType | None: Entity or None if not found
        """
        if isinstance(entity_id, str):
            entity_id = uuid.UUID(entity_id)
        stmt = select(self.model).where(self.model.id == entity_id)
        return await self.session.scalar(stmt.order_by(self.model.id))

    # async def get(self, db: AsyncSession, id: Any) -> Optional[ModelType]:
    #     stmt = select(self.model).where(self.model.id == id)
    #     result = await db.execute(stmt)
    #     return result.scalars().first()

    async def read_multi_by_ids(
        self,
        entity_ids: list[uuid.UUID | str],
    ) -> Sequence[ModelType]:
        """Get multiple entities by UUIDs.

        Args:
            entity_ids: List of UUIDs or string representations

        Returns:
            Sequence[ModelType]: List of entities
        """
        # Convert string UUIDs to UUID objects
        converted_ids = [uuid.UUID(eid) if isinstance(eid, str) else eid for eid in entity_ids]
        stmt = select(self.model).where(self.model.id.in_(converted_ids))
        result = await self.session.scalars(stmt.order_by(self.model.id))
        return result.all()

    async def read_multi(
        self,
        *,
        offset: int = 0,
        limit: int = 100,
    ) -> Sequence[ModelType]:
        """Get multiple entities (pagination optional).

        Args:
            offset (int, optional): _description_. Defaults to 0.
            limit (int, optional): _description_. Defaults to 100.

        Returns:
            Sequence[ModelType]: _description_
        """
        self.logger.debug("RepositoryBase::read_multi() called with offset={}, limit={}", offset, limit)
        stmt = select(self.model).offset(offset).limit(limit)
        result = await self.session.scalars(stmt.order_by(self.model.id))
        return result.all()

    # async def get_multi(self, db: AsyncSession, *, offset: int = 0, limit: int = 100) -> list[ModelType]:
    #     stmt = select(self.model).offset(offset).limit(limit)
    #     result = await db.execute(stmt)
    #     return result.scalars().all()

    @handle_sqlalchemy_errors_decorator
    async def create(self, *, model_in: CreateSchemaType, return_model: bool = True) -> ModelType | None:
        """Create an entity.

        Args:
            model_in (CreateSchemaType): _description_
            return_model (bool, optional): _description_. Defaults to True.

        Raises:
            RuntimeError: _description_

        Returns:
            ModelType: _description_
        """
        entity = self.model(**model_in.model_dump())
        self.session.add(entity)

        # To fetch entity
        if return_model:
            await self.session.flush()
            new = await self.read_by_id(entity.id)
            if not new:
                raise RuntimeError()
            return new
        return None

    # async def create(self, db: AsyncSession, *, obj_in: CreateSchemaType) -> ModelType:
    #     obj_in_data = jsonable_encoder(obj_in)
    #     db_obj = self.model(**obj_in_data)
    #     db.add(db_obj)
    #     await db.commit()
    #     await db.refresh(db_obj)
    #     return db_obj

    # async def update(
    #     self, session: AsyncSession, *, db_obj: ModelType, model_in: UpdateSchemaType | dict[str, Any]
    # ) -> None:
    #     self.notebook_id = notebook_id
    #     self.title = title
    #     self.content = content
    #     await self.session.flush()

    @handle_sqlalchemy_errors_decorator
    async def update(
        self,
        *,
        db_obj: ModelType,
        obj_in: UpdateSchemaType | dict[str, Any],
    ) -> ModelType | None:
        """Update an existing entity.

        Args:
            db_obj (ModelType): _description_
            obj_in (UpdateSchemaType | dict[str, Any]): _description_

        Returns:
            ModelType: _description_
        """
        obj_data = jsonable_encoder(db_obj)
        if isinstance(obj_in, dict):
            update_data = obj_in
        else:
            update_data = obj_in.model_dump(exclude_unset=True)
        for field in obj_data:
            if field in update_data:
                setattr(db_obj, field, update_data[field])
        self.session.add(db_obj)
        return db_obj

    @handle_sqlalchemy_errors_decorator
    async def delete(
        self,
        entity: ModelType,
    ) -> int:
        """Delete an entity.

        Args:
            entity (ModelType): _description_

        Returns:
            int: _description_
        """
        await self.session.delete(entity)
        return entity.id

    # async def delete(self, db: AsyncSession, *, id: int) -> ModelType:
    #     obj = await self.get(db, id)
    #     db.delete(obj)
    #     await db.commit()
    #     return obj

    @handle_sqlalchemy_errors_decorator
    async def query(
        self,
        params: dict[str, list[Any] | str | None] | None = None,
        *,
        order_by: Any | None = None,
        limit: int | None = 100,
        offset: int | None = 0,
        exact: bool = False,
    ) -> tuple[Sequence[ModelType], int]:
        """Query a list of Type[ModelType] with filters.

        Args:
            params: A dict of fields from Type[ModelType] to query.
            order_by: SQL 'ORDER BY' input. Defaults to None.
            limit: SQL 'LIMIT'. Defaults to 100.
            offset: SQL 'OFFSET'. Defaults to 0.
            exact: Whether to use exact matching for string filters.

        Returns:
            tuple[Sequence[ModelType], int]: A tuple of the entities and the total_count.
        """
        query: Select = select(self.model)
        if params:
            query = self.apply_param_filters_to_query(query=query, params=params, exact=exact)
        
        # Get count before limit/offset are applied (optimization: skip if no pagination)
        total_count: int | None = None
        if limit != 0 or offset != 0:
            count_query: Select = select(func.count()).select_from(query.subquery())
            count_result: Result = await self.session.execute(count_query)
            total_count = int(count_result.scalar_one())
        
        # Apply limit/offset/order_by
        if order_by is not None:
            query = query.order_by(order_by)
        else:
            query = query.order_by(self.model.id)
            
        if offset:
            query = query.offset(offset)
        if limit:
            query = query.limit(limit)
            
        result: Result = await self.session.execute(query)
        entities = result.scalars().all()
        
        # If no pagination was used, total_count equals result length
        if total_count is None:
            self.logger.debug("No limit/offset set, assuming total_count = len(result)")
            total_count = len(entities)
        
        return entities, total_count

    def apply_param_filters_to_query(
        self, query: Select, params: dict[str, Any | list[Any]] | None = None, exact: bool = False
    ) -> Select:
        """Takes a param dict and turns it into SQLAlchemy filters.

        Args:
            query: The query to apply filters to.
            params: A dict of filters based on the Select's fields.
            exact: Whether to use exact matching for string filters.

        Returns:
            Select: The query with the new filters applied.
        """
        if not params:
            return query
        filters = []
        for k, v in params.items():
            filters.extend(self._get_filter_list(key=k, value=v, exact=exact))
        query = query.filter(*filters)
        return query

    def _get_filter_list(
        self, key: str, value: list[Any] | Any | None, model: Any | None = None, exact: bool = False
    ) -> list[Any]:
        """Gets filters based on the param's value type.

        Args:
            key: The param dict key.
            value: The param dict value.
            model: The model that will be filtered. Defaults to None.
            exact: Whether to use exact matching for string filters.

        Returns:
            list[Any]: A list of filters.
        """
        if not model:
            model = self.model
        key = str(key).split(".")[-1]
        filters = []
        
        if not hasattr(model, key):
            return filters
            
        model_field: InstrumentedAttribute = getattr(model, key)
        if value:
            if isinstance(value, str) and "," in value:
                value = value.split(",")
            if isinstance(value, list):
                conditions = [model_field.ilike(f"%{v}%") for v in value]
                list_query = or_(*conditions)
                filters.append(list_query)
            elif isinstance(value, (int, Enum)):
                filters.append(model_field == value)
            else:
                if exact:
                    filters.append(model_field.like(f"{value}"))
                else:
                    filters.append(model_field.ilike(f"%{value}%"))
        return filters


