"""Species routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.species.models import Species
from py_dnd.features.species.schemas import SpeciesCreate, SpeciesFields, SpeciesSchema
from py_dnd.shared.grid import ColumnKind, GridColumn

SPECIES_SPEC = CatalogSpec(
    label="species",
    plural="species",
    model=Species,
    fields=SpeciesFields,
    schema=SpeciesSchema,
    create=SpeciesCreate,
    grid_columns={
        "size": GridColumn(Species.size, ColumnKind.TEXT),
        "speed": GridColumn(Species.speed, ColumnKind.TEXT),
        "creature_type": GridColumn(Species.creature_type, ColumnKind.SET),
    },
)

router = catalog_router(SPECIES_SPEC)
