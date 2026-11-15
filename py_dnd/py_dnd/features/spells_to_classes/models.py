"""SQLAlchemy Table: which classes have a spell on their spell list."""

import uuid

from sqlalchemy import ForeignKey, Index
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.features.classes.models import DndClass
from py_dnd.features.spells.models import Spell
from py_dnd.shared.models import MixinBookeeping


class SpellClass(MixinBookeeping, DndSchemaBase):
    """A spell on a class's spell list. Removing a link deletes the row (nothing else points at it)."""

    __tablename__ = "spell_class"
    __table_args__ = (
        # the primary key covers spell -> classes; this covers class -> spells
        Index("ix_dnd_spell_class_class_id", "class_id"),
        {"schema": "dnd"},
    )

    spell_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(Spell.id, ondelete="CASCADE"), primary_key=True)
    class_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(DndClass.id, ondelete="CASCADE"), primary_key=True)
    # what the spell lists after the class name: "Optional" (Tasha's optional class spells) or a
    # subclass ("Dunamancy"); null when the spell is on the class's own list
    qualifier: Mapped[str | None] = mapped_column(nullable=True)
