package characters

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/rules"
	"github.com/humoroushorse/go_dnd/internal/sheet"
)

// Option is a catalog entry the character could pick, and whether the rules let it.
type Option struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	SourceSlug     string    `json:"source_slug"`
	DndVersionYear int32     `json:"dnd_version_year"`
	Allowed        bool      `json:"allowed"`
	Reasons        []string  `json:"reasons"`
	Chosen         bool      `json:"chosen"`
	Category       string    `json:"category,omitempty"` // feats
	Level          *int32    `json:"level,omitempty"`    // spells
	Learned        bool      `json:"learned,omitempty"`  // a spell to prepare that's in the spellbook (the spellbook rule)
}

type optionsResponse struct {
	TotalEntitiesCount int            `json:"total_entities_count"`
	Limit              int            `json:"limit"`
	Offset             int            `json:"offset"`
	Filters            map[string]any `json:"filters"`
	EntitiesCount      int            `json:"entities_count"`
	Entities           []Option       `json:"entities"`
	Kind               string         `json:"kind"`
	Slot               *string        `json:"slot"`
	Level              int            `json:"level"`
}

// ReadCharacterOptions is GET /characters/{entity_id}/options/{kind}: the catalog entries for a
// pick. A slot's own filter (feat categories, a spell filter, the class's subclasses) narrows the
// list; the content rules, prerequisites and slot levels decide `allowed`.
func (h *CharacterRoutes) ReadCharacterOptions(w http.ResponseWriter, r *http.Request, id api.EntityId, kind string, p api.ReadCharacterOptionsParams) {
	out, err := h.options(r, id, kind, p)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *CharacterRoutes) options(r *http.Request, id uuid.UUID, kind string, p api.ReadCharacterOptionsParams) (optionsResponse, error) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return optionsResponse{}, err
	}
	q := db.New(h.replica)
	// the options are the builder's
	if _, err := requireCharacter(ctx, q, id, userID, access.EditBuild); err != nil {
		return optionsResponse{}, err
	}
	st, err := loadState(ctx, q, id)
	if err != nil {
		return optionsResponse{}, notFoundIfMissing(err, id)
	}
	snap, err := LoadSnapshot(ctx, q, id)
	if err != nil {
		return optionsResponse{}, err
	}
	if p.Level != nil {
		snap.Level = *p.Level
	}
	sh := sheet.Compute(snap)
	pickKind := map[string]string{"class": "class"}[kind]
	if pickKind == "" {
		pickKind = kind
	}
	var point *sheet.ChoicePoint
	if p.Slot != nil {
		i := slices.IndexFunc(sh.ChoicePoints, func(cp sheet.ChoicePoint) bool { return cp.Slot == *p.Slot })
		if i < 0 {
			return optionsResponse{}, httpx.Unprocessable("slot: the sheet at level %d has no pick %s (see its choice_points)", sh.Level, *p.Slot)
		}
		point = &sh.ChoicePoints[i]
		if !slices.Contains(point.Kinds, pickKind) {
			return optionsResponse{}, httpx.Unprocessable("slot: %s is answered by %s, not %s", *p.Slot, strings.Join(point.Kinds, " or "), kind)
		}
	}
	filtersOff := p.Filters != nil && *p.Filters == api.ReadCharacterOptionsParamsFiltersOff
	if filtersOff && st.from == "campaign" {
		return optionsResponse{}, httpx.Unprocessable("filters: a campaign's rules are limits; they can't be switched off")
	}
	entries, err := q.OptionEntries(ctx, db.OptionEntriesParams{Category: kind, BaseEdition: int32(st.rules.Edition)})
	if err != nil {
		return optionsResponse{}, err
	}
	// a spell point's picks count, not the prepared list (a prepared spell can still go in the spellbook)
	chosen, err := chosenIDs(r, q, id, !(kind == "spell" && point != nil))
	if err != nil {
		return optionsResponse{}, err
	}
	filter, err := h.kindFilter(r, q, id, kind, point, sh, snap)
	if err != nil {
		return optionsResponse{}, err
	}
	name := ""
	if p.Name != nil {
		name = strings.ToLower(strings.TrimSpace(*p.Name))
	}
	onlyAllowed := p.Allowed != nil && *p.Allowed
	all := []Option{}
	for _, e := range entries {
		if name != "" && !strings.Contains(strings.ToLower(e.Name), name) {
			continue
		}
		opt := Option{ID: e.ID, Name: e.Name, Slug: e.Slug, SourceSlug: e.SourceSlug, DndVersionYear: e.DndVersionYear,
			Reasons: []string{}, Chosen: chosen[e.ID]}
		keep, hard := filter(&opt)
		if !keep {
			continue
		}
		ok, why := st.rules.Allows(rules.Entry{Category: kind, Year: int(e.DndVersionYear), UnearthedArcana: e.IsUnearthedArcana,
			SourceSlug: e.SourceSlug, BaseEditionHasIt: e.BaseHasSlug})
		if !ok {
			opt.Reasons = append(opt.Reasons, why)
		}
		opt.Allowed = len(hard) == 0 && (ok || filtersOff)
		if onlyAllowed && !opt.Allowed && !opt.Chosen {
			continue
		}
		opt.Reasons = append(hard, opt.Reasons...)
		all = append(all, opt)
	}
	limit, offset := 0, 0
	if p.Limit != nil {
		limit = *p.Limit
	}
	if p.Offset != nil {
		offset = *p.Offset
	}
	if limit < 0 || offset < 0 {
		return optionsResponse{}, httpx.Unprocessable("limit and offset can't be negative")
	}
	page := all[min(offset, len(all)):]
	if limit > 0 && len(page) > limit {
		page = page[:limit]
	}
	filters := map[string]any{"kind": kind}
	for key, v := range map[string]*string{"slot": p.Slot, "name": p.Name} {
		if v != nil {
			filters[key] = *v
		}
	}
	if p.Filters != nil {
		filters["filters"] = string(*p.Filters)
	}
	if onlyAllowed {
		filters["allowed"] = true
	}
	return optionsResponse{TotalEntitiesCount: len(all), Limit: limit, Offset: offset, Filters: filters, EntitiesCount: len(page),
		Entities: page, Kind: kind, Slot: p.Slot, Level: sh.Level}, nil
}

// chosenIDs are the catalog entries the character already has: its class, picks and (with
// prepared) prepared spells.
func chosenIDs(r *http.Request, q *db.Queries, id uuid.UUID, prepared bool) (map[uuid.UUID]bool, error) {
	ctx := r.Context()
	out := map[uuid.UUID]bool{}
	choices, err := q.ListChoices(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, row := range choices {
		c := row.DndCharacterChoice
		for _, ref := range []*uuid.UUID{c.SpeciesID, c.SubclassID, c.BackgroundID, c.FeatID, c.SpellID, c.ItemID} {
			if ref != nil {
				out[*ref] = true
			}
		}
	}
	levels, err := q.ListLevels(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, l := range levels {
		out[l.ClassID] = true
	}
	if !prepared {
		return out, nil
	}
	spells, err := q.ListPreparedSpells(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, s := range spells {
		out[s.SpellID] = true
	}
	return out, nil
}

// spellbookIDs are the spells in the character's spellbook under the spellbook rule: the picks
// answering its spellbook choice points at this level. Empty when the rule is off.
func spellbookIDs(r *http.Request, q *db.Queries, id uuid.UUID, sh sheet.Sheet) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if sh.Spellcasting == nil || !sh.Spellcasting.SpellbookRule {
		return out, nil
	}
	inBook := map[string]bool{}
	for _, p := range sh.ChoicePoints {
		if strings.Contains(p.Slot, ":spellbook:") {
			for _, choiceID := range p.ChoiceIDs {
				inBook[choiceID] = true
			}
		}
	}
	choices, err := q.ListChoices(r.Context(), id)
	if err != nil {
		return nil, err
	}
	for _, row := range choices {
		if c := row.DndCharacterChoice; c.SpellID != nil && inBook[c.ID.String()] {
			out[*c.SpellID] = true
		}
	}
	return out, nil
}

// entryFilter says whether an entry belongs in the list, filling in its kind's fields, and the hard
// reasons it can't be picked (prerequisites, slot levels, hard rules), which filters=off doesn't lift.
type entryFilter func(opt *Option) (keep bool, hard []string)

func (h *CharacterRoutes) kindFilter(r *http.Request, q *db.Queries, id uuid.UUID, kind string, point *sheet.ChoicePoint,
	sh sheet.Sheet, snap sheet.Snapshot) (entryFilter, error) {
	ctx := r.Context()
	switch kind {
	case "subclass":
		class, err := q.CharacterClass(ctx, id)
		if err != nil {
			return nil, httpx.Unprocessable("plan level 1 (the class) before choosing a subclass")
		}
		rows, err := q.SubclassOptions(ctx)
		if err != nil {
			return nil, err
		}
		parent := map[uuid.UUID]string{}
		for _, s := range rows {
			parent[s.ID] = s.ParentClassSlug
		}
		return func(opt *Option) (bool, []string) {
			if parent[opt.ID] != class.Slug {
				return false, nil
			}
			if class.DndVersionYear == 2014 && opt.DndVersionYear == 2024 {
				return true, []string{"a 2014 class can't take a 2024 subclass"}
			}
			return true, nil
		}, nil
	case "feat":
		rows, err := q.FeatOptions(ctx)
		if err != nil {
			return nil, err
		}
		details := map[uuid.UUID]db.FeatOptionsRow{}
		for _, f := range rows {
			details[f.ID] = f
		}
		return func(opt *Option) (bool, []string) {
			d := details[opt.ID]
			if d.Category != nil {
				opt.Category = *d.Category
			}
			if point != nil && len(point.From) > 0 {
				byName := strings.HasPrefix(point.Slot, "background:") // the background names its feat
				target := opt.Category
				if byName {
					target = opt.Name
				}
				if !slices.ContainsFunc(point.From, func(f string) bool { return strings.EqualFold(f, target) }) {
					return false, nil
				}
			}
			return true, prerequisiteReasons(d.Prerequisites, sh, snap)
		}, nil
	case "spell":
		rows, err := q.SpellOptions(ctx)
		if err != nil {
			return nil, err
		}
		details := map[uuid.UUID]db.SpellOptionsRow{}
		for _, s := range rows {
			details[s.ID] = s
		}
		if point != nil {
			f := point.SpellFilter
			return func(opt *Option) (bool, []string) {
				d := details[opt.ID]
				opt.Level = &d.Level
				return f == nil || spellMatches(*f, d), nil
			}, nil
		}
		// without a slot: the class's level 1+ spells to prepare (cantrips are picks at each level)
		class, err := q.CharacterClass(ctx, id)
		if err != nil || sh.Spellcasting == nil {
			return nil, httpx.Unprocessable("the character doesn't cast spells at level %d; a feat's or trait's spell picks need slot", sh.Level)
		}
		if sh.Spellcasting.Change == "level_up" {
			return nil, httpx.Unprocessable("%s learns its spells at each level instead of preparing them: list them with slot (its choice points)", sh.Spellcasting.Source)
		}
		highest := sh.Spellcasting.PactSlotLevel
		for i, n := range sh.Spellcasting.Slots {
			if n > 0 {
				highest = max(highest, i+1)
			}
		}
		learned, err := spellbookIDs(r, q, id, sh)
		if err != nil {
			return nil, err
		}
		return func(opt *Option) (bool, []string) {
			d := details[opt.ID]
			opt.Level = &d.Level
			if !slices.Contains(d.ClassIds, class.ID) || d.Level == 0 {
				return false, nil
			}
			opt.Learned = learned[opt.ID]
			if int(d.Level) > highest {
				return true, []string{fmt.Sprintf("level %d; the highest slot at level %d is %d", d.Level, sh.Level, highest)}
			}
			return true, nil
		}, nil
	}
	return func(*Option) (bool, []string) { return true, nil }, nil
}

func spellMatches(f sheet.SpellFilter, d db.SpellOptionsRow) bool {
	if f.Level != nil && int32(*f.Level) != d.Level {
		return false
	}
	if (f.MinLevel != nil && d.Level < int32(*f.MinLevel)) || (f.MaxLevel != nil && d.Level > int32(*f.MaxLevel)) {
		return false
	}
	if f.Ritual != nil && *f.Ritual && !d.IsRitual {
		return false
	}
	if len(f.Schools) > 0 && !slices.ContainsFunc(f.Schools, func(s string) bool { return strings.EqualFold(s, d.School) }) {
		return false
	}
	if len(f.Classes) > 0 && !slices.ContainsFunc(f.Classes, func(c string) bool {
		return slices.ContainsFunc(d.ClassNames, func(n string) bool { return strings.EqualFold(c, n) })
	}) {
		return false
	}
	return true
}

// prerequisiteReasons checks a feat's structured prerequisites against the sheet: level, ability
// minimums, other feats, species. Prose-only ones (a feature, a proficiency) aren't checked.
func prerequisiteReasons(raw []byte, sh sheet.Sheet, snap sheet.Snapshot) []string {
	var p struct {
		MinLevel        *int `json:"min_level"`
		AbilityMinimums []struct {
			Ability string `json:"ability"`
			Score   int    `json:"score"`
			Choice  *struct {
				From []string `json:"from"`
			} `json:"choice"`
		} `json:"ability_minimums"`
		Feats []struct {
			Name string `json:"name"`
		} `json:"feats"`
		Species []string `json:"species"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &p) != nil {
		return nil
	}
	var out []string
	if p.MinLevel != nil && sh.Level < *p.MinLevel {
		out = append(out, fmt.Sprintf("needs level %d", *p.MinLevel))
	}
	for _, m := range p.AbilityMinimums {
		options := []string{m.Ability}
		if m.Choice != nil && len(m.Choice.From) > 0 {
			options = m.Choice.From
		}
		met := slices.ContainsFunc(options, func(a string) bool { return sh.Abilities[a].Score.Value >= m.Score })
		if !met {
			names := make([]string, len(options))
			for i, a := range options {
				names[i] = strings.ToUpper(a[:1]) + a[1:]
			}
			out = append(out, fmt.Sprintf("needs %s %d", strings.Join(names, " or "), m.Score))
		}
	}
	for _, f := range p.Feats {
		has := slices.ContainsFunc(sh.Features, func(ref sheet.FeatureRef) bool {
			return strings.HasPrefix(ref.Source, "Feat") && strings.EqualFold(ref.Name, f.Name)
		})
		if !has {
			out = append(out, "needs the "+f.Name+" feat")
		}
	}
	if len(p.Species) > 0 && (snap.Species == nil || !slices.ContainsFunc(p.Species, func(s string) bool {
		return strings.EqualFold(s, snap.Species.Name)
	})) {
		out = append(out, "needs species "+strings.Join(p.Species, " or "))
	}
	return out
}
