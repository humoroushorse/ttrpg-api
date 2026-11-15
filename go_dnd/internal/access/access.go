// Package access decides who may do what with campaigns and characters. Deny by default: every
// route asks for the one action it needs. The facts come from the access queries
// (internal/repository/access/queries), generated into the campaigns and characters packages so each
// handler asks inside its own transaction; the decisions here are pure, so the permission table is
// tested without a database (access_test.go).
//
// Someone who can't see a thing gets a 404, as if it didn't exist; someone who can see it but may not
// do the action gets a 403.
package access

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Role is the caller's role in a campaign; None when they aren't a member.
type Role string

const (
	None   Role = ""
	Player Role = "player"
	DM     Role = "dm"
)

// Who may invite, and how much of the party's characters players see.
const (
	InviteDMs          = "dms"
	InviteMembers      = "members"
	PartySheetsSummary = "summary"
	PartySheetsFull    = "full"
)

// Settings are a campaign's access settings: campaign.settings.access.
type Settings struct {
	// dms or members
	Invite string `json:"invite"`
	// summary (name, class, species, level) or full (each party member's sheet, read-only)
	PartySheets string `json:"party_sheets"`
}

// DefaultSettings: only DMs invite; players see the party list, not the sheets.
func DefaultSettings() Settings {
	return Settings{Invite: InviteDMs, PartySheets: PartySheetsSummary}
}

// ParseSettings reads settings.access from a campaign's settings document; a missing or unknown
// value is its default.
func ParseSettings(raw []byte) Settings {
	out := DefaultSettings()
	var doc struct {
		Access *Settings `json:"access"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Access == nil {
		return out
	}
	if doc.Access.Invite == InviteMembers {
		out.Invite = InviteMembers
	}
	if doc.Access.PartySheets == PartySheetsFull {
		out.PartySheets = PartySheetsFull
	}
	return out
}

// Valid reports whether every value is a known one (for a write).
func (s Settings) Valid() bool {
	return (s.Invite == InviteDMs || s.Invite == InviteMembers) &&
		(s.PartySheets == PartySheetsSummary || s.PartySheets == PartySheetsFull)
}

// Campaign is who the caller is to one campaign.
type Campaign struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	Role    Role
	// the caller is the owner
	Owner    bool
	Settings Settings
}

// CampaignFrom builds the facts from the access query's row. The owner is a DM even if their member
// row were missing.
func CampaignFrom(id, userID, ownerID uuid.UUID, role *string, settings []byte) Campaign {
	c := Campaign{ID: id, OwnerID: ownerID, Owner: userID == ownerID, Settings: ParseSettings(settings)}
	if role != nil {
		c.Role = Role(*role)
	}
	if c.Owner {
		c.Role = DM
	}
	return c
}

// CampaignAction is something done to a campaign.
type CampaignAction string

const (
	ViewCampaign      CampaignAction = "view the campaign"
	EditCampaign      CampaignAction = "change the campaign, its rules or settings"
	DeleteCampaign    CampaignAction = "delete the campaign"
	ManageDMs         CampaignAction = "promote or demote DMs"
	Invite            CampaignAction = "invite players"
	ManageInvites     CampaignAction = "see and revoke invites"
	RemoveMember      CampaignAction = "remove a member"
	Leave             CampaignAction = "leave the campaign"
	AddOwnCharacter   CampaignAction = "add a character"
	ApproveExceptions CampaignAction = "approve rule exceptions"
	Give              CampaignAction = "give items, coins, boons and banes"
	// the table (internal/tabletop); which scene and which tokens are decided below
	ViewTable    CampaignAction = "open the campaign's table"
	ManageScenes CampaignAction = "create, change or delete scenes, or move players between them"
	PlaceTokens  CampaignAction = "add, change or remove tokens"
)

// Can reports whether the caller may take the action.
func (c Campaign) Can(a CampaignAction) bool {
	member := c.Role != None
	switch a {
	case ViewCampaign, AddOwnCharacter, ViewTable:
		return member
	case Leave:
		return member && !c.Owner // the owner is a permanent DM
	case EditCampaign, ManageDMs, ManageInvites, RemoveMember, ApproveExceptions, Give, ManageScenes, PlaceTokens:
		return c.Role == DM
	case DeleteCampaign:
		return c.Owner
	case Invite:
		return c.Role == DM || (member && c.Settings.Invite == InviteMembers)
	}
	return false
}

// Require is nil when the caller may take the action, else the error to answer with.
func (c Campaign) Require(a CampaignAction) error {
	if c.Can(a) {
		return nil
	}
	switch {
	case c.Role == None:
		return httpx.NotFound("Campaign %s not found", c.ID)
	case a == DeleteCampaign:
		return httpx.Forbidden("only the campaign's owner can %s", a)
	case a == Leave:
		return httpx.Forbidden("the owner can't leave: they're the campaign's permanent DM")
	}
	return httpx.Forbidden("only the campaign's DMs can %s", a)
}

// Character is who the caller is to one character.
type Character struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	Owner   bool
	// on the owner's allow list
	Editor bool
	// the campaign it's in (nil when none), and the caller's place in it
	CampaignID *uuid.UUID
	Campaign   Campaign
}

// CharacterFrom builds the facts from the access query's row.
func CharacterFrom(id, userID, ownerID uuid.UUID, editor bool, campaignID, campaignOwnerID *uuid.UUID,
	role *string, settings []byte) Character {
	c := Character{ID: id, OwnerID: ownerID, Owner: userID == ownerID, Editor: editor, CampaignID: campaignID}
	if campaignID != nil && campaignOwnerID != nil {
		c.Campaign = CampaignFrom(*campaignID, userID, *campaignOwnerID, role, settings)
	}
	return c
}

// CharacterAction is something done to a character.
type CharacterAction string

const (
	ViewSheet CharacterAction = "see the character's sheet"
	EditBuild CharacterAction = "change the character"
	GiveTo    CharacterAction = "give the character items, coins, boons or banes"
	// the route then decides per row: the player's own items and boons, or a DM's gifts
	ChangeInventory    CharacterAction = "change the character's items, boons or banes"
	DeleteCharacter    CharacterAction = "delete the character"
	ManageEditors      CharacterAction = "choose who can edit the character"
	TransferCharacter  CharacterAction = "hand the character over"
	JoinCampaign       CharacterAction = "add the character to a campaign"
	RemoveFromCampaign CharacterAction = "take the character out of its campaign"
)

// Can reports whether the caller may take the action.
func (c Character) Can(a CharacterAction) bool {
	inCampaign := c.CampaignID != nil
	dm := inCampaign && c.Campaign.Role == DM
	switch a {
	case ViewSheet:
		partyFull := inCampaign && c.Campaign.Role == Player && c.Campaign.Settings.PartySheets == PartySheetsFull
		return c.Owner || c.Editor || dm || partyFull
	case EditBuild:
		return c.Owner || c.Editor
	case GiveTo:
		return dm
	case ChangeInventory:
		return c.Owner || c.Editor || dm
	case DeleteCharacter, ManageEditors, TransferCharacter, JoinCampaign:
		return c.Owner
	case RemoveFromCampaign:
		return inCampaign && (c.Owner || dm)
	}
	return false
}

// DMOf reports whether the caller is a DM of this campaign and the character is in it now: the DMs
// who may change that campaign's gifts.
func (c Character) DMOf(campaignID *uuid.UUID) bool {
	return campaignID != nil && c.CampaignID != nil && *campaignID == *c.CampaignID && c.Campaign.Role == DM
}

// Visible reports whether the caller may know the character exists: they can see its sheet, or it's
// on the party list of a campaign they're in.
func (c Character) Visible() bool {
	return c.Can(ViewSheet) || (c.CampaignID != nil && c.Campaign.Role != None)
}

// Require is nil when the caller may take the action, else the error to answer with.
func (c Character) Require(a CharacterAction) error {
	if c.Can(a) {
		return nil
	}
	if !c.Visible() {
		return httpx.NotFound("Character %s not found", c.ID)
	}
	switch a {
	case EditBuild:
		return httpx.Forbidden("only the character's owner and the people they choose can %s", a)
	case GiveTo:
		return httpx.Forbidden("only its campaign's DMs can %s", a)
	case ChangeInventory:
		return httpx.Forbidden("only the character's owner, its editors and its campaign's DMs can %s", a)
	case RemoveFromCampaign:
		return httpx.Forbidden("only the character's owner or its campaign's DMs can %s", a)
	}
	return httpx.Forbidden("only the character's owner can %s", a)
}
