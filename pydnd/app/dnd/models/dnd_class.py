"""SQLAlchemy Table: dnd_class definition."""

from app.dnd import schemas
from app.dnd.database.base_class import DbBase
from sqlalchemy import ARRAY, Column, ForeignKey, Integer, String
from sqlalchemy.orm import relationship


class DndClass(DbBase):
    """SQLAlchemy dnd_class model."""

    # keys
    id = Column(Integer, primary_key=True, index=True)
    source_id = Column(Integer, ForeignKey(f"{schemas.enums.DbSchemaEnum.DND.value}.source.id"))
    # fields
    name = Column(String, nullable=False, unique=True)
    description = Column(String, nullable=False)
    hit_die = Column(String, nullable=False)
    primary_ability = Column(String, nullable=False)
    saving_throw_proficiencies = Column(String, nullable=False)
    armor_proficiencies = Column(ARRAY(String), nullable=False)
    shield_proficiencies = Column(ARRAY(String), nullable=False)
    weapon_proficiencies = Column(ARRAY(String), nullable=False)
    # relationships
    source = relationship("Source")
