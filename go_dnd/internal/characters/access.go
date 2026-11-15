package characters

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
)

// requireCharacter is who the caller is to the character, or the error when they may not take the
// action (internal/access). Every route that reads or writes a character asks first.
func requireCharacter(ctx context.Context, q *db.Queries, id, userID uuid.UUID, a access.CharacterAction) (access.Character, error) {
	row, err := q.CharacterAccess(ctx, db.CharacterAccessParams{UserID: userID, CharacterID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return access.Character{}, notFound(id)
	}
	if err != nil {
		return access.Character{}, err
	}
	c := access.CharacterFrom(id, userID, row.OwnerID, row.Editor, row.CampaignID, row.CampaignOwnerID, row.CampaignRole,
		row.CampaignSettings)
	return c, c.Require(a)
}

// Viewer is what the caller may do with a character, so the UI shows only the controls that work.
type Viewer struct {
	Owner  bool `json:"owner"`
	Editor bool `json:"editor"`
	// dm, player, or "" when not in its campaign
	CampaignRole string `json:"campaign_role"`
	CanEdit      bool   `json:"can_edit"`
	// give items, coins, boons and banes (its campaign's DMs)
	CanGive bool `json:"can_give"`
	// delete it, choose its editors, hand it over, move it between campaigns (its owner)
	CanManage bool `json:"can_manage"`
}

func viewerOf(who access.Character) *Viewer {
	return &Viewer{Owner: who.Owner, Editor: who.Editor, CampaignRole: string(who.Campaign.Role),
		CanEdit: who.Can(access.EditBuild), CanGive: who.Can(access.GiveTo), CanManage: who.Can(access.ManageEditors)}
}

// callerKey carries who the caller is to the character into a write's closure (writeTx puts it there).
type callerKey struct{}

// caller is who the caller is to the character being written, as writeTx found it.
func caller(ctx context.Context) access.Character {
	who, _ := ctx.Value(callerKey{}).(access.Character)
	return who
}

// requireCampaign is who the caller is to the campaign, or the error when they may not take the action.
func requireCampaign(ctx context.Context, q *db.Queries, id, userID uuid.UUID, a access.CampaignAction) (access.Campaign, error) {
	row, err := q.CampaignAccess(ctx, db.CampaignAccessParams{UserID: userID, CampaignID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return access.Campaign{}, httpx.NotFound("Campaign %s not found", id)
	}
	if err != nil {
		return access.Campaign{}, err
	}
	c := access.CampaignFrom(id, userID, row.OwnerID, row.Role, row.Settings)
	return c, c.Require(a)
}
