"""class starting equipment options.
Revision ID: 5c1e8a7d3b20
Revises: 27b78fda2df2
Create Date: 2026-11-12 12:00:00.000000
"""
from collections.abc import Sequence
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql
# revision identifiers, used by Alembic.
revision: str = '5c1e8a7d3b20'
down_revision: str | None = '27b78fda2df2'
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None
def upgrade() -> None:
    """Database migration: upgrade."""
    pre_upgrade()
    op.add_column('dnd_class', sa.Column('starting_equipment_options', postgresql.JSONB(astext_type=sa.Text()), server_default=sa.text("'[]'::jsonb"), nullable=False), schema='dnd')
    post_upgrade()
def downgrade() -> None:
    """Database migration: downgrade."""
    pre_downgrade()
    op.drop_column('dnd_class', 'starting_equipment_options', schema='dnd')
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
