package table

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// BulkReport is what a bulk upload answers with (the same shape py_dnd sends).
type BulkReport struct {
	Filename string `json:"filename"`
	Totals   struct {
		Created   int `json:"created"`
		Updated   int `json:"updated"`
		Unchanged int `json:"unchanged"`
		Errored   int `json:"errored"`
		Warning   int `json:"warning"`
	} `json:"totals"`
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Unchanged []string `json:"unchanged"`
	Errors    []string `json:"errors"`
	Warnings  []string `json:"warnings"`
}

// NewBulkReport starts a report with empty lists (they're [] in JSON, not null).
func NewBulkReport(filename string) *BulkReport {
	return &BulkReport{Filename: filename, Created: []string{}, Updated: []string{}, Unchanged: []string{}, Errors: []string{}, Warnings: []string{}}
}

// Finish fills in the totals.
func (r *BulkReport) Finish() *BulkReport {
	r.Totals.Created, r.Totals.Updated, r.Totals.Unchanged = len(r.Created), len(r.Updated), len(r.Unchanged)
	r.Totals.Errored, r.Totals.Warning = len(r.Errors), len(r.Warnings)
	return r
}

// BulkRow is one validated row of an upload.
type BulkRow struct {
	Index  int    // the row number in the file, for messages
	Label  string // how the report names it: "acid-splash (phb_2024)"
	Record Record // a Full record
}

// SeenKey reports whether an earlier row in the file had the same natural key, and remembers this one.
func (t *Table) SeenKey(seen map[string]bool, r Record) bool {
	key := t.keyOf(r)
	if seen[key] {
		return true
	}
	seen[key] = true
	return false
}

// Upsert writes every row in one statement, matched on the natural key: new rows are inserted,
// live rows whose values differ are updated, and deleted rows are left alone (a warning). The
// report gets one line per row. Nothing is written when the statement fails.
func (t *Table) Upsert(ctx context.Context, db Querier, rows []BulkRow, userID uuid.UUID, report *BulkReport) error {
	if len(rows) == 0 {
		return nil
	}
	docs := make([]Record, len(rows))
	for i, row := range rows {
		docs[i] = row.Record
	}
	payload, err := json.Marshal(docs)
	if err != nil {
		return err
	}
	results, err := db.Query(ctx, t.upsertSQL(), string(payload), userID)
	if err != nil {
		return t.bulkError(err)
	}
	defer results.Close()
	for results.Next() {
		var ord int
		var created, deleted bool
		var changed []string
		if err := results.Scan(&ord, &created, &deleted, &changed); err != nil {
			return err
		}
		label := rows[ord-1].Label
		switch {
		case created:
			report.Created = append(report.Created, label)
		case deleted:
			report.Warnings = append(report.Warnings, label+": deleted, so not restored or updated")
		case len(changed) == 0:
			report.Unchanged = append(report.Unchanged, label)
		default:
			slices.Sort(changed)
			report.Updated = append(report.Updated, label+": "+strings.Join(changed, ", "))
		}
	}
	if err := results.Err(); err != nil {
		return t.bulkError(err)
	}
	return nil
}

// ErrUnique is a bulk upload that broke a unique constraint other than the natural key.
var ErrUnique = errors.New("unique constraint")

func (t *Table) bulkError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s (%s)", ErrUnique, pgErr.ConstraintName, pgErr.Detail)
	}
	return err
}

// upsertSQL compares, inserts and updates the whole file in one statement.
//
//	incoming: the file's rows as the table's row type, numbered in file order
//	cmp:      each row's match on the natural key and which compared columns differ
//	inserted: rows with no match
//	updated:  live matches with differences
//
// It returns one line per row: (ord, created, deleted, changed columns).
func (t *Table) upsertSQL() string {
	writable := t.writable
	var match, diffs []string
	for _, k := range t.Key {
		match = append(match, fmt.Sprintf("x.%[1]s = i.%[1]s", quoteIdent(k)))
	}
	for _, c := range t.compared {
		left, right := "x."+quoteIdent(c), "i."+quoteIdent(c)
		if t.cols[c].Type == JSON {
			// py_dnd (SQLAlchemy) stores None in a jsonb column as JSON null, we store SQL NULL; same value
			left, right = fmt.Sprintf("NULLIF(%s, 'null'::jsonb)", left), fmt.Sprintf("NULLIF(%s, 'null'::jsonb)", right)
		}
		diffs = append(diffs, fmt.Sprintf("CASE WHEN %s IS DISTINCT FROM %s THEN '%s' END", left, right, c))
	}
	return fmt.Sprintf(`WITH incoming AS (
    SELECT e.ord::int AS ord, r.*
    FROM jsonb_array_elements($1::jsonb) WITH ORDINALITY AS e(doc, ord)
    CROSS JOIN LATERAL jsonb_populate_record(NULL::%[1]s, e.doc) AS r
), cmp AS (
    SELECT i.ord, x.id AS existing_id, x.deleted_at IS NOT NULL AS deleted,
           array_remove(ARRAY[%[2]s]::text[], NULL) AS changed
    FROM incoming i
    LEFT JOIN %[1]s x ON %[3]s
), inserted AS (
    INSERT INTO %[1]s (%[4]s, created_by, updated_by)
    SELECT %[5]s, $2::uuid, $2::uuid FROM incoming i JOIN cmp c ON c.ord = i.ord WHERE c.existing_id IS NULL
    RETURNING 1
), updated AS (
    UPDATE %[1]s AS x SET (%[4]s, updated_at, updated_by) = (%[5]s, now(), $2::uuid)
    FROM incoming i JOIN cmp c ON c.ord = i.ord
    WHERE x.id = c.existing_id AND NOT c.deleted AND cardinality(c.changed) > 0
    RETURNING 1
)
SELECT c.ord, c.existing_id IS NULL, c.deleted, c.changed FROM cmp c ORDER BY c.ord`,
		t.Name, strings.Join(diffs, ", "), strings.Join(match, " AND "), columnList("", writable), columnList("i.", writable))
}
