"""Feat routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.feats.models import Feat
from py_dnd.features.feats.schemas import FeatCreate, FeatFields, FeatSchema
from py_dnd.shared.grid import ColumnKind, GridColumn

FEATS_SPEC = CatalogSpec(
    label="feat",
    model=Feat,
    fields=FeatFields,
    schema=FeatSchema,
    create=FeatCreate,
    grid_columns={
        "category": GridColumn(Feat.category, ColumnKind.SET),
    },
)

router = catalog_router(FEATS_SPEC)
