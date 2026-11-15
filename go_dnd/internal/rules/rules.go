// Package rules is the character-building rule set a campaign (or a character without one) carries
// (CHARACTER_PLAN.md section 3). It's stored as one JSON document: always read and written whole,
// never queried by field.
//
// Parse fills in every field a document leaves out, so stored documents are always complete, and
// rejects unknown fields. Decoding onto an existing rule set merges into it, which is what PATCH does.
package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// SchemaVersion is the document version this package reads and writes.
const SchemaVersion = 1

// OtherEdition says when content from the edition that isn't the base one is offered.
type OtherEdition struct {
	// never | if_missing (only when the base edition has no entry with that slug) | always
	Default string `json:"default"`
	// per-category overrides of Default; null removes one
	ByCategory map[string]string `json:"by_category"`
}

// Sources limits which books content may come from.
type Sources struct {
	// nil: every source. Later the default becomes OGC-tagged sources only.
	Allow *[]string `json:"allow"`
	Block []string  `json:"block"`
}

// OptionalRules are stored and shown, but not enforced until after the VTT; Spellbook is the
// exception (the sheet's choice points and warnings).
type OptionalRules struct {
	Feats                   bool   `json:"feats"`
	CustomOrigin            bool   `json:"custom_origin"`
	OptionalClassFeatures   bool   `json:"optional_class_features"`
	Encumbrance             string `json:"encumbrance"` // off | standard | variant
	MulticlassPrerequisites bool   `json:"multiclass_prerequisites"`
	// a class with a spellbook (Wizard) learns its spells into it at each level (picks) and prepares
	// from it: a prepared spell outside it is a warning
	Spellbook bool `json:"spellbook"`
}

// Rules is the whole rule set.
type Rules struct {
	SchemaVersion        int           `json:"schema_version"`
	Edition              int           `json:"edition"` // the base edition: 2024 or 2014
	OtherEdition         OtherEdition  `json:"other_edition"`
	AllowUnearthedArcana bool          `json:"allow_unearthed_arcana"`
	AllowPlaytest        bool          `json:"allow_playtest"`
	Sources              Sources       `json:"sources"`
	AbilityScoreMethods  []string      `json:"ability_score_methods"`
	AbilityScoreDefault  string        `json:"ability_score_default"`
	PointBuyBudget       int           `json:"point_buy_budget"`
	RolledSetsKept       int           `json:"rolled_sets_kept"`
	HPMethods            []string      `json:"hp_methods"`
	HPDefault            string        `json:"hp_default"`
	OptionalRules        OptionalRules `json:"optional_rules"`
}

var (
	editionModes   = []string{"never", "if_missing", "always"}
	categories     = []string{"species", "class", "subclass", "background", "feat", "spell", "item"}
	abilityMethods = []string{"point_buy", "standard_array", "rolled", "manual"}
	hpMethods      = []string{"average", "rolled", "manual"}
	encumbrance    = []string{"off", "standard", "variant"}
)

// Default is the rule set a new campaign or character starts with: 2024 content, other-edition
// content only where 2024 has no entry, no UA or playtest, point buy, average HP.
func Default() Rules {
	return Rules{
		SchemaVersion:       SchemaVersion,
		Edition:             2024,
		OtherEdition:        OtherEdition{Default: "if_missing", ByCategory: map[string]string{}},
		Sources:             Sources{Block: []string{}},
		AbilityScoreMethods: slices.Clone(abilityMethods),
		AbilityScoreDefault: "point_buy",
		PointBuyBudget:      27,
		RolledSetsKept:      6,
		HPMethods:           slices.Clone(hpMethods),
		HPDefault:           "average",
		OptionalRules:       OptionalRules{Feats: true, Encumbrance: "off"},
	}
}

// Clone is a deep copy (decoding onto a rule set changes its maps and slices).
func (r Rules) Clone() Rules {
	raw, _ := json.Marshal(r)
	var out Rules
	_ = json.Unmarshal(raw, &out)
	return out
}

// Parse reads a rules document onto base: fields the document leaves out keep base's values.
// Pass Default() for a new rule set, or the current one to merge a change into it.
func Parse(raw []byte, base Rules) (Rules, error) {
	out := base.Clone()
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return out, out.Validate()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return Rules{}, fmt.Errorf("rules: %v", err)
	}
	if out.OtherEdition.ByCategory == nil {
		out.OtherEdition.ByCategory = map[string]string{}
	}
	for category, mode := range out.OtherEdition.ByCategory {
		if mode == "" { // {"subclass": null} removes the override
			delete(out.OtherEdition.ByCategory, category)
		}
	}
	if out.Sources.Block == nil {
		out.Sources.Block = []string{}
	}
	return out, out.Validate()
}

// Validate checks every field.
func (r Rules) Validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if r.SchemaVersion != SchemaVersion {
		add("schema_version must be %d", SchemaVersion)
	}
	if r.Edition != 2014 && r.Edition != 2024 {
		add("edition must be 2014 or 2024")
	}
	if !slices.Contains(editionModes, r.OtherEdition.Default) {
		add("other_edition.default must be one of %s", strings.Join(editionModes, ", "))
	}
	for _, category := range sortedKeys(r.OtherEdition.ByCategory) {
		if !slices.Contains(categories, category) {
			add("other_edition.by_category: %q isn't one of %s", category, strings.Join(categories, ", "))
		} else if !slices.Contains(editionModes, r.OtherEdition.ByCategory[category]) {
			add("other_edition.by_category.%s must be one of %s", category, strings.Join(editionModes, ", "))
		}
	}
	if r.Sources.Allow != nil && slices.Contains(*r.Sources.Allow, "") {
		add("sources.allow can't contain an empty slug")
	}
	if slices.Contains(r.Sources.Block, "") {
		add("sources.block can't contain an empty slug")
	}
	checkMethods := func(field string, methods, allowed []string, def, defField string) {
		if len(methods) == 0 {
			add("%s needs at least one method", field)
		}
		seen := map[string]bool{}
		for _, m := range methods {
			switch {
			case !slices.Contains(allowed, m):
				add("%s: %q isn't one of %s", field, m, strings.Join(allowed, ", "))
			case seen[m]:
				add("%s: %q is listed twice", field, m)
			}
			seen[m] = true
		}
		if !slices.Contains(methods, def) {
			add("%s must be one of %s", defField, field)
		}
	}
	checkMethods("ability_score_methods", r.AbilityScoreMethods, abilityMethods, r.AbilityScoreDefault, "ability_score_default")
	checkMethods("hp_methods", r.HPMethods, hpMethods, r.HPDefault, "hp_default")
	if r.PointBuyBudget < 0 || r.PointBuyBudget > 200 {
		add("point_buy_budget must be 0 to 200")
	}
	if r.RolledSetsKept < 1 || r.RolledSetsKept > 100 {
		add("rolled_sets_kept must be 1 to 100")
	}
	if !slices.Contains(encumbrance, r.OptionalRules.Encumbrance) {
		add("optional_rules.encumbrance must be one of %s", strings.Join(encumbrance, ", "))
	}
	if len(problems) > 0 {
		return fmt.Errorf("rules: %s", strings.Join(problems, "; "))
	}
	return nil
}

// Entry is a catalog entry as the content rules see it.
type Entry struct {
	Category         string // species, class, subclass, background, feat, spell, item
	Year             int    // its dnd_version_year
	UnearthedArcana  bool
	SourceSlug       string
	BaseEditionHasIt bool // the base edition has a live entry with the same slug (for if_missing)
}

// Allows reports whether the rules offer e, and why not when they don't.
func (r Rules) Allows(e Entry) (bool, string) {
	if e.Year != r.Edition {
		mode := r.OtherEdition.Default
		if m, ok := r.OtherEdition.ByCategory[e.Category]; ok {
			mode = m
		}
		switch {
		case mode == "never":
			return false, fmt.Sprintf("%d content isn't allowed (base edition %d)", e.Year, r.Edition)
		case mode == "if_missing" && e.BaseEditionHasIt:
			return false, fmt.Sprintf("%d has its own version; %d content is only allowed where %d has none", r.Edition, e.Year, r.Edition)
		}
	}
	if e.UnearthedArcana && !r.AllowUnearthedArcana {
		return false, "Unearthed Arcana isn't allowed"
	}
	if r.Sources.Allow != nil && !slices.Contains(*r.Sources.Allow, e.SourceSlug) {
		return false, fmt.Sprintf("source %s isn't allowed", e.SourceSlug)
	}
	if slices.Contains(r.Sources.Block, e.SourceSlug) {
		return false, fmt.Sprintf("source %s is blocked", e.SourceSlug)
	}
	return true, ""
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
