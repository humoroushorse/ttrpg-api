"""Item routes (see ``features/catalog/router.py`` for the list)."""

from py_dnd.features.catalog.router import CatalogSpec, catalog_router
from py_dnd.features.items.models import Item
from py_dnd.features.items.schemas import ItemCreate, ItemFields, ItemSchema
from py_dnd.shared.grid import ColumnKind, GridColumn

ITEMS_SPEC = CatalogSpec(
    label="item",
    model=Item,
    fields=ItemFields,
    schema=ItemSchema,
    create=ItemCreate,
    grid_columns={
        "category": GridColumn(Item.category, ColumnKind.SET),
        "subcategory": GridColumn(Item.subcategory, ColumnKind.SET),
        "is_magic": GridColumn(Item.is_magic, ColumnKind.BOOLEAN),
        "rarity": GridColumn(Item.rarity, ColumnKind.SET),
        "requires_attunement": GridColumn(Item.requires_attunement, ColumnKind.BOOLEAN),
        "magic_item_type": GridColumn(Item.magic_item_type, ColumnKind.SET),
    },
)

router = catalog_router(ITEMS_SPEC)
