"""Background routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.backgrounds.models import Background
from py_dnd.features.backgrounds.schemas import BackgroundCreate, BackgroundFields, BackgroundSchema
from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.shared.grid import ColumnKind, GridColumn

BACKGROUNDS_SPEC = CatalogSpec(
    label="background",
    model=Background,
    fields=BackgroundFields,
    schema=BackgroundSchema,
    create=BackgroundCreate,
    grid_columns={
        "feat": GridColumn(Background.feat, ColumnKind.TEXT),
        "ability_score_bonuses": GridColumn(Background.ability_score_bonuses, ColumnKind.TEXT),
    },
)

router = catalog_router(BACKGROUNDS_SPEC)
