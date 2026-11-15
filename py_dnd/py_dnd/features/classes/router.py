"""Class routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.classes.models import DndClass
from py_dnd.features.classes.schemas import DndClassCreate, DndClassFields, DndClassSchema
from py_dnd.shared.grid import ColumnKind, GridColumn

CLASSES_SPEC = CatalogSpec(
    label="class",
    plural="classes",
    model=DndClass,
    fields=DndClassFields,
    schema=DndClassSchema,
    create=DndClassCreate,
    grid_columns={
        "hit_die_size": GridColumn(DndClass.hit_die_size, ColumnKind.NUMBER),
        "primary_ability": GridColumn(DndClass.primary_ability, ColumnKind.TEXT),
        "spellcasting_ability": GridColumn(DndClass.spellcasting_ability, ColumnKind.SET),
        "caster_progression": GridColumn(DndClass.caster_progression, ColumnKind.SET),
    },
)

router = catalog_router(CLASSES_SPEC)
