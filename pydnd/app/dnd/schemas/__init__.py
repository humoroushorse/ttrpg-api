"""Schema module attributes. (Pydantic Objects)."""

# Submodules are re-exported so callers can use `from dnd import schemas` + `schemas.spell...`
from loguru import logger

from dnd.schemas import (
    dnd_class,  # noqa: F401
    enums,  # noqa: F401
    health_check,  # noqa: F401
    jt_spell_to_class,  # noqa: F401
    responses,  # noqa: F401
    source,  # noqa: F401
    spell,  # noqa: F401
)

logger = logger.bind(name=__name__)
