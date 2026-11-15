package tabletop

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

// Each command's fields, besides type, command_id and scene_id.
var commandFields = map[string][]string{
	TokenMove:   {"token_id", "x", "y"},
	TokenAdd:    {"kind", "character_id", "controller_character_id", "name", "player_name", "number", "disposition", "player_disposition", "image_url", "x", "y", "size_cells", "layer"},
	TokenUpdate: {"token_id", "controller_character_id", "name", "player_name", "number", "disposition", "player_disposition", "image_url", "size_cells", "layer"},
	TokenRemove: {"token_id"},
	Ping:        {"x", "y"},
}

// Command types.
const (
	TokenMove   = "token.move"
	TokenAdd    = "token.add"
	TokenUpdate = "token.update"
	TokenRemove = "token.remove"
)

var dispositions = []string{"party", "friendly", "neutral", "hostile"}

func allCommandFields() []string {
	all := []string{"type", "command_id", "scene_id"}
	for _, keys := range commandFields {
		for _, k := range keys {
			if !slices.Contains(all, k) {
				all = append(all, k)
			}
		}
	}
	return all
}

// PostVttCommand is POST /vtt/campaigns/{entity_id}/commands.
func (h *TableRoutes) PostVttCommand(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	f, err := readFields(r, allCommandFields()...)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var typ string
	var sceneID uuid.UUID
	var commandID *uuid.UUID
	err = func() error {
		if present, err := f.decode("type", &typ, false); err != nil || !present {
			return firstErr(err, httpx.Unprocessable("type: field required"))
		}
		allowed, ok := commandFields[typ]
		if !ok {
			types := make([]string, 0, len(commandFields))
			for t := range commandFields {
				types = append(types, t)
			}
			sort.Strings(types)
			return httpx.Unprocessable("type must be one of %s", strings.Join(types, ", "))
		}
		var extra []string
		for key := range f {
			if key != "type" && key != "command_id" && key != "scene_id" && !slices.Contains(allowed, key) {
				extra = append(extra, key)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			return httpx.Unprocessable("%s doesn't take %s", typ, strings.Join(extra, ", "))
		}
		if present, err := f.decode("scene_id", &sceneID, false); err != nil || !present {
			return firstErr(err, httpx.Unprocessable("scene_id: field required"))
		}
		_, err := f.decode("command_id", &commandID, true)
		return err
	}()
	if err != nil {
		writeError(w, r, err)
		return
	}
	if typ == Ping {
		h.ping(w, r, id, sceneID, f)
		return
	}
	var out seqResult
	err = h.write(ctx, id, access.ViewTable, func(q *db.Queries, who access.Campaign, st db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
		if commandID != nil {
			seq, err := q.CommandSeq(ctx, db.CommandSeqParams{CampaignID: id, CommandID: *commandID})
			if err == nil { // a retry of a command already applied
				out.Seq = &seq
				return nil, nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
		}
		if err := who.RequireScene(sceneID, current(st)); err != nil {
			return nil, err
		}
		scene, err := liveScene(ctx, q, id, sceneID)
		if err != nil {
			return nil, err
		}
		characters, err := q.CampaignCharacterEditors(ctx, db.CampaignCharacterEditorsParams{CampaignID: id, UserID: userID})
		if err != nil {
			return nil, err
		}
		c := &command{ctx: ctx, q: q, f: f, who: who, scene: scene, userID: userID, commandID: commandID,
			viewer: NewViewer(userID, who, st.ActiveSceneID, st.MemberSceneID, characters)}
		var e Event
		switch typ {
		case TokenMove:
			e, err = c.move()
		case TokenAdd:
			e, err = c.add()
		case TokenUpdate:
			e, err = c.update()
		case TokenRemove:
			e, err = c.remove()
		}
		if err != nil {
			return nil, err
		}
		out.Seq, out.Token = &e.Seq, c.answer
		return []Event{e}, nil
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// command is one saved command inside its transaction.
type command struct {
	ctx       context.Context
	q         *db.Queries
	f         fields
	who       access.Campaign
	scene     db.DndVttScene
	userID    uuid.UUID
	commandID *uuid.UUID
	viewer    *Viewer
	// the token as the caller sees it, for token.add and token.update
	answer *TokenView
}

func (c *command) event(d draft) (Event, error) {
	d.sceneID = &c.scene.ID
	return appendEvent(c.ctx, c.q, c.scene.CampaignID, c.userID, c.commandID, d)
}

// token is a token on this command's scene; anything else is a 404.
func (c *command) token() (db.DndVttSceneToken, error) {
	var tokenID uuid.UUID
	if present, err := c.f.decode("token_id", &tokenID, false); err != nil || !present {
		return db.DndVttSceneToken{}, firstErr(err, httpx.Unprocessable("token_id: field required"))
	}
	t, err := c.q.Token(c.ctx, db.TokenParams{ID: tokenID, CampaignID: c.scene.CampaignID})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.SceneID != c.scene.ID) {
		return db.DndVttSceneToken{}, httpx.NotFound("Token %s not found", tokenID)
	}
	if err == nil && t.Layer == access.LayerDM && !c.who.TokenVisible(access.Token{Layer: t.Layer}) {
		return db.DndVttSceneToken{}, httpx.NotFound("Token %s not found", tokenID)
	}
	return t, err
}

// position reads x and y, and checks the token fits on the scene there.
func (c *command) position(size float64, required bool, x, y *float64) error {
	for _, axis := range []struct {
		key string
		dst *float64
	}{{"x", x}, {"y", y}} {
		present, err := c.f.decode(axis.key, axis.dst, false)
		if err != nil {
			return err
		}
		if !present && required {
			return httpx.Unprocessable("%s: field required", axis.key)
		}
	}
	return fits(c.scene, *x, *y, size)
}

// fits: a token's whole footprint is on the scene.
func fits(s db.DndVttScene, x, y, size float64) error {
	if size > float64(s.WidthCells) || size > float64(s.HeightCells) {
		return httpx.Unprocessable("size_cells: the token is bigger than the scene")
	}
	if err := finite("x", x, 0, float64(s.WidthCells)-size); err != nil {
		return err
	}
	return finite("y", y, 0, float64(s.HeightCells)-size)
}

func (c *command) record(id uuid.UUID) (TokenRecord, error) {
	row, err := c.q.SceneToken(c.ctx, db.SceneTokenParams{ID: id, CampaignID: c.scene.CampaignID})
	return tokenRecord(row), err
}

func (c *command) move() (Event, error) {
	t, err := c.token()
	if err != nil {
		return Event{}, err
	}
	rec := TokenRecord{ID: t.ID, Layer: t.Layer, CharacterID: t.CharacterID, ControllerCharacterID: t.ControllerCharacterID}
	if err := c.who.RequireMoveToken(accessToken(rec), c.viewer.Editable); err != nil {
		return Event{}, err
	}
	x, y := t.X, t.Y
	if err := c.position(t.SizeCells, true, &x, &y); err != nil {
		return Event{}, err
	}
	if err := c.q.MoveToken(c.ctx, db.MoveTokenParams{ID: t.ID, CampaignID: t.CampaignID, X: x, Y: y, By: c.userID}); err != nil {
		return Event{}, err
	}
	return c.event(draft{typ: TokenMoved, audience: layerAudience(t.Layer), payload: movePayload{TokenID: t.ID, X: x, Y: y}})
}

// tokenFields are the fields token.add and token.update share, applied to base.
type tokenFields struct {
	db.UpdateTokenParams
	kind        string
	characterID *uuid.UUID
}

func (c *command) tokenFields(base tokenFields, create bool) (tokenFields, map[string]bool, error) {
	f, in := c.f, base
	present := map[string]bool{}
	mark := func(key string, ok bool) { present[key] = ok }
	var err error
	var ok bool

	if ok, err = f.decode("controller_character_id", &in.ControllerCharacterID, true); err != nil {
		return in, nil, err
	}
	mark("controller_character_id", ok)
	if name, ok, err := f.text("name", 200, true, false); err != nil {
		return in, nil, err
	} else if ok {
		in.Name = name
		mark("name", true)
	}
	if playerName, ok, err := f.text("player_name", 200, true, true); err != nil {
		return in, nil, err
	} else if ok {
		in.PlayerName = playerName
		mark("player_name", true)
	}
	if ok, err = f.decode("number", &in.Number, true); err != nil {
		return in, nil, err
	}
	mark("number", ok)
	if in.Number != nil && *in.Number < 1 {
		return in, nil, httpx.Unprocessable("number must be 1 or more")
	}
	if ok, err = f.decode("disposition", &in.Disposition, false); err != nil {
		return in, nil, err
	}
	mark("disposition", ok)
	if in.Disposition != "" {
		if err := oneOf("disposition", in.Disposition, dispositions...); err != nil {
			return in, nil, err
		}
	}
	if _, err = f.decode("player_disposition", &in.PlayerDisposition, true); err != nil {
		return in, nil, err
	}
	if in.PlayerDisposition != nil {
		if err := oneOf("player_disposition", *in.PlayerDisposition, dispositions...); err != nil {
			return in, nil, err
		}
	}
	if image, ok, err := f.text("image_url", 2048, true, false); err != nil {
		return in, nil, err
	} else if ok {
		in.ImageUrl = image
	}
	if in.ImageUrl != nil {
		if err := imageURL("image_url", *in.ImageUrl); err != nil {
			return in, nil, err
		}
	}
	if _, err = f.decode("size_cells", &in.SizeCells, false); err != nil {
		return in, nil, err
	}
	if err := finite("size_cells", in.SizeCells, 0.25, 50); err != nil {
		return in, nil, err
	}
	if _, err = f.decode("layer", &in.Layer, false); err != nil {
		return in, nil, err
	}
	if err := oneOf("layer", in.Layer, access.LayerTokens, access.LayerDM); err != nil {
		return in, nil, err
	}
	if in.ControllerCharacterID != nil {
		if in.kind == "character" {
			return in, nil, httpx.Unprocessable("controller_character_id: only a marker has one (a character token is moved by whoever can edit the character)")
		}
		if !c.viewer.Known[*in.ControllerCharacterID] {
			return in, nil, httpx.Unprocessable("controller_character_id: not a character in this campaign")
		}
	}
	if in.kind == "marker" && in.Name == nil {
		return in, nil, httpx.Unprocessable("name: a marker needs one")
	}
	return in, present, nil
}

func (c *command) add() (Event, error) {
	if err := c.who.Require(access.PlaceTokens); err != nil {
		return Event{}, err
	}
	in := tokenFields{UpdateTokenParams: db.UpdateTokenParams{SizeCells: 1, Layer: access.LayerTokens}}
	if present, err := c.f.decode("kind", &in.kind, false); err != nil || !present {
		return Event{}, firstErr(err, httpx.Unprocessable("kind: field required"))
	}
	if err := oneOf("kind", in.kind, "character", "marker"); err != nil {
		return Event{}, err
	}
	if _, err := c.f.decode("character_id", &in.characterID, true); err != nil {
		return Event{}, err
	}
	switch {
	case in.kind == "character" && in.characterID == nil:
		return Event{}, httpx.Unprocessable("character_id: field required for a character token")
	case in.kind == "character" && !c.viewer.Known[*in.characterID]:
		return Event{}, httpx.Unprocessable("character_id: not a character in this campaign")
	case in.kind == "marker" && in.characterID != nil:
		return Event{}, httpx.Unprocessable("character_id: a marker isn't a character (give a copy its own marker)")
	}
	in, present, err := c.tokenFields(in, true)
	if err != nil {
		return Event{}, err
	}
	if !present["disposition"] {
		in.Disposition = "neutral"
		if in.kind == "character" {
			in.Disposition = "party"
		}
	}
	hiddenByDefault := in.kind == "marker" && in.Disposition == "hostile"
	if hiddenByDefault && !present["player_name"] && parseSettings(c.scene.Settings).TokenReveal == "numbers" {
		empty := "" // players see only the number
		in.PlayerName = &empty
	}
	if hiddenByDefault && !present["number"] {
		n, err := c.q.NextTokenNumber(c.ctx, db.NextTokenNumberParams{SceneID: c.scene.ID, CampaignID: c.scene.CampaignID})
		if err != nil {
			return Event{}, err
		}
		in.Number = &n
	}
	var x, y float64
	if err := c.position(in.SizeCells, true, &x, &y); err != nil {
		return Event{}, err
	}
	row, err := c.q.AddToken(c.ctx, db.AddTokenParams{CampaignID: c.scene.CampaignID, SceneID: c.scene.ID, Kind: in.kind,
		CharacterID: in.characterID, ControllerCharacterID: in.ControllerCharacterID, Name: in.Name,
		PlayerName: in.PlayerName, Number: in.Number, Disposition: in.Disposition, PlayerDisposition: in.PlayerDisposition,
		ImageUrl: in.ImageUrl, X: x, Y: y, SizeCells: in.SizeCells, Layer: in.Layer, By: c.userID})
	if err != nil {
		return Event{}, err
	}
	rec, err := c.record(row.ID)
	if err != nil {
		return Event{}, err
	}
	if view, ok := c.viewer.Token(rec); ok {
		c.answer = &view
	}
	return c.event(draft{typ: TokenAdded, audience: layerAudience(rec.Layer), payload: tokenPayload{Token: rec}})
}

func (c *command) update() (Event, error) {
	if err := c.who.Require(access.PlaceTokens); err != nil {
		return Event{}, err
	}
	t, err := c.token()
	if err != nil {
		return Event{}, err
	}
	base := tokenFields{kind: t.Kind, characterID: t.CharacterID, UpdateTokenParams: db.UpdateTokenParams{
		ControllerCharacterID: t.ControllerCharacterID, Name: t.Name, PlayerName: t.PlayerName, Number: t.Number,
		Disposition: t.Disposition, PlayerDisposition: t.PlayerDisposition, ImageUrl: t.ImageUrl,
		SizeCells: t.SizeCells, Layer: t.Layer}}
	in, _, err := c.tokenFields(base, false)
	if err != nil {
		return Event{}, err
	}
	if err := fits(c.scene, t.X, t.Y, in.SizeCells); err != nil {
		return Event{}, err
	}
	in.ID, in.CampaignID, in.By = t.ID, t.CampaignID, c.userID
	if _, err := c.q.UpdateToken(c.ctx, in.UpdateTokenParams); err != nil {
		return Event{}, err
	}
	rec, err := c.record(t.ID)
	if err != nil {
		return Event{}, err
	}
	if view, ok := c.viewer.Token(rec); ok {
		c.answer = &view
	}
	p := tokenPayload{Token: rec}
	if t.Layer != rec.Layer {
		p.PreviousLayer = t.Layer
	}
	return c.event(draft{typ: TokenUpdated, audience: layerAudience(t.Layer, rec.Layer), payload: p})
}

func (c *command) remove() (Event, error) {
	if err := c.who.Require(access.PlaceTokens); err != nil {
		return Event{}, err
	}
	t, err := c.token()
	if err != nil {
		return Event{}, err
	}
	if err := c.q.RemoveToken(c.ctx, db.RemoveTokenParams{ID: t.ID, CampaignID: t.CampaignID}); err != nil {
		return Event{}, err
	}
	return c.event(draft{typ: TokenRemoved, audience: layerAudience(t.Layer), payload: removePayload{TokenID: t.ID, Layer: t.Layer}})
}

// ping is sent live and not saved: anyone on the scene sees it, nobody later.
func (h *TableRoutes) ping(w http.ResponseWriter, r *http.Request, campaignID, sceneID uuid.UUID, f fields) {
	ctx := r.Context()
	err := func() error {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return err
		}
		q := db.New(h.replica)
		who, err := require(ctx, q, campaignID, userID, access.ViewTable)
		if err != nil {
			return err
		}
		st, err := tableState(ctx, q, campaignID, userID)
		if err != nil {
			return err
		}
		if err := who.RequireScene(sceneID, current(db.LockTableStateRow(st))); err != nil {
			return err
		}
		scene, err := liveScene(ctx, q, campaignID, sceneID)
		if err != nil {
			return err
		}
		c := &command{f: f, scene: scene}
		var x, y float64
		if err := c.position(0, true, &x, &y); err != nil {
			return err
		}
		if !h.pings.allow(userID, time.Now()) {
			return httpx.TooManyRequests("too many pings; wait a moment")
		}
		payload, _ := jsonMarshal(pingPayload{X: x, Y: y, UserID: userID})
		h.table.hub.PublishLive(ctx, Event{CampaignID: campaignID, SceneID: &sceneID, Type: Ping, Audience: AudienceAll,
			Payload: payload, ActorID: &userID, CreatedAt: time.Now().UTC()})
		return nil
	}()
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, seqResult{})
}
