package handlers

import (
	"fmt"
	"strconv"

	api "github.com/humoroushorse/go_dnd/api/generated"
	svcsources "github.com/humoroushorse/go_dnd/internal/service/sources"
	svcspells "github.com/humoroushorse/go_dnd/internal/service/spells"
)

func toAnySlice[T any](s []T) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func mapCreateSpellRequest(req api.CreateSpellRequest) svcspells.CreateSpellInput {
	input := svcspells.CreateSpellInput{
		SourceID:                   req.SourceId,
		Name:                       req.Name,
		Slug:                       req.Slug,
		DndVersion:                 req.DndVersion,
		DndVersionYear:             int32(req.DndVersionYear),
		Level:                      int32(req.Level),
		School:                     string(req.School),
		CastingTime:                req.CastingTime,
		Range:                      req.Range,
		Duration:                   req.Duration,
		Description:                req.Description,
		IsRitual:                   derefBool(req.IsRitual),
		IsUnearthedArcana:          derefBool(req.IsUnearthedArcana),
		HasVerbalComponent:         derefBool(req.HasVerbalComponent),
		HasSomaticComponent:        derefBool(req.HasSomaticComponent),
		HasMaterialComponent:       derefBool(req.HasMaterialComponent),
		HasSpellCost:               derefBool(req.HasSpellCost),
		AreMaterialsConsumed:       derefBool(req.AreMaterialsConsumed),
		IsConcentration:            derefBool(req.IsConcentration),
		HasSavingThrow:             derefBool(req.HasSavingThrow),
		Materials:                  req.Materials,
		DamageType:                 req.DamageType,
		AtHigherLevels:             req.AtHigherLevels,
		DifficultyClassSavingThrow: req.DifficultyClassSavingThrow,
		DifficultyClassType:        req.DifficultyClassType,
	}
	if req.Id != nil {
		input.ID = *req.Id
	}
	if req.SourcePage != nil {
		v := int32(*req.SourcePage)
		input.SourcePage = &v
	}
	if req.DifficultyClassSavingThrowOverride != nil {
		v := int32(*req.DifficultyClassSavingThrowOverride)
		input.DifficultyClassSavingThrowOverride = &v
	}
	return input
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// parseSpellCSV expects header row: id,source_id,name,dnd_version,dnd_version_year,level,school,casting_time,range,duration,description
func parseSpellCSV(rows [][]string) ([]svcspells.CreateSpellInput, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}
	header := rows[0]
	idx := csvIndex(header)
	var records []svcspells.CreateSpellInput
	for i, row := range rows[1:] {
		if len(row) < len(header) {
			return nil, fmt.Errorf("row %d has fewer columns than header", i+2)
		}
		level, err := strconv.Atoi(csvGet(row, idx, "level"))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid level: %w", i+2, err)
		}
		records = append(records, svcspells.CreateSpellInput{
			ID:          csvGet(row, idx, "id"),
			SourceID:    csvGet(row, idx, "source_id"),
			Name:        csvGet(row, idx, "name"),
			DndVersion:  csvGet(row, idx, "dnd_version"),
			Level:       int32(level),
			School:      csvGet(row, idx, "school"),
			CastingTime: csvGet(row, idx, "casting_time"),
			Range:       csvGet(row, idx, "range"),
			Duration:    csvGet(row, idx, "duration"),
			Description: csvGet(row, idx, "description"),
		})
	}
	return records, nil
}

// parseSourceCSV expects header row: id,name,name_short,dnd_version,dnd_version_year
func parseSourceCSV(rows [][]string) ([]svcsources.CreateSourceInput, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}
	header := rows[0]
	idx := csvIndex(header)
	var records []svcsources.CreateSourceInput
	for i, row := range rows[1:] {
		if len(row) < len(header) {
			return nil, fmt.Errorf("row %d has fewer columns than header", i+2)
		}
		year, err := strconv.Atoi(csvGet(row, idx, "dnd_version_year"))
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid dnd_version_year: %w", i+2, err)
		}
		records = append(records, svcsources.CreateSourceInput{
			ID:             csvGet(row, idx, "id"),
			Name:           csvGet(row, idx, "name"),
			NameShort:      csvGet(row, idx, "name_short"),
			DndVersion:     csvGet(row, idx, "dnd_version"),
			DndVersionYear: int32(year),
		})
	}
	return records, nil
}

func csvIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[h] = i
	}
	return m
}

func csvGet(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}
