"""class spell preparation.
Revision ID: 27b78fda2df2
Revises: 90555ce2fa37
Create Date: 2026-11-11 18:40:00.000000
"""
from collections.abc import Sequence
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql
# revision identifiers, used by Alembic.
revision: str = '27b78fda2df2'
down_revision: str | None = '90555ce2fa37'
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None
def upgrade() -> None:
    """Database migration: upgrade."""
    pre_upgrade()
    op.add_column('dnd_class', sa.Column('spell_preparation', postgresql.JSONB(astext_type=sa.Text()), nullable=True), schema='dnd')
    op.add_column('subclass', sa.Column('spell_preparation', postgresql.JSONB(astext_type=sa.Text()), nullable=True), schema='dnd')
    post_upgrade()
def downgrade() -> None:
    """Database migration: downgrade."""
    pre_downgrade()
    op.drop_column('subclass', 'spell_preparation', schema='dnd')
    op.drop_column('dnd_class', 'spell_preparation', schema='dnd')
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
