package tabletop

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/access"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

var (
	campaignID = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	ownerID    = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	playerID   = uuid.MustParse("00000000-0000-0000-0000-00000000000c")
	otherID    = uuid.MustParse("00000000-0000-0000-0000-00000000000d")
	ezmerelda  = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	ireena     = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	tavern     = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	crypt      = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
)

func ptr[T any](v T) *T { return &v }

func viewerFor(user uuid.UUID, role access.Role, active, moved *uuid.UUID) *Viewer {
	var r *string
	if role != access.None {
		r = ptr(string(role))
	}
	who := access.CampaignFrom(campaignID, user, ownerID, r, nil)
	characters := []db.CampaignCharacterEditorsRow{{CharacterID: ezmerelda, OwnerID: playerID}, {CharacterID: ireena, OwnerID: otherID}}
	return NewViewer(user, who, active, moved, characters)
}

func event(seq int64, typ, audience string, scene *uuid.UUID, payload any) Event {
	raw, _ := json.Marshal(payload)
	return Event{CampaignID: campaignID, Seq: seq, SceneID: scene, Type: typ, Audience: audience, Payload: raw,
		ActorID: ptr(ownerID), CreatedAt: time.Unix(0, 0)}
}

// the vampire spawn the players know as "Hooded figure", looking friendly
var spawn = TokenRecord{ID: uuid.MustParse("00000000-0000-0000-0000-0000000000d1"), SceneID: tavern, Kind: "marker",
	Name: ptr("Vampire Spawn"), PlayerName: ptr("Hooded figure"), Number: ptr(int32(3)), Disposition: "hostile",
	PlayerDisposition: ptr("friendly"), ImageURL: ptr("https://example.test/spawn.webp"), SizeCells: 1, Layer: "tokens"}

func sent(t *testing.T, outs []Outgoing) string {
	t.Helper()
	raw, err := json.Marshal(outs)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestPlayerNeverReceivesHiddenIdentity(t *testing.T) {
	player := viewerFor(playerID, access.Player, ptr(tavern), nil)
	dm := viewerFor(ownerID, access.DM, ptr(tavern), nil)
	for _, e := range []Event{
		event(1, TokenAdded, AudienceAll, ptr(tavern), tokenPayload{Token: spawn}),
		event(2, TokenUpdated, AudienceAll, ptr(tavern), tokenPayload{Token: spawn}),
	} {
		outs, _ := player.Apply(e)
		got := sent(t, outs)
		for _, secret := range []string{"Vampire Spawn", "hostile", "spawn.webp", "player_name", "player_disposition"} {
			if strings.Contains(got, secret) {
				t.Errorf("%s: a player's copy contains %q: %s", e.Type, secret, got)
			}
		}
		if !strings.Contains(got, "Hooded figure") || !strings.Contains(got, `"disposition":"friendly"`) {
			t.Errorf("%s: a player should see the shown name and disposition: %s", e.Type, got)
		}
		dmOuts, _ := dm.Apply(e)
		if got := sent(t, dmOuts); !strings.Contains(got, "Vampire Spawn") || !strings.Contains(got, `"player_name":"Hooded figure"`) {
			t.Errorf("%s: the DM sees the true name and what players see: %s", e.Type, got)
		}
	}
}

func TestDMLayer(t *testing.T) {
	ambush := spawn
	ambush.Layer = "dm"
	player := viewerFor(playerID, access.Player, ptr(tavern), nil)
	dm := viewerFor(ownerID, access.DM, ptr(tavern), nil)

	added := event(1, TokenAdded, layerAudience("dm"), ptr(tavern), tokenPayload{Token: ambush})
	if outs, _ := player.Apply(added); len(outs) != 0 {
		t.Errorf("a DM-layer token reached a player: %s", sent(t, outs))
	}
	if outs, _ := dm.Apply(added); len(outs) != 1 {
		t.Errorf("the DM didn't get the DM-layer token")
	}

	// revealed: it appears for players
	revealed := event(2, TokenUpdated, layerAudience("dm", "tokens"), ptr(tavern), tokenPayload{Token: spawn, PreviousLayer: "dm"})
	if outs, _ := player.Apply(revealed); len(outs) != 1 || outs[0].Type != TokenAdded {
		t.Errorf("revealing a token should be token.added for players, got %s", sent(t, outs))
	}
	// hidden again: it disappears for players, with nothing but its id
	hidden := event(3, TokenUpdated, layerAudience("tokens", "dm"), ptr(tavern), tokenPayload{Token: ambush, PreviousLayer: "tokens"})
	outs, _ := player.Apply(hidden)
	if len(outs) != 1 || outs[0].Type != TokenRemoved || strings.Contains(sent(t, outs), "Hooded") {
		t.Errorf("hiding a token should be a bare token.removed for players, got %s", sent(t, outs))
	}
}

func TestScenesFollowWhereTheViewerIs(t *testing.T) {
	player := viewerFor(playerID, access.Player, ptr(tavern), nil)
	other := viewerFor(otherID, access.Player, ptr(tavern), nil)
	dm := viewerFor(ownerID, access.DM, ptr(tavern), nil)
	inCrypt := event(1, TokenMoved, AudienceAll, ptr(crypt), movePayload{TokenID: spawn.ID, X: 1, Y: 1})
	if outs, _ := player.Apply(inCrypt); len(outs) != 0 {
		t.Error("a player in the tavern got the crypt's event")
	}
	if outs, _ := dm.Apply(inCrypt); len(outs) != 1 {
		t.Error("the DM gets every scene's events")
	}

	// the DM moves the player to the crypt; only they (and DMs) hear about it
	moved := event(2, MemberMoved, AudienceDMs, nil, memberMovedPayload{UserID: playerID, SceneID: ptr(crypt)})
	if outs, _ := player.Apply(moved); len(outs) != 1 || *player.Current() != crypt {
		t.Errorf("the moved player should be told and be in the crypt, now %v", player.Current())
	}
	if outs, _ := other.Apply(moved); len(outs) != 0 {
		t.Error("another player learned where someone was moved")
	}
	if outs, _ := player.Apply(inCrypt); len(outs) != 1 {
		t.Error("the player in the crypt now gets its events")
	}

	// switching the active scene brings everyone back together
	activated := event(3, SceneActivated, AudienceAll, nil, activatedPayload{SceneID: ptr(tavern), PreviousSceneID: ptr(tavern), MovedBack: []uuid.UUID{playerID}})
	outs, _ := player.Apply(activated)
	if *player.Current() != tavern || strings.Contains(sent(t, outs), "moved_back") {
		t.Errorf("after scene.activated the player is in the tavern without the DM's details: %v %s", player.Current(), sent(t, outs))
	}
}

func TestStreamEndsWhenAccessChanges(t *testing.T) {
	player := viewerFor(playerID, access.Player, ptr(tavern), nil)
	if _, effect := player.Apply(event(1, MemberUpdated, AudienceAll, nil, memberPayload{UserID: playerID, Role: "dm"})); effect != ReloadAndClose {
		t.Error("a player made a DM should reload as a DM")
	}
	if _, effect := player.Apply(event(2, MemberUpdated, AudienceAll, nil, memberPayload{UserID: otherID, Role: "dm"})); effect != Continue {
		t.Error("someone else's role change doesn't end this stream")
	}
	if _, effect := player.Apply(event(3, MemberLeft, AudienceAll, nil, memberPayload{UserID: playerID})); effect != Close {
		t.Error("a player who left keeps their stream")
	}
	if _, effect := player.Apply(event(4, CampaignDeleted, AudienceAll, nil, struct{}{})); effect != Close {
		t.Error("a deleted campaign keeps its streams")
	}
}

func TestCanMove(t *testing.T) {
	hers := TokenRecord{ID: uuid.New(), SceneID: tavern, Kind: "character", CharacterID: ptr(ezmerelda), Layer: "tokens", Disposition: "party"}
	raven := TokenRecord{ID: uuid.New(), SceneID: tavern, Kind: "marker", Name: ptr("Raven"), ControllerCharacterID: ptr(ezmerelda), Layer: "tokens", Disposition: "party"}
	ireenas := TokenRecord{ID: uuid.New(), SceneID: tavern, Kind: "character", CharacterID: ptr(ireena), Layer: "tokens", Disposition: "party"}
	cases := []struct {
		who  *Viewer
		tok  TokenRecord
		want bool
	}{
		{viewerFor(playerID, access.Player, ptr(tavern), nil), hers, true},
		{viewerFor(playerID, access.Player, ptr(tavern), nil), raven, true},
		{viewerFor(playerID, access.Player, ptr(tavern), nil), ireenas, false},
		{viewerFor(ownerID, access.DM, ptr(tavern), nil), ireenas, true},
	}
	for _, tc := range cases {
		view, ok := tc.who.Token(tc.tok)
		if !ok || view.CanMove != tc.want {
			t.Errorf("%s moving %v: can_move %v, want %v", tc.who.UserID, tc.tok.Name, view.CanMove, tc.want)
		}
	}
}

func TestPlayersDontGetTheTree(t *testing.T) {
	parent := uuid.New()
	s := SceneView{ID: tavern, ParentID: &parent, SortOrder: 4, Name: "Blood on the Vine"}
	if got := viewerFor(playerID, access.Player, ptr(tavern), nil).Scene(s); got.ParentID != nil || got.SortOrder != 0 {
		t.Errorf("a player's copy of a scene has its place in the tree: %+v", got)
	}
	if got := viewerFor(ownerID, access.DM, ptr(tavern), nil).Scene(s); got.ParentID == nil {
		t.Error("the DM's copy lost its parent")
	}
}
