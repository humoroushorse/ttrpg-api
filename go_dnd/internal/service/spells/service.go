package spells

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	reposources "github.com/humoroushorse/go_dnd/internal/repository/sources"
	"github.com/humoroushorse/go_dnd/internal/repository/spells"
	"github.com/humoroushorse/go_dnd/pkg/models"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrSpellNotFound = errors.New("spell not found")
	ErrInvalidLevel  = errors.New("spell level must be between 0 and 9")
	ErrInvalidSchool = errors.New("invalid spell school")
	ErrRequiredField = errors.New("required field is missing")
)

type SpellService interface {
	ListSpells(ctx context.Context, limit, offset int32) ([]spells.DndSpell, error)
	QuerySpells(ctx context.Context, name *string, level *int32, school *string, limit, offset int32) ([]spells.DndSpell, error)
	CreateSpell(ctx context.Context, input CreateSpellInput, userID uuid.UUID) (spells.DndSpell, error)
	BulkLoadSpells(ctx context.Context, records []CreateSpellInput, userID uuid.UUID) (models.BulkLoadResponse, error)
}

type Service struct {
	repo       *spells.Repository
	sourceRepo *reposources.Repository
}

func NewService(repo *spells.Repository, sourceRepo *reposources.Repository) *Service {
	return &Service{repo: repo, sourceRepo: sourceRepo}
}

type CreateSpellInput struct {
	ID                                 string           `json:"id,omitempty"`
	SourceID                           string           `json:"source_id,omitempty"`
	SourceName                         string           `json:"source_name,omitempty"`
	SourceVersion                      string           `json:"source_version,omitempty"`
	Name                               string           `json:"name"`
	Slug                               *string          `json:"slug,omitempty"`
	DndVersion                         string           `json:"dnd_version,omitempty"`
	DndVersionYear                     int32            `json:"dnd_version_year,omitempty"`
	SourcePage                         *int32           `json:"source_page,omitempty"`
	Level                              int32            `json:"level"`
	School                             string           `json:"school"`
	IsRitual                           bool             `json:"is_ritual,omitempty"`
	IsUnearthedArcana                  bool             `json:"is_unearthed_arcana,omitempty"`
	CastingTime                        string           `json:"casting_time"`
	Range                              string           `json:"range"`
	HasVerbalComponent                 bool             `json:"has_verbal_component,omitempty"`
	HasSomaticComponent                bool             `json:"has_somatic_component,omitempty"`
	HasMaterialComponent               bool             `json:"has_material_component,omitempty"`
	Materials                          *string          `json:"materials,omitempty"`
	HasSpellCost                       bool             `json:"has_spell_cost,omitempty"`
	AreMaterialsConsumed               bool             `json:"are_materials_consumed,omitempty"`
	Duration                           string           `json:"duration"`
	IsConcentration                    bool             `json:"is_concentration,omitempty"`
	Description                        string           `json:"description_html,omitempty"`
	HasSavingThrow                     bool             `json:"has_saving_throw,omitempty"`
	DifficultyClassSavingThrowOverride *int32           `json:"difficulty_class_saving_throw_override,omitempty"`
	DamageType                         *string          `json:"damage_type,omitempty"`
	AtHigherLevels                     *string          `json:"at_higher_levels,omitempty"`
	DifficultyClassSavingThrow         *string          `json:"-"`
	DifficultyClassSavingThrowRaw      *json.RawMessage `json:"difficulty_class_saving_throw,omitempty"`
	DifficultyClassType                *string          `json:"difficulty_class_type,omitempty"`
	StatBlocks                         []map[string]any `json:"stat_blocks,omitempty"`
}

func (c *CreateSpellInput) resolveDifficultyClass() {
	if c.DifficultyClassSavingThrowRaw == nil {
		return
	}
	raw := *c.DifficultyClassSavingThrowRaw
	// try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		c.DifficultyClassSavingThrow = &s
		return
	}
	// try number — convert to string
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		str := n.String()
		c.DifficultyClassSavingThrow = &str
	}
}

func (s *Service) ListSpells(ctx context.Context, limit, offset int32) ([]spells.DndSpell, int64, error) {
	total, err := s.repo.CountAllSpells(ctx)
	if err != nil {
		return nil, 0, err
	}
	results, err := s.repo.ListSpells(ctx, spells.ListSpellsParams{Limit: limit, Offset: offset})
	return results, total, err
}

func (s *Service) QuerySpells(ctx context.Context, name *string, level *int32, school *string, limit, offset int32) ([]spells.DndSpell, int64, error) {
	total, err := s.repo.CountQuerySpells(ctx, spells.CountQuerySpellsParams{
		Name:   name,
		Level:  level,
		School: school,
	})
	if err != nil {
		return nil, 0, err
	}
	results, err := s.repo.QuerySpells(ctx, spells.QuerySpellsParams{
		Limit:  limit,
		Offset: offset,
		Name:   name,
		Level:  level,
		School: school,
	})
	return results, total, err
}

func parseUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func (s *Service) CreateSpell(ctx context.Context, input CreateSpellInput, userID uuid.UUID) (spells.DndSpell, error) {
	if err := validateSpellInput(input); err != nil {
		return spells.DndSpell{}, err
	}

	statBlocksJSON, err := marshalStatBlocks(input.StatBlocks)
	if err != nil {
		return spells.DndSpell{}, fmt.Errorf("invalid stat_blocks: %w", err)
	}

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	createdBy := pgtype.UUID{Bytes: userID, Valid: true}

	idStr := input.ID
	if idStr == "" {
		idStr = uuid.New().String()
	}
	id, err := parseUUID(idStr)
	if err != nil {
		return spells.DndSpell{}, fmt.Errorf("invalid id: %w", err)
	}
	sourceID, err := parseUUID(input.SourceID)
	if err != nil {
		return spells.DndSpell{}, fmt.Errorf("invalid source_id %q: %w", input.SourceID, err)
	}

	return s.repo.CreateSpell(ctx, spells.CreateSpellParams{
		ID:                                 id,
		SourceID:                           sourceID,
		Name:                               input.Name,
		Slug:                               input.Slug,
		DndVersion:                         input.DndVersion,
		DndVersionYear:                     input.DndVersionYear,
		SourcePage:                         input.SourcePage,
		Level:                              input.Level,
		School:                             spells.DndSpellSchool(input.School),
		IsRitual:                           input.IsRitual,
		IsUnearthedArcana:                  input.IsUnearthedArcana,
		CastingTime:                        input.CastingTime,
		Range:                              input.Range,
		HasVerbalComponent:                 input.HasVerbalComponent,
		HasSomaticComponent:                input.HasSomaticComponent,
		HasMaterialComponent:               input.HasMaterialComponent,
		Materials:                          input.Materials,
		HasSpellCost:                       input.HasSpellCost,
		AreMaterialsConsumed:               input.AreMaterialsConsumed,
		Duration:                           input.Duration,
		IsConcentration:                    input.IsConcentration,
		Description:                        input.Description,
		HasSavingThrow:                     input.HasSavingThrow,
		DifficultyClassSavingThrowOverride: input.DifficultyClassSavingThrowOverride,
		DamageType:                         input.DamageType,
		AtHigherLevels:                     input.AtHigherLevels,
		DifficultyClassSavingThrow:         input.DifficultyClassSavingThrow,
		DifficultyClassType:                input.DifficultyClassType,
		StatBlocks:                         statBlocksJSON,
		CreatedAt:                          now,
		UpdatedAt:                          now,
		CreatedBy:                          createdBy,
		UpdatedBy:                          createdBy,
	})
}

// BulkLoadSpells processes every record individually — never bails early.
// Each spell ends up in created, warnings (already exists), or errors.
func (s *Service) BulkLoadSpells(ctx context.Context, records []CreateSpellInput, userID uuid.UUID) (models.BulkLoadResponse, error) {
	result := models.BulkLoadResponse{}

	// Phase 1: Resolve source_id and normalize fields for each record
	for i := range records {
		rec := &records[i]
		rec.resolveDifficultyClass()
		if rec.SourceID == "" && rec.SourceName != "" && rec.DndVersion != "" && rec.DndVersionYear > 0 {
			src, err := s.sourceRepo.GetSourceByNameVersion(ctx, rec.SourceName, rec.DndVersion, rec.DndVersionYear)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("record %d (%s): source not found for name=%q dnd_version=%q dnd_version_year=%d", i, rec.Name, rec.SourceName, rec.DndVersion, rec.DndVersionYear))
				continue
			}
			rec.SourceID = src.ID.String()
			if rec.DndVersion == "" {
				rec.DndVersion = src.DndVersion
			}
		}
	}

	// Phase 2: Process each record — validate, dedupe, insert
	for i, rec := range records {
		// Skip records that already failed source resolution
		if rec.SourceID == "" && rec.SourceName != "" {
			continue // already in errors from phase 1
		}

		if err := validateSpellInput(rec); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("record %d (%s): %v", i, rec.Name, err))
			continue
		}

		exists, err := s.repo.SpellExistsByNameVersion(ctx, rec.Name, rec.DndVersion, rec.DndVersionYear, parsedSourceID(rec.SourceID))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("record %d (%s): checking existence: %v", i, rec.Name, err))
			continue
		}
		if exists {
			existing, fetchErr := s.repo.GetSpellByUniqueKey(ctx, rec.Name, rec.DndVersion, rec.DndVersionYear, parsedSourceID(rec.SourceID))
			if fetchErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%q already exists, skipping — incoming: (dnd_version=%q dnd_version_year=%d source_id=%q) existing: (lookup failed: %v)", rec.Name, rec.DndVersion, rec.DndVersionYear, rec.SourceID, fetchErr))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%q already exists, skipping — incoming: (dnd_version=%q dnd_version_year=%d source_id=%q) existing: (id=%q dnd_version=%q dnd_version_year=%d source_id=%q)", rec.Name, rec.DndVersion, rec.DndVersionYear, rec.SourceID, existing.ID.String(), existing.DndVersion, existing.DndVersionYear, existing.SourceID.String()))
			}
			continue
		}

		spell, err := s.CreateSpell(ctx, rec, userID)
		if err != nil {
			// Treat unique constraint violations as warnings (idempotent bulk load)
			if isDuplicateKeyError(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%q already exists, skipping (constraint) — incoming: (dnd_version=%q dnd_version_year=%d source_id=%q)", rec.Name, rec.DndVersion, rec.DndVersionYear, rec.SourceID))
				continue
			}
			result.Errors = append(result.Errors, fmt.Sprintf("record %d (%s): %v", i, rec.Name, err))
			continue
		}
		result.Created = append(result.Created, fmt.Sprintf("%s (%s %d)", spell.Name, spell.DndVersion, spell.DndVersionYear))
	}

	return result, nil
}

func validateSpellInput(input CreateSpellInput) error {
	if input.Name == "" {
		return fmt.Errorf("%w: name", ErrRequiredField)
	}
	if input.SourceID == "" {
		return fmt.Errorf("%w: source_id", ErrRequiredField)
	}
	if input.DndVersion == "" {
		return fmt.Errorf("%w: dnd_version", ErrRequiredField)
	}
	if input.CastingTime == "" {
		return fmt.Errorf("%w: casting_time", ErrRequiredField)
	}
	if input.Duration == "" {
		return fmt.Errorf("%w: duration", ErrRequiredField)
	}
	if input.Description == "" {
		return fmt.Errorf("%w: description", ErrRequiredField)
	}
	if input.Level < 0 || input.Level > 9 {
		return ErrInvalidLevel
	}
	if !models.ValidSpellSchools[models.SpellSchool(input.School)] {
		return ErrInvalidSchool
	}
	return nil
}

func marshalStatBlocks(statBlocks []map[string]any) ([]byte, error) {
	if len(statBlocks) == 0 {
		return nil, nil
	}
	return json.Marshal(statBlocks)
}

// isDuplicateKeyError checks if the error is a postgres unique constraint violation
func isDuplicateKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}

// parsedSourceID parses a UUID string into pgtype.UUID for SQL queries.
func parsedSourceID(id string) pgtype.UUID {
	var u pgtype.UUID
	if id == "" {
		return u
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return u
	}
	u.Bytes = parsed
	u.Valid = true
	return u
}
