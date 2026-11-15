"""Sources schemas.

``SourceFields`` holds every field a client writes, once. ``SourceCreateInput`` (POST) and
``SourceReplaceInput`` (PUT) need the whole source, ``SourcePatchInput`` (PATCH) takes any subset.
``SourceSchema`` is what the API returns. Request bodies reject unknown keys; the bulk upload
ignores them.
"""

import uuid
from typing import TYPE_CHECKING, Any, Self

from pydantic import AliasChoices, BaseModel, ConfigDict, Field, StrictInt, model_validator

from py_dnd.shared.fields import fields_query, partial_model
from py_dnd.shared.schemas import (
    MixinBookeepingCreate,
    MixinBookeepingUpdate,
    QueryBase,
)


class SourceFields(BaseModel):
    """Every field a client writes."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    slug: str = Field(
        title="Slug",
        description="Unique key across editions; the ETL's `source_id` (phb_2024).",
    )
    name: str = Field(title="Name", description="The name of the source.")
    name_short: str = Field(
        title="Short Name",
        description="A short identifier for the source",
    )
    publish_year: StrictInt | None = Field(default=None, title="D&D Source Release Year")
    dnd_version: str = Field(title="D&D Version", description="The version of Dungeons and Dragons.")
    dnd_version_year: int = Field(
        title="D&D Version Year", description="The year that the version of Dungeons and Dragons came out."
    )
    marks: list[str] = Field(default_factory=list, title="Marks", description="Flags such as third_party.")


# Writable columns, for updates and the bulk upload's change check
SOURCE_FIELDS = frozenset(SourceFields.model_fields)


class SourceSchemaBase(SourceFields, MixinBookeepingCreate, MixinBookeepingUpdate):
    """How the source shows up in the database."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )

    id: uuid.UUID = Field(
        default_factory=uuid.uuid7,
        title="Source ID",
        description="[Generated] Unique database ID For the Source table.",
        validation_alias=AliasChoices("id", "sourceId"),
    )


class SourceSchema(SourceSchemaBase):
    """How the source shows up in the database (with relationships)."""


# What the read routes return: SourceSchema's fields, all optional, so ?fields= can leave some out.
if TYPE_CHECKING:
    SourceSchemaPartial = SourceSchema
else:
    SourceSchemaPartial = partial_model(SourceSchema)
SOURCE_READ_FIELDS = fields_query(SourceSchema)


class SourceRef(BaseModel):
    """A source as other entities return it: enough to name and link it (read only)."""

    model_config = ConfigDict(from_attributes=True)

    id: uuid.UUID = Field(title="Source ID")
    slug: str = Field(title="Slug", description="Unique key across editions (phb_2024).")
    name: str = Field(title="Name", description="The name of the source.")
    name_short: str = Field(title="Short Name", description="A short identifier for the source.")


class SourceCreateInput(SourceFields):
    """POST body: the whole source. Unknown keys are errors."""


class SourceReplaceInput(SourceFields):
    """PUT body: the whole source. Fields left out go back to their defaults."""


class SourcePatchInput(BaseModel):
    """PATCH body: only the fields to change."""

    model_config = ConfigDict(extra="forbid", validate_assignment=True, str_strip_whitespace=True)

    slug: str | None = None
    name: str | None = None
    name_short: str | None = None
    publish_year: StrictInt | None = None
    dnd_version: str | None = None
    dnd_version_year: int | None = None
    marks: list[str] | None = None

    @model_validator(mode="after")
    def _no_null_for_required(self) -> Self:
        """``null`` clears an optional field; a field the source must have can't be cleared."""
        cleared = sorted(k for k in self.model_fields_set & _NOT_NULL if getattr(self, k) is None)
        if cleared:
            raise ValueError(f"these fields can't be null: {', '.join(cleared)}")
        return self

    def changes(self) -> dict[str, Any]:
        """The fields the request sets, ready for the repository."""
        return self.model_dump(exclude_unset=True)


# Fields that can't be null: no default, or a non-None default
_NOT_NULL = frozenset(
    name
    for name, info in SourceFields.model_fields.items()
    if info.is_required() or info.default is not None or info.default_factory is not None
)
if set(SourcePatchInput.model_fields) != SOURCE_FIELDS:  # keep PATCH in step with SourceFields
    raise RuntimeError("SourcePatchInput's fields differ from SourceFields'")


class SourceCreate(SourceSchemaBase):
    """A new source row (POST and the bulk upload). Ignores keys it doesn't know."""

    model_config = ConfigDict(extra="ignore", validate_assignment=True, str_strip_whitespace=True)


class SourceUpdate(SourceFields, MixinBookeepingUpdate):
    """The columns an update writes (typing for the repository)."""


class SourceQuery(QueryBase):
    """Allowed fields for querying source."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        str_strip_whitespace=True,
    )

    name: str | None = Field(default=None, title="Names", description='Name(s) to filter on (separated by commas ",")')
