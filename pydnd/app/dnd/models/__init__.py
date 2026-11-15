"""Models module attributes (SQLAlchemy DB generation Objects)."""

from loguru import logger

from dnd.models.dnd_class import DndClass  # noqa: F401
from dnd.models.jt_spell_to_class import JtSpellToClass  # noqa: F401
from dnd.models.source import Source  # noqa: F401
from dnd.models.spell import Spell  # noqa: F401

logger = logger.bind(name=__name__)
