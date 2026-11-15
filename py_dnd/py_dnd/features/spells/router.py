"""Spell route definitions."""

import datetime
import uuid
from typing import Annotated, Any

from fastapi import APIRouter, Depends, File, HTTPException, UploadFile, status
from loguru import logger
from sqlalchemy import ColumnElement, exists, func
from sqlalchemy.exc import IntegrityError

from py_dnd.database.db import (
    AsyncMasterSessionDependency,
    AsyncReplicaSessionDependency,
)
from py_dnd.features.auth.schemas import AuthUserToken
from py_dnd.features.auth.service import UserAuth, UserAuthOptional
from py_dnd.features.classes.models import DndClass
from py_dnd.features.core.unit_of_work import SqlAlchemyUnitOfWork, sqlalchemy_uow
from py_dnd.features.sources.models import Source
from py_dnd.features.sources.schemas import SourceSchema
from py_dnd.features.sources.service import find_source_for_row
from py_dnd.features.spells.models import Spell
from py_dnd.features.spells.schemas import (
    SPELL_FIELDS,
    SPELL_READ_FIELDS,
    SpellCreate,
    SpellCreateInput,
    SpellPatchInput,
    SpellQuery,
    SpellReplaceInput,
    SpellSchema,
    SpellSchemaPartial,
)
from py_dnd.features.spells_to_classes.models import SpellClass
from py_dnd.features.user.service import CurrentUserId
from py_dnd.shared.grid import ColumnKind, GridColumn, GridQuery, grid_columns, grid_query
from py_dnd.shared.schemas import BulkLoadResponse, GenericListResponse
from py_dnd.shared.uploads import read_rows

router = APIRouter()

# Keys an upload can't set: the database and the API fill them
_SERVER_KEYS = (
    "id",
    "spell_id",
    "spellId",
    "created_at",
    "created_by",
    "updated_at",
    "updated_by",
    "deleted_at",
    "deleted_by",
)


def _log_user(current_user: AuthUserToken | None) -> dict[str, str]:
    return {"sub": current_user.sub, "preferred_username": current_user.preferred_username} if current_user else {}


def _now() -> datetime.datetime:
    return datetime.datetime.now(tz=datetime.UTC)


async def _live_spell(uow: SqlAlchemyUnitOfWork, entity_id: uuid.UUID) -> Spell:
    """The spell row, or a 404 (soft-deleted spells count as gone)."""
    spell = await uow.spell_repo.read_by_id(entity_id=entity_id, as_model=True)
    if not isinstance(spell, Spell):
        raise HTTPException(status.HTTP_404_NOT_FOUND, f"Spell {entity_id} not found")
    return spell


ReadFields = Annotated[tuple[str, ...], Depends(SPELL_READ_FIELDS)]


@router.get("", response_model_exclude_unset=True)
async def read_spells(
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
    offset: int = 0,
    limit: int = 100,
) -> list[SpellSchemaPartial]:
    """Retrieve spells. ``fields`` picks the fields returned."""
    try:
        with logger.contextualize(user=_log_user(current_user), limit=limit, offset=offset, log_threads=True):
            logger.info("Fetching spells")
            async with sqlalchemy_uow(db, None) as uow:
                entities = await uow.spell_repo.read_multi(offset=offset, limit=limit, fields=fields)
            return entities  # type: ignore[return-value]
    except HTTPException:
        # assume that the error was already logged
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


def _versions_of(slug: str) -> ColumnElement[bool]:
    """Every printing of a spell: slugs equal once hyphens are removed (bigbys-hand, bigby-s-hand)."""
    return func.replace(Spell.slug, "-", "") == slug.replace("-", "")


# What ?filter= and ?sort= accept on /spells/query (keys are the UI's column ids)
SPELL_GRID_COLUMNS = {
    **grid_columns(
        Spell,
        {
            ColumnKind.TEXT: (
                "name",
                "slug",
                "casting_time",
                "range",
                "duration",
                "materials",
                "description",
                "at_higher_levels",
            ),
            ColumnKind.NUMBER: ("level", "source_page", "difficulty_class_saving_throw_override"),
            ColumnKind.YEAR: ("dnd_version_year",),
            ColumnKind.BOOLEAN: (
                "is_ritual",
                "is_unearthed_arcana",
                "is_homebrew",
                "has_verbal_component",
                "has_somatic_component",
                "has_material_component",
                "has_spell_cost",
                "are_materials_consumed",
                "is_concentration",
                "has_saving_throw",
            ),
            ColumnKind.SET: ("school", "damage_type", "difficulty_class_type"),
        },
    ),
    # the source's slug (phb_2024); sorting by source needs a join, so not yet
    "source": GridColumn(
        Spell.source_id,
        ColumnKind.SET,
        sortable=False,
        set_condition=lambda slugs: Spell.source.has(Source.slug.in_([str(s) for s in slugs])),
    ),
    # on any of these classes' spell lists, by class slug (wizard); pair with dnd_version_year
    "classes": GridColumn(
        Spell.id,
        ColumnKind.SET,
        sortable=False,
        set_condition=lambda slugs: exists().where(
            SpellClass.spell_id == Spell.id,
            SpellClass.class_id == DndClass.id,
            DndClass.slug.in_([str(s) for s in slugs]),
            DndClass.deleted_at.is_(None),
        ),
    ),
}
SpellGrid = Annotated[GridQuery, Depends(grid_query(SPELL_GRID_COLUMNS))]


@router.get("/query", response_model_exclude_unset=True)
async def query_spells(
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
    grid: SpellGrid,
    params: SpellQuery = Depends(),
) -> GenericListResponse[SpellSchemaPartial]:
    """Retrieve spells.

    ``filter`` and ``sort`` take AG Grid's filter and sort models (JSON); ``fields`` picks the fields
    returned; ``versions_of`` finds a spell's printings. Sorted by name, then newest edition, after ``sort``.
    """
    try:
        with logger.contextualize(
            user=_log_user(current_user), params=params.model_dump(exclude_none=True), log_threads=True
        ):
            logger.info("Querying spells")
            filters = params.model_dump(exclude_none=True, exclude={"limit", "offset"})
            where = [*grid.where, *([_versions_of(params.versions_of)] if params.versions_of else [])]
            async with sqlalchemy_uow(db, None) as uow:
                entities, total_entities_count = await uow.spell_repo.query(
                    params={k: v for k, v in filters.items() if k != "versions_of"},
                    offset=params.offset,
                    limit=params.limit,
                    # the defaults after the client's sort keep pages stable
                    order_by=(*grid.order_by, Spell.name, Spell.dnd_version_year.desc(), Spell.id),
                    fields=fields,
                    where=where,
                )
            return GenericListResponse[SpellSchemaPartial](
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


@router.get("/{entity_id}", response_model_exclude_unset=True)
async def read_spell(
    entity_id: uuid.UUID,
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
) -> SpellSchemaPartial:
    """Retrieve one spell. ``fields`` picks the fields returned."""
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            async with sqlalchemy_uow(db, None) as uow:
                spell = await uow.spell_repo.read_by_id(entity_id=entity_id, fields=fields)
            if spell is None:
                raise HTTPException(status.HTTP_404_NOT_FOUND, f"Spell {entity_id} not found")
            return spell  # type: ignore[return-value]
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.get("/{source_slug}/{slug}", response_model_exclude_unset=True)
async def read_spell_by_slugs(
    source_slug: str,
    slug: str,
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
    fields: ReadFields,
) -> SpellSchemaPartial:
    """Retrieve one spell by its source's slug and its own (phb_2024/acid-splash), the UI's URL key."""
    try:
        with logger.contextualize(user=_log_user(current_user), source_slug=source_slug, slug=slug, log_threads=True):
            async with sqlalchemy_uow(db, None) as uow:
                spell = await uow.spell_repo.read_by_slugs(source_slug, slug, fields=fields)
            if spell is None:
                raise HTTPException(status.HTTP_404_NOT_FOUND, f"Spell {source_slug}/{slug} not found")
            return spell  # type: ignore[return-value]
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.post("")
async def create_spell(
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SpellCreateInput,
) -> SpellSchema:
    """Create spell."""
    try:
        with logger.contextualize(user=_log_user(current_user), slug=model_in.slug, log_threads=True):
            logger.info("Creating spell")
            now = _now()
            model_create = SpellCreate(
                **model_in.model_dump(),
                created_at=now,
                created_by=current_user_id,
                updated_at=now,
                updated_by=current_user_id,
            )
            async with sqlalchemy_uow(db, None) as uow:
                entity = await uow.spell_repo.create_and_reload(model_in=model_create)
                await uow.commit()
            return entity
    except HTTPException:
        # assume that the error was already logged
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


async def _write(
    db: AsyncMasterSessionDependency, entity_id: uuid.UUID, values: dict[str, Any], user_id: uuid.UUID
) -> SpellSchema:
    """Apply column values to a live spell and return it as stored."""
    async with sqlalchemy_uow(db, None) as uow:
        spell = await _live_spell(uow, entity_id)
        if not values:  # an empty PATCH changes nothing
            return SpellSchema.model_validate(spell)
        uow.spell_repo.apply_changes(spell, {**values, "updated_at": _now(), "updated_by": user_id})
        entity = await uow.spell_repo.flush_and_reload(spell)
        await uow.commit()
    return entity


@router.put("/{entity_id}")
async def replace_spell(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SpellReplaceInput,
) -> SpellSchema:
    """Replace a spell: every field is set, and fields left out go back to their defaults."""
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            logger.info("Replacing spell")
            return await _write(db, entity_id, model_in.model_dump(), current_user_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.patch("/{entity_id}")
async def update_spell(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    model_in: SpellPatchInput,
) -> SpellSchema:
    """Change some of a spell's fields. ``null`` clears an optional field; ``vtt`` is replaced whole."""
    try:
        changes = model_in.changes()
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, fields=sorted(changes)):
            logger.info("Updating spell")
            return await _write(db, entity_id, changes, current_user_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


@router.delete("/{entity_id}")
async def delete_spell(
    entity_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
) -> str:
    """Delete a spell (soft: it's hidden from every read and keeps its slug). Returns its id."""
    try:
        with logger.contextualize(user=_log_user(current_user), entity_id=entity_id, log_threads=True):
            logger.info("Deleting spell")
            async with sqlalchemy_uow(db, None) as uow:
                deleted = await uow.spell_repo.soft_delete(entity_id=entity_id, deleted_by=current_user_id)
                if deleted is None:
                    raise HTTPException(status.HTTP_404_NOT_FOUND, f"Spell {entity_id} not found")
                await uow.commit()
            return str(entity_id)
    except HTTPException:
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e


async def _find_source(uow: SqlAlchemyUnitOfWork, row: dict[str, Any]) -> SourceSchema | None:
    """Find a spell row's source by slug (the ETL's source_id), then name + version, then short name."""
    return await find_source_for_row(uow.source_repo, row)


async def upsert_and_mutate_report(
    uow: SqlAlchemyUnitOfWork,
    response: BulkLoadResponse,
    index: int,
    row: dict[str, Any],
    user_id: uuid.UUID,
    seen: set[tuple[uuid.UUID, str]],
) -> None:
    """Create or update one spell, keyed on (source_id, slug), and record what happened.

    Args:
        uow (SqlAlchemyUnitOfWork): the upload's unit of work (one transaction)
        response (BulkLoadResponse): the report this row adds to
        index (int): the row number in the file
        row (dict[str, Any]): the spell, as in the ETL's spell files
        user_id (uuid.UUID): the uploader's dnd.user id
        seen (set[tuple[uuid.UUID, str]]): natural keys of earlier rows in this file
    """
    label = f"row {index} [{row.get('slug') or row.get('name') or '?'}]"
    try:
        row = {k: v for k, v in row.items() if k not in _SERVER_KEYS}
        source = await _find_source(uow, row)
        if source is None:
            response.errors.append(f"{label}: source '{row.get('source_name') or row.get('source_id')}' not found")
            return
        row["source_id"] = source.id
        row.setdefault("dnd_version", source.dnd_version)
        row.setdefault("dnd_version_year", source.dnd_version_year)
        now = _now()
        incoming = SpellCreate(**row, created_at=now, created_by=user_id, updated_at=now, updated_by=user_id)
        label = f"{incoming.slug} ({source.slug})"
        key = (incoming.source_id, incoming.slug)
        # earlier rows aren't flushed, so the lookup below can't see them
        if key in seen:
            response.errors.append(f"row {index} [{label}]: same slug as an earlier row")
            return
        seen.add(key)

        existing = await uow.spell_repo.read_by_natural_key(*key)
        if existing is None:
            await uow.spell_repo.create(model_in=incoming, return_model=False)
            response.created.append(label)
            return
        if existing.deleted_at is not None:
            response.warnings.append(f"{label}: deleted, so not restored or updated")
            return
        values = incoming.model_dump(include=SPELL_FIELDS)
        changed = {k: v for k, v in values.items() if getattr(existing, k) != v}
        if not changed:
            response.unchanged.append(label)
            return
        uow.spell_repo.apply_changes(existing, {**changed, "updated_at": now, "updated_by": user_id})
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
    """Create or update spells from a file, keyed on (source, slug).

    Reports each spell as created, updated (with the fields that changed), or unchanged. Saves
    nothing if any row has an error. Supports file types: [application/json, text/csv]
    """
    try:
        with logger.contextualize(user=_log_user(current_user), filename=file.filename, log_threads=True):
            logger.info("Bulk loading spells")
            rows = await read_rows(file)
            response = BulkLoadResponse(filename=file.filename or "")

            async with sqlalchemy_uow(db, None) as uow:
                seen: set[tuple[uuid.UUID, str]] = set()
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
                        logger.error("Bulk spell commit failed: {}", str(e))
                        response.errors.append(
                            "Commit failed on a unique constraint, nothing was saved (see server logs for the row)."
                        )

            response.update_totals()
            return response
    except HTTPException:
        # assume that the error was already logged
        raise
    except Exception as e:
        logger.error("Uncaught error: {}", str(e))
        raise HTTPException(status.HTTP_500_INTERNAL_SERVER_ERROR, "Internal Error") from e
