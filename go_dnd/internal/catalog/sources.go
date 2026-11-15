package catalog

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/table"
)

// Source is a live source, as a bulk row is matched to it.
type Source struct {
	ID, Slug, Name, NameShort, DndVersion string
	DndVersionYear                        int64
}

// SourceIndex finds a bulk row's source without a query per row (there are ~150 sources).
type SourceIndex struct {
	bySlug    map[string]*Source
	byName    map[string]*Source // name \x00 dnd_version \x00 year
	byShort   map[string]*Source // lower(name_short)
	ambiguous map[string]bool    // short names more than one source has
}

// LoadSources reads every live source.
func LoadSources(ctx context.Context, db *pgxpool.Pool) (*SourceIndex, error) {
	rows, err := db.Query(ctx, `SELECT id::text, slug, name, name_short, dnd_version, dnd_version_year
FROM dnd.source WHERE deleted_at IS NULL ORDER BY dnd_version_year DESC, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idx := &SourceIndex{bySlug: map[string]*Source{}, byName: map[string]*Source{}, byShort: map[string]*Source{}, ambiguous: map[string]bool{}}
	for rows.Next() {
		s := &Source{}
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.NameShort, &s.DndVersion, &s.DndVersionYear); err != nil {
			return nil, err
		}
		idx.bySlug[s.Slug] = s
		idx.byName[nameKey(s.Name, s.DndVersion, s.DndVersionYear)] = s
		short := strings.ToLower(s.NameShort)
		if _, taken := idx.byShort[short]; taken {
			idx.ambiguous[short] = true
		} else {
			idx.byShort[short] = s
		}
	}
	return idx, rows.Err()
}

func nameKey(name, version string, year int64) string {
	return name + "\x00" + version + "\x00" + strconv.FormatInt(year, 10)
}

// Find is a row's source: the ETL's source_id as a slug; then source_name + dnd_version +
// dnd_version_year; then source_id as a short name (case-insensitive, exact, and only when one
// source has it).
func (idx *SourceIndex) Find(row map[string]any) *Source {
	sourceID, hasID := table.TextValue(row["source_id"])
	if hasID {
		if s, ok := idx.bySlug[sourceID]; ok {
			return s
		}
	}
	name, _ := table.TextValue(row["source_name"])
	version, _ := table.TextValue(row["dnd_version"])
	if year, ok := intValue(row["dnd_version_year"]); ok && name != "" && version != "" {
		if s, ok := idx.byName[nameKey(name, version, year)]; ok {
			return s
		}
	}
	if hasID {
		short := strings.ToLower(sourceID)
		if s, ok := idx.byShort[short]; ok && !idx.ambiguous[short] {
			return s
		}
	}
	return nil
}

// intValue reads a row's integer: a JSON number, or text (a CSV cell, "2024").
func intValue(v any) (int64, bool) {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		f, err := n.Float64()
		return int64(f), err == nil
	}
	if s, ok := table.TextValue(v); ok {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return int64(f), err == nil
	}
	return 0, false
}
