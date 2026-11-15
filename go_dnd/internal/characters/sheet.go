package characters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/sheet"
)

// ReadCharacterSheet is GET /characters/{entity_id}/sheet: the computed sheet at current_level, or at
// ?level= to preview another level without changing anything.
func (h *CharacterRoutes) ReadCharacterSheet(w http.ResponseWriter, r *http.Request, id api.EntityId, p api.ReadCharacterSheetParams) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	if _, err := requireCharacter(ctx, q, id, userID, access.ViewSheet); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	snap, err := LoadSnapshot(ctx, q, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if p.Level != nil {
		if *p.Level < 1 || *p.Level > 20 {
			httpx.WriteError(w, r, httpx.Unprocessable("level must be 1 to 20"))
			return
		}
		snap.Level = *p.Level
	}
	httpx.WriteJSON(w, http.StatusOK, sheet.Compute(snap))
}

var leadingNumber = regexp.MustCompile(`^\s*(\d+)`)

// LoadSnapshot reads everything the sheet needs: the character, its planned levels and class, its
// picks (deleted catalog entries included, so the sheet never breaks), items, modifiers, and the
// content-rule violations. The caller has checked access (requireCharacter).
func LoadSnapshot(ctx context.Context, q *db.Queries, id uuid.UUID) (sheet.Snapshot, error) {
	st, err := loadState(ctx, q, id)
	if err != nil {
		return sheet.Snapshot{}, notFoundIfMissing(err, id)
	}
	c := st.row
	snap := sheet.Snapshot{Level: int(c.CurrentLevel), AbilityMethod: c.AbilityMethod, BaseScores: map[string]int{},
		SpellbookRule: st.rules.OptionalRules.Spellbook}
	_ = json.Unmarshal(c.BaseScores, &snap.BaseScores)

	levels, err := q.ListLevels(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	var classID *uuid.UUID
	for _, l := range levels {
		lv := sheet.Level{Level: int(l.Level), HPMethod: l.HpMethod}
		if l.HpValue != nil {
			v := int(*l.HpValue)
			lv.HPValue = &v
		}
		snap.Levels = append(snap.Levels, lv)
		if l.Level == 1 {
			classID = &l.ClassID
		}
	}
	if classID != nil {
		cls, err := q.ClassForSheet(ctx, *classID)
		if err != nil {
			return sheet.Snapshot{}, err
		}
		snap.Class = toSheetClass(cls)
		if cls.DndVersionYear == 2024 {
			snap.SubclassLevel = 3
		} else {
			lv, err := q.SubclassLevelFor(ctx, db.SubclassLevelForParams{ClassSlug: cls.Slug, Year: cls.DndVersionYear})
			if err != nil {
				return sheet.Snapshot{}, err
			}
			snap.SubclassLevel = int(lv)
		}
	}

	choices, err := q.ListChoices(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	for _, row := range choices {
		ch := row.DndCharacterChoice
		sc := sheet.Choice{ID: ch.ID.String(), Level: int(ch.Level), Kind: ch.Kind, Slot: ch.Slot, RefName: row.RefName, Value: ch.Value}
		if ch.GrantedByID != nil {
			sc.GrantedBy = ch.GrantedByID.String()
		}
		snap.Choices = append(snap.Choices, sc)
		switch {
		case ch.SpeciesID != nil:
			sp, err := q.SpeciesForSheet(ctx, *ch.SpeciesID)
			if err != nil {
				return sheet.Snapshot{}, err
			}
			speed := 0
			if m := leadingNumber.FindStringSubmatch(sp.Speed); m != nil {
				speed, _ = strconv.Atoi(m[1])
			}
			snap.Species = &sheet.Species{Name: sp.Name, Slug: sp.Slug, Size: sp.Size, Speed: speed, Traits: features(sp.Traits, true)}
		case ch.BackgroundID != nil:
			bg, err := q.BackgroundForSheet(ctx, *ch.BackgroundID)
			if err != nil {
				return sheet.Snapshot{}, err
			}
			b := &sheet.Background{Name: bg.Name, Slug: bg.Slug, Tools: bg.ToolProficiencies, Feat: bg.Feat,
				Equipment: equipmentOptions(bg.EquipmentOptions)}
			for _, a := range strings.Split(bg.AbilityScoreBonuses, ",") {
				if code := sheet.AbilityCode(a); code != "" {
					b.Abilities = append(b.Abilities, code)
				}
			}
			for _, s := range bg.SkillProficiencies {
				if code := sheet.SkillCode(s); code != "" {
					b.Skills = append(b.Skills, code)
				}
			}
			snap.Background = b
		case ch.SubclassID != nil:
			sub, err := q.SubclassForSheet(ctx, *ch.SubclassID)
			if err != nil {
				return sheet.Snapshot{}, err
			}
			snap.Subclass = &sheet.Subclass{Name: sub.Name, Slug: sub.Slug, Level: int(ch.Level), Features: features(sub.Features, true),
				Spellcasting: spellcasting(sub.SpellcastingAbility, sub.CasterProgression, sub.SpellPreparation), Table: levelTable(sub.LevelTable)}
		}
	}

	feats, err := q.FeatsForSheet(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	for _, f := range feats {
		sf := sheet.Feat{ChoiceID: f.ChoiceID.String(), Name: f.Name, Slug: f.Slug}
		if f.Category != nil {
			sf.Category = *f.Category
		}
		_ = json.Unmarshal(f.Effects, &sf.Effects)
		snap.Feats = append(snap.Feats, sf)
	}

	prepared, err := q.ListPreparedSpells(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	for _, p := range prepared {
		snap.PreparedSpells = append(snap.PreparedSpells, sheet.PreparedSpell{Name: p.SpellName, Level: int(p.SpellLevel)})
	}

	items, err := q.ItemsForSheet(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	for _, it := range items {
		si := sheet.Item{Equipped: it.Equipped, Attuned: it.Attuned, RequiresAttunement: it.RequiresAttunement}
		switch {
		case it.CustomName != nil:
			si.Name = *it.CustomName
		case it.ItemName != nil:
			si.Name = *it.ItemName
		}
		if it.Category != nil {
			si.Category = *it.Category
		}
		if it.Subcategory != nil {
			si.Subcategory = *it.Subcategory
		}
		if it.LevelReceived != nil {
			v := int(*it.LevelReceived)
			si.LevelReceived = &v
		}
		_ = json.Unmarshal(it.Effects, &si.Effects)
		si.Properties = stringValues(it.Properties)
		snap.Items = append(snap.Items, si)
	}

	mods, err := q.ListModifiers(ctx, id)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	for _, m := range mods {
		// a DM's boon or bane counts only while the character is in the campaign that gave it
		active := m.Active && (m.CampaignID == nil || *m.CampaignID == st.campaignID)
		sm := sheet.Modifier{Label: m.Label, Target: m.Target, Op: m.Op, Value: m.Value, Active: active, AddedAs: m.AddedAs}
		if m.FromLevel != nil {
			v := int(*m.FromLevel)
			sm.FromLevel = &v
		}
		snap.Modifiers = append(snap.Modifiers, sm)
	}

	violations, err := Violations(ctx, q, id, st.rules)
	if err != nil {
		return sheet.Snapshot{}, err
	}
	if st.from == "campaign" {
		approved, err := q.CharacterExceptions(ctx, id)
		if err != nil {
			return sheet.Snapshot{}, err
		}
		violations = markApproved(violations, approved)
	}
	for _, v := range violations {
		where := "this campaign's rules"
		if st.from == "character" {
			where = "the character's own rules (picked with the filter off)"
		}
		issue := sheet.Issue{Severity: "warning", Code: "rules." + v.Category, Message: v.Name + " is outside " + where + ": " + v.Reason}
		if v.Approved {
			issue.Severity, issue.Code = "info", "rules.approved"
			issue.Message = v.Name + " is outside " + where + " (" + v.Reason + "); the DM approved it"
			if v.Note != nil && *v.Note != "" {
				issue.Message += ": " + *v.Note
			}
		}
		snap.RuleIssues = append(snap.RuleIssues, issue)
	}
	return snap, nil
}

func notFoundIfMissing(err error, id uuid.UUID) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound(id)
	}
	return err
}

func toSheetClass(cls db.ClassForSheetRow) *sheet.Class {
	out := &sheet.Class{Name: cls.Name, Slug: cls.Slug, Year: int(cls.DndVersionYear), Features: features(cls.Features, true),
		Spellcasting: spellcasting(cls.SpellcastingAbility, cls.CasterProgression, cls.SpellPreparation), Table: levelTable(cls.LevelTable)}
	if cls.HitDieSize != nil {
		out.HitDieSize = int(*cls.HitDieSize)
	}
	out.Equipment = equipmentOptions(cls.StartingEquipmentOptions)
	var p struct {
		Armor            []string           `json:"armor"`
		Weapons          []string           `json:"weapons"`
		Tools            []string           `json:"tools"`
		SavingThrows     []string           `json:"saving_throws"`
		ArmorCategories  []string           `json:"armor_categories"`
		WeaponCategories []string           `json:"weapon_categories"`
		ToolChoices      []sheet.ToolChoice `json:"tool_choices"`
		Skills           struct {
			Choose int      `json:"choose"`
			From   []string `json:"from"`
		} `json:"skills"`
	}
	_ = json.Unmarshal(cls.Proficiencies, &p)
	out.Armor, out.Weapons, out.Tools, out.SkillChoose = p.Armor, p.Weapons, p.Tools, p.Skills.Choose
	out.ArmorCategories, out.WeaponCategories, out.ToolChoices = p.ArmorCategories, p.WeaponCategories, p.ToolChoices
	for _, s := range p.SavingThrows {
		if code := sheet.AbilityCode(s); code != "" {
			out.SavingThrows = append(out.SavingThrows, code)
		}
	}
	for _, s := range p.Skills.From {
		if code := sheet.SkillCode(s); code != "" {
			out.SkillFrom = append(out.SkillFrom, code)
		}
	}
	return out
}

// spellcasting is a class's or subclass's casting; nil when it doesn't cast. preparation is the
// catalog's spell_preparation: {change, spellbook, spell_list}.
func spellcasting(ability, progression *string, preparation []byte) *sheet.Spellcasting {
	if ability == nil || *ability == "" {
		return nil
	}
	out := &sheet.Spellcasting{Ability: *ability}
	if progression != nil {
		out.Progression = *progression
	}
	var p struct {
		Change    string  `json:"change"`
		Spellbook []int   `json:"spellbook"`
		SpellList *string `json:"spell_list"`
	}
	_ = json.Unmarshal(preparation, &p)
	out.Change, out.Spellbook = p.Change, p.Spellbook
	if p.SpellList != nil {
		out.SpellList = *p.SpellList
	}
	return out
}

func levelTable(raw []byte) []sheet.LevelRow {
	var rows []sheet.LevelRow
	_ = json.Unmarshal(raw, &rows)
	return rows
}

// stringValues reads an item's properties ({"Damage": "1d8 Slashing", ...}), keeping the text ones.
func stringValues(raw []byte) map[string]string {
	var all map[string]any
	_ = json.Unmarshal(raw, &all)
	out := map[string]string{}
	for k, v := range all {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// features reads a catalog features or traits list: [{name, level, levels, effects, ...}]. A
// repeated feature is listed at each of its levels, with its effects on the first only (its atoms
// carry their own levels).
func features(raw []byte, withEffects bool) []sheet.Feature {
	var list []struct {
		Name    string         `json:"name"`
		Level   *int           `json:"level"`
		Levels  []int          `json:"levels"` // a repeated feature: Ability Score Improvement at 4, 8, 12, ...
		Effects []sheet.Effect `json:"effects"`
	}
	_ = json.Unmarshal(raw, &list)
	out := make([]sheet.Feature, 0, len(list))
	for _, f := range list {
		levels := f.Levels
		if len(levels) == 0 {
			levels = []int{1}
			if f.Level != nil && *f.Level > 0 {
				levels[0] = *f.Level
			}
		}
		for i, level := range levels {
			sf := sheet.Feature{Name: f.Name, Level: level}
			if withEffects && i == 0 {
				sf.Effects = f.Effects
			}
			out = append(out, sf)
		}
	}
	return out
}

// equipmentOptions reads a class's or background's structured starting equipment for the sheet: each
// package's label, what it gives in words, and its picks. Text-only (2014, or not in 5e.tools) gives none.
func equipmentOptions(raw []byte) []sheet.EquipmentOption {
	var list []catalogEquipmentOption
	_ = json.Unmarshal(raw, &list)
	var out []sheet.EquipmentOption
	for _, o := range list {
		if o.Label == nil || (len(o.Grants) == 0 && len(o.Picks) == 0 && len(o.Custom) == 0 && len(o.Coins) == 0) {
			continue
		}
		parts := make([]string, 0, len(o.Grants)+len(o.Custom)+len(o.Picks)+1)
		for _, g := range o.Grants {
			name := g.Name
			if g.DisplayName != nil {
				name = *g.DisplayName
			}
			if g.Quantity > 1 {
				name = fmt.Sprintf("%s ×%d", name, g.Quantity)
			}
			parts = append(parts, name)
		}
		parts = append(parts, o.Custom...)
		opt := sheet.EquipmentOption{Label: *o.Label}
		for _, p := range o.Picks {
			parts = append(parts, "a "+strings.Join(p.Categories, " or ")+" of your choice")
			opt.Picks = append(opt.Picks, sheet.EquipmentPick{Categories: p.Categories, Count: max(p.Count, 1)})
		}
		if coins := coinText(o.Coins); coins != "" {
			parts = append(parts, coins)
		}
		opt.Text = strings.Join(parts, ", ")
		out = append(out, opt)
	}
	return out
}
