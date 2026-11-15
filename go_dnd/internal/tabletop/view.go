package tabletop

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

// What each viewer may see. The rules live here, in one place, and both the state route and the
// stream use them, so a player's copy never carries a token's true name, a hidden disposition, the DM
// layer, the scene tree, or another scene's events.

// SceneView is a scene as the API returns it (VttScene).
type SceneView struct {
	ID                uuid.UUID          `json:"id"`
	CampaignID        uuid.UUID          `json:"campaign_id"`
	ParentID          *uuid.UUID         `json:"parent_id"`
	SortOrder         int32              `json:"sort_order"`
	Name              string             `json:"name"`
	BackgroundSources []BackgroundSource `json:"background_sources"`
	GridType          string             `json:"grid_type"`
	WidthCells        int32              `json:"width_cells"`
	HeightCells       int32              `json:"height_cells"`
	CellPx            float64            `json:"cell_px"`
	GridOffsetX       float64            `json:"grid_offset_x"`
	GridOffsetY       float64            `json:"grid_offset_y"`
	GridDistance      float64            `json:"grid_distance"`
	GridUnits         string             `json:"grid_units"`
	Settings          SceneSettings      `json:"settings"`
	CreatedAt         string             `json:"created_at"`
	UpdatedAt         string             `json:"updated_at"`
}

func sceneView(s db.DndVttScene) SceneView {
	sources := []BackgroundSource{}
	_ = json.Unmarshal(s.BackgroundSources, &sources)
	return SceneView{ID: s.ID, CampaignID: s.CampaignID, ParentID: s.ParentID, SortOrder: s.SortOrder, Name: s.Name,
		BackgroundSources: sources, GridType: s.GridType, WidthCells: s.WidthCells, HeightCells: s.HeightCells,
		CellPx: s.CellPx, GridOffsetX: s.GridOffsetX, GridOffsetY: s.GridOffsetY, GridDistance: s.GridDistance,
		GridUnits: s.GridUnits, Settings: parseSettings(s.Settings),
		CreatedAt: httpx.Timestamp(s.CreatedAt.Time), UpdatedAt: httpx.Timestamp(s.UpdatedAt.Time)}
}

// TokenView is a token as one viewer may see it (VttToken).
type TokenView struct {
	ID                    uuid.UUID  `json:"id"`
	SceneID               uuid.UUID  `json:"scene_id"`
	Kind                  string     `json:"kind"`
	CharacterID           *uuid.UUID `json:"character_id"`
	ControllerCharacterID *uuid.UUID `json:"controller_character_id"`
	Name                  string     `json:"name"`
	Number                *int32     `json:"number"`
	Disposition           string     `json:"disposition"`
	ImageURL              *string    `json:"image_url"`
	X                     float64    `json:"x"`
	Y                     float64    `json:"y"`
	SizeCells             float64    `json:"size_cells"`
	Layer                 string     `json:"layer"`
	CanMove               bool       `json:"can_move"`
	// DMs only: what players see instead
	PlayerName        *string `json:"player_name,omitempty"`
	PlayerDisposition *string `json:"player_disposition,omitempty"`
}

// Viewer is who a copy of the table is built for. Current and Active change as the stream applies
// scene.activated and member.moved, so later events are filtered by where the viewer is now.
type Viewer struct {
	UserID   uuid.UUID
	Campaign access.Campaign
	// the campaign's active scene, and the viewer's scene when a DM moved them elsewhere
	Active, Moved *uuid.UUID
	// the campaign's live characters, and the ones the viewer may edit (access.EditableCharacters)
	Known, Editable map[uuid.UUID]bool
}

// NewViewer builds a viewer from the table state and the campaign's characters.
func NewViewer(userID uuid.UUID, who access.Campaign, active, moved *uuid.UUID, characters []db.CampaignCharacterEditorsRow) *Viewer {
	v := &Viewer{UserID: userID, Campaign: who, Active: active, Moved: moved}
	v.SetCharacters(characters)
	return v
}

// SetCharacters (re)loads what can_move needs.
func (v *Viewer) SetCharacters(rows []db.CampaignCharacterEditorsRow) {
	facts := make([]access.CharacterFacts, len(rows))
	v.Known = make(map[uuid.UUID]bool, len(rows))
	for i, r := range rows {
		facts[i] = access.CharacterFacts{ID: r.CharacterID, OwnerID: r.OwnerID, Editor: r.Editor}
		v.Known[r.CharacterID] = true
	}
	v.Editable = access.EditableCharacters(v.UserID, facts)
}

// DM reports whether the viewer is one of the campaign's DMs.
func (v *Viewer) DM() bool { return v.Campaign.Role == access.DM }

// Current is the scene the viewer is on: where a DM moved them, else the active scene.
func (v *Viewer) Current() *uuid.UUID {
	if v.Moved != nil {
		return v.Moved
	}
	return v.Active
}

// Scene is a scene as the viewer may see it: players don't get its place in the tree.
func (v *Viewer) Scene(s SceneView) SceneView {
	if !v.DM() {
		s.ParentID, s.SortOrder = nil, 0
	}
	return s
}

func accessToken(t TokenRecord) access.Token {
	return access.Token{ID: t.ID, Layer: t.Layer, CharacterID: t.CharacterID, ControllerCharacterID: t.ControllerCharacterID}
}

// Token is the token as the viewer may see it; false when they may not know it's there.
func (v *Viewer) Token(t TokenRecord) (TokenView, bool) {
	if !v.Campaign.TokenVisible(accessToken(t)) {
		return TokenView{}, false
	}
	name := ""
	switch {
	case t.Name != nil:
		name = *t.Name
	case t.CharacterName != nil:
		name = *t.CharacterName
	}
	image := t.ImageURL
	if image == nil && t.CharacterPortrait != "" {
		portrait := t.CharacterPortrait
		image = &portrait
	}
	out := TokenView{ID: t.ID, SceneID: t.SceneID, Kind: t.Kind, CharacterID: t.CharacterID,
		ControllerCharacterID: t.ControllerCharacterID, Name: name, Number: t.Number, Disposition: t.Disposition,
		ImageURL: image, X: t.X, Y: t.Y, SizeCells: t.SizeCells, Layer: t.Layer,
		CanMove: v.Campaign.CanMoveToken(accessToken(t), v.Editable)}
	if v.DM() {
		out.PlayerName, out.PlayerDisposition = t.PlayerName, t.PlayerDisposition
		return out, true
	}
	if t.PlayerName != nil {
		// a hidden identity hides the picture too: a vampire's portrait gives it away
		out.Name, out.ImageURL = *t.PlayerName, nil
	}
	if t.PlayerDisposition != nil {
		out.Disposition = *t.PlayerDisposition
	}
	return out, true
}

// Outgoing is one stream message: `id: Seq` (none when 0), `event: Type`, `data: Data`.
type Outgoing struct {
	Seq  int64
	Type string
	Data EventView
}

// EventView is the data of a stream message (VttEvent).
type EventView struct {
	Seq       int64      `json:"seq,omitempty"`
	Type      string     `json:"type"`
	SceneID   *uuid.UUID `json:"scene_id"`
	ActorID   *uuid.UUID `json:"actor_id"`
	CreatedAt string     `json:"created_at"`
	Payload   any        `json:"payload"`
}

// Effect is what an event does to the stream besides what it sends.
type Effect int

const (
	Continue Effect = iota
	// the viewer's role changed: send `reload` and close, so they load the state as their new role
	ReloadAndClose
	// the viewer left, or the campaign is gone
	Close
)

// NeedsCharacters reports whether the event names a character the viewer's can_move facts don't know
// (one that just joined), so the stream reloads them before building the event.
func (v *Viewer) NeedsCharacters(e Event) bool {
	if e.Type != TokenAdded && e.Type != TokenUpdated {
		return false
	}
	var p tokenPayload
	if json.Unmarshal(e.Payload, &p) != nil {
		return false
	}
	for _, id := range []*uuid.UUID{p.Token.CharacterID, p.Token.ControllerCharacterID} {
		if id != nil && !v.Known[*id] {
			return true
		}
	}
	return false
}

// Apply builds the viewer's copy of e and updates where the viewer is. It returns nothing to send
// when the event isn't for them.
func (v *Viewer) Apply(e Event) ([]Outgoing, Effect) {
	out := func(typ string, payload any) []Outgoing {
		return []Outgoing{{Seq: e.Seq, Type: typ, Data: EventView{Seq: e.Seq, Type: typ, SceneID: e.SceneID,
			ActorID: e.ActorID, CreatedAt: httpx.Timestamp(e.CreatedAt), Payload: payload}}}
	}
	raw := json.RawMessage(e.Payload)

	// events that move the viewer, or end the stream, before any filtering
	switch e.Type {
	case SceneActivated:
		var p activatedPayload
		_ = json.Unmarshal(e.Payload, &p)
		v.Active, v.Moved = p.SceneID, nil
		if v.DM() {
			return out(e.Type, raw), Continue
		}
		return out(e.Type, activatedPayload{SceneID: v.Current()}), Continue
	case MemberMoved:
		var p memberMovedPayload
		_ = json.Unmarshal(e.Payload, &p)
		if p.UserID == v.UserID {
			v.Moved = p.SceneID
			return out(e.Type, memberMovedPayload{UserID: p.UserID, SceneID: v.Current()}), Continue
		}
		if v.DM() {
			return out(e.Type, raw), Continue
		}
		return nil, Continue
	case MemberUpdated:
		var p memberPayload
		_ = json.Unmarshal(e.Payload, &p)
		if p.UserID == v.UserID && access.Role(p.Role) != v.Campaign.Role {
			return out(e.Type, raw), ReloadAndClose
		}
		return out(e.Type, raw), Continue
	case MemberLeft:
		var p memberPayload
		_ = json.Unmarshal(e.Payload, &p)
		if p.UserID == v.UserID {
			return out(e.Type, raw), Close
		}
		return out(e.Type, raw), Continue
	case CampaignDeleted:
		return out(e.Type, raw), Close
	}

	if e.Audience == AudienceDMs && !v.DM() {
		return nil, Continue
	}
	// a scene's events go to players on that scene; DMs get every scene's, their tabs pick
	if e.SceneID != nil && !v.DM() {
		current := v.Current()
		if current == nil || *current != *e.SceneID {
			return nil, Continue
		}
	}

	switch e.Type {
	case TokenAdded:
		var p tokenPayload
		if json.Unmarshal(e.Payload, &p) != nil {
			return nil, Continue
		}
		if t, ok := v.Token(p.Token); ok {
			return out(e.Type, map[string]any{"token": t}), Continue
		}
		return nil, Continue
	case TokenUpdated:
		var p tokenPayload
		if json.Unmarshal(e.Payload, &p) != nil {
			return nil, Continue
		}
		before := p.Token
		if p.PreviousLayer != "" {
			before.Layer = p.PreviousLayer
		}
		t, visible := v.Token(p.Token)
		_, wasVisible := v.Token(before)
		switch {
		case visible && wasVisible:
			return out(e.Type, map[string]any{"token": t}), Continue
		case visible:
			return out(TokenAdded, map[string]any{"token": t}), Continue
		case wasVisible:
			return out(TokenRemoved, removePayload{TokenID: p.Token.ID}), Continue
		}
		return nil, Continue
	case TokenRemoved:
		var p removePayload
		_ = json.Unmarshal(e.Payload, &p)
		return out(e.Type, map[string]any{"token_id": p.TokenID}), Continue
	case SceneCreated, SceneUpdated:
		var p scenePayload
		if json.Unmarshal(e.Payload, &p) != nil {
			return nil, Continue
		}
		return out(e.Type, scenePayload{Scene: v.Scene(p.Scene)}), Continue
	}
	return out(e.Type, raw), Continue
}

// Live builds the viewer's copy of a ping; false when it's on a scene they aren't on.
func (v *Viewer) Live(e Event) (Outgoing, bool) {
	if e.SceneID != nil && !v.DM() {
		current := v.Current()
		if current == nil || *current != *e.SceneID {
			return Outgoing{}, false
		}
	}
	return Outgoing{Type: e.Type, Data: EventView{Type: e.Type, SceneID: e.SceneID, ActorID: e.ActorID,
		CreatedAt: httpx.Timestamp(e.CreatedAt), Payload: json.RawMessage(e.Payload)}}, true
}

// reloadMessage tells the client to load the state again and reconnect from its seq.
func reloadMessage(reason string) Outgoing {
	return Outgoing{Type: Reload, Data: EventView{Type: Reload, CreatedAt: httpx.Timestamp(time.Now()),
		Payload: map[string]string{"reason": reason}}}
}
