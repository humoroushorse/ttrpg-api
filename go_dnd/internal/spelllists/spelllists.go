// Package spelllists serves the class spell lists: which spells each class can learn.
//
//	GET    /classes/{class_id}/spells              the class's links
//	POST   /classes/{class_id}/spells/{spell_id}   add a spell (?qualifier=)
//	DELETE /classes/{class_id}/spells/{spell_id}   remove it
//	POST   /classes/spell-lists/bulk               add links from a spells _all.json (each row's `classes`)
//
// A listing like "Bard (Optional)" links Bard with qualifier "Optional" (Tasha's optional spells).
package spelllists

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/catalog"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	db "github.com/humoroushorse/go_dnd/internal/repository/spelllists"
	"github.com/humoroushorse/go_dnd/internal/table"
)

// SpellListRoutes serves the routes above.
type SpellListRoutes struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
}

// NewRoutes reads from replica and writes through master.
func NewRoutes(master, replica *pgxpool.Pool, users *identity.Resolver) *SpellListRoutes {
	return &SpellListRoutes{master: master, replica: replica, users: users}
}

// Link is a spell on a class's list, as the API returns it.
type Link struct {
	SpellID   uuid.UUID  `json:"spell_id"`
	ClassID   uuid.UUID  `json:"class_id"`
	Qualifier *string    `json:"qualifier"`
	CreatedAt string     `json:"created_at"`
	CreatedBy *uuid.UUID `json:"created_by"`
}

func newLink(spellID, classID uuid.UUID, qualifier *string, createdAt pgtype.Timestamptz, createdBy *uuid.UUID) Link {
	return Link{SpellID: spellID, ClassID: classID, Qualifier: qualifier, CreatedBy: createdBy,
		CreatedAt: httpx.Timestamp(createdAt.Time)}
}

type listResponse struct {
	TotalEntitiesCount int            `json:"total_entities_count"`
	Limit              int            `json:"limit"`
	Offset             int            `json:"offset"`
	Filters            map[string]any `json:"filters"`
	EntitiesCount      int            `json:"entities_count"`
	Entities           []Link         `json:"entities"`
}

func requireLive(ctx context.Context, live func(context.Context, uuid.UUID) (bool, error), id uuid.UUID, label string) error {
	ok, err := live(ctx, id)
	if err == nil && !ok {
		err = httpx.NotFound("%s %s not found", label, id)
	}
	return err
}

// ReadClassSpells is GET /classes/{class_id}/spells.
func (h *SpellListRoutes) ReadClassSpells(w http.ResponseWriter, r *http.Request, classID api.EntityId) {
	ctx := r.Context()
	q := db.New(h.replica)
	if err := requireLive(ctx, q.ClassIsLive, classID, "Class"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := q.ListClassSpells(ctx, classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	links := make([]Link, len(rows))
	for i, row := range rows {
		links[i] = newLink(row.SpellID, row.ClassID, row.Qualifier, row.CreatedAt, row.CreatedBy)
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{TotalEntitiesCount: len(links), Filters: map[string]any{},
		EntitiesCount: len(links), Entities: links})
}

// AddClassSpell is POST /classes/{class_id}/spells/{spell_id}.
func (h *SpellListRoutes) AddClassSpell(w http.ResponseWriter, r *http.Request, classID, spellID api.EntityId, p api.AddClassSpellParams) {
	ctx := r.Context()
	link, err := h.add(ctx, classID, spellID, p.Qualifier)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, link)
}

func (h *SpellListRoutes) add(ctx context.Context, classID, spellID uuid.UUID, qualifier *string) (Link, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return Link{}, err
	}
	q := db.New(h.master)
	if err := requireLive(ctx, q.ClassIsLive, classID, "Class"); err != nil {
		return Link{}, err
	}
	if err := requireLive(ctx, q.SpellIsLive, spellID, "Spell"); err != nil {
		return Link{}, err
	}
	if qualifier != nil && strings.TrimSpace(*qualifier) == "" {
		qualifier = nil
	}
	added, err := q.AddLink(ctx, db.AddLinkParams{SpellID: spellID, ClassID: classID, Qualifier: qualifier, UserID: &userID})
	if err == nil {
		return newLink(added.SpellID, added.ClassID, added.Qualifier, added.CreatedAt, added.CreatedBy), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Link{}, err
	}
	existing, err := q.GetLink(ctx, db.GetLinkParams{SpellID: spellID, ClassID: classID})
	if err != nil {
		return Link{}, err
	}
	return newLink(existing.SpellID, existing.ClassID, existing.Qualifier, existing.CreatedAt, existing.CreatedBy), nil
}

// RemoveClassSpell is DELETE /classes/{class_id}/spells/{spell_id}.
func (h *SpellListRoutes) RemoveClassSpell(w http.ResponseWriter, r *http.Request, classID, spellID api.EntityId) {
	ctx := r.Context()
	if _, err := h.users.CurrentUserID(ctx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_, err := db.New(h.master).RemoveLink(ctx, db.RemoveLinkParams{SpellID: spellID, ClassID: classID})
	if errors.Is(err, pgx.ErrNoRows) {
		err = httpx.NotFound("Spell %s isn't on class %s's list", spellID, classID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, uuid.UUID(spellID).String())
}

// "Bard (Optional)", "Wizard (Dunamancy)"
var qualified = regexp.MustCompile(`^(.+?)\s*\(([^)]+)\)$`)

// splitQualifier reads a spell's class listing: "Bard (Optional)" -> ("Bard", "Optional").
func splitQualifier(listed string) (string, string) {
	listed = strings.TrimSpace(listed)
	if m := qualified.FindStringSubmatch(listed); m != nil {
		return m[1], strings.TrimSpace(m[2])
	}
	return listed, ""
}

type bulkRow struct {
	index       int
	slug, label string
	source      *catalog.Source
	sourceUUID  uuid.UUID
	names       []string // the row's classes, as listed
}

// BulkLoadSpellLists is POST /classes/spell-lists/bulk: a few queries for the whole file
// (spells, classes, existing links, one insert), not a few per row.
func (h *SpellListRoutes) BulkLoadSpellLists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	report, err := h.bulk(ctx, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, report.Finish())
}

func (h *SpellListRoutes) bulk(ctx context.Context, r *http.Request) (*table.BulkReport, error) {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	filename, docs, err := table.ReadUpload(r)
	if err != nil {
		return nil, err
	}
	sources, err := catalog.LoadSources(ctx, h.master)
	if err != nil {
		return nil, err
	}
	report := table.NewBulkReport(filename)
	rows := make([]*bulkRow, 0, len(docs))
	var sourceIDs []uuid.UUID
	var slugs []string
	for index, doc := range docs {
		slug, _ := table.TextValue(doc["slug"])
		row := &bulkRow{index: index, slug: slug}
		if row.source = sources.Find(doc); row.source == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("row %d [%s]: source not found", index, slug))
			continue
		}
		row.sourceUUID = uuid.MustParse(row.source.ID)
		row.label = fmt.Sprintf("%s (%s)", slug, row.source.Slug)
		if listed, ok := doc["classes"].([]any); ok {
			for _, item := range listed {
				if name, ok := item.(string); ok && strings.TrimSpace(name) != "" {
					row.names = append(row.names, name)
				}
			}
		}
		rows = append(rows, row)
		sourceIDs = append(sourceIDs, row.sourceUUID)
		slugs = append(slugs, slug)
	}

	tx, err := h.master.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	spells, err := q.SpellsByKeys(ctx, db.SpellsByKeysParams{SourceIds: sourceIDs, Slugs: slugs})
	if err != nil {
		return nil, err
	}
	type key struct {
		source uuid.UUID
		slug   string
	}
	bySlug := make(map[key]db.SpellsByKeysRow, len(spells))
	var spellIDs []uuid.UUID
	for _, s := range spells {
		bySlug[key{s.SourceID, s.Slug}] = s
		spellIDs = append(spellIDs, s.ID)
	}
	classRows, err := q.LiveClasses(ctx)
	if err != nil {
		return nil, err
	}
	classes := map[string][]uuid.UUID{} // lower(name) + year -> ids
	for _, c := range classRows {
		k := fmt.Sprintf("%s\x00%d", c.Name, c.DndVersionYear)
		classes[k] = append(classes[k], c.ID)
	}
	linkRows, err := q.LinksOfSpells(ctx, spellIDs)
	if err != nil {
		return nil, err
	}
	existing := map[uuid.UUID]map[uuid.UUID]bool{}
	for _, l := range linkRows {
		if existing[l.SpellID] == nil {
			existing[l.SpellID] = map[uuid.UUID]bool{}
		}
		existing[l.SpellID][l.ClassID] = true
	}

	add := db.AddLinksParams{UserID: userID}
	for _, row := range rows {
		spell, ok := bySlug[key{row.sourceUUID, row.slug}]
		switch {
		case !ok:
			report.Errors = append(report.Errors, fmt.Sprintf("row %d [%s]: spell not found (load spells first)", row.index, row.label))
			continue
		case spell.Deleted:
			report.Warnings = append(report.Warnings, row.label+": spell deleted, so not linked")
			continue
		}
		// class id -> (name as listed, qualifier); a plain listing beats a qualified one
		type want struct{ listed, qualifier string }
		wanted := map[uuid.UUID]want{}
		var order []uuid.UUID
		for _, listed := range row.names {
			className, qualifier := splitQualifier(listed)
			ids := classes[fmt.Sprintf("%s\x00%d", strings.ToLower(className), spell.DndVersionYear)]
			if len(ids) == 0 {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: no %d class named '%s'", row.label, spell.DndVersionYear, className))
			}
			for _, id := range ids {
				previous, seen := wanted[id]
				if !seen {
					order = append(order, id)
				}
				if !seen || previous.qualifier != "" {
					wanted[id] = want{listed, qualifier}
				}
			}
		}
		var addedNames []string
		for _, id := range order {
			if existing[spell.ID][id] {
				continue
			}
			add.SpellIds = append(add.SpellIds, spell.ID)
			add.ClassIds = append(add.ClassIds, id)
			add.Qualifiers = append(add.Qualifiers, wanted[id].qualifier)
			if !slices.Contains(addedNames, wanted[id].listed) {
				addedNames = append(addedNames, wanted[id].listed)
			}
		}
		slices.Sort(addedNames)
		switch {
		case len(addedNames) == 0:
			report.Unchanged = append(report.Unchanged, row.label)
		case len(existing[spell.ID]) > 0:
			report.Updated = append(report.Updated, row.label+": added "+strings.Join(addedNames, ", "))
		default:
			report.Created = append(report.Created, row.label+": "+strings.Join(addedNames, ", "))
		}
	}
	if len(report.Errors) > 0 {
		return report, nil // nothing saved
	}
	if len(add.SpellIds) > 0 {
		if err := q.AddLinks(ctx, add); err != nil {
			return nil, err
		}
	}
	return report, tx.Commit(ctx)
}
