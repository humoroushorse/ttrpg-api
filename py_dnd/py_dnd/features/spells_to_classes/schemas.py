"""Spell list link schemas."""

import datetime
import uuid

from pydantic import BaseModel, ConfigDict, Field


class SpellClassLink(BaseModel):
    """A spell on a class's spell list."""

    model_config = ConfigDict(from_attributes=True)

    spell_id: uuid.UUID = Field(title="Spell Id")
    class_id: uuid.UUID = Field(title="Class Id")
    qualifier: str | None = Field(
        default=None,
        title="Qualifier",
        description='"Optional" (Tasha\'s optional class spells) or a subclass ("Dunamancy"); null for the class list',
    )
    created_at: datetime.datetime = Field(title="Created At")
    created_by: uuid.UUID | None = Field(default=None, title="Created By")
