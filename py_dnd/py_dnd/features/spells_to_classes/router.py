"""Class spell lists: which spells each class can learn.

    GET    /classes/{class_id}/spells              the class's links (for spell details use
                                                   /spells/query?filter={"classes": {"filterType": "set", ...}})
    POST   /classes/{class_id}/spells/{spell_id}   add a spell (returns the link if it's already there)
    DELETE /classes/{class_id}/spells/{spell_id}   remove it
    POST   /classes/spell-lists/bulk               add links from a spells ``_all.json`` (each row's ``classes``)

A listing like "Bard (Optional)" links Bard with qualifier "Optional" (Tasha's optional spells).

Include this before the class catalog router: its ``/{source_slug}/{slug}`` would otherwise match first.
"""

import re
import uuid
from typing import Annotated, Any

from fastapi import APIRouter, File, HTTPException, Query, UploadFile, status
from loguru import logger
from sqlalchemy import delete, func, select
from sqlalchemy.ext.asyncio import AsyncSession

from py_dnd.database.db import AsyncMasterSessionDependency, AsyncReplicaSessionDependency
from py_dnd.features.auth.service import UserAuth, UserAuthOptional
from py_dnd.features.catalog.router import _internal_error, _log_user, _now, bare_slug
from py_dnd.features.classes.models import DndClass
from py_dnd.features.core.unit_of_work import SqlAlchemyUnitOfWork, sqlalchemy_uow
from py_dnd.features.sources.service import find_source_for_row
from py_dnd.features.spells.models import Spell
from py_dnd.features.spells_to_classes.models import SpellClass
from py_dnd.features.spells_to_classes.schemas import SpellClassLink
from py_dnd.features.user.service import CurrentUserId
from py_dnd.shared.schemas import BulkLoadResponse, GenericListResponse
from py_dnd.shared.uploads import read_rows

router = APIRouter()

# "Bard (Optional)", "Wizard (Dunamancy)"
_QUALIFIED = re.compile(r"^(?P<name>.+?)\s*\((?P<qualifier>[^)]+)\)$")


def _split_qualifier(listed: str) -> tuple[str, str | None]:
    """A spell's class listing as (class name, qualifier): "Bard (Optional)" -> ("Bard", "Optional")."""
    match = _QUALIFIED.match(listed.strip())
    return (match["name"], match["qualifier"].strip()) if match else (listed.strip(), None)


async def _require_live(session: AsyncSession, model: Any, entity_id: uuid.UUID, label: str) -> None:
    found = await session.scalar(select(model.id).where(model.id == entity_id, model.deleted_at.is_(None)))
    if found is None:
        raise HTTPException(status.HTTP_404_NOT_FOUND, f"{label} {entity_id} not found")


@router.get("/{class_id:uuid}/spells")
async def read_class_spells(
    class_id: uuid.UUID,
    current_user: UserAuthOptional,
    db: AsyncReplicaSessionDependency,
) -> GenericListResponse[SpellClassLink]:
    """The spells on a class's list (live spells only), by spell name."""
    try:
        with logger.contextualize(user=_log_user(current_user), class_id=class_id, log_threads=True):
            async with sqlalchemy_uow(db, None) as uow:
                await _require_live(uow.db_session, DndClass, class_id, "Class")
                links = await uow.db_session.scalars(
                    select(SpellClass)
                    .join(Spell, Spell.id == SpellClass.spell_id)
                    .where(SpellClass.class_id == class_id, Spell.deleted_at.is_(None))
                    .order_by(Spell.name, Spell.dnd_version_year)
                )
                return GenericListResponse[SpellClassLink](
                    entities=[SpellClassLink.model_validate(link) for link in links]
                )
    except HTTPException:
        raise
    except Exception as e:
        raise _internal_error(e) from e


@router.post("/{class_id:uuid}/spells/{spell_id:uuid}")
async def add_class_spell(
    class_id: uuid.UUID,
    spell_id: uuid.UUID,
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    db: AsyncMasterSessionDependency,
    qualifier: Annotated[
        str | None, Query(description='"Optional" or a subclass ("Dunamancy"); omit for the class list')
    ] = None,
) -> SpellClassLink:
    """Put a spell on a class's list. Returns the link (the existing one, unchanged, if it's already there)."""
    try:
        with logger.contextualize(
            user=_log_user(current_user), class_id=class_id, spell_id=spell_id, log_threads=True
        ):
            logger.info("Adding spell to class")
            async with sqlalchemy_uow(db, None) as uow:
                session = uow.db_session
                await _require_live(session, DndClass, class_id, "Class")
                await _require_live(session, Spell, spell_id, "Spell")
                existing = await session.get(SpellClass, (spell_id, class_id))
                if existing is not None:
                    return SpellClassLink.model_validate(existing)
                now = _now()
                link = SpellClass(
                    spell_id=spell_id,
                    class_id=class_id,
                    qualifier=qualifier or None,
                    created_at=now,
                    created_by=current_user_id,
                    updated_at=now,
                    updated_by=current_user_id,
                )
                # read before the commit expires the row
                out = SpellClassLink.model_validate(link)
                session.add(link)
                await session.flush()
                await uow.commit()
                return out
    except HTTPException:
        raise
    except Exception as e:
        raise _internal_error(e) from e


@router.delete("/{class_id:uuid}/spells/{spell_id:uuid}")
async def remove_class_spell(
    class_id: uuid.UUID,
    spell_id: uuid.UUID,
    current_user: UserAuth,
    db: AsyncMasterSessionDependency,
) -> str:
    """Take a spell off a class's list (deletes the link; the spell and class stay). Returns the spell id."""
    try:
        with logger.contextualize(
            user=_log_user(current_user), class_id=class_id, spell_id=spell_id, log_threads=True
        ):
            logger.info("Removing spell from class")
            async with sqlalchemy_uow(db, None) as uow:
                removed = await uow.db_session.scalar(
                    delete(SpellClass)
                    .where(SpellClass.class_id == class_id, SpellClass.spell_id == spell_id)
                    .returning(SpellClass.spell_id)
                )
                if removed is None:
                    raise HTTPException(
                        status.HTTP_404_NOT_FOUND, f"Spell {spell_id} isn't on class {class_id}'s list"
                    )
                await uow.commit()
            return str(spell_id)
    except HTTPException:
        raise
    except Exception as e:
        raise _internal_error(e) from e


@router.post("/spell-lists/bulk")
async def bulk_class_spells(
    current_user: UserAuth,
    current_user_id: CurrentUserId,
    *,
    db: AsyncMasterSessionDependency,
    file: UploadFile = File(description='Files of type: ["text/csv", "application/json"]'),
) -> BulkLoadResponse:
    """Add spell list links from a spells file (an ETL ``_all.json``): each row's ``classes`` names.

    Spells are found by (source, slug) and classes by name in the spell's edition, so load sources,
    spells and classes first. Only adds links (never removes one). Reports each spell as created (its
    first links), updated (links added) or unchanged. A class name with no class is a warning; a
    missing spell is an error, and any error saves nothing.
    """
    try:
        with logger.contextualize(user=_log_user(current_user), filename=file.filename, log_threads=True):
            logger.info("Bulk loading spell lists")
            rows = await read_rows(file)
            response = BulkLoadResponse(filename=file.filename or "")
            async with sqlalchemy_uow(db, None) as uow:
                classes: dict[tuple[str, int], list[uuid.UUID]] = {}
                for index, row in rows:
                    try:
                        await _link_row(uow, response, index, row, current_user_id, classes)
                    except HTTPException as e:
                        response.errors.append(f"row {index} [{row.get('slug')}]: {e.detail}")
                    except Exception as e:  # pylint: disable=broad-exception-caught  # reported per row
                        response.errors.append(f"row {index} [{row.get('slug')}]: {e}")
                if response.errors:
                    await uow.rollback()
                else:
                    await uow.db_session.flush()
                    await uow.commit()
            response.update_totals()
            return response
    except HTTPException:
        raise
    except Exception as e:
        raise _internal_error(e) from e


async def _link_row(  # pylint: disable=too-many-arguments,too-many-positional-arguments  # one bulk row's context
    uow: SqlAlchemyUnitOfWork,
    response: BulkLoadResponse,
    index: int,
    row: dict[str, Any],
    user_id: uuid.UUID | None,
    classes: dict[tuple[str, int], list[uuid.UUID]],
) -> None:
    """Add one spell's missing links and record what happened."""
    session = uow.db_session
    names = [n for n in row.get("classes") or [] if isinstance(n, str) and n.strip()]
    source = await find_source_for_row(uow.source_repo, row)
    if source is None:
        response.errors.append(f"row {index} [{row.get('slug')}]: source not found")
        return
    slug = bare_slug(str(row.get("slug") or ""))
    spell = await uow.spell_repo.read_by_natural_key(source.id, slug)
    entry_label = f"{slug} ({source.slug})"
    if spell is None:
        response.errors.append(f"row {index} [{entry_label}]: spell not found (load spells first)")
        return
    if spell.deleted_at is not None:
        response.warnings.append(f"{entry_label}: spell deleted, so not linked")
        return
    # class id -> (name as listed, qualifier); a plain listing beats a qualified one
    wanted: dict[uuid.UUID, tuple[str, str | None]] = {}
    for name in names:
        class_name, qualifier = _split_qualifier(name)
        key = (class_name.lower(), spell.dnd_version_year)
        if key not in classes:
            classes[key] = list(
                await session.scalars(
                    select(DndClass.id).where(
                        func.lower(DndClass.name) == key[0],
                        DndClass.dnd_version_year == key[1],
                        DndClass.deleted_at.is_(None),
                    )
                )
            )
        if not classes[key]:
            response.warnings.append(f"{entry_label}: no {key[1]} class named '{class_name}'")
        for class_id in classes[key]:
            if class_id not in wanted or wanted[class_id][1] is not None:
                wanted[class_id] = (name, qualifier)
    existing = set(await session.scalars(select(SpellClass.class_id).where(SpellClass.spell_id == spell.id)))
    added = {class_id: name for class_id, name in wanted.items() if class_id not in existing}
    if not added:
        response.unchanged.append(entry_label)
        return
    now = _now()
    session.add_all(
        SpellClass(
            spell_id=spell.id,
            class_id=class_id,
            qualifier=added[class_id][1],
            created_at=now,
            created_by=user_id,
            updated_at=now,
            updated_by=user_id,
        )
        for class_id in added
    )
    added_names = ", ".join(sorted({name for name, _ in added.values()}))
    if existing:
        response.updated.append(f"{entry_label}: added {added_names}")
    else:
        response.created.append(f"{entry_label}: {added_names}")
