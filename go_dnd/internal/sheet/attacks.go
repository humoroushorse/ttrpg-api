package sheet

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Attack is a weapon attack (or the unarmed strike) with its bonus and damage.
type Attack struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"` // melee | ranged
	Ability    string   `json:"ability"`
	Proficient bool     `json:"proficient"`
	Bonus      Stat     `json:"bonus"`
	Damage     string   `json:"damage"`                // "1d8+3 Slashing"
	DamageNote string   `json:"damage_note,omitempty"` // "Monk: Martial Arts die"
	Versatile  string   `json:"versatile,omitempty"`   // two-handed: "1d10+3"
	Range      string   `json:"range,omitempty"`       // "20/60"
	Properties []string `json:"properties"`
	Mastery    string   `json:"mastery,omitempty"`
}

var (
	damageText    = regexp.MustCompile(`^\s*(\d+(?:d\d+)?)\s*(.*)$`)          // "1d8 Slashing", "1 Piercing"
	rangeText     = regexp.MustCompile(`(?i)range\s+(\d+/\d+)`)               // "Thrown (Range 20/60)"
	versatileText = regexp.MustCompile(`(?i)versatile\s*\(\s*(\d+d\d+)\s*\)`) // "Versatile (1d10)"
)

// splitProperties splits "Finesse, Light, Thrown (Range 20/60)" at the commas outside parentheses.
func splitProperties(text string) []string {
	out := []string{}
	depth, start := 0, 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	if last := strings.TrimSpace(text[start:]); last != "" {
		out = append(out, last)
	}
	return out
}

// hasProperty: "Thrown (Range 20/60)" has the Thrown property.
func hasProperty(props []string, name string) bool {
	return slices.ContainsFunc(props, func(p string) bool {
		f := strings.Fields(p)
		return len(f) > 0 && strings.EqualFold(f[0], name)
	})
}

// weaponProficient: the class's or a feat's weapon categories cover the weapon (Monk: martial:light
// covers martial weapons with the Light property).
func weaponProficient(categories []string, subcategory string, props []string) bool {
	simple := strings.HasPrefix(strings.ToLower(subcategory), "simple")
	for _, cat := range categories {
		base, prop, limited := strings.Cut(cat, ":")
		switch {
		case base == "simple" && simple, base == "martial" && !simple && !limited:
			return true
		case base == "martial" && !simple && limited && hasProperty(props, prop):
			return true
		}
	}
	return false
}

func (c *computer) attacks() {
	categories := []string{}
	if cls := c.s.Class; cls != nil {
		categories = append(categories, cls.WeaponCategories...)
	}
	for _, e := range c.effects(false, "weapon_proficiency") {
		categories = append(categories, e.Kind)
	}
	str, dex := c.abilityMod("str"), c.abilityMod("dex")
	maDie, maSource := "", "" // Martial Arts: Dex and the class table's die for unarmed strikes and Monk weapons
	for _, e := range c.effects(false, "martial_arts") {
		maDie, maSource = c.tableText(e.Table), e.src.label
	}
	out := []Attack{}
	unarmed := Attack{Name: "Unarmed Strike", Kind: "melee", Ability: "str", Proficient: true, Properties: []string{}}
	if maDie != "" && dex > str {
		unarmed.Ability = "dex"
	}
	unarmedMod := c.abilityMod(unarmed.Ability)
	unarmed.Bonus.add(abilityLabel(unarmed.Ability), "base", unarmedMod, "")
	unarmed.Bonus.add("Proficiency bonus", "add", c.pb, "")
	c.applyModifiers(&unarmed.Bonus, "attack.melee")
	unarmed.Damage = fmt.Sprintf("%d Bludgeoning", max(1+unarmedMod, 0))
	if maDie != "" {
		unarmed.Damage, unarmed.DamageNote = maDie+signedOrEmpty(unarmedMod)+" Bludgeoning", maSource+" die"
	}
	for _, it := range c.items {
		damage, ok := it.Properties["Damage"]
		if !ok || !it.Equipped || it.Category != "weapon" {
			continue
		}
		props := splitProperties(it.Properties["Properties"])
		a := Attack{Name: it.Name, Kind: "melee", Ability: "str", Properties: props, Mastery: it.Properties["Mastery"]}
		if strings.Contains(strings.ToLower(it.Subcategory), "ranged") {
			a.Kind, a.Ability = "ranged", "dex"
		}
		sub := strings.ToLower(it.Subcategory)
		monkWeapon := maDie != "" && strings.Contains(sub, "melee") &&
			(strings.HasPrefix(sub, "simple") || hasProperty(props, "light"))
		if (hasProperty(props, "finesse") || monkWeapon) && dex > str {
			a.Ability = "dex"
		}
		mod := c.abilityMod(a.Ability)
		a.Bonus.add(abilityLabel(a.Ability), "base", mod, "")
		a.Proficient = weaponProficient(categories, it.Subcategory, props)
		if a.Proficient {
			a.Bonus.add("Proficiency bonus", "add", c.pb, "")
		} else {
			a.Bonus.Inactive = append(a.Bonus.Inactive, Contribution{Source: "Proficiency bonus", Op: "add", Value: c.pb, Note: "not proficient"})
		}
		c.applyModifiers(&a.Bonus, "attack."+a.Kind)
		if m := damageText.FindStringSubmatch(damage); m != nil {
			die := m[1]
			if monkWeapon && dieAverage(maDie) > dieAverage(die) {
				die, a.DamageNote = maDie, maSource+" die"
			}
			a.Damage = strings.TrimSpace(fmt.Sprintf("%s%s %s", die, signedOrEmpty(mod), m[2]))
		} else {
			a.Damage = damage
		}
		for _, p := range props {
			if m := rangeText.FindStringSubmatch(p); m != nil {
				a.Range = m[1]
			}
			if m := versatileText.FindStringSubmatch(p); m != nil {
				a.Versatile = m[1] + signedOrEmpty(mod)
			}
		}
		out = append(out, a)
	}
	c.sheet.Attacks = append(out, unarmed)
}

// dieAverage: "1d8" -> 4.5, "2d6" -> 7, "1" -> 1.
func dieAverage(die string) float64 {
	var count, faces int
	if _, err := fmt.Sscanf(die, "%dd%d", &count, &faces); err == nil {
		return float64(count) * float64(faces+1) / 2
	}
	_, _ = fmt.Sscan(die, &count)
	return float64(count)
}

func signedOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return signed(n)
}
