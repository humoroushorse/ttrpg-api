"""Sources route definitions."""

import datetime
import uuid
from typing import Annotated, Any

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile, status
from loguru import logger
from sqlalchemy import func, select
from sqlalchemy.exc import IntegrityError

from py_dnd.database.db import (
    AsyncMasterSessionDependency,
    AsyncReplicaSessionDependency,
)
from py_dnd.features.auth.schemas import AuthUserToken
from py_dnd.features.auth.service import UserAuth, UserAuthOptional
from py_dnd.features.backgrounds.models import Background
from py_dnd.features.classes.models import DndClass
from py_dnd.features.conditions.models import Condition
from py_dnd.features.core.unit_of_work import SqlAlchemyUnitOfWork, sqlalchemy_uow
from py_dnd.features.feats.models import Feat
from py_dnd.features.glossary.models import GlossaryEntry
from py_dnd.features.items.models import Item
from py_dnd.features.sources.models import Source
from py_dnd.features.sources.schemas import (
    SOURCE_FIELDS,
    SOURCE_READ_FIELDS,
    SourceCreate,
    SourceCreateInput,
    SourcePatchInput,
    SourceQuery,
    SourceReplaceInput,
    SourceSchema,
    SourceSchemaPartial,
)
from py_dnd.features.species.models import Species
from py_dnd.features.spells.models import Spell
from py_dnd.features.subclasses.models import Subclass
from py_dnd.features.user.service import CurrentUserId
from py_dnd.shared.schemas import BulkLoadResponse, GenericListResponse
from py_dnd.shared.uploads import read_rows

router = APIRouter()

# Keys an upload can't set: the database and the API fill them
_SERVER_KEYS = ("id", "created_at", "created_by", "updated_at", "updated_by", "deleted_at", "deleted_by")

# what points at a source; a source with live entries can't be deleted
_SOURCE_ENTRIES: tuple[tuple[str, Any], ...] = (
    ("spell(s)", Spell),
    ("condition(s)", Condition),
    ("glossary entry(s)", GlossaryEntry),
    ("species", Species),
    ("class(es)", DndClass),
    ("subclass(es)", Subclass),
    ("background(s)", Background),
    ("feat(s)", Feat),
    ("item(s)", Item),
)

ReadFields = Annotated[tuple[str, ...], Depends(SOURCE_READ_FIELDS)]


def _log_user(current_user: AuthUserToken | None) -> dict[str, str]:
    return {"sub": current_user.sub, "preferred_username": current_user.preferred_username} if current_user else {}


def _now() -> datetime.datetime:
    return datetime.datetime.now(tz=datetime.UTC)


async def _live_source(uow: SqlAlchemyUnitOfWork, entity_id: uuid.UUID) -> Source:
    """The source row, or a 404 (soft-deleted sources count as gone)."""
    source = await uow.source_repo.read_by_id(entity_id=entity_id, as_model=True)
    if not isinstance(source, Source):
        raise HTTPException(status.HTTP_404_NOT_FOUND, f"Source {entity_id} not found")
    return source


@router.get("/query", response_model_exclude_unset=True)
async def query_sources(
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
    params: SourceQuery = Depends(),
) -> GenericListResponse[SourceSchemaPartial]:
    """Retrieve sources, by name then newest edition. ``fields`` picks the fields returned."""
    try:
        filters = params.model_dump(exclude_none=True, exclude={"limit", "offset"})
        with logger.contextualize(user=_log_user(current_user), filters=filters, log_threads=True):
            logger.info("Querying sources")
            async with sqlalchemy_uow(db, None) as uow:
                entities, total_entities_count = await uow.source_repo.query(
                    params=filters,
                    offset=params.offset,
                    limit=params.limit,
                    # the UI's source pickers page through this
                    order_by=(Source.name, Source.dnd_version_year.desc(), Source.id),
                    fields=fields,
                )
            return GenericListResponse[SourceSchemaPartial](
                entities=entities,
                total_entities_count=total_entities_count,
                limit=params.limit,
                offset=params.offset,
                filters=filters,
            )
    except HTTPException:
        # assume that the error was already logged
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.get("/slug/{slug}", response_model_exclude_unset=True)
async def read_source_by_slug(
    slug: str,
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
) -> SourceSchemaPartial:
    """Retrieve one source by its slug (phb_2024)."""
    try:
        with logger.contextualize(user=_log_user(current_user), slug=slug, log_threads=True):
            async with sqlalchemy_uow(db, None) as uow:
                source = await uow.source_repo.read_by_slug(slug, fields=fields)
            if source is None:
                raise HTTPException(status.HTTP_404_NOT_FOUND, f"Source {slug} not found")
            return source  # type: ignore[return-value]
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.get("/{entity_id}", response_model_exclude_unset=True)
async def read_source(
    entity_id: uuid.UUID,
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
) -> SourceSchemaPartial:
    """Retrieve one source. ``fields`` picks the fields returned."""
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            async with sqlalchemy_uow(db, None) as uow:
                source = await uow.source_repo.read_by_id(entity_id=entity_id, fields=fields)
            if source is None:
                raise HTTPException(status.HTTP_404_NOT_FOUND, f"Source {entity_id} not found")
            return source  # type: ignore[return-value]
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.post("")
async def create_source(
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SourceCreateInput,
) -> SourceSchema:
    """Create a source. A slug, or a name or short name in the same edition, that's taken is a 400."""
    try:
        with logger.contextualize(user=_log_user(current_user), slug=model_in.slug, log_threads=True):
            logger.info("Creating source")
            now = _now()
            model_create = SourceCreate(
                **model_in.model_dump(),
                created_at=now,
                created_by=current_user_id,
                updated_at=now,
                updated_by=current_user_id,
            )
            async with sqlalchemy_uow(db, None) as uow:
                entity = await uow.source_repo.create_and_reload(model_in=model_create)
                await uow.commit()
            return entity
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


async def _write(
    db: AsyncMasterSessionDependency, entity_id: uuid.UUID, values: dict[str, Any], user_id: uuid.UUID
) -> SourceSchema:
    """Apply column values to a live source and return it as stored."""
    async with sqlalchemy_uow(db, None) as uow:
        source = await _live_source(uow, entity_id)
        if not values:  # an empty PATCH changes nothing
            return SourceSchema.model_validate(source)
        uow.source_repo.apply_changes(source, {**values, "updated_at": _now(), "updated_by": user_id})
        entity = await uow.source_repo.flush_and_reload(source)
        await uow.commit()
    return entity


@router.put("/{entity_id}")
async def replace_source(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SourceReplaceInput,
) -> SourceSchema:
    """Replace a source: every field is set, and fields left out go back to their defaults."""
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            logger.info("Replacing source")
            return await _write(db, entity_id, model_in.model_dump(), current_user_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.patch("/{entity_id}")
async def update_source(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SourcePatchInput,
) -> SourceSchema:
    """Change some of a source's fields. ``null`` clears an optional field (``publish_year``)."""
    try:
        changes = model_in.changes()
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, fields=sorted(changes)):
            logger.info("Updating source")
            return await _write(db, entity_id, changes, current_user_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.delete("/{entity_id}")
async def delete_source(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
) -> str:
    """Delete a source (soft: hidden from every read, keeps its slug). Returns its id.

    A source that live entries (spells, classes, items, ...) still point at can't be deleted (409): delete those first.
    """
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            logger.info("Deleting source")
            async with sqlalchemy_uow(db, None) as uow:
                await _live_source(uow, entity_id)
                in_use = []
                for label, model in _SOURCE_ENTRIES:
                    count = await uow.db_session.scalar(
                        select(func.count()).where(model.source_id == entity_id, model.deleted_at.is_(None))
                    )
                    if count:
                        in_use.append(f"{count} {label}")
                if in_use:
                    raise HTTPException(
                        status.HTTP_409_CONFLICT,
                        f"Source {entity_id} has {', '.join(in_use)}; delete them first",
                    )
                await uow.source_repo.soft_delete(entity_id=entity_id, deleted_by=current_user_id)
                await uow.commit()
            return str(entity_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


async def upsert_and_mutate_report(
    uow: SqlAlchemyUnitOfWork,
    response: BulkLoadResponse,
    index: int,
    row: dict[str, Any],
    user_id: uuid.UUID,
    seen: set[str],
) -> None:
    """Create or update one source, keyed on its slug, and record what happened.

    Args:
        uow (SqlAlchemyUnitOfWork): the upload's unit of work (one transaction)
        response (BulkLoadResponse): the report this row adds to
        index (int): the row number in the file
        row (dict[str, Any]): the source, as in the ETL's ``sources_<year>.json``
        user_id (uuid.UUID): the uploader's dnd.user id
        seen (set[str]): slugs of earlier rows in this file
    """
    label = f"row {index} [{row.get('source_id') or row.get('slug') or row.get('name') or '?'}]"
    try:
        row = {k: v for k, v in row.items() if k not in _SERVER_KEYS}
        # the ETL's source_id ("phb_2024") is the slug, not the database id
        if "source_id" in row:
            row.setdefault("slug", row.pop("source_id"))
        if row.get("marks") is None:  # CSV: an empty cell
            row["marks"] = []
        now = _now()
        incoming = SourceCreate(**row, created_at=now, created_by=user_id, updated_at=now, updated_by=user_id)
        label = f"{incoming.slug} ({incoming.name})"
        # earlier rows aren't flushed, so the lookup below can't see them
        if incoming.slug in seen:
            response.errors.append(f"row {index} [{label}]: same slug as an earlier row")
            return
        seen.add(incoming.slug)

        existing = await uow.source_repo.read_by_natural_key(incoming.slug)
        if existing is None:
            await uow.source_repo.create(model_in=incoming, return_model=False)
            response.created.append(label)
            return
        if existing.deleted_at is not None:
            response.warnings.append(f"{label}: deleted, so not restored or updated")
            return
        values = incoming.model_dump(include=SOURCE_FIELDS)
        changed = {k: v for k, v in values.items() if getattr(existing, k) != v}
        if not changed:
            response.unchanged.append(label)
            return
        uow.source_repo.apply_changes(existing, {**changed, "updated_at": now, "updated_by": user_id})
        response.updated.append(f"{label}: {', '.join(sorted(changed))}")
    except HTTPException as e:
        response.errors.append(f"{label}: {e.detail}")
    except Exception as e:
        response.errors.append(f"{label}: {e}")


@router.post("/bulk")
async def bulk_create(
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    *,
    db: AsyncMasterSessionDependency,
    file: UploadFile = File(description='Files of type: ["text/csv", "application/json"]'),
) -> BulkLoadResponse:
    """Create or update sources from a file, keyed on slug (the ETL's ``source_id``).

    Reports each source as created, updated (with the fields that changed), or unchanged. Saves
    nothing if any row has an error, including a name or short name another source has in the
    same edition. Supports file types: [application/json, text/csv]
    """
    try:
        with logger.contextualize(user=_log_user(current_user), filename=file.filename, log_threads=True):
            logger.info("Bulk loading sources")
            rows = await read_rows(file)
            response = BulkLoadResponse(filename=file.filename or "")

            async with sqlalchemy_uow(db, None) as uow:
                seen: set[str] = set()
                # sequential: one AsyncSession can't run concurrent queries
                for index, row in rows:
                    await upsert_and_mutate_report(uow, response, index, row, current_user_id, seen)

                if response.errors:
                    await uow.rollback()
                else:
                    try:
                        await uow.db_session.flush()
                        await uow.commit()
                    except IntegrityError as e:
                        await uow.rollback()
                        logger.error("Bulk source commit failed: {}", str(e))
                        response.errors.append(
                            "Commit failed on a unique constraint (a name or short name another source "
                            "has in the same edition), nothing was saved (see server logs for the row)."
                        )

            response.update_totals()
            return response
    except HTTPException:
        # assume that the error was already logged
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e
