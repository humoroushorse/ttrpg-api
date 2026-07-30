package models

import "log/slog"

type SpellSchool string

const (
	SpellSchoolAbjuration    SpellSchool = "abjuration"
	SpellSchoolAlteration    SpellSchool = "alteration"
	SpellSchoolConjuration   SpellSchool = "conjuration"
	SpellSchoolDivination    SpellSchool = "divination"
	SpellSchoolEnchantment   SpellSchool = "enchantment"
	SpellSchoolEvocation     SpellSchool = "evocation"
	SpellSchoolTransmutation SpellSchool = "transmutation"
	SpellSchoolIllusion      SpellSchool = "illusion"
	SpellSchoolInvocation    SpellSchool = "invocation"
	SpellSchoolNecromancy    SpellSchool = "necromancy"
)

var ValidSpellSchools = map[SpellSchool]bool{
	SpellSchoolAbjuration:    true,
	SpellSchoolAlteration:    true,
	SpellSchoolConjuration:   true,
	SpellSchoolDivination:    true,
	SpellSchoolEnchantment:   true,
	SpellSchoolEvocation:     true,
	SpellSchoolTransmutation: true,
	SpellSchoolIllusion:      true,
	SpellSchoolInvocation:    true,
	SpellSchoolNecromancy:    true,
}

type Spell struct {
	Bookkeeping
	ID                                 string           `json:"id"`
	SourceID                           string           `json:"source_id"`
	Name                               string           `json:"name"`
	Slug                               *string          `json:"slug,omitempty"`
	DndVersion                         string           `json:"dnd_version"`
	DndVersionYear                     int              `json:"dnd_version_year"`
	SourcePage                         *int             `json:"source_page,omitempty"`
	Level                              int              `json:"level"`
	School                             SpellSchool      `json:"school"`
	IsRitual                           bool             `json:"is_ritual"`
	IsUnearthedArcana                  bool             `json:"is_unearthed_arcana"`
	CastingTime                        string           `json:"casting_time"`
	Range                              string           `json:"range"`
	HasVerbalComponent                 bool             `json:"has_verbal_component"`
	HasSomaticComponent                bool             `json:"has_somatic_component"`
	HasMaterialComponent               bool             `json:"has_material_component"`
	Materials                          *string          `json:"materials,omitempty"`
	HasSpellCost                       bool             `json:"has_spell_cost"`
	AreMaterialsConsumed               bool             `json:"are_materials_consumed"`
	Duration                           string           `json:"duration"`
	IsConcentration                    bool             `json:"is_concentration"`
	Description                        string           `json:"description"`
	HasSavingThrow                     bool             `json:"has_saving_throw"`
	DifficultyClassSavingThrowOverride *int             `json:"difficulty_class_saving_throw_override,omitempty"`
	DamageType                         *string          `json:"damage_type,omitempty"`
	AtHigherLevels                     *string          `json:"at_higher_levels,omitempty"`
	DifficultyClassSavingThrow         *string          `json:"difficulty_class_saving_throw,omitempty"`
	DifficultyClassType                *string          `json:"difficulty_class_type,omitempty"`
	StatBlocks                         []map[string]any `json:"stat_blocks,omitempty"`
}

func (s Spell) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("id", s.ID),
		slog.String("name", s.Name),
		slog.Int("level", s.Level),
		slog.String("school", string(s.School)),
	)
}
