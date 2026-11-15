// Package sheet computes a character's sheet from a snapshot of its build and the catalog entries it
// uses (CHARACTER_PLAN.md section 6). Compute is a pure function: no database, no clock, no randomness,
// so the same snapshot always gives the same sheet. That's what the golden fixtures in
// ttrpg-api/fixtures/characters/ check, and what lets the engine compile to WebAssembly for offline use.
//
// Every number comes back with its contributions (what added up to it) and the inactive ones (and
// why), so the UI's hover needs no logic of its own.
package sheet

import "encoding/json"

// Snapshot is everything the sheet is computed from. The loader in internal/characters builds it from
// the database; fixtures write it by hand.
type Snapshot struct {
	Level          int             `json:"level"` // the level to compute (current_level, or ?level=)
	AbilityMethod  string          `json:"ability_method"`
	BaseScores     map[string]int  `json:"base_scores"`
	Class          *Class          `json:"class"`
	Levels         []Level         `json:"levels"`
	Species        *Species        `json:"species"`
	Background     *Background     `json:"background"`
	Subclass       *Subclass       `json:"subclass"`
	SubclassLevel  int             `json:"subclass_level"` // when the class takes its subclass (2024: 3)
	Feats          []Feat          `json:"feats,omitempty"`
	Choices        []Choice        `json:"choices"`
	Items          []Item          `json:"items"`
	PreparedSpells []PreparedSpell `json:"prepared_spells,omitempty"`
	Modifiers      []Modifier      `json:"modifiers"`
	RuleIssues     []Issue         `json:"rule_issues"` // content-rule violations the loader found
	// the optional rule: a class with a spellbook (Wizard) learns its spells into it at each level
	// and prepares from it
	SpellbookRule bool `json:"spellbook_rule,omitempty"`
}

// Class is the character's class (single class until multiclassing).
type Class struct {
	Name             string        `json:"name"`
	Slug             string        `json:"slug,omitempty"` // wizard; slots of its choice points start with class:<slug>
	Year             int           `json:"year"`
	HitDieSize       int           `json:"hit_die_size"`
	SavingThrows     []string      `json:"saving_throws"` // ability codes: int, wis
	SkillChoose      int           `json:"skill_choose"`
	SkillFrom        []string      `json:"skill_from,omitempty"` // skill codes the class picks from
	Armor            []string      `json:"armor"`
	Weapons          []string      `json:"weapons"`
	Tools            []string      `json:"tools"`
	ToolChoices      []ToolChoice  `json:"tool_choices,omitempty"`      // Bard: 3 Musical Instruments
	ArmorCategories  []string      `json:"armor_categories,omitempty"`  // light, medium, heavy, shield
	WeaponCategories []string      `json:"weapon_categories,omitempty"` // simple, martial, martial:light
	Features         []Feature     `json:"features"`
	Spellcasting     *Spellcasting `json:"spellcasting,omitempty"`
	// the class table by level (a fixture may list only the levels it computes)
	Table []LevelRow `json:"table,omitempty"`
	// the starting equipment packages (2024: A, B); empty when the catalog has none structured
	Equipment []EquipmentOption `json:"equipment,omitempty"`
}

// EquipmentOption is one starting-equipment package, as the sheet describes it.
type EquipmentOption struct {
	Label string          `json:"label"`          // A, B
	Text  string          `json:"text,omitempty"` // what it gives, in words
	Picks []EquipmentPick `json:"picks,omitempty"`
}

// EquipmentPick is an item of the player's choice in a package: "a Musical Instrument of your choice".
type EquipmentPick struct {
	Categories []string `json:"categories"`
	Count      int      `json:"count"`
}

// ToolChoice is a class's tool pick: Count tools of any of these kinds.
type ToolChoice struct {
	Count int      `json:"count"`
	From  []string `json:"from"` // "Musical Instrument", "Artisan's Tools"
}

// Spellcasting is how a casting class or subclass casts; its numbers are in the table.
type Spellcasting struct {
	Ability     string `json:"ability"`     // int, wis, cha
	Progression string `json:"progression"` // full, half, half_up, third, pact
	// when its level 1+ spells change: long_rest (prepared from the class's list) or level_up
	// (learned at each level, as choices: Bard, Sorcerer, Warlock); "" is long_rest
	Change string `json:"change,omitempty"`
	// spells added to a spellbook at each level, levels 1-20 (Wizard: 6, then 2); empty when it keeps none
	Spellbook []int `json:"spellbook,omitempty"`
	// the class whose list it picks from, for a casting subclass (Eldritch Knight: Wizard)
	SpellList string `json:"spell_list,omitempty"`
}

// LevelRow is one row of a class table.
type LevelRow struct {
	Level         int            `json:"level"`
	Cantrips      int            `json:"cantrips"`
	Prepared      int            `json:"prepared"`
	Slots         []int          `json:"slots"` // spell levels 1-9
	PactSlots     int            `json:"pact_slots"`
	PactSlotLevel int            `json:"pact_slot_level"`
	Columns       map[string]any `json:"columns,omitempty"` // the table's other columns: Rages, Sneak Attack
}

// Feature is a class, subclass or species feature, with its effects (a repeated class feature
// carries them on its first level only).
type Feature struct {
	Name    string   `json:"name"`
	Level   int      `json:"level"`
	Effects []Effect `json:"effects,omitempty"`
}

// Level is one planned level.
type Level struct {
	Level    int    `json:"level"`
	HPMethod string `json:"hp_method"`
	HPValue  *int   `json:"hp_value"`
}

// Species is the character's species.
type Species struct {
	Name   string    `json:"name"`
	Slug   string    `json:"slug,omitempty"` // species:elf; slots of its choice points start with it
	Size   string    `json:"size"`           // "Medium", or "Medium or Small" (a choice)
	Speed  int       `json:"speed"`
	Traits []Feature `json:"traits"`
}

// Background is the character's background.
type Background struct {
	Name      string   `json:"name"`
	Slug      string   `json:"slug,omitempty"` // background:sage
	Abilities []string `json:"abilities"`      // the three a 2024 background's ASIs go to
	Skills    []string `json:"skills"`         // skill codes: arcana, history
	Tools     []string `json:"tools"`
	Feat      string   `json:"feat"` // as printed: "Magic Initiate (Wizard)"
	// the starting equipment packages, when the catalog has them structured
	Equipment []EquipmentOption `json:"equipment,omitempty"`
}

// Subclass is the character's subclass.
type Subclass struct {
	Name         string        `json:"name"`
	Slug         string        `json:"slug,omitempty"` // wizard_evoker
	Level        int           `json:"level"`          // the level it was chosen at
	Features     []Feature     `json:"features"`
	Spellcasting *Spellcasting `json:"spellcasting,omitempty"` // Eldritch Knight, Arcane Trickster
	Table        []LevelRow    `json:"table,omitempty"`
}

// Feat is a feat the character took (a choice of kind feat) and what it does.
type Feat struct {
	ChoiceID string   `json:"choice_id"`
	Name     string   `json:"name"`
	Slug     string   `json:"slug"` // feat:tough
	Category string   `json:"category,omitempty"`
	Effects  []Effect `json:"effects"`
}

// Choice is one of the character's choices.
type Choice struct {
	ID        string          `json:"id"`
	Level     int             `json:"level"`
	Kind      string          `json:"kind"`
	Slot      string          `json:"slot"`
	GrantedBy string          `json:"granted_by,omitempty"` // the granting choice's id
	RefName   string          `json:"ref_name,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

// Item is a carried item with what the catalog says it does.
type Item struct {
	Name               string            `json:"name"`
	Category           string            `json:"category"`    // armor, weapon, ...
	Subcategory        string            `json:"subcategory"` // Light Armor, Martial Melee Weapons, ...
	Equipped           bool              `json:"equipped"`
	Attuned            bool              `json:"attuned"`
	RequiresAttunement bool              `json:"requires_attunement"`
	LevelReceived      *int              `json:"level_received"`
	Properties         map[string]string `json:"properties,omitempty"` // weapons: Damage, Properties, Mastery
	Effects            []Effect          `json:"effects"`
}

// PreparedSpell is a class spell the character has prepared.
type PreparedSpell struct {
	Name  string `json:"name"`
	Level int    `json:"level"` // 0 for a cantrip
}

// Effect is one of the ETL's effect atoms (ttrpg-data-dnd/playwright-data/docs/
// effects-and-prerequisites-schema.md). Only the fields the sheet reads are here.
type Effect struct {
	Type        string        `json:"type"` // ac_set, hp_bonus, skill_proficiency, grant_spell, ...
	Text        string        `json:"text,omitempty"`
	Ability     string        `json:"ability,omitempty"`
	Amount      *int          `json:"amount,omitempty"`
	Max         *int          `json:"max,omitempty"`
	Value       *int          `json:"value,omitempty"`
	Skill       string        `json:"skill,omitempty"`
	Tool        string        `json:"tool,omitempty"`
	Language    string        `json:"language,omitempty"`
	Spell       string        `json:"spell,omitempty"` // spell slug
	Uses        string        `json:"uses,omitempty"`  // "1/long rest"
	DamageType  string        `json:"damage_type,omitempty"`
	Condition   string        `json:"condition,omitempty"`
	Mod         string        `json:"mod,omitempty"` // advantage | disadvantage
	On          []string      `json:"on,omitempty"`  // ability_check:stealth, saving_throw:int, initiative; armored, unarmored, no_shield, no_heavy_armor
	AddDex      *bool         `json:"add_dex,omitempty"`
	DexCap      *int          `json:"dex_cap,omitempty"`
	Kind        string        `json:"kind,omitempty"` // sense / speed kind, armor / weapon category
	Range       *int          `json:"range,omitempty"`
	Scale       string        `json:"scale,omitempty"`  // amount × level | pb
	Table       string        `json:"table,omitempty"`  // the amount (or die) is this class table column
	Min         *int          `json:"min,omitempty"`    // the least an ability-modifier amount can be
	Level       *int          `json:"level,omitempty"`  // from this character level
	Option      string        `json:"option,omitempty"` // only with this named option picked
	Choice      *EffectChoice `json:"choice,omitempty"`
	SpellFilter *SpellFilter  `json:"spell_filter,omitempty"`
}

// EffectChoice is the pick an effect asks for: count of from, or of anything when Any.
type EffectChoice struct {
	From  []string `json:"from,omitempty"`
	Any   bool     `json:"any,omitempty"`
	Count int      `json:"count,omitempty"`
}

// SpellFilter is which spells a spell pick may take.
type SpellFilter struct {
	Level    *int     `json:"level,omitempty"`     // exactly this level (0: a cantrip)
	MinLevel *int     `json:"min_level,omitempty"` // a class's spells: 1 up to the highest slot
	MaxLevel *int     `json:"max_level,omitempty"`
	Classes  []string `json:"classes,omitempty"`
	Schools  []string `json:"schools,omitempty"`
	Ritual   *bool    `json:"ritual,omitempty"`
}

// Modifier is a boon or bane.
type Modifier struct {
	Label     string          `json:"label"`
	Target    string          `json:"target"`
	Op        string          `json:"op"`
	Value     json.RawMessage `json:"value"`
	FromLevel *int            `json:"from_level"`
	Active    bool            `json:"active"`
	AddedAs   string          `json:"added_as"`
}
