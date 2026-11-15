"""catalog entry view.

Every catalog entry a character can pick, in one shape, so character code can check any of them
against the content rules (edition, UA, source) with one query. Autogenerate ignores views; change it here.

Revision ID: 986a9d86d5a1
Revises: 31ef9eb3cb82
Create Date: 2026-11-08 21:26:37.539469

"""

from collections.abc import Sequence

from alembic import op

# revision identifiers, used by Alembic.
revision: str = "986a9d86d5a1"
down_revision: str | None = "31ef9eb3cb82"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


VIEW_SQL = """
CREATE VIEW dnd.catalog_entry AS
SELECT 'class'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.dnd_class
UNION ALL
SELECT 'subclass'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.subclass
UNION ALL
SELECT 'species'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.species
UNION ALL
SELECT 'background'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.background
UNION ALL
SELECT 'feat'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.feat
UNION ALL
SELECT 'spell'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.spell
UNION ALL
SELECT 'item'::text AS category, id, slug, name, dnd_version_year, is_unearthed_arcana, deleted_at, source_id
    FROM dnd.item;
"""


def upgrade() -> None:
    """Database migration: upgrade."""
    pre_upgrade()

    op.execute(VIEW_SQL)

    post_upgrade()


def downgrade() -> None:
    """Database migration: downgrade."""
    pre_downgrade()

    op.execute("DROP VIEW dnd.catalog_entry")

    post_downgrade()


def pre_upgrade() -> None:
    """Processing before upgrading the schema."""
    pass


def post_upgrade() -> None:
    """Processing after upgrading the schema."""
    pass


def pre_downgrade() -> None:
    """Processing before downgrading the schema."""
    pass


def post_downgrade() -> None:
    """Processing after downgrading the schema."""
    pass
