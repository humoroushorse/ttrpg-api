"""Condition routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.conditions.models import Condition
from py_dnd.features.conditions.schemas import ConditionCreate, ConditionFields, ConditionSchema

CONDITION_SPEC = CatalogSpec(
    label="condition",
    model=Condition,
    fields=ConditionFields,
    schema=ConditionSchema,
    create=ConditionCreate,
)

router = catalog_router(CONDITION_SPEC)
