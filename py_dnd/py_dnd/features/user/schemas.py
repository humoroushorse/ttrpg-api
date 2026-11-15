"""User schemas."""

import uuid

from pydantic import BaseModel, ConfigDict, Field, field_validator

from py_dnd.shared.schemas import QueryBase


class UserSchemaBase(BaseModel):
    """How the User shows up in the database."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        from_attributes=True,
        str_strip_whitespace=True,
    )

    # Fields of the model
    id: uuid.UUID = Field(title="User ID")
    keycloak_id: uuid.UUID = Field(title="Keycloak User ID", description="The ID token's `sub`.")
    username: str | None = Field(default=None, title="Username")
    profile_picture_url: str | None = Field(default=None, title="Profile Picture URL")


class UserSchema(UserSchemaBase):
    """How the User shows up in the database (with relationships)."""


class UserCreate(BaseModel):
    """Fields used to create a user (the database generates the id)."""

    model_config = ConfigDict(
        extra="ignore",
        validate_assignment=True,
        str_strip_whitespace=True,
    )
    keycloak_id: uuid.UUID = Field(title="Keycloak User ID")
    username: str | None = Field(default=None, title="Username")
    profile_picture_url: str | None = Field(default=None, title="Profile Picture URL")

    @field_validator("keycloak_id", mode="before")
    @classmethod
    def validate_uuid(cls, v: uuid.UUID | str) -> uuid.UUID:
        """Force inputs to uuid.UUID."""
        if isinstance(v, str):
            return uuid.UUID(v.strip())
        return v

    # @field_serializer('id')
    # def serialize_uuid(self, v: uuid.UUID) -> str:
    #     return str(v)


class UserUpdate(BaseModel):
    """Fields you can update for a user."""

    model_config = ConfigDict(
        extra="ignore",
        validate_assignment=True,
        str_strip_whitespace=True,
    )

    username: str | None = Field(default=None, title="Username")
    profile_picture_url: str | None = Field(default=None, title="Profile Picture URL")


class UserQuery(QueryBase):
    """Fields you can use to query user(s)."""

    model_config = ConfigDict(
        extra="forbid",
        validate_assignment=True,
        str_strip_whitespace=True,
    )
    username: str | None = Field(default=None, title="Username")
    profile_picture_url: str | None = Field(default=None, title="Profile Picture URL")
