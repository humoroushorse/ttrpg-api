"""Glossary routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.glossary.models import GlossaryEntry
from py_dnd.features.glossary.schemas import GlossaryEntryCreate, GlossaryEntryFields, GlossaryEntrySchema
from py_dnd.shared.grid import ColumnKind, GridColumn

GLOSSARY_SPEC = CatalogSpec(
    label="glossary entry",
    model=GlossaryEntry,
    fields=GlossaryEntryFields,
    schema=GlossaryEntrySchema,
    create=GlossaryEntryCreate,
    grid_columns={"category": GridColumn(GlossaryEntry.category, ColumnKind.SET)},
)

router = catalog_router(GLOSSARY_SPEC)
