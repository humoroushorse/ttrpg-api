// Package campaigns serves /api/v1/campaigns: campaigns, their members and invites, and the rules
// characters built for them follow. internal/access decides who may do what; a campaign the caller
// isn't in is a 404, so its existence isn't revealed.
package campaigns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	db "github.com/humoroushorse/go_dnd/internal/repository/campaigns"
	"github.com/humoroushorse/go_dnd/internal/rules"
	"github.com/humoroushorse/go_dnd/internal/tabletop"
)

// CampaignRoutes serves the campaign routes.
type CampaignRoutes struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
	// members leaving, role changes and deletes reach open tables through it
	table *tabletop.Table
}

// NewRoutes reads from replica, writes through master, and tells open tables through table.
func NewRoutes(master, replica *pgxpool.Pool, users *identity.Resolver, table *tabletop.Table) *CampaignRoutes {
	return &CampaignRoutes{master: master, replica: replica, users: users, table: table}
}

// Campaign is a campaign as the API returns it.
type Campaign struct {
	ID          uuid.UUID   `json:"id"`
	OwnerID     uuid.UUID   `json:"owner_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Rules       rules.Rules `json:"rules"`
	// {access: {invite, party_sheets}}, with defaults filled in
	Settings json.RawMessage `json:"settings"`
	// the caller's role: dm or player
	Role      string     `json:"role"`
	CreatedAt string     `json:"created_at"`
	CreatedBy *uuid.UUID `json:"created_by"`
	UpdatedAt string     `json:"updated_at"`
	UpdatedBy *uuid.UUID `json:"updated_by"`
}

func toCampaign(row db.DndCampaign, role access.Role) Campaign {
	// a document stored before a field existed reads with that field's default
	stored, err := rules.Parse(row.Rules, rules.Default())
	if err != nil {
		stored = rules.Default()
	}
	return Campaign{
		ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Description: row.Description,
		Rules: stored, Settings: withAccess(row.Settings), Role: string(role),
		CreatedAt: httpx.Timestamp(row.CreatedAt.Time), CreatedBy: row.CreatedBy,
		UpdatedAt: httpx.Timestamp(row.UpdatedAt.Time), UpdatedBy: row.UpdatedBy,
	}
}

type listResponse struct {
	TotalEntitiesCount int64          `json:"total_entities_count"`
	Limit              int            `json:"limit"`
	Offset             int            `json:"offset"`
	Filters            map[string]any `json:"filters"`
	EntitiesCount      int            `json:"entities_count"`
	Entities           []Campaign     `json:"entities"`
}

func notFound(id uuid.UUID) error { return httpx.NotFound("Campaign %s not found", id) }

// require is who the caller is to the campaign, or the error when they may not take the action.
func require(ctx context.Context, q *db.Queries, id, userID uuid.UUID, a access.CampaignAction) (access.Campaign, error) {
	row, err := q.CampaignAccess(ctx, db.CampaignAccessParams{UserID: userID, CampaignID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return access.Campaign{}, notFound(id)
	}
	if err != nil {
		return access.Campaign{}, err
	}
	c := access.CampaignFrom(id, userID, row.OwnerID, row.Role, row.Settings)
	return c, c.Require(a)
}

// withAccess is a settings document with its access settings filled in: the stored ones, else defaults.
func withAccess(raw []byte) json.RawMessage {
	doc := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &doc)
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	doc["access"], _ = json.Marshal(access.ParseSettings(raw))
	out, _ := json.Marshal(doc)
	return out
}

// QueryCampaigns is GET /campaigns/query: the campaigns the caller is in, by name.
func (h *CampaignRoutes) QueryCampaigns(w http.ResponseWriter, r *http.Request, p api.QueryCampaignsParams) {
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
	rows, err := db.New(h.replica).ListCampaigns(ctx, db.ListCampaignsParams{UserID: userID, MaxRows: int32(limit), SkipRows: int32(offset)})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := listResponse{Limit: limit, Offset: offset, Filters: map[string]any{}, Entities: make([]Campaign, len(rows))}
	for i, row := range rows {
		out.Entities[i] = toCampaign(row.DndCampaign, access.Role(row.Role))
		out.TotalEntitiesCount = row.Total
	}
	out.EntitiesCount = len(rows)
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ReadCampaign is GET /campaigns/{entity_id}.
func (h *CampaignRoutes) ReadCampaign(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	who, err := require(ctx, q, id, userID, access.ViewCampaign)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := q.GetCampaign(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = notFound(id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCampaign(row, who.Role))
}

// body is a campaign request body: which fields it sets, as raw JSON.
type body map[string]json.RawMessage

var bodyFields = []string{"name", "description", "rules", "settings"}

func readBody(r *http.Request) (body, error) {
	var b body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&b); err != nil || b == nil {
		return nil, httpx.Unprocessable("The body must be a JSON object")
	}
	var unknown []string
	for key := range b {
		if !contains(bodyFields, key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		return nil, httpx.Unprocessable("%s: extra fields not permitted", strings.Join(unknown, ", "))
	}
	return b, nil
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func isNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// text reads a string field; required fields can't be empty.
func (b body) text(field string, required bool) (string, bool, error) {
	raw, ok := b[field]
	if !ok {
		return "", false, nil
	}
	var s string
	if isNull(raw) || json.Unmarshal(raw, &s) != nil {
		return "", true, httpx.Unprocessable("%s must be a string", field)
	}
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", true, httpx.Unprocessable("%s can't be empty", field)
	}
	return s, true, nil
}

// fields is a write's values. full (POST, PUT): name is required and missing fields get their
// defaults. Otherwise (PATCH) they keep current's, and rules merge into current's.
func (b body) fields(full bool, current db.DndCampaign) (name, description string, ruleSet rules.Rules, err error) {
	name, hasName, err := b.text("name", true)
	if err != nil {
		return
	}
	description, hasDescription, err := b.text("description", false)
	if err != nil {
		return
	}
	base := rules.Default()
	if !full {
		if !hasName {
			name = current.Name
		}
		if !hasDescription {
			description = current.Description
		}
		if stored, parseErr := rules.Parse(current.Rules, rules.Default()); parseErr == nil {
			base = stored
		}
	} else if !hasName {
		return "", "", rules.Rules{}, httpx.Unprocessable("name: field required")
	}
	raw, hasRules := b["rules"]
	if hasRules && isNull(raw) {
		return "", "", rules.Rules{}, httpx.Unprocessable("rules can't be null")
	}
	ruleSet, err = rules.Parse(raw, base)
	if err != nil {
		err = httpx.Unprocessable("%v", err)
	}
	return
}

// settings is a write's settings document. Only `access` can be set for now; in a PATCH it merges into
// the current access settings. Other keys stored (the VTT's, later) are kept.
func (b body) settings(full bool, current []byte) ([]byte, error) {
	base := []byte(`{}`)
	if !full {
		base = current
	}
	raw, ok := b["settings"]
	if !ok {
		return withAccess(base), nil
	}
	var in map[string]json.RawMessage
	if isNull(raw) || json.Unmarshal(raw, &in) != nil || in == nil {
		return nil, httpx.Unprocessable("settings must be an object")
	}
	for key := range in {
		if key != "access" {
			return nil, httpx.Unprocessable("settings.%s: only access can be set for now", key)
		}
	}
	merged := access.ParseSettings(base)
	if a, ok := in["access"]; ok {
		var patch struct {
			Invite      *string `json:"invite"`
			PartySheets *string `json:"party_sheets"`
		}
		decoder := json.NewDecoder(bytes.NewReader(a))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&patch); err != nil {
			return nil, httpx.Unprocessable("settings.access: %v", err)
		}
		if patch.Invite != nil {
			merged.Invite = *patch.Invite
		}
		if patch.PartySheets != nil {
			merged.PartySheets = *patch.PartySheets
		}
		if !merged.Valid() {
			return nil, httpx.Unprocessable("settings.access: invite is dms or members, party_sheets is summary or full")
		}
	}
	doc := map[string]json.RawMessage{}
	_ = json.Unmarshal(base, &doc)
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	doc["access"], _ = json.Marshal(merged)
	out, _ := json.Marshal(doc)
	return out, nil
}

// CreateCampaign is POST /campaigns: the caller is its owner and first DM.
func (h *CampaignRoutes) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, err := h.create(ctx, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCampaign(row, access.DM))
}

func (h *CampaignRoutes) create(ctx context.Context, r *http.Request) (db.DndCampaign, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return db.DndCampaign{}, err
	}
	b, err := readBody(r)
	if err != nil {
		return db.DndCampaign{}, err
	}
	name, description, ruleSet, err := b.fields(true, db.DndCampaign{})
	if err != nil {
		return db.DndCampaign{}, err
	}
	settings, err := b.settings(true, nil)
	if err != nil {
		return db.DndCampaign{}, err
	}
	tx, err := h.master.Begin(ctx)
	if err != nil {
		return db.DndCampaign{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	stored, _ := json.Marshal(ruleSet)
	row, err := q.CreateCampaign(ctx, db.CreateCampaignParams{OwnerID: userID, Name: name, Description: description,
		Rules: stored, Settings: settings})
	if err != nil {
		return db.DndCampaign{}, err
	}
	if err := q.AddMember(ctx, db.AddMemberParams{CampaignID: row.ID, UserID: userID, Role: string(access.DM), By: &userID}); err != nil {
		return db.DndCampaign{}, err
	}
	return row, tx.Commit(ctx)
}

// ReplaceCampaign is PUT /campaigns/{entity_id}: fields left out go back to their defaults.
func (h *CampaignRoutes) ReplaceCampaign(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.update(w, r, id, true)
}

// UpdateCampaign is PATCH /campaigns/{entity_id}: only the fields sent change, and `rules` merges
// into the current rules (send {"rules": {"allow_unearthed_arcana": true}} to change one).
func (h *CampaignRoutes) UpdateCampaign(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.update(w, r, id, false)
}

func (h *CampaignRoutes) update(w http.ResponseWriter, r *http.Request, id uuid.UUID, full bool) {
	ctx := r.Context()
	row, role, err := h.write(ctx, r, id, full)
	if errors.Is(err, pgx.ErrNoRows) {
		err = notFound(id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCampaign(row, role))
}

func (h *CampaignRoutes) write(ctx context.Context, r *http.Request, id uuid.UUID, full bool) (db.DndCampaign, access.Role, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	b, err := readBody(r)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	tx, err := h.master.Begin(ctx)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	who, err := require(ctx, q, id, userID, access.EditCampaign)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	current, err := q.GetCampaignForUpdate(ctx, id)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	name, description, ruleSet, err := b.fields(full, current)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	settings, err := b.settings(full, current.Settings)
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	stored, _ := json.Marshal(ruleSet)
	row, err := q.UpdateCampaign(ctx, db.UpdateCampaignParams{ID: id, UserID: &userID, Name: name, Description: description,
		Rules: stored, Settings: settings})
	if err != nil {
		return db.DndCampaign{}, "", err
	}
	return row, who.Role, tx.Commit(ctx)
}

// DeleteCampaign is DELETE /campaigns/{entity_id}: a soft delete, by its owner. Returns the id. Open
// tables close.
func (h *CampaignRoutes) DeleteCampaign(w http.ResponseWriter, r *http.Request, id api.EntityId) {
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
		if _, err := require(ctx, q, id, userID, access.DeleteCampaign); err != nil {
			return nil, err
		}
		events, err := h.table.CampaignDeleted(ctx, tx, id, userID)
		if err != nil {
			return nil, err
		}
		if _, err := q.DeleteCampaign(ctx, db.DeleteCampaignParams{ID: id, OwnerID: &userID}); err != nil {
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

// ReadDefaultRules is GET /campaigns/rules/defaults: what a new campaign starts with.
func (h *CampaignRoutes) ReadDefaultRules(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, rules.Default())
}
