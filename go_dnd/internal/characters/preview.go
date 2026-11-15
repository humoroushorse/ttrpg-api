package characters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/sheet"
)

// PreviewCharacterSheet is POST /characters/{entity_id}/sheet/preview: the sheet after some changes,
// saved nowhere. Each operation runs through its own route's code, so it gets exactly the same checks,
// inside one transaction that is rolled back at the end (each operation's own transaction becomes a
// savepoint in it; see begin).
func (h *CharacterRoutes) PreviewCharacterSheet(w http.ResponseWriter, r *http.Request, id api.EntityId, p api.PreviewCharacterSheetParams) {
	if p.Level != nil && (*p.Level < 1 || *p.Level > 20) {
		httpx.WriteError(w, r, httpx.Unprocessable("level must be 1 to 20"))
		return
	}
	snap, err := h.preview(r.Context(), r, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if p.Level != nil {
		snap.Level = *p.Level
	}
	httpx.WriteJSON(w, http.StatusOK, sheet.Compute(snap))
}

// operation is one change in a preview, as its route takes it.
type operation struct {
	Op         string          `json:"op"`
	ChoiceID   *uuid.UUID      `json:"choice_id"`
	ItemID     *uuid.UUID      `json:"item_id"`
	SpellID    *uuid.UUID      `json:"spell_id"`
	ModifierID *uuid.UUID      `json:"modifier_id"`
	Level      *int            `json:"level"`
	Body       json.RawMessage `json:"body"`
}

const maxPreviewOperations = 100

func (h *CharacterRoutes) preview(ctx context.Context, r *http.Request, id uuid.UUID) (sheet.Snapshot, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	// a preview is the builder's; each operation checks again through its own route
	if _, err := requireCharacter(ctx, db.New(h.replica), id, userID, access.EditBuild); err != nil {
		return sheet.Snapshot{}, err
	}
	var in struct {
		Operations []operation `json:"operations"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		return sheet.Snapshot{}, httpx.Unprocessable(`the body must be {"operations": [...]}: %v`, err)
	}
	if len(in.Operations) > maxPreviewOperations {
		return sheet.Snapshot{}, httpx.Unprocessable("at most %d operations", maxPreviewOperations)
	}
	tx, err := h.master.Begin(ctx)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // always rolled back: a preview saves nothing
	ctx = context.WithValue(ctx, previewTx{}, tx)
	for i, op := range in.Operations {
		if err := h.apply(ctx, id, op); err != nil {
			var e *httpx.Error
			if errors.As(err, &e) {
				return sheet.Snapshot{}, &httpx.Error{Status: e.Status, Code: e.Code, Issues: e.Issues,
					Detail: fmt.Sprintf("operations[%d] (%s): %s", i, op.Op, e.Detail)}
			}
			return sheet.Snapshot{}, err
		}
	}
	return LoadSnapshot(ctx, db.New(tx), id)
}

// apply runs one operation through its route's handler.
func (h *CharacterRoutes) apply(ctx context.Context, id uuid.UUID, op operation) error {
	need := func(field string, set bool) error {
		if !set {
			return httpx.Unprocessable("%s needs %s", op.Op, field)
		}
		return nil
	}
	body := op.Body
	if len(body) == 0 {
		body = []byte("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	rec := &recorder{header: http.Header{}}
	var check error
	switch op.Op {
	case "choice.add":
		h.CreateCharacterChoice(rec, req, id, api.CreateCharacterChoiceParams{})
	case "choice.replace":
		if check = need("choice_id", op.ChoiceID != nil); check == nil {
			h.ReplaceCharacterChoice(rec, req, id, *op.ChoiceID, api.ReplaceCharacterChoiceParams{})
		}
	case "choice.remove":
		if check = need("choice_id", op.ChoiceID != nil); check == nil {
			h.DeleteCharacterChoice(rec, req, id, *op.ChoiceID, api.DeleteCharacterChoiceParams{})
		}
	case "level.set":
		if check = need("level", op.Level != nil); check == nil {
			h.PutCharacterLevel(rec, req, id, *op.Level, api.PutCharacterLevelParams{})
		}
	case "level.remove":
		if check = need("level", op.Level != nil); check == nil {
			h.DeleteCharacterLevel(rec, req, id, *op.Level, api.DeleteCharacterLevelParams{})
		}
	case "scores.set":
		h.SetCharacterAbilityScores(rec, req, id, api.SetCharacterAbilityScoresParams{})
	case "character.update":
		h.UpdateCharacter(rec, req, id, api.UpdateCharacterParams{})
	case "item.add":
		h.CreateCharacterItem(rec, req, id, api.CreateCharacterItemParams{})
	case "item.update":
		if check = need("item_id", op.ItemID != nil); check == nil {
			h.UpdateCharacterItem(rec, req, id, *op.ItemID, api.UpdateCharacterItemParams{})
		}
	case "item.remove":
		if check = need("item_id", op.ItemID != nil); check == nil {
			h.DeleteCharacterItem(rec, req, id, *op.ItemID, api.DeleteCharacterItemParams{})
		}
	case "spell.prepare":
		if check = need("spell_id", op.SpellID != nil); check == nil {
			h.PrepareCharacterSpell(rec, req, id, *op.SpellID, api.PrepareCharacterSpellParams{})
		}
	case "spell.unprepare":
		if check = need("spell_id", op.SpellID != nil); check == nil {
			h.UnprepareCharacterSpell(rec, req, id, *op.SpellID, api.UnprepareCharacterSpellParams{})
		}
	case "modifier.add":
		h.CreateCharacterModifier(rec, req, id, api.CreateCharacterModifierParams{})
	case "modifier.update":
		if check = need("modifier_id", op.ModifierID != nil); check == nil {
			h.UpdateCharacterModifier(rec, req, id, *op.ModifierID, api.UpdateCharacterModifierParams{})
		}
	case "modifier.remove":
		if check = need("modifier_id", op.ModifierID != nil); check == nil {
			h.DeleteCharacterModifier(rec, req, id, *op.ModifierID, api.DeleteCharacterModifierParams{})
		}
	default:
		return httpx.Unprocessable("unknown op %q", op.Op)
	}
	if check != nil {
		return check
	}
	if rec.status == http.StatusOK {
		return nil
	}
	var e struct {
		Detail string `json:"detail"`
		Code   string `json:"code"`
		Issues any    `json:"issues"`
	}
	_ = json.Unmarshal(rec.body.Bytes(), &e)
	return &httpx.Error{Status: rec.status, Code: e.Code, Detail: e.Detail, Issues: e.Issues}
}

// recorder keeps a handler's response so the preview can read its status and error.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) { r.status = status }

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
