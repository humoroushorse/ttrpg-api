package table

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/grid"
	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// ParseFields reads ?fields= (comma-separated). Empty means every field; id always comes first;
// unknown names are a 422.
func (t *Table) ParseFields(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return t.fields, nil
	}
	fields := []string{"id"}
	var unknown []string
	for _, f := range strings.Split(raw, ",") {
		f = strings.TrimSpace(f)
		switch {
		case f == "" || slices.Contains(fields, f):
		case slices.Contains(t.fields, f):
			fields = append(fields, f)
		default:
			unknown = append(unknown, f)
		}
	}
	if len(unknown) > 0 {
		allowed := slices.Clone(t.fields)
		slices.Sort(allowed)
		slices.Sort(unknown)
		return nil, httpx.Unprocessable("Unknown field(s): %s. Allowed: %s", strings.Join(unknown, ", "), strings.Join(allowed, ", "))
	}
	return fields, nil
}

// ListQuery is one page of a list read.
type ListQuery struct {
	Fields []string
	// Args holds the values Grid and Where bind; List adds its own after them.
	Args *grid.Args
	Grid grid.Query
	// Where are more conditions (?name=, versions_of, ...), ANDed with Grid's.
	Where []string
	// Limit 0 means every row.
	Limit, Offset int
	// Filters is echoed back in the response (the plain query params, not filter/sort/fields).
	Filters map[string]any
}

// NameFilter is ?name=: any of the comma-separated names, case-insensitive substring.
func NameFilter(raw string, args *grid.Args) string {
	var conditions []string
	for _, name := range strings.Split(raw, ",") {
		if name = strings.TrimSpace(name); name != "" {
			conditions = append(conditions, fmt.Sprintf(`t.name ILIKE %s ESCAPE '\'`, args.Add("%"+likeEscape(name)+"%")))
		}
	}
	if len(conditions) == 0 {
		return "TRUE"
	}
	return "(" + strings.Join(conditions, " OR ") + ")"
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func likeEscape(s string) string { return likeEscaper.Replace(s) }

// List writes one page as the list response: total_entities_count, limit, offset, filters,
// entities_count, entities.
func (t *Table) List(ctx context.Context, db *pgxpool.Pool, q ListQuery) ([]byte, error) {
	entities, total, err := t.Page(ctx, db, q)
	if err != nil {
		return nil, err
	}
	return listBody(total, q, entities)
}

// Page is one page of entities (JSON each) and how many rows match in all. One round trip:
// count(*) OVER () counts every match beside the page.
func (t *Table) Page(ctx context.Context, db *pgxpool.Pool, q ListQuery) ([][]byte, int64, error) {
	where := append([]string{"t.deleted_at IS NULL"}, q.Grid.Where...)
	where = append(where, q.Where...)
	order := append(slices.Clone(q.Grid.OrderBy), t.Order)
	sql := fmt.Sprintf("SELECT %s, count(*) OVER () FROM %s WHERE %s ORDER BY %s",
		t.jsonFor(q.Fields), t.from(t.Name), strings.Join(where, " AND "), strings.Join(order, ", "))
	if q.Limit > 0 {
		sql += " LIMIT " + q.Args.Add(q.Limit)
	}
	if q.Offset > 0 {
		sql += " OFFSET " + q.Args.Add(q.Offset)
	}
	rows, err := db.Query(ctx, sql, q.Args.Values...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var entities [][]byte
	var total int64
	for rows.Next() {
		var entity []byte
		if err := rows.Scan(&entity, &total); err != nil {
			return nil, 0, err
		}
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	// a page past the end has no rows to carry the count
	if len(entities) == 0 && q.Offset > 0 {
		countSQL := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", t.from(t.Name), strings.Join(where, " AND "))
		if err := db.QueryRow(ctx, countSQL, q.Args.Values[:countArgs(q)]...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	return entities, total, nil
}

// countArgs is how many args the WHERE uses (List appends limit and offset after them).
func countArgs(q ListQuery) int {
	n := len(q.Args.Values)
	if q.Limit > 0 {
		n--
	}
	if q.Offset > 0 {
		n--
	}
	return n
}

func listBody(total int64, q ListQuery, entities [][]byte) ([]byte, error) {
	filters := q.Filters
	if filters == nil {
		filters = map[string]any{}
	}
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(`{"total_entities_count":`)
	b.WriteString(strconv.FormatInt(total, 10))
	b.WriteString(`,"limit":`)
	b.WriteString(strconv.Itoa(q.Limit))
	b.WriteString(`,"offset":`)
	b.WriteString(strconv.Itoa(q.Offset))
	b.WriteString(`,"filters":`)
	b.Write(filtersJSON)
	b.WriteString(`,"entities_count":`)
	b.WriteString(strconv.Itoa(len(entities)))
	b.WriteString(`,"entities":[`)
	for i, e := range entities {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(e)
	}
	b.WriteString("]}")
	return b.Bytes(), nil
}

// Get is one live row as JSON, or nil when there is none. where uses t (and s) and args' placeholders.
func (t *Table) Get(ctx context.Context, db *pgxpool.Pool, fields []string, where string, args ...any) ([]byte, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE t.deleted_at IS NULL AND %s LIMIT 1", t.jsonFor(fields), t.from(t.Name), where)
	var body []byte
	err := db.QueryRow(ctx, sql, args...).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return body, err
}
