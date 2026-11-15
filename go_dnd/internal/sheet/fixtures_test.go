package sheet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The golden fixtures (CHARACTER_PLAN.md section 6) live in ttrpg-api/fixtures/characters so both
// APIs' engines run the same files. Each <name>.json holds a snapshot, the levels to compute, and
// hand-checked expectations; <name>.sheet.json is the whole computed sheet per level, compared
// byte for byte. After an intended change, rewrite the goldens with UPDATE_GOLDEN=1 go test ./internal/sheet
// and review the diff.
const fixturesDir = "../../../fixtures/characters"

type fixture struct {
	Description string      `json:"description"`
	Snapshot    Snapshot    `json:"snapshot"`
	Levels      []int       `json:"levels"`
	Expect      []expectArg `json:"expect"`
}

type expectArg struct {
	Level   int    `json:"level"`
	Path    string `json:"path"`               // dotted path into the sheet's JSON
	Equals  any    `json:"equals,omitempty"`   // the value there
	HasName string `json:"has_name,omitempty"` // a list there has an entry with this name / source / code / slot / spell
	Lacks   string `json:"lacks_name,omitempty"`
}

func TestFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(fixturesDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures in %s: %v", fixturesDir, err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".sheet.json") {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var f fixture
			if err := json.Unmarshal(raw, &f); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			sheets := map[string]Sheet{}
			for _, level := range f.Levels {
				snap := f.Snapshot
				snap.Level = level
				sheets[fmt.Sprint(level)] = Compute(snap)
			}
			got, _ := json.MarshalIndent(sheets, "", "  ")
			got = append(got, '\n')
			golden := strings.TrimSuffix(file, ".json") + ".sheet.json"
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if want, err := os.ReadFile(golden); err != nil {
				t.Fatalf("no golden file %s (run with UPDATE_GOLDEN=1): %v", golden, err)
			} else if !bytes.Equal(got, want) {
				t.Errorf("%s differs from the computed sheet (UPDATE_GOLDEN=1 rewrites it; review the diff)", filepath.Base(golden))
			}
			for _, e := range f.Expect {
				checkExpect(t, sheets, e)
			}
		})
	}
}

func checkExpect(t *testing.T, sheets map[string]Sheet, e expectArg) {
	t.Helper()
	sh, ok := sheets[fmt.Sprint(e.Level)]
	if !ok {
		t.Errorf("expect at level %d: that level isn't computed", e.Level)
		return
	}
	raw, _ := json.Marshal(sh)
	var node any
	_ = json.Unmarshal(raw, &node)
	for _, part := range strings.Split(e.Path, ".") {
		switch n := node.(type) {
		case map[string]any:
			node = n[part]
		case []any: // a list index: attacks.0.damage
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(n) {
				t.Errorf("level %d %s: %q isn't an index of a %d-item list", e.Level, e.Path, part, len(n))
				return
			}
			node = n[i]
		default:
			t.Errorf("level %d %s: %q isn't an object or list", e.Level, e.Path, part)
			return
		}
	}
	switch {
	case e.HasName != "" || e.Lacks != "":
		list, _ := node.([]any)
		found := map[string]bool{}
		for _, item := range list {
			if m, ok := item.(map[string]any); ok {
				for _, key := range []string{"name", "source", "code", "slot", "spell"} {
					if v, ok := m[key]; ok {
						found[fmt.Sprint(v)] = true
					}
				}
			} else {
				found[fmt.Sprint(item)] = true
			}
		}
		if e.HasName != "" && !found[e.HasName] {
			t.Errorf("level %d %s: no entry named %q", e.Level, e.Path, e.HasName)
		}
		if e.Lacks != "" && found[e.Lacks] {
			t.Errorf("level %d %s: has %q, which it shouldn't", e.Level, e.Path, e.Lacks)
		}
	default:
		want, _ := json.Marshal(e.Equals)
		got, _ := json.Marshal(node)
		if !bytes.Equal(want, got) {
			t.Errorf("level %d %s = %s, want %s", e.Level, e.Path, got, want)
		}
	}
}
