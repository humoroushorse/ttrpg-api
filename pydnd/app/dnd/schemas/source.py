"""Schemas for the source table and its CRUD operations."""

from pydantic import BaseModel, Field


class SourceBase(BaseModel):
    """Schema base fields where everything is optional."""

    id: int | None = Field(title="ID", description="Source ID (generated)")
    name: str | None = Field(title="Name", description="Name of the source material")
    short_name: str | None = Field(title="Casting Time", description="Shorthand name of the source material")


class SourceCreate(SourceBase):
    """Schema for creation with required overrides and aditonal fields."""

    name: str = Field(title="Name", description="Name of the source material")
    short_name: str = Field(title="Casting Time", description="Shorthand name of the source material")


class SourceUpdate(SourceBase):
    """Schema for updating model with required overrides and aditonal fields."""

    id: int = Field(title="ID", description="Source ID (generated)")


class SourceResponse(SourceBase):
    """Schema for transforming ORM Model into a response object."""

    class Config:
        """Pydantic model configuration."""

        from_attributes = True
