"""Repository module list of repositories for interacting with SQLAchemy Tables."""

from loguru import logger

from dnd.repository.dnd_class import dnd_class  # noqa: F401
from dnd.repository.jt_spell_to_class import jt_spell_to_class  # noqa: F401
from dnd.repository.source import source  # noqa: F401
from dnd.repository.spell import spell  # noqa: F401

logger = logger.bind(name=__name__)
