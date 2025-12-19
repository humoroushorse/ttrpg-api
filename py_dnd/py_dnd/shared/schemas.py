"""Shared schemas."""

import datetime
import uuid
import uuid6
from typing import Any, Generic, TypeVar

from pydantic import BaseModel, Field, model_validator, field_validator, AliasChoices

T = TypeVar("T", bound=BaseModel)

class MixinUuid7Id(BaseModel):
    id: uuid.UUID = Field(
        default_factory=uuid6.uuid7,
        title="ID",
        description="[Generated] Unique database ID in UUID7 Form.",
        validation_alias=AliasChoices("id", "ID", "_id"),
    )

class MixinBookeepingCreate(BaseModel):
    """Bookeeping Mixin for creation."""

    created_at: datetime.datetime | None = Field(default=None, title="Created At", description="When this entity was created.")
    created_by: str = Field(title="Created By", description="Who created this entity.")

    @field_validator("created_at", mode="before")
    @classmethod
    def created_to_utc_and_strip_tz(cls, v):
        if isinstance(v, datetime.datetime):
            if v.tzinfo is not None:
                v = v.astimezone(datetime.timezone.utc).replace(tzinfo=None)
        return v
    
class MixinBookeepingUpdate(BaseModel):
    """Bookeeping Mixin for updates."""

    updated_at: datetime.datetime | None = Field(default=None, title="Updated At", description="When this entity was last updated.")
    updated_by: str | None = Field(default=None, title="Updated By", description="Who updated this entity last.")

    @field_validator("updated_at", mode="before")
    @classmethod
    def updated_to_utc_and_strip_tz(cls, v):
        if isinstance(v, datetime.datetime):
            if v.tzinfo is not None:
                v = v.astimezone(datetime.timezone.utc).replace(tzinfo=None)
        return v

class MixinBookeeping(MixinBookeepingCreate, MixinBookeepingUpdate):
    """Bookeeping Mixin for both creation and updates.

    This mixin combines the creation and update fields into a single model.
    It is used to ensure that both created_at/created_by and updated_at/updated_by
    fields are present in the model.
    """
    pass


class QueryBase(BaseModel):
    """Query options for pagination."""

    limit: int | None = Field(
        default=100,
        title="Limit",
    )
    offset: int | None = Field(default=0, title="Offset")


class GenericListResponse(BaseModel, Generic[T]):
    """Wrapper model for returning list objects with other metadata."""

    total_entities_count: int | None = Field(
        default=0,
        title="Total Count",
        description="How many entities exist in the database for filters (excluding limit/offset).",
    )
    limit: int | None = Field(
        default=0,
        title="Limit",
    )
    offset: int | None = Field(default=0, title="Offset")
    filters: dict | None = Field(
        default=None, title="Filters", description="Filters used when querying, includes default filter values."
    )
    entities_count: int | None = Field(
        default=0,
        title="Data Count",
        description="How many entities are returned in this response for given filters (including limit/offset).",
    )
    entities: list[T] | None = Field(
        default_factory=list, title="Entities", description="Entites returned based on given filters."
    )

    @model_validator(mode="after")
    @classmethod
    def validate_model_generic(cls, data: Any) -> Any:
        """General checks for deterniming GenericListResponse field values.

        Args:
            data (Any): _description_

        Returns:
            Any: _description_
        """
        if not data.entities_count:
            data.entities_count = len(data.entities)
        # If not provided, assume we're returning everything
        if not data.total_entities_count:
            data.total_entities_count = len(data.entities)
        return data


class BulkLoadResponse(BaseModel):
    """Response type for bulk loading resources."""

    class BulkLoadResponseTotals(BaseModel):
        """Summary report for bulk loading."""

        created: int | None = 0
        errored: int | None = 0
        warning: int | None = 0

    filename: str
    totals: BulkLoadResponseTotals = BulkLoadResponseTotals()
    created: list[str] = Field(default_factory=list)
    errors: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)

    def update_totals(self) -> None:
        """Updates totals based on current array lengths."""
        self.totals.created = len(self.created if self.created else [])
        self.totals.errored = len(self.errors if self.errors else [])
        self.totals.warning = len(self.warnings if self.warnings else [])
