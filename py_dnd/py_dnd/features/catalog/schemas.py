"""Schemas every catalog entity builds on.

Per entity, write ``<Entity>Fields(CatalogFields)`` with its own fields, then
``<Entity>Schema(<Entity>Fields, CatalogRow)`` (what the API returns) and
``<Entity>Create(<Entity>Fields, CatalogCreateRow)`` (a new row). POST and PUT take the
fields model; PATCH takes ``patch_model(<Entity>Fields)``.
"""

import datetime
import uuid
from typing import Any, Self

from pydantic import AliasChoices, BaseModel, ConfigDict, Field, create_model, model_validator

from py_dnd.features.sources.schemas import SourceRef
from py_dnd.shared.schemas import QueryBase


class CatalogFields(BaseModel):
    """The fields every catalog entry has that a client writes."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    source_id: uuid.UUID = Field(title="Source ID", description="The book's database id.")
    slug: str = Field(title="Slug", description="Unique within its source (the natural key with source_id).")
    name: str = Field(title="Name")
    dnd_version: str = Field(title="D&D Version")
    dnd_version_year: int = Field(title="D&D Version Year", description="The rules edition (2014, 2024).")
    is_unearthed_arcana: bool = Field(default=False, title="Is Unearthed Arcana")
    is_homebrew: bool = Field(default=False, title="Is Homebrew")
    marks: list[str] = Field(default_factory=list, title="Marks", description="Flags such as third_party.")
    description: str = Field(
        title="Description",
        description="HTML: the entry's text, without a heading.",
        # the ETL's name for it
        validation_alias=AliasChoices("description", "description_html"),
    )


class CatalogRow(BaseModel):
    """What the API adds to the fields: id, bookkeeping, and the source (read only)."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, from_attributes=True)

    id: uuid.UUID = Field(title="ID")
    created_at: datetime.datetime
    created_by: uuid.UUID | None
    updated_at: datetime.datetime
    updated_by: uuid.UUID | None
    source: SourceRef = Field(title="Source", description="[Read only] The book the entry is printed in.")


class CatalogCreateRow(BaseModel):
    """What a new row adds to the fields. Ignores keys it doesn't know (the ETL's files carry more)."""

    model_config = ConfigDict(extra="ignore", validate_assignment=True, str_strip_whitespace=True)

    id: uuid.UUID = Field(default_factory=uuid.uuid7)
    created_at: datetime.datetime
    created_by: uuid.UUID | None
    updated_at: datetime.datetime
    updated_by: uuid.UUID | None


class LevelRow(BaseModel):
    """One row of a leveled table (Exhaustion levels, object Armor Class by material)."""

    model_config = ConfigDict(extra="forbid")

    level: int
    effect: str


class PatchBase(BaseModel):
    """Base of ``patch_model``: only the fields a request sets change."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    @model_validator(mode="after")
    def _no_null_for_required(self) -> Self:
        """``null`` clears an optional field; a field the entry must have can't be cleared."""
        not_null = _NOT_NULL.get(type(self), frozenset())
        cleared = sorted(k for k in self.model_fields_set & not_null if getattr(self, k) is None)
        if cleared:
            raise ValueError(f"these fields can't be null: {', '.join(cleared)}")
        return self

    def changes(self) -> dict[str, Any]:
        """The fields the request sets, ready for the repository."""
        return self.model_dump(exclude_unset=True)


_NOT_NULL: dict[type[BaseModel], frozenset[str]] = {}


def patch_model(fields: type[BaseModel]) -> type[PatchBase]:
    """A PATCH body for ``fields``: every field optional; ``null`` only for the ones that allow it.

    Named ``<Entity>PatchInput`` from ``<Entity>Fields``. Lists and objects are replaced whole.
    """
    name = fields.__name__.removesuffix("Fields") + "PatchInput"
    model = create_model(  # type: ignore[call-overload]
        name,
        __base__=PatchBase,
        **{
            key: (info.annotation | None, Field(default=None, title=info.title, description=info.description))
            for key, info in fields.model_fields.items()
        },
    )
    _NOT_NULL[model] = frozenset(
        key
        for key, info in fields.model_fields.items()
        if info.is_required() or info.default is not None or info.default_factory is not None
    )
    return model  # type: ignore[no-any-return]


class CatalogQuery(QueryBase):
    """Query params every catalog list takes (besides ``?filter=``, ``?sort=``, ``?fields=``)."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    name: str | None = Field(default=None, title="Names", description='Name(s) to filter on (separated by commas ",")')
