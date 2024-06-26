"""Shared models for API responses."""

from typing import Generic, List, TypeVar

from pydantic import BaseModel, Field

T = TypeVar("T", int, str)


class GenericListResponse(BaseModel, Generic[T]):
    """Wrapper model for returning list objects with other metadata."""

    total_count: int
    limit: int
    offset: int
    data_count: int
    data: List[T]


class BulkLoadResponse(BaseModel):
    """Response type for bulk loading resources."""

    class BulkLoadResponseTotals(BaseModel):
        """Summary report for bulk loading."""

        created: int | None = 0
        errored: int | None = 0
        warning: int | None = 0

    filename: str
    totals: BulkLoadResponseTotals = BulkLoadResponseTotals()
    created: List[str] = Field(default_factory=list)
    errors: List[str] = Field(default_factory=list)
    warnings: List[str] = Field(default_factory=list)

    def update_totals(self) -> None:
        """Updates totals based on current array lengths."""
        self.totals.created = len(self.created if self.created else [])
        self.totals.errored = len(self.errors if self.errors else [])
        self.totals.warning = len(self.warnings if self.warnings else [])


class MessageResponse(BaseModel):
    """Response message."""

    message: str
