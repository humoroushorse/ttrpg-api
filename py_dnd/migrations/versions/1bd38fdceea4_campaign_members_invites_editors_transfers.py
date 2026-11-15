"""campaign members, invites, character editors and transfers, DM gifts.

Every live campaign's owner becomes its first member, as a DM.

Revision ID: 1bd38fdceea4
Revises: 5c1e8a7d3b20
Create Date: 2026-11-12 16:53:17.602689

"""
from collections.abc import Sequence
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = '1bd38fdceea4'
down_revision: str | None = '5c1e8a7d3b20'
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    """Database migration: upgrade."""
    pre_upgrade()

    op.create_table('campaign_invite',
    sa.Column('id', sa.Uuid(), server_default=sa.text('uuidv7()'), nullable=False),
    sa.Column('campaign_id', sa.Uuid(), nullable=False),
    sa.Column('token_hash', sa.String(), nullable=True),
    sa.Column('user_id', sa.Uuid(), nullable=True),
    sa.Column('expires_at', sa.DateTime(timezone=True), nullable=False),
    sa.Column('max_uses', sa.Integer(), nullable=True),
    sa.Column('uses', sa.Integer(), server_default='0', nullable=False),
    sa.Column('revoked_at', sa.DateTime(timezone=True), nullable=True),
    sa.Column('revoked_by', sa.Uuid(), nullable=True),
    sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('created_by', sa.Uuid(), nullable=True),
    sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('updated_by', sa.Uuid(), nullable=True),
    sa.CheckConstraint('(token_hash IS NULL) <> (user_id IS NULL)', name=op.f('ck_campaign_invite_link_or_user')),
    sa.CheckConstraint('max_uses IS NULL OR max_uses > 0', name=op.f('ck_campaign_invite_max_uses')),
    sa.ForeignKeyConstraint(['campaign_id'], ['dnd.campaign.id'], name=op.f('fk_campaign_invite_campaign_id_campaign'), ondelete='CASCADE'),
    sa.ForeignKeyConstraint(['created_by'], ['dnd.user.id'], name=op.f('fk_campaign_invite_created_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['revoked_by'], ['dnd.user.id'], name=op.f('fk_campaign_invite_revoked_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['updated_by'], ['dnd.user.id'], name=op.f('fk_campaign_invite_updated_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['user_id'], ['dnd.user.id'], name=op.f('fk_campaign_invite_user_id_user'), ondelete='CASCADE'),
    sa.PrimaryKeyConstraint('id', name=op.f('pk_campaign_invite')),
    sa.UniqueConstraint('token_hash', name='uq_campaign_invite_token_hash'),
    schema='dnd'
    )
    op.create_index('ix_dnd_campaign_invite_campaign_id', 'campaign_invite', ['campaign_id'], unique=False, schema='dnd')
    op.create_index(op.f('ix_dnd_campaign_invite_updated_at'), 'campaign_invite', ['updated_at'], unique=False, schema='dnd')
    op.create_index('ix_dnd_campaign_invite_user_id', 'campaign_invite', ['user_id'], unique=False, schema='dnd')
    op.create_table('campaign_member',
    sa.Column('campaign_id', sa.Uuid(), nullable=False),
    sa.Column('user_id', sa.Uuid(), nullable=False),
    sa.Column('role', sa.String(), server_default='player', nullable=False),
    sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('created_by', sa.Uuid(), nullable=True),
    sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('updated_by', sa.Uuid(), nullable=True),
    sa.CheckConstraint("role IN ('dm', 'player')", name=op.f('ck_campaign_member_role')),
    sa.ForeignKeyConstraint(['campaign_id'], ['dnd.campaign.id'], name=op.f('fk_campaign_member_campaign_id_campaign'), ondelete='CASCADE'),
    sa.ForeignKeyConstraint(['created_by'], ['dnd.user.id'], name=op.f('fk_campaign_member_created_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['updated_by'], ['dnd.user.id'], name=op.f('fk_campaign_member_updated_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['user_id'], ['dnd.user.id'], name=op.f('fk_campaign_member_user_id_user'), ondelete='CASCADE'),
    sa.PrimaryKeyConstraint('campaign_id', 'user_id', name=op.f('pk_campaign_member')),
    schema='dnd'
    )
    op.create_index(op.f('ix_dnd_campaign_member_updated_at'), 'campaign_member', ['updated_at'], unique=False, schema='dnd')
    op.create_index('ix_dnd_campaign_member_user_id', 'campaign_member', ['user_id'], unique=False, schema='dnd')
    op.create_table('character_editor',
    sa.Column('character_id', sa.Uuid(), nullable=False),
    sa.Column('user_id', sa.Uuid(), nullable=False),
    sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('created_by', sa.Uuid(), nullable=True),
    sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('updated_by', sa.Uuid(), nullable=True),
    sa.ForeignKeyConstraint(['character_id'], ['dnd.character.id'], name=op.f('fk_character_editor_character_id_character'), ondelete='CASCADE'),
    sa.ForeignKeyConstraint(['created_by'], ['dnd.user.id'], name=op.f('fk_character_editor_created_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['updated_by'], ['dnd.user.id'], name=op.f('fk_character_editor_updated_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['user_id'], ['dnd.user.id'], name=op.f('fk_character_editor_user_id_user'), ondelete='CASCADE'),
    sa.PrimaryKeyConstraint('character_id', 'user_id', name=op.f('pk_character_editor')),
    schema='dnd'
    )
    op.create_index(op.f('ix_dnd_character_editor_updated_at'), 'character_editor', ['updated_at'], unique=False, schema='dnd')
    op.create_index('ix_dnd_character_editor_user_id', 'character_editor', ['user_id'], unique=False, schema='dnd')
    op.create_table('character_transfer',
    sa.Column('character_id', sa.Uuid(), nullable=False),
    sa.Column('to_user_id', sa.Uuid(), nullable=False),
    sa.Column('expires_at', sa.DateTime(timezone=True), nullable=False),
    sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('created_by', sa.Uuid(), nullable=True),
    sa.Column('updated_at', sa.DateTime(timezone=True), server_default=sa.text('now()'), nullable=False),
    sa.Column('updated_by', sa.Uuid(), nullable=True),
    sa.ForeignKeyConstraint(['character_id'], ['dnd.character.id'], name=op.f('fk_character_transfer_character_id_character'), ondelete='CASCADE'),
    sa.ForeignKeyConstraint(['created_by'], ['dnd.user.id'], name=op.f('fk_character_transfer_created_by_user'), ondelete='SET NULL'),
    sa.ForeignKeyConstraint(['to_user_id'], ['dnd.user.id'], name=op.f('fk_character_transfer_to_user_id_user'), ondelete='CASCADE'),
    sa.ForeignKeyConstraint(['updated_by'], ['dnd.user.id'], name=op.f('fk_character_transfer_updated_by_user'), ondelete='SET NULL'),
    sa.PrimaryKeyConstraint('character_id', name=op.f('pk_character_transfer')),
    schema='dnd'
    )
    op.create_index('ix_dnd_character_transfer_to_user_id', 'character_transfer', ['to_user_id'], unique=False, schema='dnd')
    op.create_index(op.f('ix_dnd_character_transfer_updated_at'), 'character_transfer', ['updated_at'], unique=False, schema='dnd')
    op.add_column('character_item', sa.Column('given_by', sa.Uuid(), nullable=True), schema='dnd')
    op.add_column('character_item', sa.Column('given_campaign_id', sa.Uuid(), nullable=True), schema='dnd')
    op.create_foreign_key(op.f('fk_character_item_given_by_user'), 'character_item', 'user', ['given_by'], ['id'], source_schema='dnd', referent_schema='dnd', ondelete='SET NULL')
    op.create_foreign_key(op.f('fk_character_item_given_campaign_id_campaign'), 'character_item', 'campaign', ['given_campaign_id'], ['id'], source_schema='dnd', referent_schema='dnd', ondelete='SET NULL')
    op.add_column('character_modifier', sa.Column('campaign_id', sa.Uuid(), nullable=True), schema='dnd')
    op.create_foreign_key(op.f('fk_character_modifier_campaign_id_campaign'), 'character_modifier', 'campaign', ['campaign_id'], ['id'], source_schema='dnd', referent_schema='dnd', ondelete='CASCADE')

    post_upgrade()


def downgrade() -> None:
    """Database migration: downgrade."""
    pre_downgrade()

    op.drop_constraint(op.f('fk_character_modifier_campaign_id_campaign'), 'character_modifier', schema='dnd', type_='foreignkey')
    op.drop_column('character_modifier', 'campaign_id', schema='dnd')
    op.drop_constraint(op.f('fk_character_item_given_campaign_id_campaign'), 'character_item', schema='dnd', type_='foreignkey')
    op.drop_constraint(op.f('fk_character_item_given_by_user'), 'character_item', schema='dnd', type_='foreignkey')
    op.drop_column('character_item', 'given_campaign_id', schema='dnd')
    op.drop_column('character_item', 'given_by', schema='dnd')
    op.drop_index(op.f('ix_dnd_character_transfer_updated_at'), table_name='character_transfer', schema='dnd')
    op.drop_index('ix_dnd_character_transfer_to_user_id', table_name='character_transfer', schema='dnd')
    op.drop_table('character_transfer', schema='dnd')
    op.drop_index('ix_dnd_character_editor_user_id', table_name='character_editor', schema='dnd')
    op.drop_index(op.f('ix_dnd_character_editor_updated_at'), table_name='character_editor', schema='dnd')
    op.drop_table('character_editor', schema='dnd')
    op.drop_index('ix_dnd_campaign_member_user_id', table_name='campaign_member', schema='dnd')
    op.drop_index(op.f('ix_dnd_campaign_member_updated_at'), table_name='campaign_member', schema='dnd')
    op.drop_table('campaign_member', schema='dnd')
    op.drop_index('ix_dnd_campaign_invite_user_id', table_name='campaign_invite', schema='dnd')
    op.drop_index(op.f('ix_dnd_campaign_invite_updated_at'), table_name='campaign_invite', schema='dnd')
    op.drop_index('ix_dnd_campaign_invite_campaign_id', table_name='campaign_invite', schema='dnd')
    op.drop_table('campaign_invite', schema='dnd')

    post_downgrade()


def pre_upgrade() -> None:
    """Processing before upgrading the schema."""
    pass


def post_upgrade() -> None:
    """Processing after upgrading the schema: each campaign's owner is a DM."""
    op.execute(
        "INSERT INTO dnd.campaign_member (campaign_id, user_id, role, created_by, updated_by) "
        "SELECT id, owner_id, 'dm', owner_id, owner_id FROM dnd.campaign WHERE deleted_at IS NULL"
    )


def pre_downgrade() -> None:
    """Processing before downgrading the schema."""
    pass


def post_downgrade() -> None:
    """Processing after downgrading the schema."""
    pass
