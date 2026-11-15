// Package catalog serves the routes every catalog entity shares (spells, conditions, classes, ...):
//
//	GET    /query                  list: ?filter= ?sort= (AG Grid models), ?fields=, ?name=, limit/offset
//	GET    /{id}                   one entry
//	GET    /{source_slug}/{slug}   one entry by its URL key (phb_2024/acid-splash)
//	POST   ""                      create
//	PUT    /{id}                   replace (fields left out go back to their defaults)
//	PATCH  /{id}                   change some fields
//	DELETE /{id}                   soft delete
//	POST   /bulk                   create or update from an ETL file, keyed on (source, slug)
//
// Each entity's generated route methods call these with its Entity.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/grid"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	"github.com/humoroushorse/go_dnd/internal/table"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

// Entity is one catalog table and how its bulk upload reads rows.
type Entity struct {
	Table *table.Table
	// BareSlug strips the ETL's type prefix from slugs ("condition:blinded" -> "blinded").
	BareSlug bool
}

// Handler holds what every entity's routes need.
type Handler struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
}

// NewHandler reads from replica and writes through master.
func NewHandler(master, replica *pgxpool.Pool, users *identity.Resolver) *Handler {
	return &Handler{master: master, replica: replica, users: users}
}

// Params are the list query params every entity takes.
type Params struct {
	Name, Fields, Filter, Sort *string
	Limit, Offset              *int
}

// Condition is an entity-specific list param that becomes a WHERE condition (a spell's versions_of).
type Condition struct {
	Name  string // the query param, echoed in `filters`
	Value *string
	// Where returns the condition, binding values through arg.
	Where func(arg func(any) string, value string) string
}

func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// ListQuery reads the shared list params into a query against e's table.
func (e Entity) ListQuery(p Params, conditions ...Condition) (table.ListQuery, error) {
	t := e.Table
	fields, err := t.ParseFields(value(p.Fields))
	if err != nil {
		return table.ListQuery{}, err
	}
	limit := 100
	if p.Limit != nil {
		limit = *p.Limit
	}
	offset := value(p.Offset)
	if limit < 0 || offset < 0 {
		return table.ListQuery{}, httpx.Unprocessable("limit and offset can't be negative")
	}
	args := &grid.Args{}
	gridQuery, err := grid.Parse(t.GridColumns(), value(p.Filter), value(p.Sort), args)
	if err != nil {
		return table.ListQuery{}, err
	}
	q := table.ListQuery{Fields: fields, Args: args, Grid: gridQuery, Limit: limit, Offset: offset, Filters: map[string]any{}}
	if name := value(p.Name); name != "" {
		q.Where = append(q.Where, table.NameFilter(name, args))
		q.Filters["name"] = name
	}
	for _, c := range conditions {
		if v := value(c.Value); v != "" {
			q.Where = append(q.Where, c.Where(args.Add, v))
			q.Filters[c.Name] = v
		}
	}
	return q, nil
}

// Query is GET /query.
func (h *Handler) Query(w http.ResponseWriter, r *http.Request, e Entity, p Params, conditions ...Condition) {
	q, err := e.ListQuery(p, conditions...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body, err := e.Table.List(r.Context(), h.replica, q)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, body)
}

// Array is a plain JSON array of one page (the spells' original GET "" before /query existed).
func (h *Handler) Array(w http.ResponseWriter, r *http.Request, e Entity, p Params) {
	q, err := e.ListQuery(Params{Fields: p.Fields, Limit: p.Limit, Offset: p.Offset})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q.Grid.OrderBy = []string{"t.id"}
	entities, _, err := e.Table.Page(r.Context(), h.replica, q)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, append([]byte("["), append(joinJSON(entities), ']')...))
}

func joinJSON(items [][]byte) []byte {
	var out []byte
	for i, item := range items {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, item...)
	}
	return out
}

// Read is GET /{id}.
func (h *Handler) Read(w http.ResponseWriter, r *http.Request, e Entity, id uuid.UUID, fields *string) {
	h.readOne(w, r, e, fields, fmt.Sprintf("%s %s not found", capitalize(e.Table.Label), id), "t.id = $1", id)
}

// ReadBySlugs is GET /{source_slug}/{slug}.
func (h *Handler) ReadBySlugs(w http.ResponseWriter, r *http.Request, e Entity, sourceSlug, slug string, fields *string) {
	h.readOne(w, r, e, fields, fmt.Sprintf("%s %s/%s not found", capitalize(e.Table.Label), sourceSlug, slug),
		"s.slug = $1 AND t.slug = $2", sourceSlug, slug)
}

func (h *Handler) readOne(w http.ResponseWriter, r *http.Request, e Entity, rawFields *string, notFound, where string, args ...any) {
	fields, err := e.Table.ParseFields(value(rawFields))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body, err := e.Table.Get(r.Context(), h.replica, fields, where, args...)
	if err == nil && body == nil {
		err = httpx.NotFound("%s", notFound)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, body)
}

// Create is POST "".
func (h *Handler) Create(w http.ResponseWriter, r *http.Request, e Entity) {
	h.write(w, r, e, table.Full, func(ctx context.Context, record table.Record, userID uuid.UUID) ([]byte, error) {
		return e.Table.Insert(ctx, h.master, record, userID)
	})
}

// Replace is PUT /{id}.
func (h *Handler) Replace(w http.ResponseWriter, r *http.Request, e Entity, id uuid.UUID) {
	h.write(w, r, e, table.Full, h.update(e, id))
}

// Patch is PATCH /{id}.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request, e Entity, id uuid.UUID) {
	h.write(w, r, e, table.Patch, h.update(e, id))
}

func (h *Handler) update(e Entity, id uuid.UUID) func(context.Context, table.Record, uuid.UUID) ([]byte, error) {
	return func(ctx context.Context, record table.Record, userID uuid.UUID) ([]byte, error) {
		body, err := e.Table.Update(ctx, h.master, id, record, userID)
		if err == nil && body == nil {
			err = httpx.NotFound("%s %s not found", capitalize(e.Table.Label), id)
		}
		return body, err
	}
}

func (h *Handler) write(w http.ResponseWriter, r *http.Request, e Entity, mode table.Mode,
	save func(context.Context, table.Record, uuid.UUID) ([]byte, error)) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	doc, err := table.BodyDoc(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	record, err := e.Table.Normalize(doc, mode, true)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body, err := save(ctx, record, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, body)
}

// Delete is DELETE /{id}: a soft delete. Returns the id.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request, e Entity, id uuid.UUID) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deleted, err := e.Table.SoftDelete(ctx, h.master, id, userID)
	if err == nil && !deleted {
		err = httpx.NotFound("%s %s not found", capitalize(e.Table.Label), id)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	logging.FromContext(ctx).Info("deleted "+e.Table.Label, "id", id)
	httpx.WriteJSON(w, http.StatusOK, id.String())
}

// Bulk is POST /bulk: upsert on (source, slug), all or nothing. Each row's source is found by the
// ETL's source_id (a slug), source_name + edition, or short name.
func (h *Handler) Bulk(w http.ResponseWriter, r *http.Request, e Entity) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	filename, docs, err := table.ReadUpload(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sources, err := LoadSources(ctx, h.master)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	report := table.NewBulkReport(filename)
	rows := make([]table.BulkRow, 0, len(docs))
	seen := map[string]bool{}
	for index, doc := range docs {
		row, problem := e.prepareRow(index, doc, sources)
		switch {
		case problem != "":
			report.Errors = append(report.Errors, problem)
		case e.Table.SeenKey(seen, row.Record):
			report.Errors = append(report.Errors, fmt.Sprintf("row %d [%s]: same slug as an earlier row", index, row.Label))
		default:
			rows = append(rows, row)
		}
	}
	if len(report.Errors) == 0 {
		if err := e.Table.Upsert(ctx, h.master, rows, userID, report); err != nil {
			if !errors.Is(err, table.ErrUnique) {
				httpx.WriteError(w, r, err)
				return
			}
			logging.FromContext(ctx).Error("bulk upload broke a unique constraint", "table", e.Table.Name, "error", err)
			*report = *table.NewBulkReport(filename)
			report.Errors = append(report.Errors, "Commit failed on a unique constraint, nothing was saved (see server logs for the row).")
		}
	}
	logging.FromContext(ctx).Info("bulk "+e.Table.Label+" upload", "file", filename, "rows", len(docs), "errors", len(report.Errors))
	httpx.WriteJSON(w, http.StatusOK, report.Finish())
}

// prepareRow matches one ETL row to its source and validates it.
func (e Entity) prepareRow(index int, doc map[string]any, sources *SourceIndex) (table.BulkRow, string) {
	slug, _ := table.TextValue(doc["slug"])
	label := slug
	if label == "" {
		if label, _ = table.TextValue(doc["name"]); label == "" {
			label = "?"
		}
	}
	label = fmt.Sprintf("row %d [%s]", index, label)
	source := sources.Find(doc)
	if source == nil {
		missing, _ := table.TextValue(doc["source_name"])
		if missing == "" {
			missing, _ = table.TextValue(doc["source_id"])
		}
		return table.BulkRow{}, fmt.Sprintf("%s: source '%s' not found", label, missing)
	}
	if e.BareSlug && slug != "" {
		_, after, found := strings.Cut(slug, ":")
		if found {
			doc["slug"] = after
		}
	}
	doc["source_id"] = source.ID
	if _, ok := doc["dnd_version"]; !ok {
		doc["dnd_version"] = source.DndVersion
	}
	if _, ok := doc["dnd_version_year"]; !ok {
		doc["dnd_version_year"] = json.Number(strconv.FormatInt(source.DndVersionYear, 10))
	}
	record, err := e.Table.Normalize(doc, table.Full, false)
	if err != nil {
		return table.BulkRow{}, label + ": " + err.Error()
	}
	return table.BulkRow{Index: index, Label: fmt.Sprintf("%s (%s)", record["slug"], source.Slug), Record: record}, ""
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
