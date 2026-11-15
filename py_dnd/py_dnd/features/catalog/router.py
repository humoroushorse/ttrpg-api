"""Routes for a catalog entity, built from its ``CatalogSpec``.

Each entity gets the same routes spells have:

    GET    /query                   list: ?filter= ?sort= (AG Grid models), ?fields=, ?name=
    GET    /{id}                    one entry
    GET    /{source_slug}/{slug}    one entry by its URL key (phb_2024/blinded)
    POST   ""                       create
    PUT    /{id}                    replace
    PATCH  /{id}                    change some fields
    DELETE /{id}                    soft delete
    POST   /bulk                    create or update from an ETL file, keyed on (source, slug)
"""

import datetime
import uuid
from collections.abc import Mapping
from dataclasses import dataclass, field
from typing import Annotated, Any

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile, status
from loguru import logger
from pydantic import BaseModel
from sqlalchemy import ColumnElement
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.db import AsyncMasterSessionDependency, AsyncReplicaSessionDependency
from py_dnd.features.auth.schemas import AuthUserToken
from py_dnd.features.auth.service import UserAuth, UserAuthOptional
from py_dnd.features.catalog.repository import CatalogRepository
from py_dnd.features.catalog.schemas import CatalogQuery, patch_model
from py_dnd.features.core.unit_of_work import sqlalchemy_uow
from py_dnd.features.sources.models import Source
from py_dnd.features.sources.service import find_source_for_row
from py_dnd.features.user.service import CurrentUserId
from py_dnd.shared.fields import fields_query, partial_model
from py_dnd.shared.grid import ColumnKind, GridColumn, GridQuery, grid_query
from py_dnd.shared.schemas import BulkLoadResponse, GenericListResponse
from py_dnd.shared.uploads import read_rows

# Keys an upload can't set: the database and the API fill them
_SERVER_KEYS = ("id", "created_at", "created_by", "updated_at", "updated_by", "deleted_at", "deleted_by")


@dataclass(frozen=True)
class CatalogSpec:
    """What ``catalog_router`` needs to know about an entity.

    Args:
        label: singular, for logs and messages ("condition")
        model: the SQLAlchemy model (``MixinCatalogEntry`` + bookkeeping + soft delete)
        fields: what a client writes (``CatalogFields`` subclass); POST and PUT bodies
        schema: what the API returns (``<Entity>Fields`` + ``CatalogRow``)
        create: a new row (``<Entity>Fields`` + ``CatalogCreateRow``)
        grid_columns: what ``?filter=`` / ``?sort=`` accept beyond the shared ones
        plural: for route names and logs when it isn't ``label`` + "s" ("classes")
    """

    label: str
    model: type[Any]
    fields: type[BaseModel]
    schema: type[BaseModel]
    create: type[BaseModel]
    grid_columns: Mapping[str, GridColumn] = field(default_factory=dict)
    plural: str = ""

    def repository(self, session: AsyncSession) -> CatalogRepository:
        """This entity's repository on a session."""
        return CatalogRepository(session, self.model, self.schema)


def shared_grid_columns(model: type[Any]) -> dict[str, GridColumn]:
    """The ``?filter=`` / ``?sort=`` columns every catalog entity has."""
    columns = {
        "name": GridColumn(model.name, ColumnKind.TEXT),
        "slug": GridColumn(model.slug, ColumnKind.TEXT),
        "description": GridColumn(model.description, ColumnKind.TEXT),
        "dnd_version_year": GridColumn(model.dnd_version_year, ColumnKind.YEAR),
        "is_unearthed_arcana": GridColumn(model.is_unearthed_arcana, ColumnKind.BOOLEAN),
        "is_homebrew": GridColumn(model.is_homebrew, ColumnKind.BOOLEAN),
    }

    def by_source_slug(slugs: list[Any]) -> ColumnElement[bool]:
        return model.source.has(Source.slug.in_([str(s) for s in slugs]))  # type: ignore[no-any-return]

    # the source's slug (phb_2024); sorting by source needs a join, so not yet
    columns["source"] = GridColumn(model.source_id, ColumnKind.SET, sortable=False, set_condition=by_source_slug)
    return columns


def _log_user(current_user: AuthUserToken | None) -> dict[str, str]:
    return {"sub": current_user.sub, "preferred_username": current_user.preferred_username} if current_user else {}


def _now() -> datetime.datetime:
    return datetime.datetime.now(tz=datetime.UTC)


def bare_slug(slug: str) -> str:
    """The ETL's ids carry the wiki's type prefix ("condition:blinded"); the API keys on "blinded"."""
    return slug.split(":", 1)[-1]


def _internal_error(e: Exception) -> HTTPException:
    logger.error("Uncaught error: {}", str(e))
    return HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error")


# one nested function per route
def catalog_router(spec: CatalogSpec) -> APIRouter:  # pylint: disable=too-many-statements
    """The CRUD + bulk routes for one catalog entity."""
    router = APIRouter()
    label, plural = spec.label, spec.plural or f"{spec.label}s"
    model = spec.model
    schema = spec.schema
    partial = partial_model(schema)
    patch_input = patch_model(spec.fields)
    writable = frozenset(spec.fields.model_fields)
    read_fields = Annotated[tuple[str, ...], Depends(fields_query(schema))]
    grid = Annotated[GridQuery, Depends(grid_query({**shared_grid_columns(model), **spec.grid_columns}))]
    # FastAPI reads these at runtime; mypy can't type a class held in a variable
    fields_body: Any = spec.fields
    patch_body: Any = patch_input

    async def live_row(repo: CatalogRepository, entity_id: uuid.UUID) -> Any:
        row = await repo.read_by_id(entity_id=entity_id, as_model=True)
        if not isinstance(row, model):
            raise HTTPException(status.HTTP_404_NOT_FOUND, f"{label.capitalize()} {entity_id} not found")
        return row

    @router.get(
        "/query",
        name=f"query_{plural}",
        response_model=GenericListResponse[partial],  # type: ignore[valid-type]
        response_model_exclude_unset=True,
    )
    async def query(
        current_user: UserAuthOptional,
        db: AsyncReplicaSessionDependency,
        fields: read_fields,  # type: ignore[valid-type]
        grid_query_: grid,  # type: ignore[valid-type]
        params: CatalogQuery = Depends(),
    ) -> Any:
        """List entries, by name then newest edition after ``sort``."""
        try:
            filters = params.model_dump(exclude_none=True, exclude={"limit", "offset"})
            with logger.contextualize(user=_log_user(current_user), filters=filters, log_threads=True):
                async with sqlalchemy_uow(db, None) as uow:
                    entities, total = await spec.repository(uow.db_session).query(
                        params=filters,
                        offset=params.offset,
                        limit=params.limit,
                        # the defaults after the client's sort keep pages stable
                        order_by=(*grid_query_.order_by, model.name, model.dnd_version_year.desc(), model.id),
                        fields=fields,
                        where=grid_query_.where,
                    )
                return GenericListResponse[partial](  # type: ignore[valid-type]
                    entities=entities,
                    total_entities_count=total,
                    limit=params.limit,
                    offset=params.offset,
                    filters=filters,
                )
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.get("/{entity_id}", name=f"read_{label}", response_model=partial, response_model_exclude_unset=True)
    async def read(
        entity_id: uuid.UUID,
        current_user: UserAuthOptional,
        db: AsyncReplicaSessionDependency,
        fields: read_fields,  # type: ignore[valid-type]
    ) -> Any:
        """One entry. ``fields`` picks the fields returned."""
        try:
            with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
                async with sqlalchemy_uow(db, None) as uow:
                    row = await spec.repository(uow.db_session).read_by_id(entity_id=entity_id, fields=fields)
                if row is None:
                    raise HTTPException(status.HTTP_404_NOT_FOUND, f"{label.capitalize()} {entity_id} not found")
                return row
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.get(
        "/{source_slug}/{slug}",
        name=f"read_{label}_by_slugs",
        response_model=partial,
        response_model_exclude_unset=True,
    )
    async def read_by_slugs(
        source_slug: str,
        slug: str,
        current_user: UserAuthOptional,
        db: AsyncReplicaSessionDependency,
        fields: read_fields,  # type: ignore[valid-type]
    ) -> Any:
        """One entry by its source's slug and its own (phb_2024/blinded), the UI's URL key."""
        try:
            with logger.contextualize(user=_log_user(current_user), source_slug=source_slug, slug=slug):
                async with sqlalchemy_uow(db, None) as uow:
                    row = await spec.repository(uow.db_session).read_by_slugs(source_slug, slug, fields=fields)
                if row is None:
                    raise HTTPException(
                        status.HTTP_404_NOT_FOUND, f"{label.capitalize()} {source_slug}/{slug} not found"
                    )
                return row
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.post("", name=f"create_{label}", response_model=schema)
    async def create(
        current_user: UserAuth,
        current_user_id: CurrentUserId,
        db: AsyncMasterSessionDependency,
        model_in: fields_body,
    ) -> Any:
        """Create an entry. A (source, slug) that's taken is a 400."""
        try:
            with logger.contextualize(user=_log_user(current_user), slug=model_in.slug, log_threads=True):
                logger.info("Creating {}", label)
                now = _now()
                row = spec.create(
                    **model_in.model_dump(),
                    created_at=now,
                    created_by=current_user_id,
                    updated_at=now,
                    updated_by=current_user_id,
                )
                async with sqlalchemy_uow(db, None) as uow:
                    entity = await spec.repository(uow.db_session).create_and_reload(model_in=row)
                    await uow.commit()
                return entity
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    async def write(db: AsyncSession, entity_id: uuid.UUID, values: dict[str, Any], user_id: uuid.UUID) -> Any:
        """Apply column values to a live row and return it as stored."""
        async with sqlalchemy_uow(db, None) as uow:
            repo = spec.repository(uow.db_session)
            row = await live_row(repo, entity_id)
            if not values:  # an empty PATCH changes nothing
                return schema.model_validate(row)
            repo.apply_changes(row, {**values, "updated_at": _now(), "updated_by": user_id})
            entity = await repo.flush_and_reload(row)
            await uow.commit()
        return entity

    @router.put("/{entity_id}", name=f"replace_{label}", response_model=schema)
    async def replace(
        entity_id: uuid.UUID,
        current_user: UserAuth,
        current_user_id: CurrentUserId,
        db: AsyncMasterSessionDependency,
        model_in: fields_body,
    ) -> Any:
        """Replace an entry: every field is set, and fields left out go back to their defaults."""
        try:
            with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
                logger.info("Replacing {}", label)
                return await write(db, entity_id, model_in.model_dump(), current_user_id)
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.patch("/{entity_id}", name=f"update_{label}", response_model=schema)
    async def update(
        entity_id: uuid.UUID,
        current_user: UserAuth,
        current_user_id: CurrentUserId,
        db: AsyncMasterSessionDependency,
        model_in: patch_body,
    ) -> Any:
        """Change some fields. ``null`` clears an optional field; lists and objects are replaced whole."""
        try:
            changes = model_in.changes()
            with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, fields=sorted(changes)):
                logger.info("Updating {}", label)
                return await write(db, entity_id, changes, current_user_id)
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.delete("/{entity_id}", name=f"delete_{label}")
    async def delete(
        entity_id: uuid.UUID,
        current_user: UserAuth,
        current_user_id: CurrentUserId,
        db: AsyncMasterSessionDependency,
    ) -> str:
        """Delete an entry (soft: hidden from every read, keeps its slug). Returns its id."""
        try:
            with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
                logger.info("Deleting {}", label)
                async with sqlalchemy_uow(db, None) as uow:
                    deleted = await spec.repository(uow.db_session).soft_delete(
                        entity_id=entity_id, deleted_by=current_user_id
                    )
                    if deleted is None:
                        raise HTTPException(status.HTTP_404_NOT_FOUND, f"{label.capitalize()} {entity_id} not found")
                    await uow.commit()
                return str(entity_id)
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    @router.post("/bulk", name=f"bulk_{plural}")
    async def bulk(
        current_user: UserAuth,
        current_user_id: CurrentUserId,
        *,
        db: AsyncMasterSessionDependency,
        file: UploadFile = File(description='Files of type: ["text/csv", "application/json"]'),
    ) -> BulkLoadResponse:
        """Create or update entries from a file (an ETL ``_all.json``), keyed on (source, slug).

        Reports each entry as created, updated (with the fields that changed), or unchanged. Saves
        nothing if any row has an error. Supports file types: [application/json, text/csv]
        """
        try:
            with logger.contextualize(user=_log_user(current_user), filename=file.filename, log_threads=True):
                logger.info("Bulk loading {}", plural)
                rows = await read_rows(file)
                response = BulkLoadResponse(filename=file.filename or "")
                async with sqlalchemy_uow(db, None) as uow:
                    repo = spec.repository(uow.db_session)
                    seen: set[tuple[uuid.UUID, str]] = set()
                    # sequential: one AsyncSession can't run concurrent queries
                    for index, row in rows:
                        label_ = f"row {index} [{row.get('slug') or row.get('name') or '?'}]"
                        try:
                            await _upsert_row(uow, repo, response, index, row, current_user_id, seen)
                        except HTTPException as e:
                            response.errors.append(f"{label_}: {e.detail}")
                        except Exception as e:  # pylint: disable=broad-exception-caught  # reported per row
                            response.errors.append(f"{label_}: {e}")
                    if response.errors:
                        await uow.rollback()
                    else:
                        try:
                            await uow.db_session.flush()
                            await uow.commit()
                        except IntegrityError as e:
                            await uow.rollback()
                            logger.error("Bulk {} commit failed: {}", label, str(e))
                            response.errors.append(
                                "Commit failed on a unique constraint, nothing was saved (see server logs for the row)."
                            )
                response.update_totals()
                return response
        except HTTPException:
            raise
        except Exception as e:
            raise _internal_error(e) from e

    async def _upsert_row(
        uow: Any,
        repo: CatalogRepository,
        response: BulkLoadResponse,
        index: int,
        row: dict[str, Any],
        user_id: uuid.UUID,
        seen: set[tuple[uuid.UUID, str]],
    ) -> None:
        """Create or update one entry and record what happened."""
        row = {k: v for k, v in row.items() if k not in _SERVER_KEYS}
        if isinstance(row.get("slug"), str):
            row["slug"] = bare_slug(row["slug"])
        source = await find_source_for_row(uow.source_repo, row)
        if source is None:
            response.errors.append(
                f"row {index} [{row.get('slug')}]: source '{row.get('source_name') or row.get('source_id')}' not found"
            )
            return
        row["source_id"] = source.id
        row.setdefault("dnd_version", source.dnd_version)
        row.setdefault("dnd_version_year", source.dnd_version_year)
        now = _now()
        incoming: Any = spec.create(**row, created_at=now, created_by=user_id, updated_at=now, updated_by=user_id)
        entry_label = f"{incoming.slug} ({source.slug})"
        key = (incoming.source_id, incoming.slug)
        # earlier rows aren't flushed, so the lookup below can't see them
        if key in seen:
            response.errors.append(f"row {index} [{entry_label}]: same slug as an earlier row")
            return
        seen.add(key)
        existing = await repo.read_by_natural_key(*key)
        if existing is None:
            await repo.create(model_in=incoming, return_model=False)
            response.created.append(entry_label)
            return
        if existing.deleted_at is not None:
            response.warnings.append(f"{entry_label}: deleted, so not restored or updated")
            return
        values = incoming.model_dump(include=writable)
        changed = {k: v for k, v in values.items() if getattr(existing, k) != v}
        if not changed:
            response.unchanged.append(entry_label)
            return
        repo.apply_changes(existing, {**changed, "updated_at": now, "updated_by": user_id})
        response.updated.append(f"{entry_label}: {', '.join(sorted(changed))}")

    return router
