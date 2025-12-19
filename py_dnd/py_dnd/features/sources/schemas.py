"""Sources schemas."""

from pydantic import AliasChoices, BaseModel, ConfigDict, Field, StrictInt

from py_dnd.shared.schemas import MixinBookeeping, MixinBookeepingCreate, MixinBookeepingUpdate, QueryBase, MixinUuid7Id


class SourceSchema(MixinBookeeping, MixinUuid7Id):
    """How the source shows up in the database."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )

    # Fields of the model
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


class SourceCreate(BaseModel):
    """Allowed fields for creating a source."""
    model_config = ConfigDict(
        extra="ignore",
        validate_assignment=True,
        from_attributes=True,
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


class SourceCreateDerrived(SourceCreate, MixinBookeepingCreate, MixinUuid7Id):
    """Allowed fields for creating a source."""
    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )
    pass


class SourceUpdate(SourceSchema):
    """Allowed fields for editing a source."""
    model_config = ConfigDict(
        extra="ignore",
        validate_assignment=True,
        from_attributes=True,
    )

    name: str | None = Field(
        default=None, 
        title="Name", 
        description="The name of the source."
    )
    name_short: str | None = Field(
        default=None,
        title="Short Name",
        description="A short identifier for the source",
    )
    publish_year: StrictInt | None = Field(
        default=None, 
        title="D&D Source Release Year",
        description="The year that the source was published (the source year not Dungeons and Dragons year)."
    )
    dnd_version: str | None = Field(
        default=None,
        title="D&D Version",
        description="The version of Dungeons and Dragons."
    )
    dnd_version_year: int | None = Field(
        default=None, 
        title="D&D Version Year", 
        description="The year that the version of Dungeons and Dragons came out (not typically the source_year)."
    )


class SourceUpdateDerrived(SourceUpdate, MixinBookeepingUpdate):
    """Allowed fields for editing a source."""
    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )


class SourceQuery(QueryBase):
    """Allowed fields for querying source."""
    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
    )

    name: str | None = Field(default=None, title="Names", description='Name(s) to filter on (separated by commas ",")')
