package access

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

var (
	friendID  = uuid.MustParse("00000000-0000-0000-0000-00000000000f")
	otherID   = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	ezmerelda = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	ireena    = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	tavern    = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	crypt     = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
)

// The table's people: the campaign's owner and another DM; player owns Ezmerelda, friend may edit her,
// other owns Ireena; and someone not in the campaign.
var tablePeople = map[string]struct {
	id   uuid.UUID
	role Role
}{
	"owner":    {ownerID, DM},
	"dm":       {dmID, DM},
	"player":   {playerID, Player},
	"friend":   {friendID, Player},
	"other":    {otherID, Player},
	"outsider": {outsideID, None},
}

// the campaign's characters, as the editors query reports them to viewer
func characterFacts(viewer uuid.UUID) []CharacterFacts {
	return []CharacterFacts{
		{ID: ezmerelda, OwnerID: playerID, Editor: viewer == friendID},
		{ID: ireena, OwnerID: otherID},
	}
}

func ptr(id uuid.UUID) *uuid.UUID { return &id }

func TestEditableCharacters(t *testing.T) {
	want := map[string][]uuid.UUID{
		"owner": nil, "dm": nil, "outsider": nil, // DMs move tokens by role, not by editing the sheet
		"player": {ezmerelda},
		"friend": {ezmerelda},
		"other":  {ireena},
	}
	for name, p := range tablePeople {
		got := EditableCharacters(p.id, characterFacts(p.id))
		if len(got) != len(want[name]) {
			t.Errorf("%s: editable %v, want %v", name, got, want[name])
			continue
		}
		for _, id := range want[name] {
			if !got[id] {
				t.Errorf("%s: can't edit %s, want editable %v", name, id, want[name])
			}
		}
	}
}

func TestMoveToken(t *testing.T) {
	tokens := map[string]Token{
		"ezmerelda":          {Layer: LayerTokens, CharacterID: ptr(ezmerelda)},
		"ezmerelda, dm only": {Layer: LayerDM, CharacterID: ptr(ezmerelda)},
		"her raven":          {Layer: LayerTokens, ControllerCharacterID: ptr(ezmerelda)},
		"ireena":             {Layer: LayerTokens, CharacterID: ptr(ireena)},
		"ireena's wolf":      {Layer: LayerTokens, ControllerCharacterID: ptr(ireena)},
		"goblin":             {Layer: LayerTokens},
		"ambush":             {Layer: LayerDM},
	}
	all := []string{"ezmerelda", "ezmerelda, dm only", "her raven", "ireena", "ireena's wolf", "goblin", "ambush"}
	// who may move what
	table := map[string][]string{
		"owner":    all,
		"dm":       all,
		"player":   {"ezmerelda", "her raven"},
		"friend":   {"ezmerelda", "her raven"},
		"other":    {"ireena", "ireena's wolf"},
		"outsider": nil,
	}
	hidden := map[string]bool{"ezmerelda, dm only": true, "ambush": true}
	for name, p := range tablePeople {
		c := CampaignFrom(campaign, p.id, ownerID, role(p.role), nil)
		editable := EditableCharacters(p.id, characterFacts(p.id))
		for label, token := range tokens {
			want := contains(table[name], label)
			if got := c.CanMoveToken(token, editable); got != want {
				t.Errorf("%s moves %s = %v, want %v", name, label, got, want)
			}
			if want := p.role == DM || (p.role == Player && !hidden[label]); c.TokenVisible(token) != want {
				t.Errorf("%s sees %s = %v, want %v", name, label, c.TokenVisible(token), want)
			}
			err := c.RequireMoveToken(token, editable)
			wantStatus := 0
			switch {
			case want:
			case p.role == None, hidden[label]:
				wantStatus = http.StatusNotFound // nothing to see
			default:
				wantStatus = http.StatusForbidden
			}
			if got := status(err); got != wantStatus || (wantStatus == 0) != (err == nil) {
				t.Errorf("%s: RequireMoveToken(%s) = %v (status %d), want status %d", name, label, err, got, wantStatus)
			}
		}
	}
}

func TestSceneVisible(t *testing.T) {
	cases := []struct {
		who     string
		current *uuid.UUID
		sees    []uuid.UUID
	}{
		{"dm", nil, []uuid.UUID{tavern, crypt}},            // any scene, before there's an active one too
		{"owner", ptr(tavern), []uuid.UUID{tavern, crypt}}, //
		{"player", ptr(tavern), []uuid.UUID{tavern}},       // the active scene
		{"player", ptr(crypt), []uuid.UUID{crypt}},         // moved to the crypt: not the tavern any more
		{"player", nil, nil},                               // no active scene yet
		{"outsider", ptr(tavern), nil},
	}
	for _, tc := range cases {
		p := tablePeople[tc.who]
		c := CampaignFrom(campaign, p.id, ownerID, role(p.role), nil)
		for _, scene := range []uuid.UUID{tavern, crypt} {
			want := false
			for _, s := range tc.sees {
				want = want || s == scene
			}
			if got := c.SceneVisible(scene, tc.current); got != want {
				t.Errorf("%s on %v sees %s = %v, want %v", tc.who, tc.current, scene, got, want)
			}
			err := c.RequireScene(scene, tc.current)
			if want != (err == nil) || (!want && status(err) != http.StatusNotFound) {
				t.Errorf("%s on %v: RequireScene(%s) = %v, want nil or a 404", tc.who, tc.current, scene, err)
			}
		}
	}
}
