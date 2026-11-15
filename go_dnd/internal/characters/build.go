package characters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/rules"
)

// Choice is one choice the player made, at the level it was made.
type Choice struct {
	ID              uuid.UUID       `json:"id"`
	Level           int32           `json:"level"`
	Kind            string          `json:"kind"`
	Slot            string          `json:"slot"`
	GrantedByID     *uuid.UUID      `json:"granted_by_id"`
	SpeciesID       *uuid.UUID      `json:"species_id"`
	SubclassID      *uuid.UUID      `json:"subclass_id"`
	BackgroundID    *uuid.UUID      `json:"background_id"`
	FeatID          *uuid.UUID      `json:"feat_id"`
	SpellID         *uuid.UUID      `json:"spell_id"`
	ItemID          *uuid.UUID      `json:"item_id"`
	RefName         string          `json:"ref_name,omitempty"` // the picked entry's name
	Value           json.RawMessage `json:"value"`
	OverridesFilter bool            `json:"overrides_filter"`
}

// Item is an item the character carries.
type Item struct {
	ID                uuid.UUID  `json:"id"`
	ItemID            *uuid.UUID `json:"item_id"`
	ItemName          *string    `json:"item_name"`
	ItemSlug          *string    `json:"item_slug"`
	CustomName        *string    `json:"custom_name"`
	CustomDescription *string    `json:"custom_description"`
	Quantity          int32      `json:"quantity"`
	Equipped          bool       `json:"equipped"`
	Attuned           bool       `json:"attuned"`
	LevelReceived     *int32     `json:"level_received"`
	Note              *string    `json:"note"`
	GrantedByID       *uuid.UUID `json:"granted_by_id"`
	// a campaign DM's gift: a custom one's name and description are the DMs' to change
	GivenBy         *uuid.UUID `json:"given_by"`
	GivenCampaignID *uuid.UUID `json:"given_campaign_id"`
}

// PreparedSpell is a class spell the character has prepared.
type PreparedSpell struct {
	SpellID    uuid.UUID `json:"spell_id"`
	ClassID    uuid.UUID `json:"class_id"`
	SpellName  string    `json:"spell_name"`
	SpellSlug  string    `json:"spell_slug"`
	SpellLevel int32     `json:"spell_level"`
	ClassSlug  string    `json:"class_slug"`
}

// Modifier is a boon or bane.
type Modifier struct {
	ID        uuid.UUID       `json:"id"`
	Label     string          `json:"label"`
	Note      *string         `json:"note"`
	Target    string          `json:"target"`
	Op        string          `json:"op"`
	Value     json.RawMessage `json:"value"`
	FromLevel *int32          `json:"from_level"`
	Active    bool            `json:"active"`
	AddedAs   string          `json:"added_as"`
	// given by this campaign's DM: it counts only while the character is in it, and only its DMs change it
	CampaignID *uuid.UUID `json:"campaign_id"`
}

func toChoice(c db.DndCharacterChoice, refName string) Choice {
	return Choice{ID: c.ID, Level: c.Level, Kind: c.Kind, Slot: c.Slot, GrantedByID: c.GrantedByID, SpeciesID: c.SpeciesID,
		SubclassID: c.SubclassID, BackgroundID: c.BackgroundID, FeatID: c.FeatID, SpellID: c.SpellID, ItemID: c.ItemID,
		RefName: refName, Value: c.Value, OverridesFilter: c.OverridesFilter}
}

func toItem(i db.DndCharacterItem, name, slug *string) Item {
	return Item{ID: i.ID, ItemID: i.ItemID, ItemName: name, ItemSlug: slug, CustomName: i.CustomName,
		CustomDescription: i.CustomDescription, Quantity: i.Quantity, Equipped: i.Equipped, Attuned: i.Attuned,
		LevelReceived: i.LevelReceived, Note: i.Note, GrantedByID: i.GrantedByID, GivenBy: i.GivenBy,
		GivenCampaignID: i.GivenCampaignID}
}

func toModifier(m db.DndCharacterModifier) Modifier {
	return Modifier{ID: m.ID, Label: m.Label, Note: m.Note, Target: m.Target, Op: m.Op, Value: m.Value,
		FromLevel: m.FromLevel, Active: m.Active, AddedAs: m.AddedAs, CampaignID: m.CampaignID}
}

// loadBuild fills in the character's choices, items, prepared spells and modifiers.
func loadBuild(ctx context.Context, q *db.Queries, id uuid.UUID, out *Detail) error {
	choices, err := q.ListChoices(ctx, id)
	if err != nil {
		return err
	}
	out.Choices = make([]Choice, len(choices))
	for i, c := range choices {
		out.Choices[i] = toChoice(c.DndCharacterChoice, c.RefName)
	}
	items, err := q.ListItems(ctx, id)
	if err != nil {
		return err
	}
	out.Items = make([]Item, len(items))
	for i, it := range items {
		out.Items[i] = toItem(it.DndCharacterItem, it.ItemName, it.ItemSlug)
	}
	spells, err := q.ListPreparedSpells(ctx, id)
	if err != nil {
		return err
	}
	out.PreparedSpells = make([]PreparedSpell, len(spells))
	for i, s := range spells {
		out.PreparedSpells[i] = PreparedSpell{SpellID: s.SpellID, ClassID: s.ClassID, SpellName: s.SpellName,
			SpellSlug: s.SpellSlug, SpellLevel: s.SpellLevel, ClassSlug: s.ClassSlug}
	}
	mods, err := q.ListModifiers(ctx, id)
	if err != nil {
		return err
	}
	out.Modifiers = make([]Modifier, len(mods))
	for i, m := range mods {
		out.Modifiers[i] = toModifier(m)
	}
	return nil
}

// charState is what most writes need to know about the character.
type charState struct {
	row        db.DndCharacter
	campaignID uuid.UUID
	rules      rules.Rules
	from       string // campaign | character
}

// loadState reads the character for a route that has already checked access (requireCharacter).
func loadState(ctx context.Context, q *db.Queries, id uuid.UUID) (charState, error) {
	row, err := q.GetCharacter(ctx, id)
	if err != nil {
		return charState{}, err
	}
	r, from, err := effectiveRules(ctx, q, row.DndCharacter, row.CampaignID)
	return charState{row: row.DndCharacter, campaignID: row.CampaignID, rules: r, from: from}, err
}

// checkEntry refuses a catalog entry the rules don't offer, or one deleted from the catalog. A
// character with its own rules may pick it anyway with override (stored as overrides_filter); a
// campaign's rules are limits.
func checkEntry(ctx context.Context, q *db.Queries, st charState, category string, id uuid.UUID, override bool) error {
	e, err := q.EntryForRules(ctx, db.EntryForRulesParams{Category: category, ID: id, BaseEdition: int32(st.rules.Edition)})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && e.Deleted) {
		return httpx.Unprocessable("%s_id: %s %s not found", category, category, id)
	}
	if err != nil {
		return err
	}
	ok, why := st.rules.Allows(rules.Entry{Category: category, Year: int(e.DndVersionYear), UnearthedArcana: e.IsUnearthedArcana,
		SourceSlug: e.SourceSlug, BaseEditionHasIt: e.BaseHasSlug})
	switch {
	case ok:
		return nil
	case override && st.from == "character":
		return nil
	case st.from == "character":
		return httpx.Unprocessable("%s_id: %s is outside this character's rules (%s); send overrides_filter: true to pick it anyway", category, e.Name, why)
	default:
		return httpx.Unprocessable("%s_id: %s isn't allowed in this campaign (%s)", category, e.Name, why)
	}
}

// Issue is a rule the character breaks.
type Issue struct {
	Category string    `json:"category"`
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Reason   string    `json:"reason"`
	Approved bool      `json:"approved"` // the DM accepted it as an exception
	Note     *string   `json:"note,omitempty"`
}

// Violations lists the catalog entries the character uses that ruleSet doesn't offer (or that were
// deleted from the catalog). Campaigns use it before adding a character and after a rules change.
func Violations(ctx context.Context, q *db.Queries, characterID uuid.UUID, ruleSet rules.Rules) ([]Issue, error) {
	picks, err := q.CharacterPicks(ctx, db.CharacterPicksParams{CharacterID: characterID, BaseEdition: int32(ruleSet.Edition)})
	if err != nil {
		return nil, err
	}
	issues := []Issue{}
	for _, p := range picks {
		if p.Deleted {
			issues = append(issues, Issue{Category: p.Category, ID: p.RefID, Name: p.Name, Reason: "removed from the catalog"})
			continue
		}
		ok, why := ruleSet.Allows(rules.Entry{Category: p.Category, Year: int(p.DndVersionYear), UnearthedArcana: p.IsUnearthedArcana,
			SourceSlug: p.SourceSlug, BaseEditionHasIt: p.BaseHasSlug})
		if !ok {
			issues = append(issues, Issue{Category: p.Category, ID: p.RefID, Name: p.Name, Reason: why})
		}
	}
	return issues, nil
}

// --- choices ---

var (
	choiceKinds = []string{"species", "subclass", "background", "feat", "asi", "skill", "expertise", "tool", "language",
		"fighting_style", "equipment_option", "spell", "saving_throw", "resistance", "spellcasting_ability", "option"}
	// kinds that pick a catalog row, and its column
	refKinds = map[string]string{"species": "species", "subclass": "subclass", "background": "background", "feat": "feat", "spell": "spell"}
	// one per character (single class: one subclass)
	singleKinds = []string{"species", "background", "subclass"}
	abilities   = []string{"str", "dex", "con", "int", "wis", "cha"}
	skills      = []string{"acrobatics", "animal_handling", "arcana", "athletics", "deception", "history", "insight", "intimidation",
		"investigation", "medicine", "nature", "perception", "performance", "persuasion", "religion", "sleight_of_hand", "stealth", "survival"}
	refFields = []string{"species_id", "subclass_id", "background_id", "feat_id", "spell_id", "item_id"}
)

type choiceInput struct {
	id        *uuid.UUID
	level     int32
	kind      string
	slot      string
	grantedBy *uuid.UUID
	refs      map[string]*uuid.UUID // species_id, ... -> id
	value     []byte
	override  bool
}

func readChoice(f fields) (choiceInput, error) {
	in := choiceInput{refs: map[string]*uuid.UUID{}}
	if _, err := f.decode("id", &in.id, true); err != nil {
		return in, err
	}
	if present, err := f.decode("level", &in.level, false); err != nil {
		return in, err
	} else if !present {
		return in, httpx.Unprocessable("level: field required")
	}
	if err := checkLevel("level", in.level); err != nil {
		return in, err
	}
	if _, err := f.decode("kind", &in.kind, false); err != nil {
		return in, err
	}
	if !slices.Contains(choiceKinds, in.kind) {
		return in, httpx.Unprocessable("kind must be one of %s", strings.Join(choiceKinds, ", "))
	}
	if _, err := f.decode("slot", &in.slot, false); err != nil {
		return in, err
	}
	if in.slot = strings.TrimSpace(in.slot); in.slot == "" {
		return in, httpx.Unprocessable("slot: field required (the choice point it answers, e.g. class:wizard:skills)")
	}
	if _, err := f.decode("granted_by_id", &in.grantedBy, true); err != nil {
		return in, err
	}
	set := 0
	for _, field := range refFields {
		var id *uuid.UUID
		if _, err := f.decode(field, &id, true); err != nil {
			return in, err
		}
		if id != nil {
			in.refs[field] = id
			set++
		}
	}
	if set > 1 {
		return in, httpx.Unprocessable("a choice picks at most one of %s", strings.Join(refFields, ", "))
	}
	if raw, ok := f["value"]; ok && !isNull(raw) {
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil || v == nil {
			return in, httpx.Unprocessable("value must be an object")
		}
		in.value = raw
	}
	if _, err := f.decode("overrides_filter", &in.override, false); err != nil {
		return in, err
	}
	return in, nil
}

// checkChoice applies the kind's shape and the hard rules (CHARACTER_PLAN.md section 3).
func checkChoice(ctx context.Context, q *db.Queries, st charState, characterID uuid.UUID, in choiceInput, exceptID uuid.UUID) error {
	if col, needsRef := refKinds[in.kind]; needsRef {
		if in.refs[col+"_id"] == nil || len(in.refs) != 1 {
			return httpx.Unprocessable("a %s choice needs %s_id", in.kind, col)
		}
		if err := checkEntry(ctx, q, st, col, *in.refs[col+"_id"], in.override); err != nil {
			return err
		}
	} else if len(in.refs) > 0 && (in.kind != "equipment_option" || in.refs["item_id"] == nil) {
		return httpx.Unprocessable("a %s choice doesn't pick a catalog row (only equipment_option may set item_id)", in.kind)
	}
	if in.kind == "equipment_option" && in.refs["item_id"] != nil {
		if live, err := q.CatalogItemLive(ctx, *in.refs["item_id"]); err != nil {
			return err
		} else if !live {
			return httpx.Unprocessable("item_id: item %s not found", *in.refs["item_id"])
		}
	}
	if in.kind == "species" || in.kind == "background" {
		if in.level != 1 {
			return httpx.Unprocessable("a %s is chosen at level 1", in.kind)
		}
	}
	if in.kind == "spell" && in.grantedBy == nil && !classSpellSlot.MatchString(in.slot) {
		return httpx.Unprocessable("a spell choice is a class's per-level pick (slot class:<class>:cantrips:<level>, " +
			":spells:<level> or :spellbook:<level>) or a feat's or feature's own (with granted_by_id); a class's other spells are prepared")
	}
	if slices.Contains(singleKinds, in.kind) {
		n, err := q.CountChoicesOfKind(ctx, db.CountChoicesOfKindParams{CharacterID: characterID, Kind: in.kind, ExceptID: exceptID})
		if err != nil {
			return err
		}
		if n > 0 {
			return httpx.Conflict("the character already has a %s; replace that choice instead", in.kind)
		}
	}
	if in.grantedBy != nil {
		if *in.grantedBy == exceptID {
			return httpx.Unprocessable("granted_by_id can't be the choice itself")
		}
		if _, err := q.GetChoice(ctx, db.GetChoiceParams{ID: *in.grantedBy, CharacterID: characterID}); errors.Is(err, pgx.ErrNoRows) {
			return httpx.Unprocessable("granted_by_id: choice %s not found on this character", *in.grantedBy)
		} else if err != nil {
			return err
		}
	}
	if err := checkValue(in); err != nil {
		return err
	}
	if in.kind == "subclass" {
		return checkSubclass(ctx, q, characterID, *in.refs["subclass_id"], in.level)
	}
	return nil
}

func checkValue(in choiceInput) error {
	var v map[string]any
	_ = json.Unmarshal(in.value, &v)
	switch in.kind {
	case "asi":
		increases, ok := v["increases"].(map[string]any)
		if !ok || len(increases) == 0 {
			return httpx.Unprocessable(`value: an asi needs {"increases": {"str": 2, ...}}`)
		}
		total := 0.0
		for ability, amount := range increases {
			n, isNumber := amount.(float64)
			if !slices.Contains(abilities, ability) || !isNumber || n < 1 || n > 2 || n != float64(int(n)) {
				return httpx.Unprocessable("value.increases: abilities are %s, each +1 or +2", strings.Join(abilities, ", "))
			}
			total += n
		}
		if total > 3 {
			return httpx.Unprocessable("value.increases: at most +3 in all")
		}
	case "skill", "expertise":
		if skill, _ := v["skill"].(string); !slices.Contains(skills, skill) {
			return httpx.Unprocessable(`value: a %s needs {"skill": one of %s}`, in.kind, strings.Join(skills, ", "))
		}
	case "saving_throw", "spellcasting_ability":
		if ability, _ := v["ability"].(string); !slices.Contains(abilities, ability) {
			return httpx.Unprocessable(`value: a %s needs {"ability": one of %s}`, in.kind, strings.Join(abilities, ", "))
		}
	case "resistance":
		if t, _ := v["damage_type"].(string); strings.TrimSpace(t) == "" {
			return httpx.Unprocessable(`value: a resistance needs {"damage_type": "fire"}`)
		}
	case "option":
		if o, _ := v["option"].(string); strings.TrimSpace(o) == "" {
			return httpx.Unprocessable(`value: an option needs {"option": "..."} (one of the choice point's from)`)
		}
	case "tool", "language", "fighting_style", "equipment_option":
		if len(v) == 0 && len(in.refs) == 0 {
			return httpx.Unprocessable(`value: a %s needs a value ({"%s": "..."})`, in.kind, in.kind)
		}
	}
	return nil
}

// checkSubclass: the subclass belongs to the character's class, a 2014 class can't take a 2024
// subclass, and the subclass is taken at its level (a 2024 class always at 3, even a 2014 subclass).
func checkSubclass(ctx context.Context, q *db.Queries, characterID, subclassID uuid.UUID, level int32) error {
	class, err := q.CharacterClass(ctx, characterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Unprocessable("plan level 1 (the class) before choosing a subclass")
	}
	if err != nil {
		return err
	}
	sub, err := q.SubclassInfo(ctx, subclassID)
	if err != nil {
		return err
	}
	if sub.ParentClassSlug != class.Slug {
		return httpx.Unprocessable("subclass_id: that's a %s subclass, and the character is a %s", sub.ParentClassSlug, class.Slug)
	}
	if class.DndVersionYear == 2014 && sub.DndVersionYear == 2024 {
		return httpx.Unprocessable("subclass_id: a 2014 class can't take a 2024 subclass")
	}
	want := sub.LevelGained
	if class.DndVersionYear == 2024 {
		want = 3
	}
	if level != want {
		return httpx.Unprocessable("level: this subclass is chosen at level %d", want)
	}
	return nil
}

func choiceParams(in choiceInput) (granted, species, subclass, background, feat, spell, item *uuid.UUID) {
	return in.grantedBy, in.refs["species_id"], in.refs["subclass_id"], in.refs["background_id"], in.refs["feat_id"],
		in.refs["spell_id"], in.refs["item_id"]
}

// classSpellSlot is a class's own spell picks (the sheet's choice points): cantrips and learned
// spells at each level, a spellbook's, and cantrips a class feature adds (Divine Order: Thaumaturge).
var classSpellSlot = regexp.MustCompile(`^class:[a-z0-9_-]+:((cantrips|spells|spellbook):([0-9]+|copied)|[a-z0-9-]+:cantrips_known)$`)

var choiceFields = []string{"id", "level", "kind", "slot", "granted_by_id", "species_id", "subclass_id", "background_id",
	"feat_id", "spell_id", "item_id", "value", "overrides_filter"}

// CreateCharacterChoice is POST /characters/{entity_id}/choices.
func (h *CharacterRoutes) CreateCharacterChoice(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.CreateCharacterChoiceParams) {
	f, err := readFields(r, choiceFields...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		in, err := readChoice(f)
		if err != nil {
			return change{}, err
		}
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		if in.id != nil { // a replayed offline create
			if _, err := q.GetChoice(ctx, db.GetChoiceParams{ID: *in.id, CharacterID: id}); err == nil {
				return change{}, nil // already created
			}
		}
		if err := checkChoice(ctx, q, st, id, in, uuid.Nil); err != nil {
			return change{}, err
		}
		g, sp, sc, bg, ft, sl, it := choiceParams(in)
		saved, err := q.InsertChoice(ctx, db.InsertChoiceParams{ID: in.id, CharacterID: id, Level: in.level, Kind: in.kind, Slot: in.slot,
			GrantedByID: g, SpeciesID: sp, SubclassID: sc, BackgroundID: bg, FeatID: ft, SpellID: sl, ItemID: it,
			Value: in.value, OverridesFilter: in.override, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		if isEquipmentSlot(in.slot) {
			if saved.Value, err = grantEquipment(ctx, q, st, id, saved.ID, in.slot, in.value, userID); err != nil {
				return change{}, err
			}
			if err := q.SetChoiceValue(ctx, db.SetChoiceValueParams{Value: saved.Value, ID: saved.ID, CharacterID: id}); err != nil {
				return change{}, err
			}
		}
		return change{action: "choice.add", entity: "character_choice", entityID: &saved.ID, after: toChoice(saved, "")}, nil
	})
}

// ReplaceCharacterChoice is PUT /characters/{entity_id}/choices/{choice_id}: the whole choice. Choices
// it granted stay linked to it, and the sheet flags any that no longer fit.
func (h *CharacterRoutes) ReplaceCharacterChoice(w http.ResponseWriter, r *http.Request, id api.EntityId, choiceID api.ChoiceId, _ api.ReplaceCharacterChoiceParams) {
	f, err := readFields(r, choiceFields...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetChoice(ctx, db.GetChoiceParams{ID: choiceID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Choice %s not found", choiceID)
		}
		if err != nil {
			return change{}, err
		}
		in, err := readChoice(f)
		if err != nil {
			return change{}, err
		}
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		if err := checkChoice(ctx, q, st, id, in, choiceID); err != nil {
			return change{}, err
		}
		// a new equipment package takes back the old one's untouched items and coins; the same one keeps its record
		regrant := false
		if isEquipmentSlot(existing.Slot) || isEquipmentSlot(in.slot) {
			if existing.Slot == in.slot && strings.EqualFold(pickedOption(existing.Value), pickedOption(in.value)) {
				in.value = existing.Value
			} else {
				if err := ungrantEquipment(ctx, q, st, id, existing.Value, userID); err != nil {
					return change{}, err
				}
				regrant = isEquipmentSlot(in.slot)
			}
		}
		g, sp, sc, bg, ft, sl, it := choiceParams(in)
		saved, err := q.UpdateChoice(ctx, db.UpdateChoiceParams{ID: choiceID, CharacterID: id, Level: in.level, Kind: in.kind, Slot: in.slot,
			GrantedByID: g, SpeciesID: sp, SubclassID: sc, BackgroundID: bg, FeatID: ft, SpellID: sl, ItemID: it,
			Value: in.value, OverridesFilter: in.override, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		if regrant {
			if saved.Value, err = grantEquipment(ctx, q, st, id, saved.ID, in.slot, in.value, userID); err != nil {
				return change{}, err
			}
			if err := q.SetChoiceValue(ctx, db.SetChoiceValueParams{Value: saved.Value, ID: saved.ID, CharacterID: id}); err != nil {
				return change{}, err
			}
		}
		return change{action: "choice.update", entity: "character_choice", entityID: &saved.ID,
			before: toChoice(existing, ""), after: toChoice(saved, "")}, nil
	})
}

// DeleteCharacterChoice is DELETE /characters/{entity_id}/choices/{choice_id}. A choice that granted
// other choices (a background's feat) is a 409 until those are removed or re-linked; items it granted
// stay.
func (h *CharacterRoutes) DeleteCharacterChoice(w http.ResponseWriter, r *http.Request, id api.EntityId, choiceID api.ChoiceId, _ api.DeleteCharacterChoiceParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetChoice(ctx, db.GetChoiceParams{ID: choiceID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Choice %s not found", choiceID)
		}
		if err != nil {
			return change{}, err
		}
		deps, err := q.ChoiceDependents(ctx, choiceID)
		if err != nil {
			return change{}, err
		}
		var blocking []string
		for _, d := range deps {
			if d.Kind == "choice" {
				blocking = append(blocking, d.ID.String())
			}
		}
		if len(blocking) > 0 {
			return change{}, httpx.Conflict("this choice granted %d other choice(s); remove or re-link them first: %s",
				len(blocking), strings.Join(blocking, ", "))
		}
		if isEquipmentSlot(existing.Slot) {
			st, err := loadState(ctx, q, id)
			if err != nil {
				return change{}, err
			}
			if err := ungrantEquipment(ctx, q, st, id, existing.Value, userID); err != nil {
				return change{}, err
			}
		}
		if err := q.DeleteChoice(ctx, db.DeleteChoiceParams{ID: choiceID, CharacterID: id}); err != nil {
			return change{}, err
		}
		return change{action: "choice.remove", entity: "character_choice", entityID: &existing.ID, before: toChoice(existing, "")}, nil
	})
}

// --- items ---

var itemFields = []string{"id", "item_id", "custom_name", "custom_description", "quantity", "equipped", "attuned",
	"level_received", "note", "granted_by_id"}

func optionalText(f fields, key string, current *string) (*string, error) {
	raw, ok := f[key]
	if !ok {
		return current, nil
	}
	if isNull(raw) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, httpx.Unprocessable("%s must be a string", key)
	}
	if s = strings.TrimSpace(s); s == "" {
		return nil, nil
	}
	return &s, nil
}

type itemValues struct {
	customName, customDescription, note *string
	quantity                            int32
	equipped, attuned                   bool
	levelReceived                       *int32
}

func readItemValues(f fields, cur itemValues) (itemValues, error) {
	var err error
	if cur.customName, err = optionalText(f, "custom_name", cur.customName); err != nil {
		return cur, err
	}
	if cur.customDescription, err = optionalText(f, "custom_description", cur.customDescription); err != nil {
		return cur, err
	}
	if cur.note, err = optionalText(f, "note", cur.note); err != nil {
		return cur, err
	}
	if _, err := f.decode("quantity", &cur.quantity, false); err != nil {
		return cur, err
	}
	if cur.quantity < 0 {
		return cur, httpx.Unprocessable("quantity can't be negative")
	}
	if _, err := f.decode("equipped", &cur.equipped, false); err != nil {
		return cur, err
	}
	if _, err := f.decode("attuned", &cur.attuned, false); err != nil {
		return cur, err
	}
	if _, err := f.decode("level_received", &cur.levelReceived, true); err != nil {
		return cur, err
	}
	if cur.levelReceived != nil {
		if err := checkLevel("level_received", *cur.levelReceived); err != nil {
			return cur, err
		}
	}
	return cur, nil
}

func itemChange(i db.DndCharacterItem) map[string]any {
	return map[string]any{"item_id": i.ItemID, "custom_name": i.CustomName, "quantity": i.Quantity, "equipped": i.Equipped,
		"attuned": i.Attuned, "level_received": i.LevelReceived, "note": i.Note}
}

// CreateCharacterItem is POST /characters/{entity_id}/items: a catalog item (item_id) or a custom
// one (custom_name). Items aren't held to the content rules: a DM can hand out anything.
func (h *CharacterRoutes) CreateCharacterItem(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.CreateCharacterItemParams) {
	f, err := readFields(r, itemFields...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		var clientID, itemID, grantedBy *uuid.UUID
		for key, dst := range map[string]**uuid.UUID{"id": &clientID, "item_id": &itemID, "granted_by_id": &grantedBy} {
			if _, err := f.decode(key, dst, true); err != nil {
				return change{}, err
			}
		}
		if clientID != nil {
			if _, err := q.GetItem(ctx, db.GetItemParams{ID: *clientID, CharacterID: id}); err == nil {
				return change{}, nil // already created
			}
		}
		v, err := readItemValues(f, itemValues{quantity: 1})
		if err != nil {
			return change{}, err
		}
		if itemID == nil && v.customName == nil {
			return change{}, httpx.Unprocessable("an item needs item_id (from the catalog) or custom_name")
		}
		if itemID != nil {
			if live, err := q.CatalogItemLive(ctx, *itemID); err != nil {
				return change{}, err
			} else if !live {
				return change{}, httpx.Unprocessable("item_id: item %s not found", *itemID)
			}
		}
		if grantedBy != nil {
			if _, err := q.GetChoice(ctx, db.GetChoiceParams{ID: *grantedBy, CharacterID: id}); errors.Is(err, pgx.ErrNoRows) {
				return change{}, httpx.Unprocessable("granted_by_id: choice %s not found on this character", *grantedBy)
			} else if err != nil {
				return change{}, err
			}
		}
		saved, err := q.InsertItem(ctx, db.InsertItemParams{ID: clientID, CharacterID: id, ItemID: itemID, CustomName: v.customName,
			CustomDescription: v.customDescription, Quantity: v.quantity, Equipped: v.equipped, Attuned: v.attuned,
			LevelReceived: v.levelReceived, Note: v.note, GrantedByID: grantedBy, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		return change{action: "item.add", entity: "character_item", entityID: &saved.ID, after: itemChange(saved)}, nil
	})
}

// UpdateCharacterItem is PATCH /characters/{entity_id}/items/{item_id}: quantity, equipped,
// attuned, level_received, note, custom_name, custom_description. The player's (the owner's and
// editors'), except a DM's gift's custom_name and custom_description, which are its campaign's DMs'.
func (h *CharacterRoutes) UpdateCharacterItem(w http.ResponseWriter, r *http.Request, id api.EntityId, itemID api.ItemId, _ api.UpdateCharacterItemParams) {
	f, err := readFields(r, "custom_name", "custom_description", "quantity", "equipped", "attuned", "level_received", "note")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.ChangeInventory, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetItem(ctx, db.GetItemParams{ID: itemID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Item %s not found on this character", itemID)
		}
		if err != nil {
			return change{}, err
		}
		if err := mayChangeItem(caller(ctx), existing, f); err != nil {
			return change{}, err
		}
		v, err := readItemValues(f, itemValues{customName: existing.CustomName, customDescription: existing.CustomDescription,
			note: existing.Note, quantity: existing.Quantity, equipped: existing.Equipped, attuned: existing.Attuned,
			levelReceived: existing.LevelReceived})
		if err != nil {
			return change{}, err
		}
		if existing.ItemID == nil && v.customName == nil {
			return change{}, httpx.Unprocessable("custom_name: a custom item needs a name")
		}
		saved, err := q.UpdateItem(ctx, db.UpdateItemParams{ID: itemID, CharacterID: id, CustomName: v.customName,
			CustomDescription: v.customDescription, Quantity: v.quantity, Equipped: v.equipped, Attuned: v.attuned,
			LevelReceived: v.levelReceived, Note: v.note, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		return change{action: "item.update", entity: "character_item", entityID: &saved.ID, before: itemChange(existing), after: itemChange(saved)}, nil
	})
}

// DeleteCharacterItem is DELETE /characters/{entity_id}/items/{item_id}: the player drops an item
// (gifts too), or a DM takes back a gift from their campaign that nobody has changed since.
func (h *CharacterRoutes) DeleteCharacterItem(w http.ResponseWriter, r *http.Request, id api.EntityId, itemID api.ItemId, _ api.DeleteCharacterItemParams) {
	h.write(w, r, id, access.ChangeInventory, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetItem(ctx, db.GetItemParams{ID: itemID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Item %s not found on this character", itemID)
		}
		if err != nil {
			return change{}, err
		}
		if who := caller(ctx); !who.Can(access.EditBuild) {
			switch {
			case existing.GivenBy == nil || !who.DMOf(existing.GivenCampaignID):
				return change{}, httpx.Forbidden("only the character's owner and editors can drop an item; a DM takes back only their campaign's gifts")
			case !existing.UpdatedAt.Time.Equal(existing.CreatedAt.Time):
				return change{}, httpx.Conflict("the player has changed this gift since it was given; ask them to drop it")
			}
		}
		if err := q.DeleteItem(ctx, db.DeleteItemParams{ID: itemID, CharacterID: id}); err != nil {
			return change{}, err
		}
		return change{action: "item.remove", entity: "character_item", entityID: &existing.ID, before: itemChange(existing)}, nil
	})
}

// --- prepared spells ---

// PrepareCharacterSpell is PUT /characters/{entity_id}/prepared-spells/{spell_id}: a spell on the
// class's list, within the content rules. Going over the class's limit is allowed (the sheet warns).
func (h *CharacterRoutes) PrepareCharacterSpell(w http.ResponseWriter, r *http.Request, id api.EntityId, spellID api.SpellId, _ api.PrepareCharacterSpellParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		class, err := q.CharacterClass(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.Unprocessable("plan level 1 (the class) before preparing spells")
		}
		if err != nil {
			return change{}, err
		}
		if err := checkEntry(ctx, q, st, "spell", spellID, false); err != nil {
			return change{}, err
		}
		prep, err := q.ClassSpellPreparation(ctx, db.ClassSpellPreparationParams{ClassID: class.ID, SpellID: spellID})
		if err != nil {
			return change{}, err
		}
		if prep.SpellLevel == 0 {
			return change{}, httpx.Unprocessable("spell_id: cantrips are picked at each class level (the class's cantrip choice points), not prepared")
		}
		if prep.Change == "level_up" {
			return change{}, httpx.Unprocessable("%s learns its spells at each level instead of preparing them: pick them at their levels", class.Name)
		}
		onList, err := q.SpellOnClassList(ctx, db.SpellOnClassListParams{SpellID: spellID, ClassID: class.ID})
		if err != nil {
			return change{}, err
		}
		if !onList {
			return change{}, httpx.Unprocessable("spell_id: that spell isn't on the %s spell list", class.Name)
		}
		if _, err := q.AddPreparedSpell(ctx, db.AddPreparedSpellParams{CharacterID: id, SpellID: spellID, ClassID: class.ID, UserID: &userID}); err != nil {
			return change{}, err
		}
		sid := uuid.UUID(spellID)
		return change{action: "spell.prepare", entity: "spell", entityID: &sid, after: map[string]any{"class_id": class.ID}}, nil
	})
}

// UnprepareCharacterSpell is DELETE /characters/{entity_id}/prepared-spells/{spell_id}.
func (h *CharacterRoutes) UnprepareCharacterSpell(w http.ResponseWriter, r *http.Request, id api.EntityId, spellID api.SpellId, _ api.UnprepareCharacterSpellParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		class, err := q.CharacterClass(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Spell %s isn't prepared", spellID)
		}
		if err != nil {
			return change{}, err
		}
		n, err := q.RemovePreparedSpell(ctx, db.RemovePreparedSpellParams{CharacterID: id, SpellID: spellID, ClassID: class.ID})
		if err != nil {
			return change{}, err
		}
		if n == 0 {
			return change{}, httpx.NotFound("Spell %s isn't prepared", spellID)
		}
		sid := uuid.UUID(spellID)
		return change{action: "spell.unprepare", entity: "spell", entityID: &sid, before: map[string]any{"class_id": class.ID}}, nil
	})
}

// --- modifiers (boons and banes) ---

var (
	modifierOps = []string{"add", "set", "base", "min", "max", "adv", "disadv", "resist", "immune", "vuln", "condition_immune", "grant"}
	targetShape = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)
)

type modifierValues struct {
	label     string
	note      *string
	target    string
	op        string
	value     []byte
	fromLevel *int32
	active    bool
	addedAs   string
}

func readModifier(f fields, cur modifierValues, full bool) (modifierValues, error) {
	if full {
		for _, key := range []string{"label", "target", "op", "value"} {
			if _, ok := f[key]; !ok {
				return cur, httpx.Unprocessable("%s: field required", key)
			}
		}
	}
	if _, err := f.decode("label", &cur.label, false); err != nil {
		return cur, err
	}
	if cur.label = strings.TrimSpace(cur.label); cur.label == "" {
		return cur, httpx.Unprocessable("label can't be empty")
	}
	var err error
	if cur.note, err = optionalText(f, "note", cur.note); err != nil {
		return cur, err
	}
	if _, err := f.decode("target", &cur.target, false); err != nil {
		return cur, err
	}
	if !targetShape.MatchString(cur.target) {
		return cur, httpx.Unprocessable("target must be a dotted path like ac, ability.str, skill.stealth")
	}
	if _, err := f.decode("op", &cur.op, false); err != nil {
		return cur, err
	}
	if !slices.Contains(modifierOps, cur.op) {
		return cur, httpx.Unprocessable("op must be one of %s", strings.Join(modifierOps, ", "))
	}
	if raw, ok := f["value"]; ok {
		if isNull(raw) || !json.Valid(raw) {
			return cur, httpx.Unprocessable("value must be JSON (2, -1, {\"dice\": \"1d4\"}, ...)")
		}
		cur.value = raw
	}
	if _, err := f.decode("from_level", &cur.fromLevel, true); err != nil {
		return cur, err
	}
	if cur.fromLevel != nil {
		if err := checkLevel("from_level", *cur.fromLevel); err != nil {
			return cur, err
		}
	}
	if _, err := f.decode("active", &cur.active, false); err != nil {
		return cur, err
	}
	if _, err := f.decode("added_as", &cur.addedAs, false); err != nil {
		return cur, err
	}
	if cur.addedAs != "player" && cur.addedAs != "dm" {
		return cur, httpx.Unprocessable("added_as must be player or dm")
	}
	return cur, nil
}

var modifierFields = []string{"label", "note", "target", "op", "value", "from_level", "active", "added_as"}

// CreateCharacterModifier is POST /characters/{entity_id}/modifiers: a boon or bane ("+1 AC, DM's
// blessing"). The hard limits still apply on the sheet (scores stay 1-30).
func (h *CharacterRoutes) CreateCharacterModifier(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.CreateCharacterModifierParams) {
	f, err := readFields(r, append([]string{"id"}, modifierFields...)...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		var clientID *uuid.UUID
		if _, err := f.decode("id", &clientID, true); err != nil {
			return change{}, err
		}
		if v, ok, err := addedAs(f); err != nil || (ok && v != "player") {
			return change{}, httpx.Unprocessable("added_as: a DM gives boons and banes through the campaign's gift route")
		}
		if clientID != nil {
			if _, err := q.GetModifier(ctx, db.GetModifierParams{ID: *clientID, CharacterID: id}); err == nil {
				return change{}, nil // already created
			}
		}
		v, err := readModifier(f, modifierValues{active: true, addedAs: "player"}, true)
		if err != nil {
			return change{}, err
		}
		saved, err := q.InsertModifier(ctx, db.InsertModifierParams{ID: clientID, CharacterID: id, Label: v.label, Note: v.note,
			Target: v.target, Op: v.op, Value: v.value, FromLevel: v.fromLevel, Active: v.active, AddedAs: v.addedAs, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		return change{action: "modifier.add", entity: "character_modifier", entityID: &saved.ID, after: toModifier(saved)}, nil
	})
}

// UpdateCharacterModifier is PATCH /characters/{entity_id}/modifiers/{modifier_id} (switch a curse
// off with {"active": false}). A DM's gift is its campaign's DMs' to change, not the player's.
func (h *CharacterRoutes) UpdateCharacterModifier(w http.ResponseWriter, r *http.Request, id api.EntityId, modifierID api.ModifierId, _ api.UpdateCharacterModifierParams) {
	f, err := readFields(r, modifierFields...)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.ChangeInventory, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetModifier(ctx, db.GetModifierParams{ID: modifierID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Modifier %s not found on this character", modifierID)
		}
		if err != nil {
			return change{}, err
		}
		if err := mayChangeModifier(caller(ctx), existing); err != nil {
			return change{}, err
		}
		if v, ok, err := addedAs(f); err != nil || (ok && v != existing.AddedAs) {
			return change{}, httpx.Unprocessable("added_as: it stays %s", existing.AddedAs)
		}
		v, err := readModifier(f, modifierValues{label: existing.Label, note: existing.Note, target: existing.Target, op: existing.Op,
			value: existing.Value, fromLevel: existing.FromLevel, active: existing.Active, addedAs: existing.AddedAs}, false)
		if err != nil {
			return change{}, err
		}
		saved, err := q.UpdateModifier(ctx, db.UpdateModifierParams{ID: modifierID, CharacterID: id, Label: v.label, Note: v.note,
			Target: v.target, Op: v.op, Value: v.value, FromLevel: v.fromLevel, Active: v.active, AddedAs: v.addedAs, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		return change{action: "modifier.update", entity: "character_modifier", entityID: &saved.ID, before: toModifier(existing), after: toModifier(saved)}, nil
	})
}

// DeleteCharacterModifier is DELETE /characters/{entity_id}/modifiers/{modifier_id}. A DM's gift is
// its campaign's DMs' to remove.
func (h *CharacterRoutes) DeleteCharacterModifier(w http.ResponseWriter, r *http.Request, id api.EntityId, modifierID api.ModifierId, _ api.DeleteCharacterModifierParams) {
	h.write(w, r, id, access.ChangeInventory, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		existing, err := q.GetModifier(ctx, db.GetModifierParams{ID: modifierID, CharacterID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Modifier %s not found on this character", modifierID)
		}
		if err != nil {
			return change{}, err
		}
		if err := mayChangeModifier(caller(ctx), existing); err != nil {
			return change{}, err
		}
		if err := q.DeleteModifier(ctx, db.DeleteModifierParams{ID: modifierID, CharacterID: id}); err != nil {
			return change{}, err
		}
		return change{action: "modifier.remove", entity: "character_modifier", entityID: &existing.ID, before: toModifier(existing)}, nil
	})
}
