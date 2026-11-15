package sheet

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// effectSource is a feat, species trait, or class or subclass feature whose effect atoms apply to
// the character.
type effectSource struct {
	label     string // "Feat: Tough", "Dwarf: Dwarven Toughness", "Monk: Unarmored Defense"
	slot      string // what its choice points' slots start with: feat:tough, class:bard:expertise
	grantedBy string // the choice its picks hang off (the feat's, species' or subclass's); "" for a class feature
	level     int    // the level it was gained at
	effects   []Effect
	slots     []string // per atom: the slot of its pick ("" when it asks for none)
}

// activeEffect is an atom that applies at this level, with where it comes from.
type activeEffect struct {
	Effect
	src *effectSource
}

// ChoicePoint is a pick the build asks for and how far it's answered. The UI answers one by posting
// choices with this slot, granted_by_id and one of these kinds.
type ChoicePoint struct {
	Slot         string       `json:"slot"`
	Kinds        []string     `json:"kinds"`  // the choice kinds that answer it
	Source       string       `json:"source"` // "Feat: Magic Initiate"
	GrantedByID  string       `json:"granted_by_id,omitempty"`
	Level        int          `json:"level"`
	Count        int          `json:"count"`
	Chosen       int          `json:"chosen"`
	ChoiceIDs    []string     `json:"choice_ids,omitempty"` // the choices that answer it, so the UI needn't match them
	From         []string     `json:"from,omitempty"`
	Any          bool         `json:"any,omitempty"`
	Amount       int          `json:"amount,omitempty"`        // asi: up to this much per ability
	AbilityCount int          `json:"ability_count,omitempty"` // asi: amount × this many points in all
	SpellFilter  *SpellFilter `json:"spell_filter,omitempty"`
	Text         string       `json:"text,omitempty"`
	// takes any number (spells copied into a spellbook): count is however many are chosen
	Open bool `json:"open,omitempty"`
	// what each value in From gives (the starting equipment packages)
	Options []PointOption `json:"options,omitempty"`

	match    func(Choice) bool // when set, which choices answer it (older slots matched loosely)
	reported bool              // a dedicated check reports it (class.skills, asi.missing, ...)
}

// PointOption is one value a choice point offers, in words ("A": "Costume ×2, Mirror, ... 11 GP").
type PointOption struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// pointKinds are the choice kinds that answer an atom's pick.
var pointKinds = map[string][]string{
	"ability_score_increase":    {"asi"},
	"skill_proficiency":         {"skill"},
	"expertise":                 {"expertise"},
	"tool_proficiency":          {"tool"},
	"skill_or_tool_proficiency": {"skill", "tool"},
	"language":                  {"language"},
	"saving_throw_proficiency":  {"saving_throw"},
	"resistance":                {"resistance"},
	"grant_spell":               {"spell"},
	"spellcasting_ability":      {"spellcasting_ability"},
	"option":                    {"option"},
	"grant_feat":                {"feat"},
}

// valueKey is the field of a choice's value that holds the pick.
var valueKey = map[string]string{"skill": "skill", "expertise": "skill", "tool": "tool", "language": "language",
	"saving_throw": "ability", "resistance": "damage_type", "option": "option", "spellcasting_ability": "ability",
	"equipment_option": "option"}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// pickValue reads a choice's pick: {"skill": "arcana"} -> arcana.
func pickValue(ch Choice) string {
	var v map[string]any
	_ = json.Unmarshal(ch.Value, &v)
	s, _ := v[valueKey[ch.Kind]].(string)
	return s
}

func (c *computer) choiceOfKind(kind string) string {
	for _, ch := range c.choices {
		if ch.Kind == kind {
			return ch.ID
		}
	}
	return ""
}

var featOption = regexp.MustCompile(`^\s*(.*?)\s*\(([^)]*)\)\s*$`)

// backgroundFeat splits "Magic Initiate (Wizard)" into the feat and its option.
func backgroundFeat(text string) (name, option string) {
	if m := featOption.FindStringSubmatch(text); m != nil {
		return m[1], m[2]
	}
	return strings.TrimSpace(text), ""
}

// slotBase is "<kind>:<slug>": the catalog stores bare slugs (elf), the ETL's files prefixed ones
// (species:elf); both give species:elf.
func slotBase(kind, slug, name string) string {
	if slug = strings.TrimPrefix(slug, kind+":"); slug == "" {
		slug = slugify(name)
	}
	return kind + ":" + slug
}

func speciesSlot(sp *Species) string { return slotBase("species", sp.Slug, sp.Name) }

func classSlot(cls *Class) string { return slotBase("class", cls.Slug, cls.Name) }

func subclassSlot(sub *Subclass) string { return slotBase("subclass", sub.Slug, sub.Name) }

func backgroundSlot(b *Background) string { return slotBase("background", b.Slug, b.Name) }

// collectSources gathers the species traits and feats with effects, the options their choices
// picked (a background's "Magic Initiate (Wizard)" picks Wizard for that feat), and each grant's
// ability increase cap.
func (c *computer) collectSources() {
	c.options = map[string][]string{}
	for _, ch := range c.choices {
		if ch.Kind == "option" && ch.GrantedBy != "" {
			c.options[ch.GrantedBy] = append(c.options[ch.GrantedBy], pickValue(ch))
		}
	}
	if b := c.s.Background; b != nil {
		if name, option := backgroundFeat(b.Feat); option != "" {
			bg := c.choiceOfKind("background")
			for _, ch := range c.choices {
				if ch.Kind == "feat" && bg != "" && ch.GrantedBy == bg && strings.EqualFold(ch.RefName, name) {
					c.options[ch.ID] = append(c.options[ch.ID], option)
					c.preset[ch.ID] = true
				}
			}
		}
	}
	if sp := c.s.Species; sp != nil {
		id := c.choiceOfKind("species")
		for _, t := range sp.Traits {
			if len(t.Effects) > 0 && max(t.Level, 1) <= c.level {
				c.sources = append(c.sources, effectSource{label: sp.Name + ": " + t.Name,
					slot: speciesSlot(sp) + ":" + slugify(t.Name), grantedBy: id, level: max(t.Level, 1), effects: t.Effects})
			}
		}
	}
	if cls := c.s.Class; cls != nil {
		for _, f := range cls.Features {
			if len(f.Effects) > 0 && f.Level <= c.level {
				c.sources = append(c.sources, effectSource{label: cls.Name + ": " + f.Name,
					slot: classSlot(cls) + ":" + slugify(f.Name), level: f.Level, effects: f.Effects})
			}
		}
	}
	if sub := c.s.Subclass; sub != nil && sub.Level <= c.level {
		id := c.choiceOfKind("subclass")
		for _, f := range sub.Features {
			if len(f.Effects) > 0 && f.Level <= c.level {
				c.sources = append(c.sources, effectSource{label: sub.Name + ": " + f.Name,
					slot: subclassSlot(sub) + ":" + slugify(f.Name), grantedBy: id, level: f.Level, effects: f.Effects})
			}
		}
	}
	for _, f := range c.s.Feats {
		ch, ok := c.active[f.ChoiceID]
		if !ok {
			continue
		}
		c.sources = append(c.sources, effectSource{label: "Feat: " + f.Name, slot: slotBase("feat", f.Slug, f.Name), grantedBy: f.ChoiceID,
			level: ch.Level, effects: f.Effects})
	}
	for i := range c.sources {
		src := &c.sources[i]
		src.slots = atomSlots(src)
		for j, e := range src.effects {
			if e.Type == "ability_score_increase" && e.Choice != nil && e.Max != nil {
				c.asiCap[src.grantedBy] = *e.Max
			}
			if e.Type == "option" && src.grantedBy == "" { // a class feature's option (Divine Order)
				for _, ch := range c.choices {
					if ch.Kind == "option" && ch.GrantedBy == "" && ch.Slot == src.slots[j] {
						c.options[optionKey(src)] = append(c.options[optionKey(src)], pickValue(ch))
					}
				}
			}
		}
	}
}

// optionKey is whose options an atom's option is looked up in: the grant's choice (a species' traits
// share one), or a class feature's own slot.
func optionKey(src *effectSource) string {
	if src.grantedBy != "" {
		return src.grantedBy
	}
	return "slot:" + src.slot
}

// atomSlots is the slot of each atom's pick ("" when it asks for none): the source's slot, the atom
// type, the option it belongs to, and a number from the second of a kind on. Numbered over every
// atom, so a level-gated one keeps its slot.
func atomSlots(src *effectSource) []string {
	slots := make([]string, len(src.effects))
	seen := map[string]int{}
	for i, e := range src.effects {
		if e.Choice == nil || pointKinds[e.Type] == nil {
			continue
		}
		key := e.Type
		if e.Option != "" {
			key += ":" + slugify(e.Option)
		}
		seen[key]++
		slots[i] = src.slot + ":" + key
		if seen[key] > 1 {
			slots[i] += fmt.Sprintf(":%d", seen[key])
		}
	}
	return slots
}

// applies is whether an atom counts at this level: it's reached its level and its option is picked.
func (c *computer) applies(e Effect, src *effectSource) bool {
	if e.Level != nil && *e.Level > c.level {
		return false
	}
	return e.Option == "" || slices.Contains(c.options[optionKey(src)], e.Option)
}

// wears reports whether the armor an atom needs is worn (Defense: armored; Unarmored Defense:
// unarmored, no_shield; Fast Movement: no_heavy_armor), and why not.
func (c *computer) wears(on []string) (bool, string) {
	for _, cond := range on {
		switch {
		case cond == "armored" && !c.armored:
			return false, "only while wearing armor"
		case cond == "unarmored" && c.armored:
			return false, "not while wearing armor"
		case cond == "no_shield" && c.shield:
			return false, "not while using a Shield"
		case cond == "no_heavy_armor" && c.heavy:
			return false, "not in Heavy armor"
		}
	}
	return true, ""
}

// gear notes what's worn, for the armor conditions.
func (c *computer) gear() {
	for _, it := range c.items {
		if !it.Equipped {
			continue
		}
		if it.Subcategory == "Shield" {
			c.shield = true
			continue
		}
		for _, e := range it.Effects {
			if e.Type == "ac_set" {
				c.armored = true
				c.heavy = c.heavy || strings.Contains(strings.ToLower(it.Subcategory), "heavy")
			}
		}
	}
}

// tableText is a class table column at this level ("1d8", "+10 ft.", "3").
func (c *computer) tableText(column string) string {
	v, ok := c.sheet.ClassTable[column]
	if !ok {
		return ""
	}
	return fmt.Sprint(v)
}

var leadingInt = regexp.MustCompile(`[-+]?\d+`)

// tableNumber is a class table column's number at this level: "+10 ft." -> 10, "—" -> 0.
func (c *computer) tableNumber(column string) int {
	n := 0
	if m := leadingInt.FindString(c.tableText(column)); m != "" {
		_, _ = fmt.Sscan(m, &n)
	}
	return n
}

// effects are the atoms of these types that apply, fixed ones only (no pick to make) unless picks.
func (c *computer) effects(picks bool, types ...string) []activeEffect {
	var out []activeEffect
	for i := range c.sources {
		src := &c.sources[i]
		for _, e := range src.effects {
			if slices.Contains(types, e.Type) && (picks || e.Choice == nil) && c.applies(e, src) {
				out = append(out, activeEffect{e, src})
			}
		}
	}
	return out
}

// amount is what an atom adds: an ability's modifier (with its minimum), a class table column, or
// its amount, times the level or proficiency bonus when it scales.
func (c *computer) amount(e Effect, levels int) (int, string) {
	switch {
	case e.Ability != "":
		n, note := c.abilityMod(e.Ability), abilityLabel(e.Ability)
		if e.Min != nil && n < *e.Min {
			n, note = *e.Min, note+fmt.Sprintf(", minimum %+d", *e.Min)
		}
		return n, note
	case e.Table != "":
		return c.tableNumber(e.Table), e.Table + " (class table)"
	}
	n := 0
	if e.Amount != nil {
		n = *e.Amount
	}
	switch e.Scale {
	case "level":
		return n * levels, fmt.Sprintf("%+d × %d levels", n, levels)
	case "pb":
		return n * c.pb, "proficiency bonus"
	}
	return n, ""
}

// sourceOf is the effect source a choice's picks hang off.
func (c *computer) sourceOf(grantedBy string) *effectSource {
	for i := range c.sources {
		if c.sources[i].grantedBy == grantedBy {
			return &c.sources[i]
		}
	}
	return nil
}

// --- choice points ---

func (c *computer) addPoint(p ChoicePoint) {
	if p.Count == 0 && !p.Open {
		p.Count = 1
	}
	for _, ch := range c.choices {
		if p.answers(ch) {
			p.Chosen++
			p.ChoiceIDs = append(p.ChoiceIDs, ch.ID)
		}
	}
	if p.Open {
		p.Count = p.Chosen
	}
	c.points = append(c.points, p)
}

func (p ChoicePoint) answers(ch Choice) bool {
	if p.match != nil {
		return p.match(ch)
	}
	return ch.GrantedBy == p.GrantedByID && ch.Slot == p.Slot && slices.Contains(p.Kinds, ch.Kind)
}

// choicePoints lists every pick the build asks for: the origin, the class's, and each atom's.
func (c *computer) choicePoints() {
	s := c.s
	noGrant := func(kinds ...string) func(Choice) bool {
		return func(ch Choice) bool { return ch.GrantedBy == "" && slices.Contains(kinds, ch.Kind) }
	}
	c.addPoint(ChoicePoint{Slot: "origin:species", Kinds: []string{"species"}, Source: "Origin", Level: 1,
		match: noGrant("species"), reported: true})
	c.addPoint(ChoicePoint{Slot: "origin:background", Kinds: []string{"background"}, Source: "Origin", Level: 1,
		match: noGrant("background"), reported: true})
	if cls := s.Class; cls != nil {
		base := classSlot(cls)
		c.addPoint(ChoicePoint{Slot: base + ":skills", Kinds: []string{"skill"}, Source: cls.Name, Level: 1,
			Count: cls.SkillChoose, From: cls.SkillFrom, reported: true,
			match: func(ch Choice) bool {
				return ch.Kind == "skill" && ch.GrantedBy == "" && strings.HasPrefix(ch.Slot, "class:")
			}})
		for i, tc := range cls.ToolChoices {
			slot := base + ":tools"
			if i > 0 {
				slot += fmt.Sprintf(":%d", i+1)
			}
			c.addPoint(ChoicePoint{Slot: slot, Kinds: []string{"tool"}, Source: cls.Name, Level: 1, Count: tc.Count, Any: true,
				Text: "any " + strings.Join(tc.From, " or ")})
		}
		if s.SubclassLevel > 0 && c.level >= s.SubclassLevel {
			c.addPoint(ChoicePoint{Slot: base + ":subclass", Kinds: []string{"subclass"}, Source: cls.Name,
				Level: s.SubclassLevel, match: noGrant("subclass"), reported: true})
		}
		for _, f := range cls.Features {
			if f.Level > c.level || !strings.Contains(strings.ToLower(f.Name), "ability score improvement") {
				continue
			}
			level := f.Level
			c.addPoint(ChoicePoint{Slot: fmt.Sprintf("%s:asi:%d", base, level), Kinds: []string{"asi", "feat"}, Source: cls.Name, Level: level,
				Amount: 2, AbilityCount: 1, Any: true, reported: true,
				Text:  "an Ability Score Improvement (+2 to one score or +1 to two) or a feat",
				match: func(ch Choice) bool { return isASIAnswer(ch, base, level) }})
		}
		c.spellPoints()
		c.equipmentPoints(base, cls.Name, "", cls.Equipment)
	}
	if b := s.Background; b != nil {
		bg := c.choiceOfKind("background")
		base := backgroundSlot(b)
		if len(b.Abilities) > 0 {
			c.addPoint(ChoicePoint{Slot: base + ":asi", Kinds: []string{"asi"}, Source: "Background: " + b.Name,
				GrantedByID: bg, Level: 1, From: b.Abilities, Amount: 2, reported: true,
				Text:  "+2 to one and +1 to another, or +1 to all three",
				match: func(ch Choice) bool { return ch.Kind == "asi" && bg != "" && ch.GrantedBy == bg }})
		}
		for _, t := range b.Tools {
			if isToolPick(t) {
				c.addPoint(ChoicePoint{Slot: base + ":tool", Kinds: []string{"tool"}, Source: "Background: " + b.Name,
					GrantedByID: bg, Level: 1, Any: true, Text: t})
			}
		}
		if b.Feat != "" {
			name, _ := backgroundFeat(b.Feat)
			c.addPoint(ChoicePoint{Slot: base + ":feat", Kinds: []string{"feat"}, Source: "Background: " + b.Name,
				GrantedByID: bg, Level: 1, From: []string{name}, Text: b.Feat,
				match: func(ch Choice) bool { return ch.Kind == "feat" && bg != "" && ch.GrantedBy == bg }})
		}
	}
	if b := s.Background; b != nil {
		c.equipmentPoints(backgroundSlot(b), "Background: "+b.Name, c.choiceOfKind("background"), b.Equipment)
	}
	if sp := s.Species; sp != nil && strings.Contains(sp.Size, " or ") {
		c.addPoint(ChoicePoint{Slot: speciesSlot(sp) + ":size", Kinds: []string{"option"}, Source: sp.Name,
			GrantedByID: c.choiceOfKind("species"), Level: 1, From: strings.Split(sp.Size, " or ")})
	}
	for i := range c.sources {
		src := &c.sources[i]
		for j, e := range src.effects {
			kinds, slot := pointKinds[e.Type], src.slots[j]
			if slot == "" || !c.applies(e, src) || (e.Type == "option" && c.preset[src.grantedBy]) {
				continue
			}
			level := src.level
			if e.Level != nil && *e.Level > level {
				level = *e.Level
			}
			p := ChoicePoint{Slot: slot, Kinds: kinds, Source: src.label, GrantedByID: src.grantedBy, Level: level,
				Count: e.Choice.Count, From: e.Choice.From, Any: e.Choice.Any, SpellFilter: e.SpellFilter, Text: e.Text}
			if e.Type == "ability_score_increase" {
				p.Count, p.Amount, p.AbilityCount = 1, 1, max(e.Choice.Count, 1)
				if e.Amount != nil {
					p.Amount = *e.Amount
				}
			}
			if e.Type == "tool_proficiency" && e.Tool != "" && p.Any {
				p.Text = strings.TrimSpace(p.Text + " (any " + e.Tool + ")")
			}
			c.addPoint(p)
		}
	}
	c.checkPoints()
	c.sheet.ChoicePoints = c.points
}

// equipmentPoints: the starting equipment pick (package A or B), and then the picked package's own
// picks ("a Musical Instrument of your choice"), answered with an item's name. Saving them adds the
// items (the API does that, not the sheet).
func (c *computer) equipmentPoints(base, source, grantedBy string, options []EquipmentOption) {
	if len(options) == 0 {
		return
	}
	labels := make([]string, len(options))
	packages := make([]PointOption, len(options))
	for i, o := range options {
		labels[i], packages[i] = o.Label, PointOption{Label: o.Label, Text: o.Text}
	}
	c.addPoint(ChoicePoint{Slot: base + ":equipment", Kinds: []string{"equipment_option"}, Source: source, GrantedByID: grantedBy,
		Level: 1, From: labels, Options: packages, Text: "starting equipment"})
	for _, id := range c.points[len(c.points)-1].ChoiceIDs {
		picked := pickValue(c.byID[id])
		for _, o := range options {
			if !strings.EqualFold(o.Label, picked) {
				continue
			}
			for i, pk := range o.Picks {
				slot := base + ":equipment:pick"
				if i > 0 {
					slot += fmt.Sprintf(":%d", i+1)
				}
				c.addPoint(ChoicePoint{Slot: slot, Kinds: []string{"equipment_option"}, Source: source, GrantedByID: id, Level: 1,
					Count: pk.Count, Any: true, Text: "a " + strings.Join(pk.Categories, " or ") + " of your choice"})
			}
		}
	}
}

// isASIAnswer: an increase or feat at the class's ASI level, slot class:<class>:asi or :asi:<level>.
func isASIAnswer(ch Choice, classBase string, level int) bool {
	return ch.Level == level && ch.GrantedBy == "" && (ch.Kind == "asi" || ch.Kind == "feat") &&
		strings.HasPrefix(ch.Slot, classBase+":asi")
}

// checkPoints: missing and extra picks, picks outside a point's options, and picks a point no longer
// asks for (an option or feat that changed).
func (c *computer) checkPoints() {
	for _, p := range c.points {
		if !p.reported {
			where := p.Source // which of a class's points at several levels (cantrips at 1, 4, 10)
			if p.Level > 1 {
				where += fmt.Sprintf(", level %d", p.Level)
			}
			switch {
			case p.Chosen < p.Count:
				c.warn("choice.missing", fmt.Sprintf("%s: %s (%d of %d chosen)", where, describePoint(p), p.Chosen, p.Count))
			case p.Chosen > p.Count:
				c.warn("choice.extra", fmt.Sprintf("%s: %d chosen, it gives %d", where, p.Chosen, p.Count))
			}
		}
		for _, ch := range c.choices {
			if p.answers(ch) {
				c.checkAnswer(p, ch)
			}
		}
	}
	for _, ch := range c.choices {
		if ch.GrantedBy == "" {
			continue
		}
		granting, ok := c.byID[ch.GrantedBy]
		if !ok || (granting.Kind != "feat" && granting.Kind != "species" && granting.Kind != "background") {
			continue
		}
		asks, answered := false, false
		for _, p := range c.points {
			if p.GrantedByID == ch.GrantedBy {
				asks = true
				answered = answered || p.answers(ch)
			}
		}
		if asks && !answered && ch.Kind != "equipment_option" {
			c.warn("choice.orphan", fmt.Sprintf("%s (%s) no longer answers a pick of %s: remove it or re-link it",
				describeChoice(ch), ch.Slot, granting.RefName))
		}
	}
}

func (c *computer) checkAnswer(p ChoicePoint, ch Choice) {
	switch ch.Kind {
	case "asi":
		if p.match != nil && p.reported { // class and background increases have their own checks
			return
		}
		var v struct {
			Increases map[string]int `json:"increases"`
		}
		_ = json.Unmarshal(ch.Value, &v)
		total := 0
		for a, n := range v.Increases {
			total += n
			if (!p.Any && len(p.From) > 0 && !slices.Contains(p.From, a)) || n > p.Amount {
				c.warn("choice.invalid", fmt.Sprintf("%s: +%d %s isn't allowed (%s)", p.Source, n, a, describePoint(p)))
			}
		}
		if want := p.Amount * p.AbilityCount; total != want {
			c.warn("choice.invalid", fmt.Sprintf("%s: the increases add up to +%d, not +%d", p.Source, total, want))
		}
	case "feat":
		if len(p.From) == 0 {
			return
		}
		if strings.HasSuffix(p.Slot, ":feat") && strings.HasPrefix(p.Slot, "background:") {
			if !strings.EqualFold(p.From[0], ch.RefName) {
				c.warn("background.feat", fmt.Sprintf("%s gives %s; %s was chosen", strings.TrimPrefix(p.Source, "Background: "), p.Text, ch.RefName))
			}
			return
		}
		for _, f := range c.s.Feats {
			if f.ChoiceID == ch.ID && !slices.ContainsFunc(p.From, func(cat string) bool { return strings.EqualFold(cat, f.Category) }) {
				c.warn("choice.invalid", fmt.Sprintf("%s: %s is a %s feat, not %s", p.Source, f.Name, orNone(f.Category), strings.Join(p.From, " or ")))
			}
		}
	default:
		if valueKey[ch.Kind] == "" || p.Any || len(p.From) == 0 {
			return
		}
		if picked := pickValue(ch); !slices.ContainsFunc(p.From, func(o string) bool { return strings.EqualFold(o, picked) }) {
			c.warn("choice.invalid", fmt.Sprintf("%s: %s isn't one of %s", p.Source, orNone(picked), strings.Join(p.From, ", ")))
		}
	}
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// kindNoun is a choice kind in words: singular, plural.
var kindNoun = map[string][2]string{"asi": {"ability increase", "ability increases"}, "skill": {"skill", "skills"},
	"expertise": {"skill for Expertise", "skills for Expertise"}, "tool": {"tool", "tools"},
	"language": {"language", "languages"}, "saving_throw": {"saving throw", "saving throws"},
	"resistance": {"damage type", "damage types"}, "spell": {"spell", "spells"},
	"spellcasting_ability": {"spellcasting ability", "spellcasting abilities"}, "option": {"option", "options"},
	"feat": {"feat", "feats"}, "subclass": {"subclass", "subclasses"}, "species": {"species", "species"},
	"background": {"background", "backgrounds"}, "equipment_option": {"equipment option", "equipment options"}}

func noun(kind string, count int) string {
	if count == 1 {
		return kindNoun[kind][0]
	}
	return kindNoun[kind][1]
}

func describePoint(p ChoicePoint) string {
	from := ""
	if len(p.From) > 0 && !p.Any {
		from = ": " + strings.Join(p.From, ", ")
	}
	if p.Kinds[0] == "asi" {
		if p.AbilityCount > 1 {
			return fmt.Sprintf("choose +%d to each of %d ability scores%s", p.Amount, p.AbilityCount, from)
		}
		return fmt.Sprintf("choose +%d to an ability score%s", p.Amount, from)
	}
	nouns := make([]string, 0, len(p.Kinds))
	for _, k := range p.Kinds {
		nouns = append(nouns, noun(k, p.Count))
	}
	out := fmt.Sprintf("choose %d %s", p.Count, strings.Join(nouns, " or "))
	switch {
	case p.SpellFilter != nil:
		return out + " (" + describeFilter(*p.SpellFilter) + ")"
	case p.Text != "" && (p.Kinds[0] == "feat" || p.Kinds[0] == "tool"):
		return out + ": " + p.Text
	}
	return out + from
}

// describeFilter: {level 0, classes [Wizard]} -> "Wizard cantrips"; {min 1, max 2} -> "level 1-2 spells".
func describeFilter(f SpellFilter) string {
	parts := []string{}
	if f.Ritual != nil && *f.Ritual {
		parts = append(parts, "ritual")
	}
	if len(f.Schools) > 0 {
		parts = append(parts, strings.Join(f.Schools, " or "))
	}
	if len(f.Classes) > 0 {
		parts = append(parts, strings.Join(f.Classes, " or "))
	}
	low, high := f.MinLevel, f.MaxLevel
	switch {
	case f.Level != nil && *f.Level == 0:
		parts = append(parts, "cantrips")
	case f.Level != nil:
		parts = append(parts, fmt.Sprintf("level %d spells", *f.Level))
	case low != nil && high != nil && *low == *high:
		parts = append(parts, fmt.Sprintf("level %d spells", *high))
	case low != nil && high != nil:
		parts = append(parts, fmt.Sprintf("level %d-%d spells", *low, *high))
	case high != nil:
		parts = append(parts, fmt.Sprintf("spells up to level %d", *high))
	default:
		parts = append(parts, "spells")
	}
	return strings.Join(parts, " ")
}

func describeChoice(ch Choice) string {
	if ch.RefName != "" {
		return ch.RefName
	}
	if v := pickValue(ch); v != "" {
		return noun(ch.Kind, 1) + " " + v
	}
	return noun(ch.Kind, 1)
}
