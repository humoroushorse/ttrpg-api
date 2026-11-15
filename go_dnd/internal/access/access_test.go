package access

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

var (
	ownerID   = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	dmID      = uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	playerID  = uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	editorID  = uuid.MustParse("00000000-0000-0000-0000-00000000000d")
	outsideID = uuid.MustParse("00000000-0000-0000-0000-00000000000e")
	campaign  = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	character = uuid.MustParse("00000000-0000-0000-0000-0000000000c2")
)

func role(r Role) *string {
	if r == None {
		return nil
	}
	s := string(r)
	return &s
}

func settings(invite, party string) []byte {
	return []byte(`{"access": {"invite": "` + invite + `", "party_sheets": "` + party + `"}}`)
}

// The campaign's people: its owner, another DM, a player, and someone not in it.
func campaignAs(user uuid.UUID, r Role, s []byte) Campaign {
	return CampaignFrom(campaign, user, ownerID, role(r), s)
}

func TestCampaignPermissions(t *testing.T) {
	defaults := []byte(`{}`)
	people := map[string]Campaign{
		"owner":    campaignAs(ownerID, DM, defaults),
		"dm":       campaignAs(dmID, DM, defaults),
		"player":   campaignAs(playerID, Player, defaults),
		"outsider": campaignAs(outsideID, None, defaults),
	}
	// the permission table: action -> who may
	table := map[CampaignAction][]string{
		ViewCampaign:      {"owner", "dm", "player"},
		EditCampaign:      {"owner", "dm"},
		DeleteCampaign:    {"owner"},
		ManageDMs:         {"owner", "dm"},
		Invite:            {"owner", "dm"},
		ManageInvites:     {"owner", "dm"},
		RemoveMember:      {"owner", "dm"},
		Leave:             {"dm", "player"},
		AddOwnCharacter:   {"owner", "dm", "player"},
		ApproveExceptions: {"owner", "dm"},
		Give:              {"owner", "dm"},
		ViewTable:         {"owner", "dm", "player"},
		ManageScenes:      {"owner", "dm"},
		PlaceTokens:       {"owner", "dm"},
	}
	for action, allowed := range table {
		for name, c := range people {
			want := contains(allowed, name)
			if got := c.Can(action); got != want {
				t.Errorf("%s: can %q = %v, want %v", name, action, got, want)
			}
			err := c.Require(action)
			switch {
			case want && err != nil:
				t.Errorf("%s: Require(%q) = %v, want nil", name, action, err)
			case !want && name == "outsider" && status(err) != http.StatusNotFound:
				t.Errorf("%s: Require(%q) = %v, want a 404", name, action, err)
			case !want && name != "outsider" && status(err) != http.StatusForbidden:
				t.Errorf("%s: Require(%q) = %v, want a 403", name, action, err)
			}
		}
	}
}

func TestInviteSetting(t *testing.T) {
	player := campaignAs(playerID, Player, settings(InviteMembers, PartySheetsSummary))
	if !player.Can(Invite) {
		t.Error("with invite: members, a player can invite")
	}
	if player.Can(ManageInvites) || player.Can(RemoveMember) {
		t.Error("inviting doesn't let a player manage invites or members")
	}
	if campaignAs(outsideID, None, settings(InviteMembers, PartySheetsSummary)).Can(Invite) {
		t.Error("an outsider never invites")
	}
}

func TestOwnerIsAlwaysDM(t *testing.T) {
	// even without a member row
	c := CampaignFrom(campaign, ownerID, ownerID, nil, nil)
	if c.Role != DM || !c.Can(DeleteCampaign) {
		t.Errorf("owner without a member row: role %q, can delete %v", c.Role, c.Can(DeleteCampaign))
	}
}

func TestParseSettings(t *testing.T) {
	cases := map[string]Settings{
		``:                                 DefaultSettings(),
		`{}`:                               DefaultSettings(),
		`{"access": {}}`:                   DefaultSettings(),
		`{"access": {"invite": "anyone"}}`: DefaultSettings(),
		`{"access": {"invite": "members", "party_sheets": "full"}}`: {Invite: InviteMembers, PartySheets: PartySheetsFull},
	}
	for raw, want := range cases {
		if got := ParseSettings([]byte(raw)); got != want {
			t.Errorf("ParseSettings(%s) = %+v, want %+v", raw, got, want)
		}
	}
}

// A character in the campaign, owned by the player, with one editor. viewer is who's asking.
func characterAs(viewer uuid.UUID, r Role, editor bool, party string) Character {
	cid, gOwner := campaign, ownerID
	return CharacterFrom(character, viewer, playerID, editor, &cid, &gOwner, role(r), settings(InviteDMs, party))
}

func TestCharacterPermissions(t *testing.T) {
	for _, party := range []string{PartySheetsSummary, PartySheetsFull} {
		people := map[string]Character{
			"character owner": characterAs(playerID, Player, false, party),
			"editor":          characterAs(editorID, None, true, party),
			"campaign owner":  characterAs(ownerID, DM, false, party),
			"dm":              characterAs(dmID, DM, false, party),
			"other player":    characterAs(outsideID, Player, false, party),
			"outsider":        characterAs(outsideID, None, false, party),
		}
		view := []string{"character owner", "editor", "campaign owner", "dm"}
		if party == PartySheetsFull {
			view = append(view, "other player")
		}
		table := map[CharacterAction][]string{
			ViewSheet:          view,
			EditBuild:          {"character owner", "editor"},
			GiveTo:             {"campaign owner", "dm"},
			ChangeInventory:    {"character owner", "editor", "campaign owner", "dm"},
			DeleteCharacter:    {"character owner"},
			ManageEditors:      {"character owner"},
			TransferCharacter:  {"character owner"},
			JoinCampaign:       {"character owner"},
			RemoveFromCampaign: {"character owner", "campaign owner", "dm"},
		}
		for action, allowed := range table {
			for name, c := range people {
				want := contains(allowed, name)
				if got := c.Can(action); got != want {
					t.Errorf("party %s, %s: can %q = %v, want %v", party, name, action, got, want)
				}
				if !want {
					wantStatus := http.StatusForbidden
					if name == "outsider" {
						wantStatus = http.StatusNotFound
					}
					if got := status(c.Require(action)); got != wantStatus {
						t.Errorf("party %s, %s: Require(%q) status %d, want %d", party, name, action, got, wantStatus)
					}
				}
			}
		}
	}
}

func TestCharacterOutsideACampaign(t *testing.T) {
	// a DM of some other campaign is no one to this character
	c := CharacterFrom(character, dmID, playerID, false, nil, nil, nil, nil)
	if c.Visible() || c.Can(GiveTo) || c.Can(RemoveFromCampaign) {
		t.Errorf("no campaign: visible %v, give %v, remove %v", c.Visible(), c.Can(GiveTo), c.Can(RemoveFromCampaign))
	}
	owner := CharacterFrom(character, playerID, playerID, false, nil, nil, nil, nil)
	if owner.Can(RemoveFromCampaign) || !owner.Can(EditBuild) {
		t.Errorf("owner, no campaign: remove %v, edit %v", owner.Can(RemoveFromCampaign), owner.Can(EditBuild))
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func status(err error) int {
	if e, ok := err.(*httpx.Error); ok {
		return e.Status
	}
	return 0
}

func TestDMOf(t *testing.T) {
	other := uuid.MustParse("00000000-0000-0000-0000-0000000000c9")
	dm := characterAs(dmID, DM, false, PartySheetsSummary)
	if !dm.DMOf(&campaign) || dm.DMOf(&other) || dm.DMOf(nil) {
		t.Errorf("DMOf: this campaign %v, another %v, none %v", dm.DMOf(&campaign), dm.DMOf(&other), dm.DMOf(nil))
	}
	if characterAs(playerID, Player, false, PartySheetsSummary).DMOf(&campaign) {
		t.Error("a player isn't a DM of the campaign")
	}
}
