"""Shared schemas."""

import datetime
import uuid
from typing import Annotated, Any, Self

from pydantic import BaseModel, Field, model_validator


class MixinBookeepingCreate:
    """Entity Creation Bookeeping Mixin. The ids are dnd.user ids (None once that user is purged)."""

    created_at: Annotated[datetime.datetime, Field()]
    created_by: Annotated[uuid.UUID | None, Field()]


class MixinBookeepingUpdate:
    """Entity Update Bookeeping Mixin."""

    updated_at: Annotated[datetime.datetime, Field()]
    updated_by: Annotated[uuid.UUID | None, Field()]


class MixinImageUrl:
    """Schema Mixin for adding image information."""

    image_url: str | None = Field(default=None, title="Image URL")
    image_url_description: str | None = Field(default=None, title="Image URL description (for accessability)")

    @model_validator(mode="before")
    @classmethod
    def image_must_also_have_description(cls, data: Any) -> Any:
        """Validator for forcing images to have descriptions for accessability reasons."""
        if isinstance(data, dict):
            if data.get("image_url") and not data.get("image_url_description"):
                raise ValueError("Image must also have a description for accessability reasons")
        return data


class QueryBase(BaseModel):
    """Query options for pagination."""

    limit: int | None = Field(
        default=100,
        title="Limit",
    )
    offset: int | None = Field(default=0, title="Offset")


class GenericListResponse[T: BaseModel](BaseModel):
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
    def validate_model_generic(self) -> Self:
        """Default the entity counts from `entities` when they are not provided.

        Returns:
            Self: The validated model.
        """
        entities = self.entities or []
        if not self.entities_count:
            self.entities_count = len(entities)
        # If not provided, assume we're returning everything
        if not self.total_entities_count:
            self.total_entities_count = len(entities)
        return self


class BulkLoadResponse(BaseModel):
    """Response type for bulk loading resources."""

    class BulkLoadResponseTotals(BaseModel):
        """Summary report for bulk loading."""

        created: int | None = 0
        updated: int | None = 0
        unchanged: int | None = 0
        errored: int | None = 0
        warning: int | None = 0

    filename: str
    totals: BulkLoadResponseTotals = BulkLoadResponseTotals()
    created: list[str] = Field(default_factory=list)
    updated: list[str] = Field(default_factory=list, description="Existing rows the upload changed.")
    unchanged: list[str] = Field(default_factory=list, description="Existing rows that already matched.")
    errors: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)

    def update_totals(self) -> None:
        """Updates totals based on current array lengths."""
        self.totals.created = len(self.created)
        self.totals.updated = len(self.updated)
        self.totals.unchanged = len(self.unchanged)
        self.totals.errored = len(self.errors)
        self.totals.warning = len(self.warnings)
