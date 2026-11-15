"""Subclass routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.subclasses.models import Subclass
from py_dnd.features.subclasses.schemas import SubclassCreate, SubclassFields, SubclassSchema
from py_dnd.shared.grid import ColumnKind, GridColumn

SUBCLASSES_SPEC = CatalogSpec(
    label="subclass",
    model=Subclass,
    fields=SubclassFields,
    schema=SubclassSchema,
    create=SubclassCreate,
    grid_columns={
        "parent_class_slug": GridColumn(Subclass.parent_class_slug, ColumnKind.SET),
        "level_gained": GridColumn(Subclass.level_gained, ColumnKind.NUMBER),
        "spellcasting_ability": GridColumn(Subclass.spellcasting_ability, ColumnKind.SET),
        "caster_progression": GridColumn(Subclass.caster_progression, ColumnKind.SET),
    },
)

router = catalog_router(SUBCLASSES_SPEC)
