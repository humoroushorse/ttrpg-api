// Package sources serves /api/v1/sources: the books every catalog entry points at.
package sources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/grid"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	"github.com/humoroushorse/go_dnd/internal/table"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

// Table is dnd.source.
var Table = (&table.Table{
	Name:  "dnd.source",
	Label: "source",
	Key:   []string{"slug"},
	Order: "t.name, t.dnd_version_year DESC, t.id",
	UniqueMessage: "Commit failed on a unique constraint (a name or short name another source has in the same " +
		"edition), nothing was saved (see server logs for the row).",
	Cols: []table.Col{
		{Name: "id", Type: table.UUID, Server: true},
		{Name: "slug", Type: table.Text, Required: true},
		{Name: "name", Type: table.Text, Required: true},
		{Name: "name_short", Type: table.Text, Required: true},
		{Name: "publish_year", Type: table.Int, Nullable: true},
		{Name: "dnd_version", Type: table.Text, Required: true},
		{Name: "dnd_version_year", Type: table.Int, Required: true},
		{Name: "marks", Type: table.TextArray},
		{Name: "created_at", Type: table.Time, Server: true},
		{Name: "created_by", Type: table.UUID, Server: true, Nullable: true},
		{Name: "updated_at", Type: table.Time, Server: true},
		{Name: "updated_by", Type: table.UUID, Server: true, Nullable: true},
	},
}).Init()

// dependents are the live entries that keep a source from being deleted, as the 409 names them.
var dependents = []struct{ table, label string }{
	{"dnd.spell", "spell(s)"},
	{"dnd.condition", "condition(s)"},
	{"dnd.glossary_entry", "glossary entry(s)"},
	{"dnd.species", "species"},
	{"dnd.dnd_class", "class(es)"},
	{"dnd.subclass", "subclass(es)"},
	{"dnd.background", "background(s)"},
	{"dnd.feat", "feat(s)"},
	{"dnd.item", "item(s)"},
}

// dependentsSQL counts every dependent table's live entries for $1 in one round trip.
var dependentsSQL = func() string {
	parts := make([]string, len(dependents))
	for i, d := range dependents {
		parts[i] = fmt.Sprintf("SELECT %d, count(*) FROM %s WHERE source_id = $1 AND deleted_at IS NULL", i, d.table)
	}
	return strings.Join(parts, " UNION ALL ")
}()

// SourceRoutes serves the source routes.
type SourceRoutes struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
}

// NewRoutes reads from replica and writes through master.
func NewRoutes(master, replica *pgxpool.Pool, users *identity.Resolver) *SourceRoutes {
	return &SourceRoutes{master: master, replica: replica, users: users}
}

func value[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// QuerySources is GET /sources/query: by name then newest edition.
func (h *SourceRoutes) QuerySources(w http.ResponseWriter, r *http.Request, params api.QuerySourcesParams) {
	fields, err := Table.ParseFields(value(params.Fields))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	limit := 100
	if params.Limit != nil {
		limit = *params.Limit
	}
	offset := value(params.Offset)
	if limit < 0 || offset < 0 {
		httpx.WriteError(w, r, httpx.Unprocessable("limit and offset can't be negative"))
		return
	}
	q := table.ListQuery{Fields: fields, Args: &grid.Args{}, Limit: limit, Offset: offset, Filters: map[string]any{}}
	if name := value(params.Name); name != "" {
		q.Where = append(q.Where, table.NameFilter(name, q.Args))
		q.Filters["name"] = name
	}
	body, err := Table.List(r.Context(), h.replica, q)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, body)
}

// ReadSourceBySlug is GET /sources/slug/{slug}.
func (h *SourceRoutes) ReadSourceBySlug(w http.ResponseWriter, r *http.Request, slug string, params api.ReadSourceBySlugParams) {
	h.readOne(w, r, value(params.Fields), "t.slug = $1", slug, "Source "+slug+" not found")
}

// ReadSource is GET /sources/{entity_id}.
func (h *SourceRoutes) ReadSource(w http.ResponseWriter, r *http.Request, id api.EntityId, params api.ReadSourceParams) {
	h.readOne(w, r, value(params.Fields), "t.id = $1", uuid.UUID(id), fmt.Sprintf("Source %s not found", id))
}

func (h *SourceRoutes) readOne(w http.ResponseWriter, r *http.Request, rawFields, where string, arg any, notFound string) {
	fields, err := Table.ParseFields(rawFields)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body, err := Table.Get(r.Context(), h.replica, fields, where, arg)
	if err == nil && body == nil {
		err = httpx.NotFound("%s", notFound)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteRaw(w, http.StatusOK, body)
}

// CreateSource is POST /sources.
func (h *SourceRoutes) CreateSource(w http.ResponseWriter, r *http.Request) {
	h.write(w, r, table.Full, func(ctx context.Context, record table.Record, userID uuid.UUID) ([]byte, error) {
		return Table.Insert(ctx, h.master, record, userID)
	})
}

// ReplaceSource is PUT /sources/{entity_id}: fields left out go back to their defaults.
func (h *SourceRoutes) ReplaceSource(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.write(w, r, table.Full, h.update(uuid.UUID(id)))
}

// UpdateSource is PATCH /sources/{entity_id}.
func (h *SourceRoutes) UpdateSource(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.write(w, r, table.Patch, h.update(uuid.UUID(id)))
}

func (h *SourceRoutes) update(id uuid.UUID) func(context.Context, table.Record, uuid.UUID) ([]byte, error) {
	return func(ctx context.Context, record table.Record, userID uuid.UUID) ([]byte, error) {
		body, err := Table.Update(ctx, h.master, id, record, userID)
		if err == nil && body == nil {
			err = httpx.NotFound("Source %s not found", id)
		}
		return body, err
	}
}

func (h *SourceRoutes) write(w http.ResponseWriter, r *http.Request, mode table.Mode,
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
	record, err := Table.Normalize(doc, mode, true)
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

// DeleteSource is DELETE /sources/{entity_id}: a soft delete, refused (409) while live entries use it.
func (h *SourceRoutes) DeleteSource(w http.ResponseWriter, r *http.Request, entityID api.EntityId) {
	ctx := r.Context()
	id := uuid.UUID(entityID)
	userID, err := h.users.CurrentUserID(ctx)
	if err == nil {
		err = h.delete(ctx, id, userID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	logging.FromContext(ctx).Info("deleted source", "source_id", id)
	httpx.WriteJSON(w, http.StatusOK, id.String())
}

func (h *SourceRoutes) delete(ctx context.Context, id, userID uuid.UUID) error {
	tx, err := h.master.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	var live bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM dnd.source WHERE id = $1 AND deleted_at IS NULL)", id).Scan(&live); err != nil {
		return err
	}
	if !live {
		return httpx.NotFound("Source %s not found", id)
	}
	rows, err := tx.Query(ctx, dependentsSQL, id)
	if err != nil {
		return err
	}
	var inUse []string
	for rows.Next() {
		var i int
		var count int64
		if err := rows.Scan(&i, &count); err != nil {
			rows.Close()
			return err
		}
		if count > 0 {
			inUse = append(inUse, fmt.Sprintf("%d %s", count, dependents[i].label))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(inUse) > 0 {
		return httpx.Conflict("Source %s has %s; delete them first", id, strings.Join(inUse, ", "))
	}
	if _, err := Table.SoftDelete(ctx, tx, id, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BulkLoadSources is POST /sources/bulk: upsert on slug (the ETL's source_id), all or nothing.
func (h *SourceRoutes) BulkLoadSources(w http.ResponseWriter, r *http.Request) {
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
	report := table.NewBulkReport(filename)
	rows := make([]table.BulkRow, 0, len(docs))
	seen := map[string]bool{}
	for index, doc := range docs {
		row, problem := prepareRow(index, doc)
		switch {
		case problem != "":
			report.Errors = append(report.Errors, problem)
		case Table.SeenKey(seen, row.Record):
			report.Errors = append(report.Errors, fmt.Sprintf("row %d [%s]: same slug as an earlier row", index, row.Label))
		default:
			rows = append(rows, row)
		}
	}
	if len(report.Errors) == 0 {
		if err := Table.Upsert(ctx, h.master, rows, userID, report); err != nil {
			if !errors.Is(err, table.ErrUnique) {
				httpx.WriteError(w, r, err)
				return
			}
			logging.FromContext(ctx).Error("bulk source upload broke a unique constraint", "error", err)
			*report = *table.NewBulkReport(filename)
			report.Errors = append(report.Errors, Table.UniqueMessage)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, report.Finish())
}

// prepareRow validates one row of sources_<year>.json. The ETL calls the slug source_id.
func prepareRow(index int, doc map[string]any) (table.BulkRow, string) {
	label := fmt.Sprintf("row %d [%s]", index, firstText(doc, "source_id", "slug", "name"))
	if _, hasSlug := doc["slug"]; !hasSlug {
		if sourceID, ok := doc["source_id"]; ok {
			doc["slug"] = sourceID
		}
	}
	delete(doc, "source_id")
	record, err := Table.Normalize(doc, table.Full, false)
	if err != nil {
		return table.BulkRow{}, label + ": " + err.Error()
	}
	return table.BulkRow{Index: index, Label: fmt.Sprintf("%s (%s)", record["slug"], record["name"]), Record: record}, ""
}

func firstText(doc map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := table.TextValue(doc[key]); ok && text != "" {
			return text
		}
	}
	return "?"
}
