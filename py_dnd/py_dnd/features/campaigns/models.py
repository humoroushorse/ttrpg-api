"""SQLAlchemy Tables: campaigns and which characters are in them (CHARACTER_PLAN.md section 4)."""

import datetime
import uuid
from typing import Any

from sqlalchemy import CheckConstraint, DateTime, ForeignKey, Index, UniqueConstraint, text
from sqlalchemy.orm import Mapped, mapped_column

from py_dnd.database.base_class import DndSchemaBase
from py_dnd.shared.models import USER_ID_FK, MixinBookeeping, MixinSoftDelete, soft_delete_index

EMPTY_JSON_OBJECT = text("'{}'::jsonb")


class Campaign(MixinBookeeping, MixinSoftDelete, DndSchemaBase):
    """A campaign: its DM and the rules characters built for it follow."""

    __tablename__ = "campaign"
    __table_args__ = (
        Index("ix_dnd_campaign_owner_id", "owner_id"),
        soft_delete_index("campaign"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    # a permanent DM (also a campaign_member row). RESTRICT: deleting the user means deleting or
    # handing over their campaigns first (the purge job decides)
    owner_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(USER_ID_FK, ondelete="RESTRICT"), nullable=False)
    name: Mapped[str] = mapped_column(nullable=False)
    description: Mapped[str] = mapped_column(server_default="", nullable=False)
    # the rule set (CHARACTER_PLAN.md section 3); the API validates it and fills in defaults
    rules: Mapped[dict[str, Any]] = mapped_column(server_default=EMPTY_JSON_OBJECT, nullable=False)
    # {access: {invite, party_sheets}}; later the VTT's (reactions, health cues, grid)
    settings: Mapped[dict[str, Any]] = mapped_column(server_default=EMPTY_JSON_OBJECT, nullable=False)


class CampaignMember(MixinBookeeping, DndSchemaBase):
    """Who's in a campaign, as a DM or a player. The owner is always a DM."""

    __tablename__ = "campaign_member"
    __table_args__ = (
        CheckConstraint("role IN ('dm', 'player')", name="role"),
        Index("ix_dnd_campaign_member_user_id", "user_id"),
        {"schema": "dnd"},
    )

    campaign_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.campaign.id", ondelete="CASCADE"), primary_key=True)
    user_id: Mapped[uuid.UUID] = mapped_column(ForeignKey(USER_ID_FK, ondelete="CASCADE"), primary_key=True)
    role: Mapped[str] = mapped_column(server_default="player", nullable=False)


class CampaignInvite(MixinBookeeping, DndSchemaBase):
    """An invite to join a campaign as a player.

    A link (token_hash) works for anyone signed in until it expires, is revoked or runs out of uses; a
    user's invite (user_id) waits for them to accept or decline. created_by is who invited.
    """

    __tablename__ = "campaign_invite"
    __table_args__ = (
        CheckConstraint("(token_hash IS NULL) <> (user_id IS NULL)", name="link_or_user"),
        CheckConstraint("max_uses IS NULL OR max_uses > 0", name="max_uses"),
        UniqueConstraint("token_hash", name="uq_campaign_invite_token_hash"),
        Index("ix_dnd_campaign_invite_campaign_id", "campaign_id"),
        Index("ix_dnd_campaign_invite_user_id", "user_id"),
        {"schema": "dnd"},
    )

    id: Mapped[uuid.UUID] = mapped_column(primary_key=True, server_default=text("uuidv7()"))
    campaign_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.campaign.id", ondelete="CASCADE"), nullable=False)
    # sha256 of the link's token, hex; the token itself is shown once and never stored
    token_hash: Mapped[str | None] = mapped_column(nullable=True)
    user_id: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="CASCADE"), nullable=True)
    expires_at: Mapped[datetime.datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    # null: any number of uses until it expires
    max_uses: Mapped[int | None] = mapped_column(nullable=True)
    uses: Mapped[int] = mapped_column(server_default="0", nullable=False)
    # revoked by a DM, or declined by the invited user
    revoked_at: Mapped[datetime.datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    revoked_by: Mapped[uuid.UUID | None] = mapped_column(ForeignKey(USER_ID_FK, ondelete="SET NULL"), nullable=True)


class CampaignCharacter(MixinBookeeping, DndSchemaBase):
    """A character in a campaign. Joining changes nothing on the character."""

    __tablename__ = "campaign_character"
    __table_args__ = (
        Index("ix_dnd_campaign_character_character_id", "character_id"),
        {"schema": "dnd"},
    )

    campaign_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("dnd.campaign.id", ondelete="CASCADE"), primary_key=True)
    character_id: Mapped[uuid.UUID] = mapped_column(
        ForeignKey("dnd.character.id", ondelete="CASCADE"), primary_key=True
    )
    # warnings the DM accepted after a rules change
    approved_exceptions: Mapped[list[dict[str, Any]]] = mapped_column(
        server_default=text("'[]'::jsonb"), nullable=False
    )
