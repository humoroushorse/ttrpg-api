package sheet

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// SpellcastingSheet is the class's (or casting subclass's) spellcasting at this level.
type SpellcastingSheet struct {
	Source   string `json:"source"`  // Wizard, Eldritch Knight
	Ability  string `json:"ability"` // int
	SaveDC   Stat   `json:"save_dc"`
	Attack   Stat   `json:"attack"`
	Cantrips *Limit `json:"cantrips,omitempty"` // null when the class has no cantrips (Paladin); picked at each level
	// level 1+ spells: prepared from the class's list (change long_rest), or picked at each level
	// (change level_up: Bard, Sorcerer, Warlock)
	Prepared      Limit  `json:"prepared"`
	Change        string `json:"change"`                  // long_rest | level_up
	HasSpellbook  bool   `json:"has_spellbook,omitempty"` // the class keeps one (Wizard): the spellbook rule applies to it
	SpellbookRule bool   `json:"spellbook_rule,omitempty"`
	Slots         []int  `json:"slots"` // spell levels 1-9
	PactSlots     int    `json:"pact_slots,omitempty"`
	PactSlotLevel int    `json:"pact_slot_level,omitempty"`
}

// Limit is how many the class allows and how many the character has. Over or under the limit is a
// warning, never a block: the player can prepare too many, then unprepare.
type Limit struct {
	Limit  int `json:"limit"`
	Chosen int `json:"chosen"`
}

// casting is who casts and how: the class, or a casting subclass once it's chosen (Eldritch Knight).
type casting struct {
	source    string // Wizard, Eldritch Knight
	sc        *Spellcasting
	table     []LevelRow
	slot      string // its spell picks' slots start with this: class:wizard
	grantedBy string // "" for the class; the subclass choice for a subclass
	list      string // the class list its spells come from
}

func (c *computer) casting() *casting {
	cls, sub := c.s.Class, c.s.Subclass
	if cls == nil {
		return nil
	}
	if cls.Spellcasting != nil {
		return &casting{source: cls.Name, sc: cls.Spellcasting, table: cls.Table, slot: classSlot(cls), list: cls.Name}
	}
	if sub != nil && sub.Spellcasting != nil && sub.Level <= c.level {
		list := sub.Spellcasting.SpellList
		if list == "" {
			list = cls.Name
		}
		return &casting{source: sub.Name, sc: sub.Spellcasting, table: sub.Table, slot: subclassSlot(sub),
			grantedBy: c.choiceOfKind("subclass"), list: list}
	}
	return nil
}

// learns: its level 1+ spells are picked at each level, not prepared from the list.
func (k *casting) learns() bool { return k.sc.Change == "level_up" }

// spellbook: it keeps a spellbook and the spellbook rule is on.
func (k *casting) spellbook(s Snapshot) bool { return s.SpellbookRule && len(k.sc.Spellbook) > 0 }

func rowAt(table []LevelRow, level int) *LevelRow {
	for i := range table {
		if table[i].Level == level {
			return &table[i]
		}
	}
	return nil
}

// highestSlot is the highest spell level a row has slots for, pact slots included.
func highestSlot(r *LevelRow) int {
	highest := r.PactSlotLevel
	for i, n := range r.Slots {
		if n > 0 {
			highest = max(highest, i+1)
		}
	}
	return highest
}

func intp(n int) *int { return &n }

// isSpellPick: a spell choice answering one of the caster's own picks of this kind (cantrips,
// spells, spellbook), not a feat's or trait's.
func (k *casting) isSpellPick(ch Choice, kind string) bool {
	return ch.Kind == "spell" && ch.GrantedBy == k.grantedBy && strings.HasPrefix(ch.Slot, k.slot+":"+kind+":")
}

// GrantedSpell is a spell a feat or species trait gives (class spells are prepared, not granted).
type GrantedSpell struct {
	Spell   string `json:"spell,omitempty"` // the catalog slug of a fixed spell (misty-step)
	Name    string `json:"name,omitempty"`  // a picked spell's name
	Source  string `json:"source"`
	Uses    string `json:"uses,omitempty"` // "1/long rest"; empty: cast as usual
	Ability string `json:"ability,omitempty"`
	SaveDC  int    `json:"save_dc,omitempty"`
	Attack  int    `json:"attack,omitempty"`
	Note    string `json:"note,omitempty"`
}

func (c *computer) row(table []LevelRow) *LevelRow {
	for i := range table {
		if table[i].Level == c.level {
			return &table[i]
		}
	}
	return nil
}

// spellcasting: the save DC, attack, slots and limits at this level, and the prepared list's
// warnings. The cantrip and learned-spell picks are choice points (spellPoints), which fill in their
// counts.
func (c *computer) spellcasting() {
	cls := c.s.Class
	if cls == nil {
		return
	}
	k := c.casting()
	if k == nil {
		if len(c.s.PreparedSpells) > 0 {
			c.warn("spells.no_casting", fmt.Sprintf("%d spells prepared, but %s doesn't cast spells at level %d", len(c.s.PreparedSpells), cls.Name, c.level))
		}
		if hasFeature(cls.Features, "spellcasting", "pact magic") {
			c.notComputed = append(c.notComputed, "spellcasting: the class has no table in the catalog (load it with the 5e.tools fields)")
		}
		return
	}
	source, sc := k.source, k.sc
	r := c.row(k.table)
	if r == nil {
		c.notComputed = append(c.notComputed, fmt.Sprintf("spellcasting: %s's table has no level %d", source, c.level))
		return
	}
	mod := c.abilityMod(sc.Ability)
	out := &SpellcastingSheet{Source: source, Ability: sc.Ability, Slots: r.Slots, PactSlots: r.PactSlots, PactSlotLevel: r.PactSlotLevel,
		Change: "long_rest", HasSpellbook: len(sc.Spellbook) > 0, SpellbookRule: k.spellbook(c.s)}
	if k.learns() {
		out.Change = "level_up"
	}
	if out.Slots == nil {
		out.Slots = make([]int, 9)
	}
	out.SaveDC.add("Base", "base", 8, "")
	out.SaveDC.add("Proficiency bonus", "add", c.pb, "")
	out.SaveDC.add(abilityLabel(sc.Ability), "add", mod, "")
	c.applyModifiers(&out.SaveDC, "spell.dc")
	out.Attack.add("Proficiency bonus", "base", c.pb, "")
	out.Attack.add(abilityLabel(sc.Ability), "add", mod, "")
	c.applyModifiers(&out.Attack, "spell.attack")
	out.SaveDC.Roll, out.SaveDC.Advantage, out.SaveDC.Disadvantage = "", nil, nil
	if slices.ContainsFunc(k.table, func(t LevelRow) bool { return t.Cantrips > 0 }) {
		limit := r.Cantrips
		for _, e := range c.effects(false, "cantrips_known") { // Divine Order: Thaumaturge
			n, _ := c.amount(e.Effect, c.level)
			limit += n
		}
		out.Cantrips = &Limit{Limit: limit} // Chosen: spellPoints
	}
	out.Prepared = Limit{Limit: r.Prepared}
	highest := highestSlot(r)
	var spellbook []string // the learned spells' names, under the spellbook rule
	for _, ch := range c.choices {
		if out.SpellbookRule && k.isSpellPick(ch, "spellbook") {
			spellbook = append(spellbook, ch.RefName)
		}
	}
	leveled := 0
	for _, p := range c.s.PreparedSpells {
		switch {
		case p.Level == 0:
			c.warn("spells.cantrip_prepared", fmt.Sprintf("%s is a cantrip on the prepared list: cantrips are picked at each level now, so remove it and pick it there", p.Name))
			continue
		case k.learns():
			continue // warned once below
		}
		leveled++
		if p.Level > highest {
			c.warn("spells.level", fmt.Sprintf("%s is a level %d spell; %s %d's highest slot is level %d", p.Name, p.Level, source, c.level, highest))
		}
		if out.SpellbookRule && !slices.ContainsFunc(spellbook, func(s string) bool { return strings.EqualFold(s, p.Name) }) {
			c.warn("spells.spellbook", fmt.Sprintf("%s is prepared but isn't in the spellbook", p.Name))
		}
	}
	switch {
	case k.learns():
		if n := len(c.s.PreparedSpells) - countLevel0(c.s.PreparedSpells); n > 0 {
			c.warn("spells.learned", fmt.Sprintf("%s learns its spells at each level instead of preparing them: %d spells on the prepared list don't count", source, n))
		}
	case leveled > r.Prepared:
		c.warn("spells.prepared", fmt.Sprintf("prepared spells: %d of %d at %s %d: unprepare %d", leveled, r.Prepared, source, c.level, leveled-r.Prepared))
	case leveled < r.Prepared:
		c.warn("spells.prepared", fmt.Sprintf("prepared spells: %d of %d at %s %d: prepare %d more", leveled, r.Prepared, source, c.level, r.Prepared-leveled))
	}
	out.Prepared.Chosen = leveled
	c.sheet.Spellcasting = out
}

func countLevel0(spells []PreparedSpell) int {
	n := 0
	for _, p := range spells {
		if p.Level == 0 {
			n++
		}
	}
	return n
}

// spellPoints are the caster's own spell picks, by the level that gives them: cantrips (the table's
// increase, and extra ones like Thaumaturge's), level 1+ spells for a class that learns them (Bard),
// and under the spellbook rule the spellbook's (Wizard: 6, then 2 a level, and any copied in). Each
// counts once the character reaches its level, so planned levels can be picked ahead.
func (c *computer) spellPoints() {
	k := c.casting()
	if k == nil {
		return
	}
	sc := c.sheet.Spellcasting // nil when the table has no row for this level
	cantripsChosen, learnedChosen := 0, 0
	add := func(p ChoicePoint) int {
		p.Kinds, p.Source, p.GrantedByID = []string{"spell"}, k.source+p.Source, k.grantedBy
		p.Text = describeFilter(*p.SpellFilter)
		c.addPoint(p)
		return c.points[len(c.points)-1].Chosen
	}
	upTo := func(r *LevelRow) *SpellFilter {
		return &SpellFilter{MinLevel: intp(1), MaxLevel: intp(highestSlot(r)), Classes: []string{k.list}}
	}
	cantrips := &SpellFilter{Level: intp(0), Classes: []string{k.list}}
	prev, last := LevelRow{}, (*LevelRow)(nil)
	for lv := 1; lv <= c.level; lv++ {
		r := rowAt(k.table, lv)
		if r == nil {
			continue // a fixture's table may skip levels
		}
		if n := r.Cantrips - prev.Cantrips; n > 0 {
			cantripsChosen += add(ChoicePoint{Slot: fmt.Sprintf("%s:cantrips:%d", k.slot, lv), Level: lv, Count: n, SpellFilter: cantrips})
		}
		if n := r.Prepared - prev.Prepared; n > 0 && k.learns() {
			learnedChosen += add(ChoicePoint{Slot: fmt.Sprintf("%s:spells:%d", k.slot, lv), Level: lv, Count: n, SpellFilter: upTo(r)})
		}
		if k.spellbook(c.s) && lv <= len(k.sc.Spellbook) && k.sc.Spellbook[lv-1] > 0 {
			add(ChoicePoint{Slot: fmt.Sprintf("%s:spellbook:%d", k.slot, lv), Source: ": Spellbook", Level: lv,
				Count: k.sc.Spellbook[lv-1], SpellFilter: upTo(r)})
		}
		prev, last = *r, r
	}
	if k.spellbook(c.s) && last != nil {
		add(ChoicePoint{Slot: k.slot + ":spellbook:copied", Source: ": Spellbook (copied in)", Level: 1, Open: true, SpellFilter: upTo(last)})
	}
	if sc == nil {
		return
	}
	if sc.Cantrips != nil {
		for _, e := range c.effects(false, "cantrips_known") { // Divine Order: Thaumaturge
			n, _ := c.amount(e.Effect, c.level)
			p := ChoicePoint{Slot: e.src.slot + ":cantrips_known", Kinds: []string{"spell"}, Source: e.src.label, GrantedByID: e.src.grantedBy,
				Level: e.src.level, Count: n, SpellFilter: cantrips, Text: describeFilter(*cantrips)}
			c.addPoint(p)
			cantripsChosen += c.points[len(c.points)-1].Chosen
		}
		sc.Cantrips.Chosen = cantripsChosen
	}
	if k.learns() {
		sc.Prepared.Chosen = learnedChosen
	}
}

func hasFeature(features []Feature, names ...string) bool {
	for _, f := range features {
		if slices.Contains(names, strings.ToLower(f.Name)) {
			return true
		}
	}
	return false
}

// grantedSpells lists the spells feats and species traits give: the fixed ones and the picked ones.
func (c *computer) grantedSpells() {
	out := []GrantedSpell{}
	for i := range c.sources {
		src := &c.sources[i]
		for j, e := range src.effects {
			if e.Type != "grant_spell" || !c.applies(e, src) {
				continue
			}
			ability := c.spellAbility(e, src)
			base := GrantedSpell{Source: src.label, Uses: e.Uses, Ability: ability, Note: e.Text}
			if ability != "" {
				base.SaveDC = 8 + c.pb + c.abilityMod(ability)
				base.Attack = c.pb + c.abilityMod(ability)
			}
			if e.Choice == nil {
				base.Spell = e.Spell
				out = append(out, base)
				continue
			}
			for _, ch := range c.choices {
				if ch.Kind == "spell" && ch.GrantedBy == src.grantedBy && ch.Slot == src.slots[j] {
					picked := base
					picked.Name = ch.RefName
					out = append(out, picked)
				}
			}
		}
	}
	c.sheet.Spells = out
}

// spellAbility is the ability a granted spell is cast with: the atom's, else the one picked for the
// grant (Magic Initiate, Elven Lineage), else the one the grant increased (Fey-Touched).
func (c *computer) spellAbility(e Effect, src *effectSource) string {
	if e.Ability != "" {
		return e.Ability
	}
	for _, ch := range c.choices {
		if ch.GrantedBy == src.grantedBy && ch.Kind == "spellcasting_ability" {
			return pickValue(ch)
		}
	}
	for _, ch := range c.choices {
		if ch.GrantedBy == src.grantedBy && ch.Kind == "asi" {
			var v struct {
				Increases map[string]int `json:"increases"`
			}
			_ = json.Unmarshal(ch.Value, &v)
			if len(v.Increases) == 1 {
				for a := range v.Increases {
					return a
				}
			}
		}
	}
	return ""
}
