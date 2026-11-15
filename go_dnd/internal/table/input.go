package table

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Mode is what a write needs from its input.
type Mode int

const (
	// Full: POST, PUT and bulk rows. Required columns must be there; missing ones get their default.
	Full Mode = iota
	// Patch: only the columns present change; null can't clear a column that isn't nullable.
	Patch
)

// csvCell is a value from a CSV upload: text that is converted by its column's type.
type csvCell string

// Record is a validated row: column name -> value, ready to marshal for jsonb_populate_record.
type Record map[string]any

// Columns are the record's column names in the table's column order.
func (t *Table) columnsOf(r Record) []string {
	names := make([]string, 0, len(r))
	for _, name := range t.writable {
		if _, ok := r[name]; ok {
			names = append(names, name)
		}
	}
	return names
}

// Normalize validates a request body or bulk row (decoded with UseNumber) into a Record.
// strict rejects unknown keys (request bodies); bulk rows ignore them (the ETL's files carry more).
func (t *Table) Normalize(doc map[string]any, mode Mode, strict bool) (Record, error) {
	record := Record{}
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var unknown []string
	for _, key := range keys {
		name, ok := t.inputs[key]
		if !ok {
			unknown = append(unknown, key)
			continue
		}
		if _, seen := record[name]; seen && key != name {
			continue // the column's own name wins over an alias
		}
		value, err := t.cols[name].convert(doc[key])
		if err != nil {
			return nil, httpx.Unprocessable("%s: %v", key, err)
		}
		record[name] = value
	}
	if strict && len(unknown) > 0 {
		return nil, httpx.Unprocessable("%s: extra fields not permitted", strings.Join(unknown, ", "))
	}
	for _, name := range t.writable {
		c := t.cols[name]
		value, present := record[name]
		switch {
		case !present && mode == Full && c.Required:
			return nil, httpx.Unprocessable("%s: field required", name)
		case !present && mode == Full:
			record[name] = c.defaultValue()
		case present && value == nil && !c.Nullable && !strict && mode == Full && !c.Required:
			// a bulk row's empty cell or null: the default, as if it were left out
			record[name] = c.defaultValue()
		case present && value == nil && !c.Nullable:
			return nil, httpx.Unprocessable("%s: can't be null", name)
		}
	}
	return record, nil
}

func (c *Col) defaultValue() any {
	if c.Default != nil {
		return c.Default
	}
	if c.Nullable {
		return nil
	}
	switch c.Type {
	case Text:
		return ""
	case Int:
		return int64(0)
	case Bool:
		return false
	case TextArray:
		return []string{}
	case JSON:
		if c.Shape == "array" {
			return []any{}
		}
		return map[string]any{}
	}
	return nil
}

// convert checks one input value against the column's type (and Coerce / Check) and returns it
// ready for JSON.
func (c *Col) convert(value any) (any, error) {
	if c.Coerce != nil {
		value = c.Coerce(value)
	}
	converted, err := c.convertType(value)
	if err != nil || converted == nil || c.Check == nil {
		return converted, err
	}
	return c.Check(converted)
}

func (c *Col) convertType(value any) (any, error) {
	if cell, ok := value.(csvCell); ok {
		return c.convertCSV(string(cell))
	}
	if value == nil {
		return nil, nil // Normalize decides whether null is allowed
	}
	switch c.Type {
	case Text:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		return strings.TrimSpace(s), nil
	case Int:
		n, ok := value.(json.Number)
		if !ok {
			return nil, fmt.Errorf("must be an integer")
		}
		i, err := n.Int64()
		if err != nil {
			return nil, fmt.Errorf("must be an integer")
		}
		return i, nil
	case Bool:
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return b, nil
	case TextArray:
		items, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("must be a list of strings")
		}
		out := make([]string, len(items))
		for i, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("must be a list of strings")
			}
			out[i] = strings.TrimSpace(s)
		}
		return out, nil
	case JSON:
		switch value.(type) {
		case map[string]any:
			if c.Shape == "array" {
				return nil, fmt.Errorf("must be a list")
			}
		case []any:
			if c.Shape == "object" {
				return nil, fmt.Errorf("must be an object")
			}
		default:
			return nil, fmt.Errorf("must be JSON (an object or a list)")
		}
		return value, nil
	case UUID:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a UUID")
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("must be a UUID")
		}
		return id.String(), nil
	}
	return nil, fmt.Errorf("can't be written")
}

// convertCSV reads a CSV cell by the column's type. An empty cell is null.
func (c *Col) convertCSV(cell string) (any, error) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return nil, nil
	}
	switch c.Type {
	case Int:
		// spreadsheets write integers with gaps as floats (112.0)
		if f, err := strconv.ParseFloat(cell, 64); err == nil && f == float64(int64(f)) {
			return int64(f), nil
		}
		return nil, fmt.Errorf("must be an integer")
	case Bool:
		b, err := strconv.ParseBool(cell)
		if err != nil {
			return nil, fmt.Errorf("must be a boolean")
		}
		return b, nil
	case TextArray, JSON:
		decoder := json.NewDecoder(strings.NewReader(cell))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("must be JSON")
		}
		return c.convertType(value)
	}
	return c.convertType(cell)
}

// keyOf is the natural key's values as text, for duplicate checks.
func (t *Table) keyOf(r Record) string {
	parts := make([]string, len(t.Key))
	for i, k := range t.Key {
		parts[i] = fmt.Sprint(r[k])
	}
	return strings.Join(parts, "\x00")
}

// Writable reports whether name is a column clients write.
func (t *Table) Writable(name string) bool { return slices.Contains(t.writable, name) }

// TextValue is v as text when it's a string (JSON) or a cell (CSV).
func TextValue(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case csvCell:
		return string(s), true
	}
	return "", false
}
