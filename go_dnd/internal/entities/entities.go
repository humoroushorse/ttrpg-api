// Package entities describes the catalog entities that use the shared catalog routes unchanged
// (conditions, glossary, ...). `go run ./cmd/gencatalog` turns List into their OpenAPI paths
// (api/openapi/src/catalog.gen.yaml) and route methods (routes.gen.go).
package entities

import (
	"encoding/json"
	"fmt"

	"github.com/humoroushorse/go_dnd/internal/catalog"
	"github.com/humoroushorse/go_dnd/internal/grid"
	"github.com/humoroushorse/go_dnd/internal/table"
)

// Entity is a catalog entity and how its routes are named.
type Entity struct {
	catalog.Entity
	Path     string // URL segment: "conditions"
	Tag      string // OpenAPI tag: "Condition"
	Singular string // operation names: "Condition" (readCondition)
	Plural   string // operation names: "Conditions" (queryConditions)
}

// columns are the shared catalog columns, the entity's own in between (in the table's order).
func columns(own ...table.Col) []table.Col {
	cols := []table.Col{
		{Name: "id", Type: table.UUID, Server: true},
		{Name: "source_id", Type: table.UUID, Required: true},
		{Name: "slug", Type: table.Text, Required: true, Grid: grid.Text},
		{Name: "name", Type: table.Text, Required: true, Grid: grid.Text},
		{Name: "dnd_version", Type: table.Text, Required: true},
		{Name: "dnd_version_year", Type: table.Int, Required: true, Grid: grid.Year},
		{Name: "is_unearthed_arcana", Type: table.Bool, Grid: grid.Boolean},
		{Name: "is_homebrew", Type: table.Bool, Grid: grid.Boolean},
		{Name: "marks", Type: table.TextArray},
		{Name: "description", Type: table.Text, Required: true, Aliases: []string{"description_html"}, Grid: grid.Text},
	}
	cols = append(cols, own...)
	return append(cols,
		table.Col{Name: "created_at", Type: table.Time, Server: true},
		table.Col{Name: "created_by", Type: table.UUID, Server: true, Nullable: true},
		table.Col{Name: "updated_at", Type: table.Time, Server: true},
		table.Col{Name: "updated_by", Type: table.UUID, Server: true, Nullable: true},
	)
}

func newEntity(name, label, path, tag, singular, plural string, own ...table.Col) Entity {
	t := (&table.Table{
		Name:   name,
		Label:  label,
		Source: true,
		Key:    []string{"source_id", "slug"},
		Order:  "t.name, t.dnd_version_year DESC, t.id",
		Cols:   columns(own...),
	}).Init()
	return Entity{Entity: catalog.Entity{Table: t, BareSlug: true}, Path: path, Tag: tag, Singular: singular, Plural: plural}
}

// checkLevels: a leveled table's rows (Exhaustion levels), each exactly {level: int, effect: text}.
func checkLevels(v any) (any, error) {
	for i, item := range v.([]any) {
		row, ok := item.(map[string]any)
		_, isNumber := row["level"].(json.Number)
		_, isText := row["effect"].(string)
		if !ok || len(row) != 2 || !isNumber || !isText {
			return nil, fmt.Errorf("[%d] must be {level: integer, effect: string}", i)
		}
	}
	return v, nil
}

var (
	levels  = table.Col{Name: "levels", Type: table.JSON, Shape: "array", Check: checkLevels}
	effects = table.Col{Name: "effects", Type: table.JSON, Shape: "array"}
)

// Conditions is dnd.condition (Blinded, Exhaustion, ...).
var Conditions = newEntity("dnd.condition", "condition", "conditions", "Condition", "Condition", "Conditions",
	levels, effects)

// Glossary is dnd.glossary_entry: rules glossary entries.
var Glossary = newEntity("dnd.glossary_entry", "glossary entry", "glossary", "Glossary", "GlossaryEntry", "GlossaryEntries",
	table.Col{Name: "category", Type: table.Text, Default: "general", Grid: grid.Set},
	levels, effects)

func text(name string, grid grid.Kind) table.Col {
	return table.Col{Name: name, Type: table.Text, Grid: grid}
}

func optionalText(name string, grid grid.Kind) table.Col {
	return table.Col{Name: name, Type: table.Text, Nullable: true, Grid: grid}
}

func required(name string, grid grid.Kind) table.Col {
	return table.Col{Name: name, Type: table.Text, Required: true, Grid: grid}
}

func jsonList(name string) table.Col { return table.Col{Name: name, Type: table.JSON, Shape: "array"} }

func textList(name string) table.Col { return table.Col{Name: name, Type: table.TextArray} }

// Species is dnd.species: species (2024) or race / lineage (2014).
var Species = newEntity("dnd.species", "species", "species", "Species", "Species", "Species",
	required("size", grid.Text), // "Medium", "Small", "Medium or Small"
	required("speed", grid.Text),
	required("creature_type", grid.Set),
	jsonList("traits"),
)

// Classes is dnd.dnd_class (class is a keyword in Python and Go).
var Classes = newEntity("dnd.dnd_class", "class", "classes", "Class", "Class", "Classes",
	required("hit_die", 0), // as printed: "1d10 per fighter level"
	table.Col{Name: "hit_die_size", Type: table.Int, Nullable: true, Grid: grid.Number},
	text("primary_ability", grid.Text),
	table.Col{Name: "proficiencies", Type: table.JSON, Shape: "object"},
	text("starting_equipment", 0),
	jsonList("features"),
	textList("subclass_slugs"),
	optionalText("spell_list", 0),
	// from 5e.tools for a caster: int / wis / cha; full, half, half_up, third, pact; levels 1-20
	optionalText("spellcasting_ability", grid.Set),
	optionalText("caster_progression", grid.Set),
	jsonList("level_table"),
	// from 5e.tools for a caster: {change: long_rest | level_up, spellbook: [spells added per level],
	// spell_list: a subclass's class list}
	table.Col{Name: "spell_preparation", Type: table.JSON, Shape: "object", Nullable: true},
	// from 5e.tools (2024): the A / B packages {label, items, coins, grants, picks, custom}
	jsonList("starting_equipment_options"),
)

// Subclasses is dnd.subclass.
var Subclasses = newEntity("dnd.subclass", "subclass", "subclasses", "Subclass", "Subclass", "Subclasses",
	required("parent_class_slug", grid.Set),
	table.Col{Name: "level_gained", Type: table.Int, Grid: grid.Number},
	jsonList("features"),
	textList("spell_list_additions"),
	// from 5e.tools for a caster: int / wis / cha; full, half, half_up, third, pact; levels 1-20
	optionalText("spellcasting_ability", grid.Set),
	optionalText("caster_progression", grid.Set),
	jsonList("level_table"),
	table.Col{Name: "spell_preparation", Type: table.JSON, Shape: "object", Nullable: true},
)

// Backgrounds is dnd.background.
var Backgrounds = newEntity("dnd.background", "background", "backgrounds", "Background", "Background", "Backgrounds",
	text("ability_score_bonuses", grid.Text),
	textList("skill_proficiencies"),
	textList("tool_proficiencies"),
	text("feat", grid.Text),
	text("equipment_text", 0),
	jsonList("equipment_options"),
)

// Feats is dnd.feat.
var Feats = newEntity("dnd.feat", "feat", "feats", "Feat", "Feat", "Feats",
	optionalText("category", grid.Set),
	table.Col{Name: "prerequisites", Type: table.JSON, Shape: "object", Nullable: true},
	effects,
)

// Items is dnd.item: mundane gear and magic items share the table (is_magic).
var Items = newEntity("dnd.item", "item", "items", "Item", "Item", "Items",
	optionalText("category", grid.Set),
	optionalText("subcategory", grid.Set),
	optionalText("cost", 0),
	optionalText("weight", 0),
	table.Col{Name: "properties", Type: table.JSON, Shape: "object"},
	effects,
	table.Col{Name: "is_magic", Type: table.Bool, Grid: grid.Boolean},
	optionalText("magic_item_type", grid.Set),
	optionalText("base_item", 0),
	optionalText("rarity", grid.Set),
	optionalText("rarity_text", 0),
	table.Col{Name: "requires_attunement", Type: table.Bool, Grid: grid.Boolean},
	optionalText("attunement_requirement", 0),
)

// List is every entity, in route order.
var List = []Entity{Conditions, Glossary, Species, Classes, Subclasses, Backgrounds, Feats, Items}
