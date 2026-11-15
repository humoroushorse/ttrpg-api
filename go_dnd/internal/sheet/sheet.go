package sheet

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// Contribution is one thing that adds up to a number.
type Contribution struct {
	Source string `json:"source"`
	Op     string `json:"op"` // base, add, set, min
	Value  int    `json:"value"`
	Note   string `json:"note,omitempty"`
}

// Stat is a computed number and what made it.
type Stat struct {
	Value         int            `json:"value"`
	Contributions []Contribution `json:"contributions"`
	Inactive      []Contribution `json:"inactive,omitempty"`
	// Roll is advantage, disadvantage or normal; Advantage / Disadvantage list the sources.
	Roll         string   `json:"roll,omitempty"`
	Advantage    []string `json:"advantage,omitempty"`
	Disadvantage []string `json:"disadvantage,omitempty"`
}

func (s *Stat) add(source, op string, value int, note string) {
	s.Contributions = append(s.Contributions, Contribution{Source: source, Op: op, Value: value, Note: note})
	switch op {
	case "set":
		s.Value = value
	default:
		s.Value += value
	}
}

// Ability is a score and its modifier.
type Ability struct {
	Score    Stat `json:"score"`
	Modifier int  `json:"modifier"`
}

// Skill is a skill bonus.
type Skill struct {
	Stat
	Ability    string `json:"ability"`
	Proficient bool   `json:"proficient"`
	Expertise  bool   `json:"expertise"` // picked; it only counts with proficiency
	// what the bonus adds: none, half (Jack of All Trades), proficient, expertise (counted)
	Proficiency string `json:"proficiency"`
}

// Save is a saving throw bonus.
type Save struct {
	Stat
	Proficient bool `json:"proficient"`
}

// Issue is something the player should look at. Errors break a hard rule; warnings don't block; info
// is an approved exception.
type Issue struct {
	Severity string `json:"severity"` // error | warning | info
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// FeatureRef is a feature the character has at this level.
type FeatureRef struct {
	Name   string `json:"name"`
	Level  int    `json:"level"`
	Source string `json:"source"`
}

// Defense is a resistance, immunity or vulnerability.
type Defense struct {
	Type    string `json:"type"`    // resistance, immunity, vulnerability
	Against string `json:"against"` // a damage type (fire) or a condition (charmed)
	Source  string `json:"source"`
}

// RollNote is an advantage or disadvantage that only applies sometimes ("to end the Poisoned
// condition"), so it can't be folded into a number.
type RollNote struct {
	Mod       string   `json:"mod"` // advantage | disadvantage
	On        []string `json:"on"`  // saving_throw, ability_check
	Condition string   `json:"condition,omitempty"`
	Source    string   `json:"source"`
	Text      string   `json:"text,omitempty"`
}

// Sheet is the computed sheet.
type Sheet struct {
	Level            int                `json:"level"`
	ProficiencyBonus Stat               `json:"proficiency_bonus"`
	Abilities        map[string]Ability `json:"abilities"`
	SavingThrows     map[string]Save    `json:"saving_throws"`
	Skills           map[string]Skill   `json:"skills"`
	Passive          map[string]Stat    `json:"passive"`
	Initiative       Stat               `json:"initiative"`
	ArmorClass       Stat               `json:"armor_class"`
	HitPoints        struct {
		Max     Stat   `json:"max"`
		HitDice string `json:"hit_dice"`
	} `json:"hit_points"`
	Speed         map[string]Stat     `json:"speed"`
	Senses        map[string]Stat     `json:"senses"` // darkvision, blindsight, ...: the range in feet
	Size          string              `json:"size"`
	Features      []FeatureRef        `json:"features"`
	Proficiencies map[string][]string `json:"proficiencies"`
	Resistances   []Defense           `json:"resistances"`
	Conditional   []RollNote          `json:"conditional"`
	Attacks       []Attack            `json:"attacks"`
	Spellcasting  *SpellcastingSheet  `json:"spellcasting"` // null when the character doesn't cast
	Spells        []GrantedSpell      `json:"spells"`       // from feats and species traits
	ClassTable    map[string]any      `json:"class_table,omitempty"`
	ChoicePoints  []ChoicePoint       `json:"choice_points"`
	Issues        []Issue             `json:"issues"`
	// NotComputed lists what the sheet can't work out yet (until the catalog has the data)
	NotComputed []string `json:"not_computed"`
}

// Abilities in sheet order.
var Abilities = []string{"str", "dex", "con", "int", "wis", "cha"}

var abilityNames = map[string]string{"strength": "str", "dexterity": "dex", "constitution": "con",
	"intelligence": "int", "wisdom": "wis", "charisma": "cha"}

// AbilityCode reads "Intelligence" (or "int") as int; "" when it isn't an ability.
func AbilityCode(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if code, ok := abilityNames[n]; ok {
		return code
	}
	if slices.Contains(Abilities, n) {
		return n
	}
	return ""
}

// SkillAbility is each skill's ability.
var SkillAbility = map[string]string{
	"acrobatics": "dex", "animal_handling": "wis", "arcana": "int", "athletics": "str", "deception": "cha",
	"history": "int", "insight": "wis", "intimidation": "cha", "investigation": "int", "medicine": "wis",
	"nature": "int", "perception": "wis", "performance": "cha", "persuasion": "cha", "religion": "int",
	"sleight_of_hand": "dex", "stealth": "dex", "survival": "wis",
}

// SkillCode reads "Sleight of Hand" as sleight_of_hand; "" when it isn't a skill.
func SkillCode(name string) string {
	code := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "_")
	if _, ok := SkillAbility[code]; ok {
		return code
	}
	return ""
}

var methodLabel = map[string]string{"point_buy": "Point buy", "standard_array": "Standard array", "rolled": "Rolled", "manual": "Manual"}

func mod(score int) int {
	if score >= 10 {
		return (score - 10) / 2
	}
	return -((11 - score) / 2)
}

func signed(n int) string { return fmt.Sprintf("%+d", n) }

// computer holds the snapshot filtered to the level being computed.
type computer struct {
	s           Snapshot
	level       int
	pb          int
	choices     []Choice
	byID        map[string]Choice // every choice
	active      map[string]Choice // the choices made by this level
	items       []Item
	modifiers   []Modifier
	off         []offModifier       // modifiers that don't apply at this level, for the hover
	sources     []effectSource      // feats and species traits with effects
	options     map[string][]string // grant's choice id -> the named options picked for it
	preset      map[string]bool     // feat choices whose option the background fixed
	asiCap      map[string]int      // grant's choice id -> its ability increases' cap (20, or 30 for boons)
	points      []ChoicePoint
	notComputed []string
	armored     bool // wearing armor (a shield isn't armor)
	heavy       bool // wearing Heavy armor
	shield      bool // using a shield
	sheet       *Sheet
}

type offModifier struct {
	Modifier
	reason string
}

// Compute is the sheet for s at s.Level.
func Compute(s Snapshot) Sheet {
	level := min(max(s.Level, 1), 20)
	c := &computer{s: s, level: level, pb: 2 + (level-1)/4, byID: map[string]Choice{}, active: map[string]Choice{},
		preset: map[string]bool{}, asiCap: map[string]int{}, sheet: &Sheet{Level: level}}
	for _, ch := range s.Choices {
		c.byID[ch.ID] = ch
		if ch.Level <= level {
			c.choices = append(c.choices, ch)
			c.active[ch.ID] = ch
		}
	}
	for _, it := range s.Items {
		if it.LevelReceived == nil || *it.LevelReceived <= level {
			c.items = append(c.items, it)
		}
	}
	for _, m := range s.Modifiers {
		switch {
		case !m.Active:
			c.off = append(c.off, offModifier{m, "switched off"})
		case m.FromLevel != nil && *m.FromLevel > level:
			c.off = append(c.off, offModifier{m, fmt.Sprintf("from level %d", *m.FromLevel)})
		default:
			c.modifiers = append(c.modifiers, m)
		}
	}
	sh := c.sheet
	sh.Resistances, sh.Conditional, sh.Issues = []Defense{}, []RollNote{}, []Issue{}
	c.notComputed = []string{
		"class resources and active features (Rage, Bardic Inspiration, Channel Divinity, Weapon Mastery picks)",
		"magic weapon and armor bonuses (+1 Longsword)",
		"carrying capacity",
	}
	sh.ProficiencyBonus = Stat{}
	sh.ProficiencyBonus.add(fmt.Sprintf("Level %d", level), "base", c.pb, "")
	c.gear()
	c.classTable()
	c.collectSources()
	c.abilities()
	c.saves()
	c.skills()
	c.initiative()
	c.armorClass()
	c.hitPoints()
	c.speed()
	c.senses()
	c.features()
	c.proficiencies()
	c.spellcasting()
	c.attacks()
	c.choicePoints()
	c.grantedSpells()
	c.unappliedModifiers()
	c.issues()
	sh.Issues = append(sh.Issues, s.RuleIssues...)
	sh.NotComputed = c.notComputed
	return *sh
}

// modifiersFor are the active modifiers on a target, in order.
func (c *computer) modifiersFor(target string) []Modifier {
	var out []Modifier
	for _, m := range c.modifiers {
		if m.Target == target {
			out = append(out, m)
		}
	}
	return out
}

func modifierSource(m Modifier) string {
	if m.AddedAs == "dm" {
		return "DM: " + m.Label
	}
	return m.Label
}

// number reads a modifier's value as an integer (2, -1); ok is false for dice or formulas.
func number(raw json.RawMessage) (int, bool) {
	var n float64
	if err := json.Unmarshal(raw, &n); err != nil || n != float64(int(n)) {
		return 0, false
	}
	return int(n), true
}

// applyModifiers adds the target's add/set/min modifiers, records adv/disadv, and lists the target's
// modifiers that don't apply at this level as inactive.
func (c *computer) applyModifiers(st *Stat, target string) {
	c.listOff(st, target)
	for _, m := range c.modifiersFor(target) {
		switch m.Op {
		case "add", "set":
			if n, ok := number(m.Value); ok {
				st.add(modifierSource(m), m.Op, n, "")
			}
		case "min": // "becomes at least" (Gauntlets of Ogre Power)
			if n, ok := number(m.Value); ok {
				if n > st.Value {
					st.add(modifierSource(m), "set", n, fmt.Sprintf("at least %d", n))
				} else {
					st.Inactive = append(st.Inactive, Contribution{Source: modifierSource(m), Op: "min", Value: n, Note: "already higher"})
				}
			}
		case "adv":
			st.Advantage = append(st.Advantage, modifierSource(m))
		case "disadv":
			st.Disadvantage = append(st.Disadvantage, modifierSource(m))
		}
	}
	setRoll(st)
}

func (c *computer) listOff(st *Stat, target string) {
	for _, m := range c.off {
		if m.Target == target {
			n, _ := number(m.Value)
			st.Inactive = append(st.Inactive, Contribution{Source: modifierSource(m.Modifier), Op: m.Op, Value: n, Note: m.reason})
		}
	}
}

func setRoll(st *Stat) {
	switch {
	case len(st.Advantage) > 0 && len(st.Disadvantage) == 0:
		st.Roll = "advantage"
	case len(st.Disadvantage) > 0 && len(st.Advantage) == 0:
		st.Roll = "disadvantage"
	case len(st.Advantage) > 0:
		st.Roll = "normal" // both cancel
	default:
		st.Roll = ""
	}
}

// rollModifiers adds the advantage / disadvantage that feats, traits and equipped items give on a
// roll ("saving_throw:int", "ability_check:stealth"); ones tied to a condition go to the sheet's
// conditional notes instead (once, from the caller that owns them).
func (c *computer) rollModifiers(st *Stat, on ...string) {
	matches := func(list []string) bool {
		return slices.ContainsFunc(list, func(o string) bool { return slices.Contains(on, o) })
	}
	for _, e := range c.effects(false, "roll_modifier") {
		if e.Condition == "" && matches(e.On) {
			c.addRoll(st, e.Mod, e.src.label)
		}
	}
	for _, it := range c.items {
		if !it.Equipped {
			continue
		}
		for _, e := range it.Effects {
			if e.Type == "roll_modifier" && e.Condition == "" && matches(e.On) {
				c.addRoll(st, e.Mod, it.Name)
			}
		}
	}
}

// rollBonuses adds the bonuses feats and features give to a roll (Aura of Protection on saving
// throws: the Charisma modifier, minimum +1).
func (c *computer) rollBonuses(st *Stat, on ...string) {
	for _, e := range c.effects(false, "roll_bonus") {
		if slices.ContainsFunc(e.On, func(o string) bool { return slices.Contains(on, o) }) {
			n, note := c.amount(e.Effect, c.level)
			st.add(e.src.label, "add", n, note)
		}
	}
}

// halfProficiency is Jack of All Trades: half the proficiency bonus (rounded down) on ability checks
// that don't already add it. Reports whether it applied.
func (c *computer) halfProficiency(st *Stat) bool {
	for _, e := range c.effects(false, "half_proficiency") {
		if slices.Contains(e.On, "ability_check") {
			st.add(e.src.label, "add", c.pb/2, "half proficiency bonus")
			return true
		}
	}
	return false
}

// classTable is the class table's other columns at this level (Rages, Martial Arts).
func (c *computer) classTable() {
	if cls := c.s.Class; cls != nil {
		if r := c.row(cls.Table); r != nil && len(r.Columns) > 0 {
			c.sheet.ClassTable = r.Columns
		}
	}
}

func (c *computer) addRoll(st *Stat, mod, source string) {
	switch mod {
	case "advantage":
		st.Advantage = append(st.Advantage, source)
	case "disadvantage":
		st.Disadvantage = append(st.Disadvantage, source)
	}
}

func (c *computer) abilities() {
	sh := c.sheet
	sh.Abilities = map[string]Ability{}
	label := methodLabel[c.s.AbilityMethod]
	if label == "" {
		label = "Base score"
	}
	type asi struct {
		source string
		level  int
		amount int
		cap    int
	}
	increases := map[string][]asi{}
	for _, ch := range c.choices {
		if ch.Kind != "asi" {
			continue
		}
		var v struct {
			Increases map[string]int `json:"increases"`
		}
		_ = json.Unmarshal(ch.Value, &v)
		source := fmt.Sprintf("Ability Score Improvement (level %d)", ch.Level)
		if g, ok := c.byID[ch.GrantedBy]; ok && g.RefName != "" {
			source = strings.ToUpper(g.Kind[:1]) + g.Kind[1:] + ": " + g.RefName
		}
		limit := 20
		if n, ok := c.asiCap[ch.GrantedBy]; ok {
			limit = n
		}
		for ability, amount := range v.Increases {
			increases[ability] = append(increases[ability], asi{source, ch.Level, amount, limit})
		}
	}
	for _, e := range c.effects(false, "ability_score_increase") {
		if e.Ability != "" && e.Amount != nil {
			limit := 20
			if e.Max != nil {
				limit = *e.Max
			}
			increases[e.Ability] = append(increases[e.Ability], asi{e.src.label, e.src.level, *e.Amount, limit})
		}
	}
	for _, a := range Abilities {
		st := Stat{}
		base, ok := c.s.BaseScores[a]
		if !ok {
			base = 10
			st.add("Not set yet", "base", base, "")
		} else {
			st.add(label, "base", base, "")
		}
		list := increases[a]
		sort.SliceStable(list, func(i, j int) bool { return list[i].level < list[j].level })
		for _, inc := range list {
			room := max(inc.cap-st.Value, 0)
			switch {
			case room == 0:
				st.Inactive = append(st.Inactive, Contribution{Source: inc.source, Op: "add", Value: inc.amount, Note: fmt.Sprintf("already %d (the cap)", inc.cap)})
			case inc.amount > room:
				st.add(inc.source, "add", room, fmt.Sprintf("+%d capped at %d", inc.amount, inc.cap))
			default:
				st.add(inc.source, "add", inc.amount, "")
			}
		}
		c.applyModifiers(&st, "ability."+a)
		if st.Value > 30 || st.Value < 1 {
			limit := 30
			if st.Value < 1 {
				limit = 1
			}
			st.add("Hard limit", "set", limit, "scores stay between 1 and 30")
		}
		st.Roll, st.Advantage, st.Disadvantage = "", nil, nil
		sh.Abilities[a] = Ability{Score: st, Modifier: mod(st.Value)}
	}
}

func (c *computer) abilityMod(a string) int { return c.sheet.Abilities[a].Modifier }

func abilityLabel(a string) string {
	if a == "" {
		return "Ability modifier"
	}
	return strings.ToUpper(a[:1]) + a[1:] + " modifier"
}

func (c *computer) saves() {
	proficient := map[string]string{}
	if cls := c.s.Class; cls != nil {
		for _, a := range cls.SavingThrows {
			proficient[a] = cls.Name
		}
	}
	for _, e := range c.effects(false, "saving_throw_proficiency") {
		if _, ok := proficient[e.Ability]; !ok && e.Ability != "" {
			proficient[e.Ability] = e.src.label
		}
	}
	for _, ch := range c.choices {
		if a := pickValue(ch); ch.Kind == "saving_throw" && a != "" {
			if _, ok := proficient[a]; !ok {
				proficient[a] = c.choiceSource(ch)
			}
		}
	}
	c.sheet.SavingThrows = map[string]Save{}
	for _, a := range Abilities {
		sv := Save{}
		sv.add(abilityLabel(a), "base", c.abilityMod(a), "")
		if from, ok := proficient[a]; ok {
			sv.Proficient = true
			sv.add("Proficiency ("+from+")", "add", c.pb, "")
		}
		c.rollBonuses(&sv.Stat, "saving_throw", "saving_throw:"+a)
		c.rollModifiers(&sv.Stat, "saving_throw", "saving_throw:"+a)
		c.applyModifiers(&sv.Stat, "save.all")
		c.applyModifiers(&sv.Stat, "save."+a)
		c.sheet.SavingThrows[a] = sv
	}
	for _, e := range c.effects(false, "roll_modifier") {
		if e.Condition != "" {
			c.sheet.Conditional = append(c.sheet.Conditional, RollNote{Mod: e.Mod, On: e.On, Condition: e.Condition, Source: e.src.label, Text: e.Text})
		}
	}
}

func (c *computer) skills() {
	pb := c.pb
	proficient := map[string]string{} // skill -> where the proficiency comes from
	sources := map[string][]string{}  // skill -> every source of it, to warn about duplicates
	if b := c.s.Background; b != nil {
		for _, sk := range b.Skills {
			if code := SkillCode(sk); code != "" {
				proficient[code] = "Background: " + b.Name
				sources[code] = append(sources[code], proficient[code])
			}
		}
	}
	expertise := map[string]string{}
	for _, e := range c.effects(false, "skill_proficiency", "expertise") {
		code := SkillCode(e.Skill)
		switch {
		case code == "":
		case e.Type == "skill_proficiency":
			if _, ok := proficient[code]; !ok {
				proficient[code] = e.src.label
			}
			sources[code] = append(sources[code], e.src.label)
		default:
			expertise[code] = e.src.label
		}
	}
	for _, ch := range c.choices {
		if ch.Kind != "skill" && ch.Kind != "expertise" {
			continue
		}
		code := SkillCode(pickValue(ch))
		if code == "" {
			continue
		}
		from := c.choiceSource(ch)
		if ch.Kind == "skill" {
			proficient[code] = from
			sources[code] = append(sources[code], from)
		} else {
			expertise[code] = from
		}
	}
	for _, code := range slices.Sorted(maps.Keys(sources)) {
		c.warnDuplicate(skillName(code), sources[code])
	}
	c.sheet.Skills = map[string]Skill{}
	for code, a := range SkillAbility {
		sk := Skill{Ability: a, Proficiency: "none"}
		sk.add(abilityLabel(a), "base", c.abilityMod(a), "")
		if from, ok := proficient[code]; ok {
			sk.Proficient, sk.Proficiency = true, "proficient"
			sk.add("Proficiency ("+from+")", "add", pb, "")
		} else if c.halfProficiency(&sk.Stat) {
			sk.Proficiency = "half"
		}
		if from, ok := expertise[code]; ok {
			sk.Expertise = true
			if sk.Proficient {
				sk.Proficiency = "expertise"
				sk.add("Expertise ("+from+")", "add", pb, "")
			} else {
				sk.Inactive = append(sk.Inactive, Contribution{Source: "Expertise (" + from + ")", Op: "add", Value: pb, Note: "needs proficiency"})
			}
		}
		c.rollBonuses(&sk.Stat, "ability_check", "ability_check:"+a, "ability_check:"+code)
		c.rollModifiers(&sk.Stat, "ability_check", "ability_check:"+a, "ability_check:"+code)
		c.applyModifiers(&sk.Stat, "skill."+code)
		c.sheet.Skills[code] = sk
	}
	c.sheet.Passive = map[string]Stat{}
	for _, code := range []string{"perception", "investigation", "insight"} {
		st := Stat{}
		st.add("Base", "base", 10, "")
		st.add(strings.ToUpper(code[:1])+code[1:], "add", c.sheet.Skills[code].Value, "")
		c.applyModifiers(&st, "passive."+code)
		st.Roll, st.Advantage, st.Disadvantage = "", nil, nil
		c.sheet.Passive[code] = st
	}
}

// choiceSource names where a choice came from: the choice that granted it, or its slot.
func (c *computer) choiceSource(ch Choice) string {
	if g, ok := c.byID[ch.GrantedBy]; ok && g.RefName != "" {
		return g.RefName
	}
	if strings.HasPrefix(ch.Slot, "class:") && c.s.Class != nil {
		return c.s.Class.Name
	}
	return ch.Slot
}

func (c *computer) initiative() {
	st := Stat{}
	st.add("Dex modifier", "base", c.abilityMod("dex"), "")
	addsPB := false
	for _, e := range c.effects(false, "initiative_bonus") {
		n, note := c.amount(e.Effect, c.level)
		st.add(e.src.label, "add", n, note)
		addsPB = addsPB || e.Scale == "pb"
	}
	if !addsPB { // Initiative is a Dexterity check: Jack of All Trades counts unless Alert added the bonus
		c.halfProficiency(&st)
	}
	c.rollModifiers(&st, "initiative", "ability_check", "ability_check:dex")
	c.applyModifiers(&st, "initiative")
	c.sheet.Initiative = st
}

type acBase struct {
	source  string
	value   int
	dex     int // the Dex part, already capped
	note    string
	ability string // another ability's modifier added (Unarmored Defense: con, wis)
	mod     int
}

func (c *computer) armorClass() {
	dex := c.abilityMod("dex")
	candidates := []acBase{{source: "Unarmored", value: 10, dex: dex}}
	var inactive []Contribution
	armorWorn, shields := 0, 0
	st := Stat{}
	for _, it := range c.items {
		isShield := it.Subcategory == "Shield"
		for _, e := range it.Effects {
			switch {
			case e.Type == "ac_set" && e.Value != nil && !isShield:
				b := acBase{source: it.Name, value: *e.Value}
				if e.AddDex != nil && *e.AddDex {
					b.dex = dex
					if e.DexCap != nil && dex > *e.DexCap {
						b.dex, b.note = *e.DexCap, fmt.Sprintf("Dex capped at %+d", *e.DexCap)
					}
				}
				if it.Equipped {
					candidates = append(candidates, b)
					armorWorn++
				} else {
					inactive = append(inactive, Contribution{Source: it.Name, Op: "base", Value: b.value + b.dex, Note: "not worn"})
				}
			case e.Type == "ac_bonus" && e.Amount != nil:
				if it.Equipped {
					shields++
					if shields == 1 || !isShield {
						st.Contributions = append(st.Contributions, Contribution{Source: it.Name, Op: "add", Value: *e.Amount})
					} else {
						inactive = append(inactive, Contribution{Source: it.Name, Op: "add", Value: *e.Amount, Note: "one shield at a time"})
					}
				} else {
					inactive = append(inactive, Contribution{Source: it.Name, Op: "add", Value: *e.Amount, Note: "not equipped"})
				}
			}
		}
	}
	for _, e := range c.effects(false, "ac_bonus") {
		if e.Amount == nil {
			continue
		}
		if ok, why := c.wears(e.On); !ok {
			inactive = append(inactive, Contribution{Source: e.src.label, Op: "add", Value: *e.Amount, Note: why})
			continue
		}
		st.Contributions = append(st.Contributions, Contribution{Source: e.src.label, Op: "add", Value: *e.Amount})
	}
	var featureBases []acBase // Unarmored Defense and the like
	for _, e := range c.effects(false, "ac_set") {
		if e.Value == nil {
			continue
		}
		b := acBase{source: e.src.label, value: *e.Value}
		if e.AddDex != nil && *e.AddDex {
			b.dex = dex
			if e.DexCap != nil && dex > *e.DexCap {
				b.dex, b.note = *e.DexCap, fmt.Sprintf("Dex capped at %+d", *e.DexCap)
			}
		}
		if e.Ability != "" {
			b.ability, b.mod = e.Ability, c.abilityMod(e.Ability)
		}
		if ok, why := c.wears(e.On); !ok {
			inactive = append(inactive, Contribution{Source: b.source, Op: "base", Value: b.value + b.dex + b.mod, Note: why})
			continue
		}
		featureBases = append(featureBases, b)
	}
	if armorWorn > 0 {
		candidates = candidates[1:] // wearing armor rules out the unarmored base
		inactive = append(inactive, Contribution{Source: "Unarmored", Op: "base", Value: 10 + dex, Note: "wearing armor"})
	}
	candidates = append(candidates, featureBases...)
	for _, m := range c.modifiersFor("ac") {
		if m.Op != "base" {
			continue
		}
		if n, ok := number(m.Value); ok {
			candidates = append(candidates, acBase{source: modifierSource(m), value: n})
			continue
		}
		var f struct {
			Value  int  `json:"value"`
			AddDex bool `json:"add_dex"`
			DexCap *int `json:"dex_cap"`
		}
		if json.Unmarshal(m.Value, &f) == nil {
			b := acBase{source: modifierSource(m), value: f.Value}
			if f.AddDex {
				b.dex = dex
				if f.DexCap != nil && dex > *f.DexCap {
					b.dex = *f.DexCap
				}
			}
			candidates = append(candidates, b)
		}
	}
	total := func(b acBase) int { return b.value + b.dex + b.mod }
	best := 0
	for i, b := range candidates {
		if total(b) > total(candidates[best]) {
			best = i
		}
	}
	for i, b := range candidates {
		if i != best {
			inactive = append(inactive, Contribution{Source: b.source, Op: "base", Value: total(b), Note: "lower than " + candidates[best].source})
		}
	}
	chosen := candidates[best]
	adds := st.Contributions
	st = Stat{}
	st.add(chosen.source, "base", chosen.value, "")
	if chosen.dex != 0 || chosen.note != "" {
		st.add("Dex modifier", "add", chosen.dex, chosen.note)
	}
	if chosen.ability != "" {
		st.add(abilityLabel(chosen.ability), "add", chosen.mod, "")
	}
	for _, a := range adds {
		st.add(a.Source, "add", a.Value, a.Note)
	}
	for _, m := range c.modifiersFor("ac") {
		if (m.Op == "add" || m.Op == "set") && json.Valid(m.Value) {
			if n, ok := number(m.Value); ok {
				st.add(modifierSource(m), m.Op, n, "")
			}
		}
	}
	st.Inactive = inactive
	c.listOff(&st, "ac")
	c.sheet.ArmorClass = st
	if armorWorn > 1 {
		c.warn("armor.two", "more than one suit of armor is equipped; the best one counts")
	}
	if shields > 1 {
		c.warn("armor.shields", "more than one shield is equipped; one counts")
	}
}

func (c *computer) hitPoints() {
	hp := &c.sheet.HitPoints
	cls := c.s.Class
	if cls == nil || cls.HitDieSize == 0 {
		hp.Max = Stat{Contributions: []Contribution{}}
		return
	}
	die := cls.HitDieSize
	planned := map[int]Level{}
	for _, l := range c.s.Levels {
		planned[l.Level] = l
	}
	st := Stat{}
	counted := 0
	for lv := 1; lv <= c.level; lv++ {
		l, ok := planned[lv]
		if !ok {
			continue
		}
		counted++
		switch {
		case lv == 1:
			st.add(fmt.Sprintf("Level 1 %s (max d%d)", cls.Name, die), "base", die, "")
		case l.HPMethod == "average" || l.HPValue == nil:
			st.add(fmt.Sprintf("Level %d (average d%d)", lv, die), "add", die/2+1, "")
		default:
			value := min(max(*l.HPValue, 1), die)
			note := ""
			if value != *l.HPValue {
				note = fmt.Sprintf("%d is outside 1-%d", *l.HPValue, die)
			}
			st.add(fmt.Sprintf("Level %d (%s d%d)", lv, l.HPMethod, die), "add", value, note)
		}
	}
	if counted > 0 {
		st.add(fmt.Sprintf("Con modifier × %d", counted), "add", c.abilityMod("con")*counted, signed(c.abilityMod("con"))+" per level")
	}
	for _, e := range c.effects(false, "hp_bonus") {
		n, note := c.amount(e.Effect, counted)
		st.add(e.src.label, "add", n, note)
	}
	for _, m := range c.modifiersFor("hp.per_level") {
		if n, ok := number(m.Value); ok && m.Op == "add" {
			st.add(modifierSource(m)+fmt.Sprintf(" × %d", counted), "add", n*counted, "")
		}
	}
	c.applyModifiers(&st, "hp.max")
	st.Roll, st.Advantage, st.Disadvantage = "", nil, nil
	if st.Value < counted { // at least 1 per level
		st.add("Minimum", "set", counted, "at least 1 HP per level")
	}
	hp.Max = st
	hp.HitDice = fmt.Sprintf("%dd%d", counted, die)
}

// speed: the best base for each kind (the species' walking speed, a Wood Elf's 35), plus increases.
// A speed with neither a value nor an amount equals the walking speed (Athlete's climb speed).
func (c *computer) speed() {
	type base struct {
		source string
		value  int
	}
	bases := map[string][]base{}
	if sp := c.s.Species; sp != nil && sp.Speed > 0 {
		bases["walk"] = []base{{sp.Name, sp.Speed}}
		c.sheet.Size = sp.Size
	} else {
		bases["walk"] = []base{{"Default (no species chosen)", 30}}
	}
	atoms := c.effects(false, "speed")
	for _, e := range atoms {
		if ok, _ := c.wears(e.On); e.Value != nil && ok {
			bases[e.Kind] = append(bases[e.Kind], base{e.src.label, *e.Value})
		}
	}
	speeds := map[string]Stat{}
	build := func(kind string) Stat {
		st := Stat{}
		list := bases[kind]
		best := 0
		for i, b := range list {
			if b.value > list[best].value {
				best = i
			}
		}
		if len(list) > 0 {
			st.add(list[best].source, "base", list[best].value, "")
			for i, b := range list {
				if i != best {
					st.Inactive = append(st.Inactive, Contribution{Source: b.source, Op: "base", Value: b.value, Note: "lower than " + list[best].source})
				}
			}
		}
		for _, e := range atoms {
			if e.Kind != kind || (e.Amount == nil && e.Table == "") {
				continue
			}
			n, note := c.amount(e.Effect, c.level)
			if ok, why := c.wears(e.On); !ok {
				st.Inactive = append(st.Inactive, Contribution{Source: e.src.label, Op: "add", Value: n, Note: why})
			} else if n != 0 {
				st.add(e.src.label, "add", n, note)
			}
		}
		c.applyModifiers(&st, "speed."+kind)
		st.Roll, st.Advantage, st.Disadvantage = "", nil, nil
		return st
	}
	speeds["walk"] = build("walk")
	for _, e := range atoms {
		if e.Kind == "walk" || e.Kind == "" {
			continue
		}
		if _, done := speeds[e.Kind]; done {
			continue
		}
		if ok, _ := c.wears(e.On); !ok && e.Value == nil && e.Amount == nil && e.Table == "" {
			continue // Roving's climb speed in Heavy armor
		}
		if len(bases[e.Kind]) == 0 {
			bases[e.Kind] = []base{{e.src.label + " (equal to walking speed)", speeds["walk"].Value}}
		}
		speeds[e.Kind] = build(e.Kind)
	}
	for _, kind := range c.setTargets("speed.") { // a Fly spell as a modifier: speed.fly set 60
		if _, done := speeds[kind]; !done {
			speeds[kind] = build(kind)
		}
	}
	if c.sheet.Size == "" && c.s.Species != nil {
		c.sheet.Size = c.s.Species.Size
	}
	if sp := c.s.Species; sp != nil && strings.Contains(sp.Size, " or ") {
		for _, ch := range c.choices {
			if ch.Kind == "option" && ch.Slot == speciesSlot(sp)+":size" {
				c.sheet.Size = pickValue(ch)
			}
		}
	}
	c.sheet.Speed = speeds
}

// senses: each sense's longest range, from traits, feats and equipped items.
func (c *computer) senses() {
	type sense struct {
		source string
		feet   int
	}
	found := map[string][]sense{}
	var extends []activeEffect // Umbral Sight: 60 feet, or 60 more if you already have it
	for _, e := range c.effects(false, "sense") {
		switch {
		case e.Kind == "":
		case e.Amount != nil:
			extends = append(extends, e)
		case e.Range != nil:
			found[e.Kind] = append(found[e.Kind], sense{e.src.label, *e.Range})
		}
	}
	for _, it := range c.items {
		for _, e := range it.Effects {
			if e.Type == "sense" && e.Range != nil && e.Kind != "" && (it.Equipped || it.Attuned) {
				found[e.Kind] = append(found[e.Kind], sense{it.Name, *e.Range})
			}
		}
	}
	extended := map[string][]activeEffect{}
	for _, e := range extends {
		if len(found[e.Kind]) > 0 {
			extended[e.Kind] = append(extended[e.Kind], e)
		} else if e.Range != nil {
			found[e.Kind] = append(found[e.Kind], sense{e.src.label, *e.Range})
		}
	}
	for _, kind := range c.setTargets("sense.") { // a Darkvision spell as a modifier: sense.darkvision set 150
		if _, ok := found[kind]; !ok {
			found[kind] = nil
		}
	}
	out := map[string]Stat{}
	for kind, list := range found {
		st := Stat{}
		best := 0
		for i, s := range list {
			if s.feet > list[best].feet {
				best = i
			}
		}
		if len(list) > 0 {
			st.add(list[best].source, "base", list[best].feet, "")
		}
		for i, s := range list {
			if i != best {
				st.Inactive = append(st.Inactive, Contribution{Source: s.source, Op: "base", Value: s.feet, Note: "shorter than " + list[best].source})
			}
		}
		for _, e := range extended[kind] {
			st.add(e.src.label, "add", *e.Amount, "")
		}
		c.applyModifiers(&st, "sense."+kind)
		st.Roll, st.Advantage, st.Disadvantage = "", nil, nil
		out[kind] = st
	}
	c.sheet.Senses = out
}

// setTargets are the kinds a "set" modifier gives under a prefix (speed.fly -> fly).
func (c *computer) setTargets(prefix string) []string {
	var out []string
	for _, m := range c.modifiers {
		if kind, ok := strings.CutPrefix(m.Target, prefix); ok && m.Op == "set" && kind != "" && !slices.Contains(out, kind) {
			out = append(out, kind)
		}
	}
	return out
}

func (c *computer) features() {
	out := []FeatureRef{}
	if cls := c.s.Class; cls != nil {
		for _, f := range cls.Features {
			if f.Level <= c.level {
				out = append(out, FeatureRef{Name: f.Name, Level: f.Level, Source: cls.Name})
			}
		}
	}
	if sub := c.s.Subclass; sub != nil && sub.Level <= c.level {
		for _, f := range sub.Features {
			if f.Level <= c.level {
				out = append(out, FeatureRef{Name: f.Name, Level: f.Level, Source: sub.Name})
			}
		}
	}
	if sp := c.s.Species; sp != nil {
		for _, t := range sp.Traits {
			if t.Level <= c.level {
				out = append(out, FeatureRef{Name: t.Name, Level: max(t.Level, 1), Source: sp.Name})
			}
		}
	}
	for _, ch := range c.choices {
		if ch.Kind == "feat" && ch.RefName != "" {
			out = append(out, FeatureRef{Name: ch.RefName, Level: ch.Level, Source: "Feat (" + c.choiceSource(ch) + ")"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Level < out[j].Level })
	c.sheet.Features = out
}

var categoryNames = map[string]string{"light": "Light armor", "medium": "Medium armor", "heavy": "Heavy armor",
	"shield": "Shields", "simple": "Simple weapons", "martial": "Martial weapons", "improvised": "Improvised weapons"}

func categoryName(kind string) string {
	if name, ok := categoryNames[kind]; ok {
		return name
	}
	if base, prop, ok := strings.Cut(kind, ":"); ok {
		return categoryNames[base] + " with the " + strings.ToUpper(prop[:1]) + prop[1:] + " property"
	}
	return kind
}

// namesOr is the categories by name (Light armor, Shields), or the class's text when it has none.
func namesOr(categories, text []string) []string {
	if len(categories) == 0 {
		return text
	}
	out := make([]string, 0, len(categories))
	for _, k := range categories {
		out = append(out, categoryName(k))
	}
	return out
}

// isToolPick: a background's "Choose one kind of Gaming Set" is a pick, not a tool.
func isToolPick(tool string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(tool)), "choose")
}

func (c *computer) proficiencies() {
	p := map[string][]string{"armor": {}, "weapons": {}, "tools": {}, "languages": {}}
	toolNames := map[string]string{}     // lowercased -> as first listed
	toolSources := map[string][]string{} // lowercased -> every source of it, to warn about duplicates
	addTool := func(name, source string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, ok := toolNames[key]; !ok {
			toolNames[key] = name
		}
		toolSources[key] = append(toolSources[key], source)
		p["tools"] = append(p["tools"], name)
	}
	if cls := c.s.Class; cls != nil {
		p["armor"] = append(p["armor"], namesOr(cls.ArmorCategories, cls.Armor)...)
		p["weapons"] = append(p["weapons"], namesOr(cls.WeaponCategories, cls.Weapons)...)
		for _, t := range cls.Tools {
			addTool(t, cls.Name)
		}
	}
	if b := c.s.Background; b != nil {
		for _, t := range b.Tools {
			if !isToolPick(t) { // "Choose one kind of Gaming Set" is a choice point
				addTool(t, "Background: "+b.Name)
			}
		}
	}
	for _, e := range c.effects(false, "armor_proficiency", "weapon_proficiency", "tool_proficiency", "language") {
		switch e.Type {
		case "armor_proficiency":
			p["armor"] = append(p["armor"], categoryName(e.Kind)+" ("+e.src.label+")")
		case "weapon_proficiency":
			p["weapons"] = append(p["weapons"], categoryName(e.Kind)+" ("+e.src.label+")")
		case "tool_proficiency":
			addTool(e.Tool, e.src.label)
		case "language":
			p["languages"] = append(p["languages"], e.Language)
		}
	}
	for _, ch := range c.choices {
		s := pickValue(ch)
		switch {
		case s == "":
		case ch.Kind == "tool":
			addTool(s, c.choiceSource(ch))
		case ch.Kind == "language":
			p["languages"] = append(p["languages"], s)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(toolSources)) {
		c.warnDuplicate(toolNames[key], toolSources[key])
	}
	for _, e := range c.effects(false, "resistance", "immunity", "vulnerability") {
		against := e.DamageType
		if against == "" {
			against = e.Condition
		}
		c.sheet.Resistances = append(c.sheet.Resistances, Defense{Type: e.Type, Against: against, Source: e.src.label})
	}
	for _, ch := range c.choices {
		if t := pickValue(ch); ch.Kind == "resistance" && t != "" {
			source := c.choiceSource(ch)
			if src := c.sourceOf(ch.GrantedBy); src != nil {
				source = src.label
			}
			c.sheet.Resistances = append(c.sheet.Resistances, Defense{Type: "resistance", Against: t, Source: source})
		}
	}
	for _, m := range c.modifiers {
		kind := map[string]string{"resist": "resistance", "immune": "immunity", "vuln": "vulnerability"}[m.Op]
		var s string
		if kind != "" && json.Unmarshal(m.Value, &s) == nil && s != "" {
			c.sheet.Resistances = append(c.sheet.Resistances, Defense{Type: kind, Against: s, Source: modifierSource(m)})
		}
	}
	for key, list := range p { // Herbalism Kit from the background and from Warrior of Mercy is one proficiency
		seen := map[string]bool{}
		unique := list[:0]
		for _, item := range list {
			if k := strings.ToLower(strings.TrimSpace(item)); !seen[k] {
				seen[k] = true
				unique = append(unique, item)
			}
		}
		p[key] = unique
	}
	c.sheet.Proficiencies = p
}

var appliedTargets = []string{"ac", "initiative", "hp.max", "hp.per_level", "save.all", "spell.dc", "spell.attack",
	"attack.melee", "attack.ranged"}

// unappliedModifiers warns about active modifiers the sheet doesn't use yet.
func (c *computer) unappliedModifiers() {
	for _, m := range c.modifiers {
		target := m.Target
		suffix := func(prefix string) string { return strings.TrimPrefix(target, prefix) }
		applied := slices.Contains(appliedTargets, target) ||
			(strings.HasPrefix(target, "ability.") && slices.Contains(Abilities, suffix("ability."))) ||
			(strings.HasPrefix(target, "save.") && slices.Contains(Abilities, suffix("save."))) ||
			(strings.HasPrefix(target, "skill.") && SkillAbility[suffix("skill.")] != "") ||
			(strings.HasPrefix(target, "speed.") && c.sheet.Speed[suffix("speed.")].Contributions != nil) ||
			(strings.HasPrefix(target, "sense.") && c.sheet.Senses[suffix("sense.")].Contributions != nil) ||
			slices.Contains([]string{"passive.perception", "passive.investigation", "passive.insight"}, target)
		opApplied := slices.Contains([]string{"add", "set", "min", "adv", "disadv", "base", "resist", "immune", "vuln"}, m.Op)
		if m.Op == "resist" || m.Op == "immune" || m.Op == "vuln" {
			applied = true
		}
		if !applied || !opApplied {
			c.warn("modifier.unapplied", fmt.Sprintf("%q (%s %s) isn't applied by the sheet yet", m.Label, m.Op, m.Target))
		} else if (m.Op == "add" || m.Op == "set" || m.Op == "min") && target != "ac" {
			if _, ok := number(m.Value); !ok {
				c.warn("modifier.unapplied", fmt.Sprintf("%q: only whole-number values are applied so far", m.Label))
			}
		}
	}
}

func (c *computer) warn(code, message string) {
	c.sheet.Issues = append(c.sheet.Issues, Issue{Severity: "warning", Code: code, Message: message})
}

// warnDuplicate: a proficiency from more than one source. Allowed; the player may change either source.
func (c *computer) warnDuplicate(name string, sources []string) {
	if len(sources) > 1 {
		c.warn("proficiency.duplicate", fmt.Sprintf("Proficient in %s %d times (%s): the extra adds nothing",
			name, len(sources), strings.Join(sources, ", ")))
	}
}

// skillName is a skill code in words: sleight_of_hand -> Sleight of Hand.
func skillName(code string) string {
	words := strings.Split(code, "_")
	for i, w := range words {
		if w != "of" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// issues are the choices still to make and the rules the build breaks.
func (c *computer) issues() {
	s := c.s
	if s.Class == nil {
		c.warn("class.missing", "no class: plan level 1")
	}
	for lv := 1; lv <= c.level; lv++ {
		found := false
		for _, l := range s.Levels {
			found = found || l.Level == lv
		}
		if !found {
			c.warn("level.unplanned", fmt.Sprintf("level %d isn't planned yet (current level %d)", lv, c.level))
			break
		}
	}
	if s.Species == nil {
		c.warn("species.missing", "no species chosen")
	}
	if s.Background == nil {
		c.warn("background.missing", "no background chosen")
	}
	if s.SubclassLevel > 0 && c.level >= s.SubclassLevel && (s.Subclass == nil || s.Subclass.Level > c.level) {
		c.warn("subclass.missing", fmt.Sprintf("choose a subclass (at level %d)", s.SubclassLevel))
	}
	if cls := s.Class; cls != nil {
		chosen := 0
		for _, ch := range c.choices {
			if ch.Kind == "skill" && strings.HasPrefix(ch.Slot, "class:") && ch.GrantedBy == "" {
				chosen++
			}
		}
		if chosen < cls.SkillChoose {
			c.warn("class.skills", fmt.Sprintf("%s: choose %d skills (%d chosen)", cls.Name, cls.SkillChoose, chosen))
		} else if chosen > cls.SkillChoose {
			c.warn("class.skills", fmt.Sprintf("%s: %d skills chosen, the class gives %d", cls.Name, chosen, cls.SkillChoose))
		}
		for _, f := range cls.Features {
			if f.Level > c.level || !strings.Contains(strings.ToLower(f.Name), "ability score improvement") {
				continue
			}
			taken := false
			for _, ch := range c.choices {
				taken = taken || isASIAnswer(ch, classSlot(cls), f.Level)
			}
			if !taken {
				c.warn("asi.missing", fmt.Sprintf("level %d: choose an Ability Score Improvement or a feat", f.Level))
			}
		}
	}
	if b := s.Background; b != nil && len(b.Abilities) > 0 {
		total, used := 0, map[string]int{}
		var bgID string
		for _, ch := range s.Choices {
			if ch.Kind == "background" {
				bgID = ch.ID
			}
		}
		for _, ch := range s.Choices {
			if ch.Kind != "asi" || ch.GrantedBy != bgID || bgID == "" {
				continue
			}
			var v struct {
				Increases map[string]int `json:"increases"`
			}
			_ = json.Unmarshal(ch.Value, &v)
			for a, n := range v.Increases {
				total += n
				used[a] += n
				if !slices.Contains(b.Abilities, a) {
					c.warn("background.asi", fmt.Sprintf("%s's ability increases go to %s, not %s", b.Name, strings.Join(b.Abilities, ", "), a))
				}
			}
		}
		switch {
		case total == 0:
			c.warn("background.asi", fmt.Sprintf("%s: choose +2/+1 or +1/+1/+1 across %s", b.Name, strings.Join(b.Abilities, ", ")))
		case total != 3:
			c.warn("background.asi", fmt.Sprintf("%s: the increases add up to +%d, not +3", b.Name, total))
		}
	}
	attuned := 0
	for _, it := range c.items {
		if it.Attuned && it.RequiresAttunement {
			attuned++
		}
	}
	if attuned > 3 {
		c.warn("items.attunement", fmt.Sprintf("%d items attuned; the limit is 3", attuned))
	}
	if _, ok := s.BaseScores["str"]; !ok {
		c.warn("scores.missing", "ability scores aren't set yet")
	}
}
