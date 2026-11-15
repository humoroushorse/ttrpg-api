// Package grid turns AG Grid's filter and sort models (`?filter=`, `?sort=`, JSON in the query
// string) into SQL conditions and an ORDER BY.
//
//	?filter={"level":{"filterType":"number","type":"lessThanOrEqual","filter":3}}
//	?sort=[{"colId":"level","sort":"asc"}]
//
// Each route lists the columns a client may filter and sort, and a column's kind fixes the filter
// it takes. Column ids are looked up in that list and never reach SQL; values are always bind
// parameters. Anything else is a 422. One condition per column: AG Grid's combined model
// (`operator` + `conditions`) is rejected, as in py_dnd.
package grid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Kind is what a column holds, which fixes the filter it takes.
type Kind int

const (
	Text    Kind = iota + 1 // agTextColumnFilter, case-insensitive
	Number                  // agNumberColumnFilter; inRange includes both ends
	Date                    // agDateColumnFilter on a timestamp, compared by day
	Year                    // agDateColumnFilter on an integer year column, compared by year
	Boolean                 // appBooleanColumnFilter
	Set                     // appSetColumnFilter / appApiSetColumnFilter: {"values": [...]}
)

var filterTypes = map[Kind]string{
	Text: "text", Number: "number", Date: "date", Year: "date", Boolean: "boolean", Set: "set",
}

// Column is one column clients may filter and sort.
type Column struct {
	// SQL is the expression to filter and sort on, e.g. "t.level". Written by us, never by clients.
	SQL  string
	Kind Kind
	// NoSort rejects ?sort= on this column.
	NoSort bool
	// SetType is the SQL element type set values are sent as ("text" by default, "int").
	SetType string
	// SetCondition replaces `SQL = ANY(values)` for set columns that need a join or a subquery.
	// arg binds the values (never empty, nulls removed) and returns the placeholder.
	SetCondition func(arg func(any) string, values []string) string
}

// Columns maps AG Grid column ids to columns.
type Columns map[string]Column

// Args collects bind parameters for one statement and hands out their placeholders.
type Args struct{ Values []any }

// Add binds v and returns its placeholder ($1, $2, ...).
func (a *Args) Add(v any) string {
	a.Values = append(a.Values, v)
	return "$" + strconv.Itoa(len(a.Values))
}

// Query is the parsed ?filter= and ?sort=: conditions to AND into WHERE, and ORDER BY terms.
type Query struct {
	Where   []string
	OrderBy []string
}

type sortItem struct {
	ColID string `json:"colId"`
	Sort  string `json:"sort"`
}

// filterModel holds every field any filter type has; check() rejects the ones its type doesn't take.
type filterModel struct {
	FilterType string            `json:"filterType"`
	Type       string            `json:"type"`
	Filter     json.RawMessage   `json:"filter"`
	FilterTo   json.RawMessage   `json:"filterTo"`
	DateFrom   *string           `json:"dateFrom"`
	DateTo     *string           `json:"dateTo"`
	Values     []json.RawMessage `json:"values"`
}

// fields each filterType may carry besides filterType and type
var allowedFields = map[string][]string{
	"text":    {"filter"},
	"number":  {"filter", "filterTo"},
	"date":    {"dateFrom", "dateTo"},
	"boolean": {},
	"set":     {"values"},
}

var operators = map[string][]string{
	"text":    {"contains", "notContains", "equals", "notEqual", "startsWith", "endsWith", "blank", "notBlank"},
	"number":  {"equals", "notEqual", "lessThan", "lessThanOrEqual", "greaterThan", "greaterThanOrEqual", "inRange", "blank", "notBlank"},
	"date":    {"equals", "notEqual", "lessThan", "greaterThan", "inRange", "blank", "notBlank"},
	"boolean": {"true", "false", "blank", "notBlank"},
}

var comparisons = map[string]string{
	"equals": "=", "notEqual": "<>", "lessThan": "<", "lessThanOrEqual": "<=", "greaterThan": ">", "greaterThanOrEqual": ">=",
}

// Parse checks filterJSON and sortJSON (either may be empty) against columns and binds their values into args.
func Parse(columns Columns, filterJSON, sortJSON string, args *Args) (Query, error) {
	var q Query
	if filterJSON != "" {
		var model map[string]json.RawMessage
		if err := decodeStrict(filterJSON, &model); err != nil {
			return q, httpx.Unprocessable("?filter= isn't a JSON object: %v", err)
		}
		// sorted, so the same filter always builds the same SQL (and reuses its prepared statement)
		ids := make([]string, 0, len(model))
		for id := range model {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			column, ok := columns[id]
			if !ok {
				return q, httpx.Unprocessable("Can't filter on '%s'. Filterable: %s", id, strings.Join(columns.names(false), ", "))
			}
			condition, err := column.condition(id, model[id], args)
			if err != nil {
				return q, err
			}
			q.Where = append(q.Where, condition)
		}
	}
	if sortJSON != "" {
		var items []sortItem
		if err := decodeStrict(sortJSON, &items); err != nil {
			return q, httpx.Unprocessable("?sort= isn't a JSON list of {colId, sort}: %v", err)
		}
		for i, item := range items {
			column, ok := columns[item.ColID]
			if !ok || column.NoSort {
				return q, httpx.Unprocessable("Can't sort on '%s'. Sortable: %s", item.ColID, strings.Join(columns.names(true), ", "))
			}
			switch item.Sort {
			case "asc":
				q.OrderBy = append(q.OrderBy, column.SQL+" ASC NULLS LAST")
			case "desc":
				q.OrderBy = append(q.OrderBy, column.SQL+" DESC NULLS LAST")
			default:
				return q, httpx.Unprocessable("?sort= is invalid: %d.sort must be 'asc' or 'desc'", i)
			}
		}
	}
	return q, nil
}

func (c Columns) names(sortableOnly bool) []string {
	names := make([]string, 0, len(c))
	for name, column := range c {
		if !sortableOnly || !column.NoSort {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func decodeStrict(raw string, into any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(into)
}

func (c Column) condition(id string, raw json.RawMessage, args *Args) (string, error) {
	var model filterModel
	if err := decodeStrict(string(raw), &model); err != nil {
		return "", httpx.Unprocessable("?filter= is invalid: %s: %v", id, err)
	}
	expected := filterTypes[c.Kind]
	if model.FilterType != expected {
		return "", httpx.Unprocessable("Column '%s' takes a %s filter, not %s", id, expected, model.FilterType)
	}
	if err := model.check(id, raw); err != nil {
		return "", err
	}
	switch c.Kind {
	case Text:
		return c.text(id, model, args)
	case Number:
		return c.number(id, model, args)
	case Date, Year:
		return c.date(id, model, args)
	case Boolean:
		return c.boolean(model), nil
	default:
		return c.set(id, model, args)
	}
}

// check rejects fields this filterType doesn't take and operators it doesn't have.
func (m filterModel) check(id string, raw json.RawMessage) error {
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &keys)
	allowed := allowedFields[m.FilterType]
	for key := range keys {
		if key != "filterType" && key != "type" && !slices.Contains(allowed, key) {
			return httpx.Unprocessable("?filter= is invalid: %s.%s: extra field not allowed for a %s filter", id, key, m.FilterType)
		}
	}
	if ops, ok := operators[m.FilterType]; ok && !slices.Contains(ops, m.Type) {
		return httpx.Unprocessable("?filter= is invalid: %s.type: must be one of %s", id, strings.Join(ops, ", "))
	}
	return nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (c Column) text(id string, m filterModel, args *Args) (string, error) {
	switch m.Type {
	case "blank":
		return fmt.Sprintf("(%s IS NULL OR %s = '')", c.SQL, c.SQL), nil
	case "notBlank":
		return fmt.Sprintf("(%s IS NOT NULL AND %s <> '')", c.SQL, c.SQL), nil
	}
	var value string
	if len(m.Filter) > 0 && !bytes.Equal(m.Filter, []byte("null")) {
		if err := json.Unmarshal(m.Filter, &value); err != nil {
			return "", httpx.Unprocessable("?filter= is invalid: %s.filter must be a string", id)
		}
	}
	if value == "" {
		return "", httpx.Unprocessable("'%s' needs a filter value", m.Type)
	}
	escaped := likeEscaper.Replace(value)
	switch m.Type {
	case "contains":
		return fmt.Sprintf(`%s ILIKE %s ESCAPE '\'`, c.SQL, args.Add("%"+escaped+"%")), nil
	case "notContains":
		return fmt.Sprintf(`%s NOT ILIKE %s ESCAPE '\'`, c.SQL, args.Add("%"+escaped+"%")), nil
	case "startsWith":
		return fmt.Sprintf(`%s ILIKE %s ESCAPE '\'`, c.SQL, args.Add(escaped+"%")), nil
	case "endsWith":
		return fmt.Sprintf(`%s ILIKE %s ESCAPE '\'`, c.SQL, args.Add("%"+escaped)), nil
	case "equals":
		return fmt.Sprintf("lower(%s) = lower(%s)", c.SQL, args.Add(value)), nil
	default: // notEqual
		return fmt.Sprintf("lower(%s) <> lower(%s)", c.SQL, args.Add(value)), nil
	}
}

// numberArg binds a filter number: whole numbers as bigint (so an int column's index still applies),
// others as numeric.
func numberArg(raw json.RawMessage, args *Args) (string, float64, bool, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", 0, false, nil
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", 0, false, err
	}
	if value == math.Trunc(value) && math.Abs(value) < 1<<53 {
		return args.Add(int64(value)) + "::bigint", value, true, nil
	}
	return args.Add(strconv.FormatFloat(value, 'f', -1, 64)) + "::numeric", value, true, nil
}

func (c Column) number(id string, m filterModel, args *Args) (string, error) {
	switch m.Type {
	case "blank":
		return c.SQL + " IS NULL", nil
	case "notBlank":
		return c.SQL + " IS NOT NULL", nil
	}
	low, lowValue, ok, err := numberArg(m.Filter, args)
	if err != nil {
		return "", httpx.Unprocessable("?filter= is invalid: %s.filter must be a number", id)
	}
	if !ok {
		return "", httpx.Unprocessable("'%s' needs a filter value", m.Type)
	}
	if m.Type != "inRange" {
		return fmt.Sprintf("%s %s %s", c.SQL, comparisons[m.Type], low), nil
	}
	high, highValue, ok, err := numberArg(m.FilterTo, args)
	if err != nil {
		return "", httpx.Unprocessable("?filter= is invalid: %s.filterTo must be a number", id)
	}
	if !ok {
		return "", httpx.Unprocessable("'inRange' needs both ends")
	}
	if highValue < lowValue {
		low, high = high, low
	}
	return fmt.Sprintf("%s BETWEEN %s AND %s", c.SQL, low, high), nil
}

// day reads AG Grid's "YYYY-MM-DD" (it may add " hh:mm:ss", which is ignored).
func day(value *string) (time.Time, bool, error) {
	if value == nil {
		return time.Time{}, false, nil
	}
	text := *value
	if len(text) > 10 {
		text = text[:10]
	}
	parsed, err := time.Parse(time.DateOnly, text)
	if err != nil {
		return time.Time{}, false, httpx.Unprocessable("'%s' isn't a date (YYYY-MM-DD)", *value)
	}
	return parsed, true, nil
}

func (c Column) date(id string, m filterModel, args *Args) (string, error) {
	switch m.Type {
	case "blank":
		return c.SQL + " IS NULL", nil
	case "notBlank":
		return c.SQL + " IS NOT NULL", nil
	}
	low, ok, err := day(m.DateFrom)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", httpx.Unprocessable("'%s' needs dateFrom", m.Type)
	}
	high, hasHigh, err := day(m.DateTo)
	if err != nil {
		return "", err
	}
	if hasHigh && high.Before(low) {
		low, high = high, low
	}
	bind := func(t time.Time) string {
		if c.Kind == Year { // an integer year column: compare the years
			return args.Add(int64(t.Year())) + "::bigint"
		}
		return args.Add(t.Format(time.DateOnly)) + "::date"
	}
	expression := c.SQL
	if c.Kind == Date {
		expression = fmt.Sprintf("(%s)::date", c.SQL)
	}
	if m.Type != "inRange" {
		return fmt.Sprintf("%s %s %s", expression, comparisons[m.Type], bind(low)), nil
	}
	if !hasHigh {
		return "", httpx.Unprocessable("'inRange' needs both ends")
	}
	return fmt.Sprintf("%s BETWEEN %s AND %s", expression, bind(low), bind(high)), nil
}

func (c Column) boolean(m filterModel) string {
	switch m.Type {
	case "true":
		return c.SQL + " IS TRUE"
	case "false":
		return c.SQL + " IS FALSE"
	case "blank":
		return c.SQL + " IS NULL"
	default:
		return c.SQL + " IS NOT NULL"
	}
}

func (c Column) set(id string, m filterModel, args *Args) (string, error) {
	if m.Values == nil {
		return "", httpx.Unprocessable("?filter= is invalid: %s.values: field required", id)
	}
	hasNull := false
	values := make([]string, 0, len(m.Values))
	for _, raw := range m.Values {
		if bytes.Equal(raw, []byte("null")) {
			hasNull = true
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			values = append(values, text)
			continue
		}
		var number json.Number
		if err := json.Unmarshal(raw, &number); err != nil {
			return "", httpx.Unprocessable("?filter= is invalid: %s.values must be strings, numbers or null", id)
		}
		values = append(values, number.String())
	}
	condition := "FALSE"
	switch {
	case len(values) == 0:
	case c.SetCondition != nil:
		condition = c.SetCondition(args.Add, values)
	case c.SetType == "int":
		ints := make([]int64, len(values))
		for i, v := range values {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return "", httpx.Unprocessable("Set values must be ints")
			}
			ints[i] = n
		}
		condition = fmt.Sprintf("%s = ANY(%s::bigint[])", c.SQL, args.Add(ints))
	default:
		condition = fmt.Sprintf("%s = ANY(%s::text[])", c.SQL, args.Add(values))
	}
	// AG Grid lists blanks as a value (null)
	if hasNull {
		return fmt.Sprintf("(%s OR %s IS NULL)", condition, c.SQL), nil
	}
	return condition, nil
}
