package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Validates: Requirements 1.1
func TestProperty_SpellJSONRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	validSchools := []interface{}{
		SpellSchoolAbjuration,
		SpellSchoolAlteration,
		SpellSchoolConjuration,
		SpellSchoolDivination,
		SpellSchoolEnchantment,
		SpellSchoolEvocation,
		SpellSchoolTransmutation,
		SpellSchoolIllusion,
		SpellSchoolInvocation,
		SpellSchoolNecromancy,
	}

	properties.Property("Spell JSON round-trip preserves all fields", prop.ForAll(
		func(id, sourceID, name, dndVersion string, level int, school SpellSchool, isRitual bool, castingTime, rangeStr, duration, description string) bool {
			original := Spell{
				Bookkeeping: Bookkeeping{
					CreatedAt: time.Now().UTC().Truncate(time.Second),
					CreatedBy: uuid.New(),
					UpdatedAt: time.Now().UTC().Truncate(time.Second),
					UpdatedBy: uuid.New(),
				},
				ID:          id,
				SourceID:    sourceID,
				Name:        name,
				DndVersion:  dndVersion,
				Level:       level,
				School:      school,
				IsRitual:    isRitual,
				CastingTime: castingTime,
				Range:       rangeStr,
				Duration:    duration,
				Description: description,
			}

			data, err := json.Marshal(original)
			if err != nil {
				t.Logf("marshal failed: %v", err)
				return false
			}

			var decoded Spell
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Logf("unmarshal failed: %v", err)
				return false
			}

			return decoded.ID == original.ID &&
				decoded.SourceID == original.SourceID &&
				decoded.Name == original.Name &&
				decoded.DndVersion == original.DndVersion &&
				decoded.Level == original.Level &&
				decoded.School == original.School &&
				decoded.IsRitual == original.IsRitual &&
				decoded.CastingTime == original.CastingTime &&
				decoded.Range == original.Range &&
				decoded.Duration == original.Duration &&
				decoded.Description == original.Description
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 9),
		gen.OneConstOf(validSchools...),
		gen.Bool(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
