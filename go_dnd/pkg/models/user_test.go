package models

import (
	"encoding/json"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Validates: Requirements 1.3
func TestProperty_DndUserJSONRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("DndUser JSON round-trip preserves all fields", prop.ForAll(
		func(id, username string) bool {
			original := DndUser{
				ID:       id,
				Username: &username,
			}

			data, err := json.Marshal(original)
			if err != nil {
				t.Logf("marshal failed: %v", err)
				return false
			}

			var decoded DndUser
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Logf("unmarshal failed: %v", err)
				return false
			}

			if decoded.ID != original.ID {
				return false
			}
			if original.Username == nil {
				return decoded.Username == nil
			}
			return decoded.Username != nil && *decoded.Username == *original.Username
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.Property("DndUser with nil optional fields round-trips correctly", prop.ForAll(
		func(id string) bool {
			original := DndUser{
				ID:       id,
				Username: nil,
			}

			data, err := json.Marshal(original)
			if err != nil {
				return false
			}

			var decoded DndUser
			if err := json.Unmarshal(data, &decoded); err != nil {
				return false
			}

			return decoded.ID == original.ID && decoded.Username == nil && decoded.ProfilePictureURL == nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
