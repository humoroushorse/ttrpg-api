// Package table reads and writes one catalog table from a description of its columns.
//
// Every catalog table (sources, spells, conditions, classes, ...) has the same routes: list with
// AG Grid filters and sparse fields, read by id or natural key, create, replace, patch, soft delete,
// and a bulk upsert. This package writes their SQL once, from a Table:
//
//   - Reads return JSON built by Postgres (json_build_object over the requested fields), so the API
//     writes rows through without scanning or re-encoding them, and ?fields= costs nothing extra.
//   - Writes send the row as one jsonb parameter that jsonb_populate_record turns into the table's
//     row type, so Postgres does the type conversion.
//   - A bulk upload is one statement: compare, insert and update every row of the file at once.
//
// Column names come from the Table, never from a request; request values are bind parameters.
package table

import (
	"fmt"
	"slices"
	"strings"

	"github.com/humoroushorse/go_dnd/internal/grid"
)

// Type is a column's Postgres type, as far as validation and JSON output care.
type Type int

const (
	Text      Type = iota + 1 // text / varchar
	Int                       // integer
	Bool                      // boolean
	TextArray                 // text[]
	JSON                      // jsonb (Shape says object or array)
	UUID                      // uuid
	Time                      // timestamptz (server-set only)
)

// Col describes one column.
type Col struct {
	Name     string
	Type     Type
	Nullable bool
	// Required: writes must include it. Otherwise a write that leaves it out gets Default.
	Required bool
	// Default is the value when a write leaves the column out. nil means the type's zero value
	// ("", 0, false, [], {}), or null when Nullable. It must match the database's server default.
	Default any
	// Server columns are set by the API (id, created_at, ...): returned, never written by clients.
	Server bool
	// Shape is "object" or "array" for JSON columns.
	Shape string
	// Grid makes the column filterable (and sortable unless NoSort) under its own name.
	Grid   grid.Kind
	NoSort bool
	// Aliases are other input keys for the column (the ETL's description_html for description).
	Aliases []string
	// Coerce adjusts an input value before its type is checked (a number for a text column).
	Coerce func(any) any
	// Check validates (and may normalize) a value after its type is checked: an enum, a range, a
	// JSON document's schema.
	Check func(any) (any, error)
}

// Table describes a table and how its routes talk about it.
type Table struct {
	Name  string // "dnd.feat"
	Label string // "feat", for messages
	Cols  []Col
	// Source: the table has source_id, and reads return a `source` object ({id, slug, name, name_short}).
	Source bool
	// Key is the natural key the bulk upload matches on: {"source_id", "slug"} or {"slug"}.
	Key []string
	// Order is the ORDER BY after the client's sort, which keeps pages stable ("t.name, t.id").
	Order string
	// Grid holds filterable columns beyond the Cols' own (set filters over joins, ...).
	Grid grid.Columns
	// UniqueMessage is the bulk error when a row breaks a unique constraint other than the key.
	UniqueMessage string

	cols     map[string]*Col
	inputs   map[string]string // input key (name or alias) -> column name
	writable []string          // client-writable columns, in Cols order
	compared []string          // writable columns minus the key: what a bulk update compares
	fields   []string          // response fields, in order
	gridCols grid.Columns
}

// Init checks the description and derives what the queries need. Call it once at startup.
func (t *Table) Init() *Table {
	t.cols = make(map[string]*Col, len(t.Cols))
	t.inputs = map[string]string{}
	t.gridCols = grid.Columns{}
	for i := range t.Cols {
		c := &t.Cols[i]
		if _, dup := t.cols[c.Name]; dup {
			panic(fmt.Sprintf("%s: column %s defined twice", t.Name, c.Name))
		}
		t.cols[c.Name] = c
		t.fields = append(t.fields, c.Name)
		if !c.Server {
			t.writable = append(t.writable, c.Name)
			t.inputs[c.Name] = c.Name
			for _, alias := range c.Aliases {
				t.inputs[alias] = c.Name
			}
			if !slices.Contains(t.Key, c.Name) {
				t.compared = append(t.compared, c.Name)
			}
		}
		if c.Grid != 0 {
			t.gridCols[c.Name] = grid.Column{SQL: "t." + quoteIdent(c.Name), Kind: c.Grid, NoSort: c.NoSort, SetType: setType(c.Type)}
		}
	}
	if t.Source {
		t.fields = append(t.fields, "source")
		t.gridCols["source"] = grid.Column{
			SQL: "t.source_id", Kind: grid.Set, NoSort: true,
			SetCondition: func(arg func(any) string, slugs []string) string {
				return fmt.Sprintf("s.slug = ANY(%s::text[])", arg(slugs))
			},
		}
	}
	for name, column := range t.Grid {
		t.gridCols[name] = column
	}
	for _, key := range t.Key {
		if _, ok := t.cols[key]; !ok {
			panic(fmt.Sprintf("%s: key column %s isn't a column", t.Name, key))
		}
	}
	return t
}

// GridColumns are what ?filter= and ?sort= accept.
func (t *Table) GridColumns() grid.Columns { return t.gridCols }

// Fields are the response fields, in order (what ?fields= accepts).
func (t *Table) Fields() []string { return t.fields }

func setType(typ Type) string {
	if typ == Int {
		return "int"
	}
	return "text"
}

// quoteIdent quotes a column name (a spell has "range"). Names come from Tables, never requests,
// and never contain quotes.
func quoteIdent(name string) string { return `"` + name + `"` }

// fieldExpr is the JSON value of one response field.
func (t *Table) fieldExpr(name string) string {
	if name == "source" {
		return "json_build_object('id', s.id, 'slug', s.slug, 'name', s.name, 'name_short', s.name_short)"
	}
	expr := "t." + quoteIdent(name)
	if t.cols[name].Type == Time {
		// the same shape py_dnd sends (2026-11-07T04:36:18.790238Z)
		return fmt.Sprintf(`to_char(%s AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')`, expr)
	}
	return expr
}

// jsonFor builds the JSON object of the given fields. json_build_object takes at most 100
// arguments, so wider objects are built from jsonb pieces (which don't keep key order).
func (t *Table) jsonFor(fields []string) string {
	const pairsPerCall = 50
	pairs := make([]string, len(fields))
	for i, f := range fields {
		pairs[i] = fmt.Sprintf("'%s', %s", f, t.fieldExpr(f))
	}
	if len(pairs) <= pairsPerCall {
		return "json_build_object(" + strings.Join(pairs, ", ") + ")"
	}
	var parts []string
	for start := 0; start < len(pairs); start += pairsPerCall {
		end := min(start+pairsPerCall, len(pairs))
		parts = append(parts, "jsonb_build_object("+strings.Join(pairs[start:end], ", ")+")")
	}
	return "(" + strings.Join(parts, " || ") + ")::json"
}

// from is the FROM clause every read shares: the table as t, and its source as s.
func (t *Table) from(relation string) string {
	if t.Source {
		return fmt.Sprintf("%s t JOIN dnd.source s ON s.id = t.source_id", relation)
	}
	return relation + " t"
}

func columnList(prefix string, names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = prefix + quoteIdent(n)
	}
	return strings.Join(quoted, ", ")
}
