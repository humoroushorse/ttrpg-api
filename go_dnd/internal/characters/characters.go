// Package characters serves /api/v1/characters: a character, its rules, and its planned levels.
// Owner only for now: someone else's character is a 404.
//
// Every write runs in one transaction that locks the character, bumps its version, and adds a
// character_change row. Two headers support offline sync (CHARACTER_PLAN.md section 11):
//
//	If-Match: <version>   the write is a 409 if the character changed since that version
//	Op-Id: <uuid>         a replayed operation that was already applied isn't applied again
package characters

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/rules"
	"github.com/humoroushorse/go_dnd/internal/tabletop"
)

// CharacterRoutes serves the character routes.
type CharacterRoutes struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
	// a character leaving its campaign, or deleted, leaves open tables through it
	table *tabletop.Table
}

// NewRoutes reads from replica, writes through master, and tells open tables through table.
func NewRoutes(master, replica *pgxpool.Pool, users *identity.Resolver, table *tabletop.Table) *CharacterRoutes {
	return &CharacterRoutes{master: master, replica: replica, users: users, table: table}
}

// writeTxKey carries a write's transaction to the few changes that also log table events (taking the
// character out of its campaign), which need it beyond this package's queries.
type writeTxKey struct{}

// ClassRef names a level's class.
type ClassRef struct {
	ID             uuid.UUID `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	DndVersionYear int32     `json:"dnd_version_year"`
	SourceSlug     string    `json:"source_slug"`
}

// Level is one planned level.
type Level struct {
	ID       uuid.UUID       `json:"id"`
	Level    int32           `json:"level"`
	Class    ClassRef        `json:"class"`
	HPMethod string          `json:"hp_method"`
	HPValue  *int32          `json:"hp_value"`
	HPRolls  json.RawMessage `json:"hp_rolls"`
}

// Character is a character as the API returns it.
type Character struct {
	ID               uuid.UUID       `json:"id"`
	OwnerID          uuid.UUID       `json:"owner_id"`
	Name             string          `json:"name"`
	CampaignID       *uuid.UUID      `json:"campaign_id"`
	CurrentLevel     int32           `json:"current_level"`
	Rules            rules.Rules     `json:"rules"`
	RulesFrom        string          `json:"rules_from"` // campaign | character
	AbilityMethod    string          `json:"ability_method"`
	BaseScores       json.RawMessage `json:"base_scores"`
	AbilityRolls     json.RawMessage `json:"ability_rolls"`
	AbilityRollCount int32           `json:"ability_roll_count"`
	Currency         json.RawMessage `json:"currency"`
	Details          json.RawMessage `json:"details"`
	Version          int32           `json:"version"`
	CreatedAt        string          `json:"created_at"`
	CreatedBy        *uuid.UUID      `json:"created_by"`
	UpdatedAt        string          `json:"updated_at"`
	UpdatedBy        *uuid.UUID      `json:"updated_by"`
	// what the caller may do with it (the list fills in owner and editor only)
	Viewer *Viewer `json:"viewer,omitempty"`
}

// Detail is a character with its whole build: what GET /characters/{id} and every write answer with.
type Detail struct {
	Character
	Levels         []Level         `json:"levels"`
	Choices        []Choice        `json:"choices"`
	Items          []Item          `json:"items"`
	PreparedSpells []PreparedSpell `json:"prepared_spells"`
	Modifiers      []Modifier      `json:"modifiers"`
}

func notFound(id uuid.UUID) error { return httpx.NotFound("Character %s not found", id) }

// effectiveRules is the campaign's rules when the character is in one, otherwise its own.
func effectiveRules(ctx context.Context, q *db.Queries, c db.DndCharacter, campaignID uuid.UUID) (rules.Rules, string, error) {
	if campaignID != uuid.Nil {
		raw, err := q.CampaignRules(ctx, campaignID)
		if err == nil {
			r, parseErr := rules.Parse(raw, rules.Default())
			if parseErr != nil {
				r = rules.Default()
			}
			return r, "campaign", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return rules.Rules{}, "", err
		}
	}
	r, err := rules.Parse(c.Rules, rules.Default())
	if err != nil {
		r = rules.Default()
	}
	return r, "character", nil
}

func toCharacter(c db.DndCharacter, campaignID uuid.UUID, r rules.Rules, from string) Character {
	out := Character{
		ID: c.ID, OwnerID: c.OwnerID, Name: c.Name, CurrentLevel: c.CurrentLevel, Rules: r, RulesFrom: from,
		AbilityMethod: c.AbilityMethod, BaseScores: c.BaseScores, AbilityRolls: c.AbilityRolls,
		AbilityRollCount: c.AbilityRollCount, Currency: c.Currency, Details: c.Details, Version: c.Version,
		CreatedAt: httpx.Timestamp(c.CreatedAt.Time), CreatedBy: c.CreatedBy,
		UpdatedAt: httpx.Timestamp(c.UpdatedAt.Time), UpdatedBy: c.UpdatedBy,
	}
	if campaignID != uuid.Nil {
		out.CampaignID = &campaignID
	}
	return out
}

// load is the character as the API returns it, for a route that has already checked access.
func (h *CharacterRoutes) load(ctx context.Context, q *db.Queries, id uuid.UUID) (Detail, error) {
	row, err := q.GetCharacter(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, notFound(id)
	}
	if err != nil {
		return Detail{}, err
	}
	r, from, err := effectiveRules(ctx, q, row.DndCharacter, row.CampaignID)
	if err != nil {
		return Detail{}, err
	}
	out := Detail{Character: toCharacter(row.DndCharacter, row.CampaignID, r, from)}
	levels, err := q.ListLevels(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	out.Levels = make([]Level, len(levels))
	for i, l := range levels {
		out.Levels[i] = Level{ID: l.ID, Level: l.Level, HPMethod: l.HpMethod, HPValue: l.HpValue, HPRolls: l.HpRolls,
			Class: ClassRef{ID: l.ClassID, Slug: l.ClassSlug, Name: l.ClassName, DndVersionYear: l.ClassYear, SourceSlug: l.ClassSourceSlug}}
	}
	if err := loadBuild(ctx, q, id, &out); err != nil {
		return Detail{}, err
	}
	return out, nil
}

type listResponse struct {
	TotalEntitiesCount int64          `json:"total_entities_count"`
	Limit              int            `json:"limit"`
	Offset             int            `json:"offset"`
	Filters            map[string]any `json:"filters"`
	EntitiesCount      int            `json:"entities_count"`
	Entities           []Character    `json:"entities"`
}

// QueryCharacters is GET /characters/query: the caller's characters by name, and the ones they may edit
// (without levels).
func (h *CharacterRoutes) QueryCharacters(w http.ResponseWriter, r *http.Request, p api.QueryCharactersParams) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	limit, offset := 100, 0
	if p.Limit != nil {
		limit = *p.Limit
	}
	if p.Offset != nil {
		offset = *p.Offset
	}
	if limit < 0 || offset < 0 {
		httpx.WriteError(w, r, httpx.Unprocessable("limit and offset can't be negative"))
		return
	}
	q := db.New(h.replica)
	rows, err := q.ListCharacters(ctx, db.ListCharactersParams{UserID: userID, MaxRows: int32(limit), SkipRows: int32(offset)})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := listResponse{Limit: limit, Offset: offset, Filters: map[string]any{}, Entities: make([]Character, len(rows)), EntitiesCount: len(rows)}
	for i, row := range rows {
		ruleSet, from, err := effectiveRules(ctx, q, row.DndCharacter, row.CampaignID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out.Entities[i] = toCharacter(row.DndCharacter, row.CampaignID, ruleSet, from)
		owner := row.DndCharacter.OwnerID == userID
		out.Entities[i].Viewer = &Viewer{Owner: owner, Editor: !owner}
		out.TotalEntitiesCount = row.Total
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ReadCharacter is GET /characters/{entity_id}, with its planned levels.
func (h *CharacterRoutes) ReadCharacter(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	who, err := requireCharacter(ctx, q, id, userID, access.ViewSheet)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := h.load(ctx, q, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out.Viewer = viewerOf(who)
	httpx.WriteJSON(w, http.StatusOK, out)
}

// fields is a request body: which keys it sets, as raw JSON.
type fields map[string]json.RawMessage

func readFields(r *http.Request, allowed ...string) (fields, error) {
	var f fields
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil || f == nil {
		return nil, httpx.Unprocessable("The body must be a JSON object")
	}
	var unknown []string
	for key := range f {
		found := false
		for _, a := range allowed {
			found = found || a == key
		}
		if !found {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		return nil, httpx.Unprocessable("%s: extra fields not permitted", strings.Join(unknown, ", "))
	}
	return f, nil
}

func isNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// decode reads f[key] into dst when present. Present and null is an error unless nullable.
func (f fields) decode(key string, dst any, nullable bool) (bool, error) {
	raw, ok := f[key]
	if !ok {
		return false, nil
	}
	if isNull(raw) && !nullable {
		return true, httpx.Unprocessable("%s can't be null", key)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return true, httpx.Unprocessable("%s: %v", key, err)
	}
	return true, nil
}

// object reads a JSON object field (details, currency) as stored bytes.
func (f fields) object(key string, current []byte) ([]byte, error) {
	var m map[string]any
	present, err := f.decode(key, &m, false)
	if err != nil || !present {
		return current, err
	}
	if m == nil {
		return nil, httpx.Unprocessable("%s must be an object", key)
	}
	return json.Marshal(m)
}

func checkLevel(field string, level int32) error {
	if level < 1 || level > 20 {
		return httpx.Unprocessable("%s must be 1 to 20", field)
	}
	return nil
}

// write runs fn in a transaction that checks the caller may take the action, locks the character,
// checks If-Match, skips an already applied Op-Id, bumps the version, and logs the change (as the
// caller, who may not be the owner). It answers with the character.
func (h *CharacterRoutes) write(w http.ResponseWriter, r *http.Request, id uuid.UUID, a access.CharacterAction,
	fn func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error)) {
	ctx := r.Context()
	out, err := h.writeTx(ctx, r, id, a, fn)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// change is what a write logs.
type change struct {
	action   string
	entity   string
	entityID *uuid.UUID
	before   any
	after    any
	// table events the write logged, published once it commits
	table []tabletop.Event
}

// previewTx carries the transaction a sheet preview runs its operations in (see preview.go).
type previewTx struct{}

// begin starts a write's transaction: a savepoint inside a preview's transaction, so the preview
// can roll every operation back; a new transaction otherwise.
func (h *CharacterRoutes) begin(ctx context.Context) (pgx.Tx, error) {
	if outer, ok := ctx.Value(previewTx{}).(pgx.Tx); ok {
		return outer.Begin(ctx)
	}
	return h.master.Begin(ctx)
}

func (h *CharacterRoutes) writeTx(ctx context.Context, r *http.Request, id uuid.UUID, a access.CharacterAction,
	fn func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error)) (Detail, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return Detail{}, err
	}
	opID, err := headerUUID(r, "Op-Id")
	if err != nil {
		return Detail{}, err
	}
	tx, err := h.begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	who, err := requireCharacter(ctx, q, id, userID, a)
	if err != nil {
		return Detail{}, err
	}
	ctx = context.WithValue(ctx, callerKey{}, who)
	ctx = context.WithValue(ctx, writeTxKey{}, tx)
	version, err := q.LockCharacter(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, notFound(id)
	}
	if err != nil {
		return Detail{}, err
	}
	if opID != nil {
		applied, err := q.OpApplied(ctx, opID)
		if err != nil {
			return Detail{}, err
		}
		if applied { // a replay: answer with the character as it is
			return h.load(ctx, q, id)
		}
	}
	if expected := r.Header.Get("If-Match"); expected != "" {
		if want, err := strconv.Atoi(strings.Trim(expected, `"`)); err != nil || int32(want) != version {
			return Detail{}, httpx.Conflict("Character %s is at version %d, not %s; reload it and try again", id, version, expected)
		}
	}
	c, err := fn(ctx, q, userID)
	if err != nil {
		return Detail{}, constraintError(err)
	}
	if c.action != "" { // an empty change is a replay of a create that already happened
		if err := q.TouchCharacter(ctx, db.TouchCharacterParams{ID: id, UserID: &userID}); err != nil {
			return Detail{}, err
		}
		if err := logChange(ctx, q, id, userID, opID, c); err != nil {
			return Detail{}, err
		}
	}
	out, err := h.load(ctx, q, id)
	if err != nil {
		return Detail{}, err
	}
	out.Viewer = viewerOf(who)
	if err := tx.Commit(ctx); err != nil {
		return Detail{}, err
	}
	// inside a preview the commit only released a savepoint, and the whole preview is rolled back
	if _, preview := ctx.Value(previewTx{}).(pgx.Tx); !preview {
		h.table.Publish(ctx, c.table)
	}
	return out, nil
}

func logChange(ctx context.Context, q *db.Queries, id, userID uuid.UUID, opID *uuid.UUID, c change) error {
	asJSON := func(v any) []byte {
		if v == nil {
			return nil
		}
		raw, _ := json.Marshal(v)
		return raw
	}
	var entity *string
	if c.entity != "" {
		entity = &c.entity
	}
	return q.LogChange(ctx, db.LogChangeParams{CharacterID: id, UserID: &userID, Action: c.action, Entity: entity,
		EntityID: c.entityID, Before: asJSON(c.before), After: asJSON(c.after), OpID: opID})
}

func headerUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.Header.Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, httpx.Unprocessable("%s must be a uuid", name)
	}
	return &id, nil
}

func constraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return httpx.Conflict("%s", pgErr.Detail)
		case "23503", "23514":
			return httpx.Unprocessable("%s", pgErr.Message)
		}
	}
	return err
}

// CreateCharacter is POST /characters: for a campaign (which supplies the rules) or with its own
// rules. Accepts a client-made id, so an offline create can be replayed.
// The Op-Id header is read from the request (the generated params carry it too).
func (h *CharacterRoutes) CreateCharacter(w http.ResponseWriter, r *http.Request, _ api.CreateCharacterParams) {
	ctx := r.Context()
	out, err := h.create(ctx, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *CharacterRoutes) create(ctx context.Context, r *http.Request) (Detail, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return Detail{}, err
	}
	f, err := readFields(r, "id", "name", "campaign_id", "rules", "current_level", "currency", "details")
	if err != nil {
		return Detail{}, err
	}
	var name string
	if present, err := f.decode("name", &name, false); err != nil {
		return Detail{}, err
	} else if name = strings.TrimSpace(name); !present || name == "" {
		return Detail{}, httpx.Unprocessable("name: field required")
	}
	var id, campaignID *uuid.UUID
	if _, err := f.decode("id", &id, true); err != nil {
		return Detail{}, err
	}
	if _, err := f.decode("campaign_id", &campaignID, true); err != nil {
		return Detail{}, err
	}
	level := int32(1)
	if _, err := f.decode("current_level", &level, false); err != nil {
		return Detail{}, err
	}
	if err := checkLevel("current_level", level); err != nil {
		return Detail{}, err
	}
	currency, err := f.object("currency", []byte(`{"cp":0,"sp":0,"ep":0,"gp":0,"pp":0}`))
	if err != nil {
		return Detail{}, err
	}
	details, err := f.object("details", []byte(`{}`))
	if err != nil {
		return Detail{}, err
	}

	tx, err := h.master.Begin(ctx)
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	if id != nil { // a replayed offline create: answer with what's there, if it's the caller's
		if who, err := requireCharacter(ctx, q, *id, userID, access.EditBuild); err == nil && who.Owner {
			return h.load(ctx, q, *id)
		}
	}
	var ruleSet rules.Rules
	var ownRules []byte
	if campaignID != nil {
		if _, has := f["rules"]; has {
			return Detail{}, httpx.Unprocessable("rules: a character in a campaign uses the campaign's rules")
		}
		if _, err := requireCampaign(ctx, q, *campaignID, userID, access.AddOwnCharacter); err != nil {
			return Detail{}, err
		}
		raw, err := q.CampaignRules(ctx, *campaignID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, httpx.NotFound("Campaign %s not found", *campaignID)
		}
		if err != nil {
			return Detail{}, err
		}
		if ruleSet, err = rules.Parse(raw, rules.Default()); err != nil {
			ruleSet = rules.Default()
		}
	} else {
		if ruleSet, err = rules.Parse(f["rules"], rules.Default()); err != nil {
			return Detail{}, httpx.Unprocessable("%v", err)
		}
		ownRules, _ = json.Marshal(ruleSet)
	}
	baseScores := []byte(`{}`)
	if ruleSet.AbilityScoreDefault == "point_buy" {
		baseScores = []byte(`{"str":8,"dex":8,"con":8,"int":8,"wis":8,"cha":8}`)
	}
	newID, err := q.CreateCharacter(ctx, db.CreateCharacterParams{ID: id, OwnerID: userID, Name: name, CurrentLevel: level,
		Rules: ownRules, AbilityMethod: ruleSet.AbilityScoreDefault, BaseScores: baseScores, Currency: currency, Details: details})
	if err != nil {
		return Detail{}, constraintError(err)
	}
	if campaignID != nil {
		if err := q.LinkCampaign(ctx, db.LinkCampaignParams{CampaignID: *campaignID, CharacterID: newID, UserID: &userID}); err != nil {
			return Detail{}, err
		}
	}
	opID, err := headerUUID(r, "Op-Id")
	if err != nil {
		return Detail{}, err
	}
	if err := logChange(ctx, q, newID, userID, opID, change{action: "character.create", after: map[string]any{"name": name, "campaign_id": campaignID}}); err != nil {
		return Detail{}, err
	}
	out, err := h.load(ctx, q, newID)
	if err != nil {
		return Detail{}, err
	}
	out.Viewer = &Viewer{Owner: true, CanEdit: true, CanManage: true}
	return out, tx.Commit(ctx)
}

// UpdateCharacter is PATCH /characters/{entity_id}: name, current_level, currency, details, rules
// (merged into its own; a campaign's characters use the campaign's), and class_id (changes the
// class on every planned level; single class until multiclassing).
func (h *CharacterRoutes) UpdateCharacter(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.UpdateCharacterParams) {
	f, err := readFields(r, "name", "current_level", "currency", "details", "rules", "class_id")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		row, err := q.GetCharacter(ctx, id)
		if err != nil {
			return change{}, err
		}
		c := row.DndCharacter
		before := map[string]any{}
		after := map[string]any{}
		name := c.Name
		if present, err := f.decode("name", &name, false); err != nil {
			return change{}, err
		} else if present {
			if name = strings.TrimSpace(name); name == "" {
				return change{}, httpx.Unprocessable("name can't be empty")
			}
			before["name"], after["name"] = c.Name, name
		}
		level := c.CurrentLevel
		if present, err := f.decode("current_level", &level, false); err != nil {
			return change{}, err
		} else if present {
			if err := checkLevel("current_level", level); err != nil {
				return change{}, err
			}
			before["current_level"], after["current_level"] = c.CurrentLevel, level
		}
		currency, err := f.object("currency", c.Currency)
		if err != nil {
			return change{}, err
		}
		details, err := f.object("details", c.Details)
		if err != nil {
			return change{}, err
		}
		for _, key := range []string{"currency", "details"} {
			if _, ok := f[key]; ok {
				after[key] = f[key]
			}
		}
		ownRules := c.Rules
		if raw, has := f["rules"]; has {
			if row.CampaignID != uuid.Nil {
				return change{}, httpx.Unprocessable("rules: a character in a campaign uses the campaign's rules")
			}
			base, parseErr := rules.Parse(c.Rules, rules.Default())
			if parseErr != nil {
				base = rules.Default()
			}
			merged, err := rules.Parse(raw, base)
			if err != nil {
				return change{}, httpx.Unprocessable("%v", err)
			}
			ownRules, _ = json.Marshal(merged)
			before["rules"], after["rules"] = json.RawMessage(c.Rules), json.RawMessage(ownRules)
		}
		if err := q.UpdateCharacter(ctx, db.UpdateCharacterParams{ID: id, UserID: &userID, Name: name, CurrentLevel: level,
			Rules: ownRules, Currency: currency, Details: details}); err != nil {
			return change{}, err
		}
		var classID uuid.UUID
		if present, err := f.decode("class_id", &classID, false); err != nil {
			return change{}, err
		} else if present {
			ruleSet, _, err := effectiveRules(ctx, q, c, row.CampaignID)
			if err != nil {
				return change{}, err
			}
			if err := checkClass(ctx, q, ruleSet, classID); err != nil {
				return change{}, err
			}
			if err := q.SetClassOnAllLevels(ctx, db.SetClassOnAllLevelsParams{CharacterID: id, ClassID: classID, UserID: &userID}); err != nil {
				return change{}, err
			}
			after["class_id"] = classID
		}
		return change{action: "character.update", entity: "character", entityID: &c.ID, before: before, after: after}, nil
	})
}

// checkClass refuses a class the rules don't offer, or one deleted from the catalog.
func checkClass(ctx context.Context, q *db.Queries, ruleSet rules.Rules, classID uuid.UUID) error {
	k, err := q.ClassForRules(ctx, db.ClassForRulesParams{ID: classID, BaseEdition: int32(ruleSet.Edition)})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && k.Deleted) {
		return httpx.Unprocessable("class_id: class %s not found", classID)
	}
	if err != nil {
		return err
	}
	ok, why := ruleSet.Allows(rules.Entry{Category: "class", Year: int(k.DndVersionYear), UnearthedArcana: k.IsUnearthedArcana,
		SourceSlug: k.SourceSlug, BaseEditionHasIt: k.BaseHasSlug})
	if !ok {
		return httpx.Unprocessable("class_id: %s", why)
	}
	return nil
}

// DeleteCharacter is DELETE /characters/{entity_id}: a soft delete, by its owner. Returns the id. Its
// tokens leave open tables (they stay in the database, hidden).
func (h *CharacterRoutes) DeleteCharacter(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	events, err := func() ([]tabletop.Event, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return nil, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		who, err := requireCharacter(ctx, q, id, userID, access.DeleteCharacter)
		if err != nil {
			return nil, err
		}
		var events []tabletop.Event
		if who.CampaignID != nil {
			if events, err = h.table.CharacterDeleted(ctx, tx, *who.CampaignID, id, userID); err != nil {
				return nil, err
			}
		}
		if _, err := q.DeleteCharacter(ctx, db.DeleteCharacterParams{ID: id, OwnerID: &userID}); err != nil {
			return nil, err
		}
		return events, tx.Commit(ctx)
	}()
	if errors.Is(err, pgx.ErrNoRows) {
		err = notFound(id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.table.Publish(ctx, events)
	httpx.WriteJSON(w, http.StatusOK, uuid.UUID(id).String())
}

// PutCharacterLevel is PUT /characters/{entity_id}/levels/{level}: plan a level or change one.
// Levels are contiguous from 1, and every level has the same class (change it with PATCH class_id).
// hp_method average (the default from the rules) or manual with hp_value; rolled comes from the
// HP roll route.
func (h *CharacterRoutes) PutCharacterLevel(w http.ResponseWriter, r *http.Request, id api.EntityId, level int, _ api.PutCharacterLevelParams) {
	f, err := readFields(r, "class_id", "hp_method", "hp_value")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		if err := checkLevel("level", int32(level)); err != nil {
			return change{}, err
		}
		row, err := q.GetCharacter(ctx, id)
		if err != nil {
			return change{}, err
		}
		ruleSet, _, err := effectiveRules(ctx, q, row.DndCharacter, row.CampaignID)
		if err != nil {
			return change{}, err
		}
		top, err := q.TopLevel(ctx, id)
		if err != nil {
			return change{}, err
		}
		if int32(level) > top+1 {
			return change{}, httpx.Unprocessable("level %d: plan level %d first (levels go up one at a time)", level, top+1)
		}
		existing, err := q.GetLevel(ctx, db.GetLevelParams{CharacterID: id, Level: int32(level)})
		isNew := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !isNew {
			return change{}, err
		}
		var classID uuid.UUID
		if present, err := f.decode("class_id", &classID, false); err != nil {
			return change{}, err
		} else if !present {
			if isNew {
				return change{}, httpx.Unprocessable("class_id: field required")
			}
			classID = existing.ClassID
		}
		if isNew || classID != existing.ClassID {
			if err := checkClass(ctx, q, ruleSet, classID); err != nil {
				return change{}, err
			}
		}
		if _, err := q.OtherClass(ctx, db.OtherClassParams{CharacterID: id, Level: int32(level), ClassID: classID}); err == nil {
			return change{}, httpx.Unprocessable("class_id: every level has the same class until multiclassing; change it with PATCH class_id")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return change{}, err
		}
		method := ruleSet.HPDefault
		var value *int32
		if !isNew {
			method, value = existing.HpMethod, existing.HpValue
		}
		if method == "rolled" && isNew {
			method = "average" // a roll comes from the HP roll route
		}
		if present, err := f.decode("hp_method", &method, false); err != nil {
			return change{}, err
		} else if present {
			switch {
			case method == "rolled":
				return change{}, httpx.Unprocessable("hp_method: rolled HP comes from the HP roll route")
			case method != "average" && method != "manual":
				return change{}, httpx.Unprocessable("hp_method must be average or manual")
			case !contains(ruleSet.HPMethods, method):
				return change{}, httpx.Unprocessable("hp_method: the rules don't allow %s HP", method)
			}
			if method == "average" {
				value = nil
			}
		}
		if _, err := f.decode("hp_value", &value, true); err != nil {
			return change{}, err
		}
		if method == "manual" && value == nil && level > 1 {
			return change{}, httpx.Unprocessable("hp_value: manual HP needs a value")
		}
		if value != nil && *value < 1 {
			return change{}, httpx.Unprocessable("hp_value must be at least 1")
		}
		saved, err := q.UpsertLevel(ctx, db.UpsertLevelParams{CharacterID: id, Level: int32(level), ClassID: classID,
			HpMethod: method, HpValue: value, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		c := change{action: "level.set", entity: "character_level", entityID: &saved.ID,
			after: map[string]any{"level": level, "class_id": classID, "hp_method": method, "hp_value": value}}
		if !isNew {
			c.before = map[string]any{"level": level, "class_id": existing.ClassID, "hp_method": existing.HpMethod, "hp_value": existing.HpValue}
		}
		return c, nil
	})
}

// DeleteCharacterLevel is DELETE /characters/{entity_id}/levels/{level}: only the top planned level
// (lowering current_level is how you level down without losing the plan).
func (h *CharacterRoutes) DeleteCharacterLevel(w http.ResponseWriter, r *http.Request, id api.EntityId, level int, _ api.DeleteCharacterLevelParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		top, err := q.TopLevel(ctx, id)
		if err != nil {
			return change{}, err
		}
		if int32(level) != top {
			if top == 0 || int32(level) > top {
				return change{}, httpx.NotFound("Level %d isn't planned", level)
			}
			return change{}, httpx.Unprocessable("only the top planned level (%d) can be removed", top)
		}
		existing, err := q.GetLevel(ctx, db.GetLevelParams{CharacterID: id, Level: int32(level)})
		if err != nil {
			return change{}, err
		}
		if err := q.DeleteLevel(ctx, db.DeleteLevelParams{CharacterID: id, Level: int32(level)}); err != nil {
			return change{}, err
		}
		return change{action: "level.remove", entity: "character_level", entityID: &existing.ID,
			before: map[string]any{"level": level, "class_id": existing.ClassID, "hp_method": existing.HpMethod, "hp_value": existing.HpValue}}, nil
	})
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
