"""Core repository classes/functions for building reposigtories."""

from __future__ import annotations

import uuid
from collections.abc import AsyncIterator, Sequence
from enum import Enum
from typing import Any

import loguru
from fastapi.encoders import jsonable_encoder
from pydantic import BaseModel
from sqlalchemy import ColumnElement, Result, Select, func, inspect, or_, select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import load_only
from sqlalchemy.orm.attributes import InstrumentedAttribute

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.database.exceptions import handle_sqlalchemy_errors_decorator
from py_dnd.shared.fields import partial_model


class RepositoryBase[
    ModelType: DndSchemaBase,
    ModelSchemaType: BaseModel,
    ModelSchemaBaseType: BaseModel,
    CreateSchemaType: BaseModel,
    UpdateSchemaType: BaseModel,
]:
    """Base repositiroy.

    Args:
        Generic (_type_): typings for repository.
    """

    def __init__(
        self,
        session: AsyncSession,
        model: type[ModelType],
        schema: type[ModelSchemaType],
        schema_base: type[ModelSchemaBaseType],
        logger: loguru.Logger | None = None,
    ):
        """RepositoryBase.

        CRUD object with default methods to Create, Read, Update, Delete (CRUD).
        **Parameters**
        * `model`: A SQLAlchemy model class
        * `schema`: A Pydantic model (schema) class
        """
        self.session = session
        self.model = model
        self.schema = schema
        self.schema_base = schema_base
        self.logger = logger if logger else loguru.logger

    def _live(self, stmt: Select) -> Select:
        """Skip soft-deleted rows on tables that have them (MixinSoftDelete)."""
        deleted_at = getattr(self.model, "deleted_at", None)
        return stmt if deleted_at is None else stmt.where(deleted_at.is_(None))

    def _select(self, fields: Sequence[str] | None) -> Select:
        """The live rows, loading only ``fields``' columns when given (sparse fieldsets).

        Relationships in ``fields`` load with their mapped strategy (``lazy="joined"``), not here.
        """
        stmt = self._live(select(self.model))
        if fields:
            columns = inspect(self.model).column_attrs.keys()
            stmt = stmt.options(load_only(*(getattr(self.model, f) for f in fields if f in columns)))
        return stmt

    def _out(self, entity: ModelType, fields: Sequence[str] | None) -> BaseModel:
        """The response object: the full schema, or the partial one with only ``fields`` set."""
        if not fields:
            return self.schema.model_validate(entity)
        # only the loaded columns: touching a deferred one would lazy-load, which async can't
        return partial_model(self.schema).model_validate({f: getattr(entity, f) for f in fields})

    @handle_sqlalchemy_errors_decorator
    async def read_by_id(
        self, entity_id: int | str | uuid.UUID, as_model: bool = False, fields: Sequence[str] | None = None
    ) -> BaseModel | ModelType | None:
        """Get an entity by id.

        Args:
            entity_id (: int | str | uuid.UUID): _description_
            as_model (bool): return the ORM entity (for updates) instead of a schema
            fields (Sequence[str] | None): sparse fieldset (schema field names) for the schema

        Returns:
            ModelSchemaType | None: _description_
        """
        stmt = self._select(None if as_model else fields).where(self.model.id == entity_id)
        model: ModelType | None = await self.session.scalar(stmt)
        if as_model or model is None:
            return model
        return self._out(model, fields)

    # async def get(self, db: AsyncSession, id: Any) -> Optional[ModelType]:
    #     stmt = select(self.model).where(self.model.id == id)
    #     result = await db.execute(stmt)
    #     return result.scalars().first()

    @handle_sqlalchemy_errors_decorator
    async def read_multi_by_ids(
        self,
        entity_ids: list[int | str | uuid.UUID],
    ) -> AsyncIterator[ModelType]:
        """Get multiple entities by ids.

        Args:
            entity_ids (list[int | str | uuid.UUID]): _description_

        Returns:
            AsyncIterator[ModelType]: _description_

        Yields:
            Iterator[AsyncIterator[ModelType]]: _description_
        """
        stmt = self._live(select(self.model).where(self.model.id.in_(entity_ids)))
        stream = await self.session.stream_scalars(stmt.order_by(self.model.id))
        async for row in stream:
            yield row

    @handle_sqlalchemy_errors_decorator
    async def read_multi(
        self,
        *,
        offset: int = 0,
        limit: int = 100,
        fields: Sequence[str] | None = None,
    ) -> list[BaseModel]:
        # ) -> AsyncIterator[ModelType]:
        """Get multiple entities (pagination optional).

        Args:
            offset (int, optional): _description_. Defaults to 0.
            limit (int, optional): _description_. Defaults to 100.
            fields (Sequence[str] | None): sparse fieldset (schema field names)

        Returns:
            AsyncIterator[ModelType]: _description_

        Yields:
            Iterator[AsyncIterator[ModelType]]: _description_
        """
        self.logger.debug("RepositoryBase::read_multi() called with offset={}, limit={}", offset, limit)
        stmt = self._select(fields).offset(offset).limit(limit)
        # stream = await self.session.stream_scalars(stmt.order_by(self.model.id))
        # async for row in stream:
        #     yield row
        res = await self.session.scalars(stmt.order_by(self.model.id))
        return [self._out(e, fields) for e in res]

    # async def get_multi(self, db: AsyncSession, *, offset: int = 0, limit: int = 100) -> list[ModelType]:
    #     stmt = select(self.model).offset(offset).limit(limit)
    #     result = await db.execute(stmt)
    #     return result.scalars().all()

    @handle_sqlalchemy_errors_decorator
    async def create(self, *, model_in: CreateSchemaType, return_model: bool = True) -> ModelSchemaType | None:
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
        if return_model:
            await self.session.flush()
            # new = await self.read_by_id(entity.id)
            # if not new:
            #     raise RuntimeError()
            # Note: do not define relationships here or you get greenlit errors
            #   TODO: look into a better solution?
            return self.schema_base.model_validate(entity)
        return None

    def apply_changes(self, entity: ModelType, values: dict[str, Any]) -> None:
        """Set columns on a loaded entity; written on the next flush."""
        for key, value in values.items():
            setattr(entity, key, value)

    @handle_sqlalchemy_errors_decorator
    async def create_and_reload(self, *, model_in: CreateSchemaType) -> ModelSchemaType:
        """Create an entity and return it as stored, with database defaults and generated columns."""
        entity = self.model(**model_in.model_dump())
        self.session.add(entity)
        return await self.flush_and_reload(entity)

    @handle_sqlalchemy_errors_decorator
    async def flush_and_reload(self, entity: ModelType) -> ModelSchemaType:
        """Write pending changes, then re-read the row.

        Generated columns and SQL defaults expire on write, and an expired attribute can't lazy-load
        in an async session, so read them back before building the response.
        """
        await self.session.flush()
        await self.session.refresh(entity)
        return self.schema.model_validate(entity)

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
        db_obj: ModelType | None = None,
        obj_in: UpdateSchemaType | dict[str, Any],
    ) -> ModelSchemaBaseType | None:
        """Update an existing entity.

        Args:
            db_obj (ModelType): _description_
            obj_in (UpdateSchemaType | dict[str, Any]): _description_

        Returns:
            ModelSchemaBaseType: _description_
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
        return self.schema_base.model_validate(db_obj)

    @handle_sqlalchemy_errors_decorator
    async def delete(
        self,
        entity_id: int | str | uuid.UUID,
    ) -> int | str | uuid.UUID | None:
        """Delete an entity.

        Args:
            entity (ModelType): _description_

        Returns:
            int: _description_
        """
        stmt = select(self.model).where(self.model.id == entity_id)
        model: ModelType = await self.session.scalar(stmt.order_by(self.model.id))
        if not model:
            return None
        await self.session.delete(model)
        # await self.session.flush()
        return entity_id

    @handle_sqlalchemy_errors_decorator
    async def soft_delete(
        self,
        entity_id: int | str | uuid.UUID,
        deleted_by: uuid.UUID | None,
    ) -> int | str | uuid.UUID | None:
        """Mark an entity deleted (MixinSoftDelete tables); reads skip it from then on.

        Returns the id, or None when there's no live entity with it.
        """
        model: ModelType | None = await self.session.scalar(
            self._live(select(self.model).where(self.model.id == entity_id))
        )
        if not model:
            return None
        model.deleted_at = func.now()
        model.deleted_by = deleted_by
        self.session.add(model)
        return entity_id

    # async def delete(self, db: AsyncSession, *, id: int) -> ModelType:
    #     obj = await self.get(db, id)
    #     db.delete(obj)
    #     await db.commit()
    #     return obj

    @handle_sqlalchemy_errors_decorator
    async def query(
        self,
        params: dict[str, list[Any] | str | None],
        *,
        order_by: Any | None = None,
        limit: int | None = 100,
        offset: int | None = 0,
        exact: bool = False,
        fields: Sequence[str] | None = None,
        where: Sequence[ColumnElement[bool]] = (),
    ) -> tuple[Sequence[Any], int]:
        """Query a list of Type[ModelType] with filters.

        Args:
            db (Session): A SQLAlchemy Session.
            params (dict[str, list[Any] | str | None]): A dict of fields from Type[ModelType] to query.
            order_by (Any, None, optional): SQL 'ORDER BY' input. Defaults to None.
            limit (int | None, optional): SQL 'LIMIT'. Defaults to 100.
            offset (int | None, optional): SQL 'OFFSET'. Defaults to 0.
            fields (Sequence[str] | None): sparse fieldset (schema field names)
            where (Sequence[ColumnElement[bool]]): more conditions, for filters that aren't a column

        Returns:
            tuple[Sequence[ModelType], int]: A tuple of the entities and the total_count.
        """
        total_count: int | None = None
        query: Select = self._select(fields).where(*where)
        if params:
            query = self.apply_param_filters_to_query(query=query, params=params, exact=exact)
        # get count before limit/offset are applied
        if limit != 0 or offset != 0:
            query_count: Select = select(func.count()).select_from(query.subquery())
            query_count_result: Result = await self.session.execute(query_count)
            total_count = int(query_count_result.scalar_one())
        # apply limit/offset/order_by (default to id so pages are stable)
        if order_by is None:
            order_by = (self.model.id,)
        elif not isinstance(order_by, (list, tuple)):
            order_by = (order_by,)
        query = query.order_by(*order_by)
        if offset:
            query = query.offset(offset)
        if limit:
            query = query.limit(limit)
        result: Result = await self.session.execute(query)
        # if no limit/offset assume count is lenght of result
        entities = [self._out(e, fields) for e in result.scalars().all()]
        if total_count is None:
            self.logger.debug("No limit/offset set, assuming total_count = len(result)")
            total_count = len(entities)
        return entities, total_count

    def apply_param_filters_to_query(
        self, query: Select, params: dict[str, Any | list[Any]] | None = None, exact: bool = False
    ) -> ModelType:
        """Takes a param dict and turns it into SQLAlchemy filters.

        Args:
            query (Select): The query to apply filters to.
            params (dict[str, Any | list[Any]] | None, optional):
                A dict of filters based on the Select's fields. Defaults to None.

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
            key (str): The param dict key.
            value (list[Any] | Any | None, optional): The param dict value.
            model (Any | None, optional): The model that will be filtered. Defaults to None.

        Returns:
            list[Any]: A list of filters.
        """
        if not model:
            model = self.model
        key = str(key).split(".")[-1]
        filters = []
        model_field: InstrumentedAttribute = getattr(model, key)
        if value:
            if isinstance(value, str) and "," in value:
                value = value.split(",")
            if isinstance(value, list):
                conditions = [model_field.ilike(f"%{v}%") for v in value]
                list_query = or_(*conditions)
                filters.append(list_query)
                # filters.append(model_field.in_(value))
            elif isinstance(value, (int, Enum)):
                filters.append(model_field == value)
            elif isinstance(value, uuid.UUID):
                filters.append(model_field == value)
            else:
                if exact:  # equality: LIKE would treat "_" in slugs (phb_2024) as a wildcard
                    filters.append(model_field == value)
                else:
                    filters.append(model_field.ilike(f"%{value}%"))
        return filters
