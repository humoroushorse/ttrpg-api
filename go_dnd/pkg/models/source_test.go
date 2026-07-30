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

// Validates: Requirements 1.2
func TestProperty_SourceJSONRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Source JSON round-trip preserves all fields", prop.ForAll(
		func(id, name, nameShort, dndVersion string, dndVersionYear int) bool {
			original := Source{
				Bookkeeping: Bookkeeping{
					CreatedAt: time.Now().UTC().Truncate(time.Second),
					CreatedBy: uuid.New(),
					UpdatedAt: time.Now().UTC().Truncate(time.Second),
					UpdatedBy: uuid.New(),
				},
				ID:             id,
				Name:           name,
				NameShort:      nameShort,
				DndVersion:     dndVersion,
				DndVersionYear: dndVersionYear,
			}

			data, err := json.Marshal(original)
			if err != nil {
				t.Logf("marshal failed: %v", err)
				return false
			}

			var decoded Source
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Logf("unmarshal failed: %v", err)
				return false
			}

			return decoded.ID == original.ID &&
				decoded.Name == original.Name &&
				decoded.NameShort == original.NameShort &&
				decoded.DndVersion == original.DndVersion &&
				decoded.DndVersionYear == original.DndVersionYear &&
				decoded.PublishYear == original.PublishYear
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(1970, 2100),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
