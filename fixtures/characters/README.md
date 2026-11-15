# Character sheet golden fixtures

Shared by both APIs' rules engines (CHARACTER_PLAN.md section 6). Each `<name>.json` holds:

- `snapshot`: the character's build and the catalog data it uses, in the engine's input shape
  (go_dnd `internal/sheet/snapshot.go`)
- `levels`: which levels to compute
- `expect`: hand-checked numbers (`path` into the sheet; `equals`, `has_name` or `lacks_name`, which
  match a list entry's name, source, code, slot or spell; a path can index a list: `attacks.0.damage`)

`<name>.sheet.json` is the whole computed sheet per level, compared byte for byte. After an intended
change: `UPDATE_GOLDEN=1 go test ./internal/sheet` in go_dnd, then review the diff. The hand-checked
`expect` values are what catch a wrong golden.

`live_check.py` builds each fixture as a real character through a running go_dnd (live catalog,
real choice routes), checks its `expect` values and diffs the headline numbers against the golden
sheet. The characters stay ("Fixture <name>") so they can be opened in the UI:
`TTRPG_USER=... TTRPG_PASSWORD=... python3 live_check.py [fixture ...]`. The fixtures whose catalog
parts are copied from the ETL match their goldens; the older hand-written ones differ where the live
data has more (spellcasting, darkvision, species picks).

| Fixture | What it pins down |
|---|---|
| `wizard_levels` | one build viewed at 1, 4 and 7: HP, saves, skills, proficiency bonus; the subclass from 3 |
| `con_14`, `con_16` | base Con 14 vs 16: HP at every level |
| `fighter_armor` | AC: chain mail + shield + a DM's +1, inactive bases, Stealth disadvantage, a "becomes at least" item |
| `subclass_evoker`, `subclass_abjurer` | a subclass retrained at 3 changes only the subclass features |
| `feat_tough`, `feat_tough_swapped` | Tough at 4 vs Great Weapon Master at 4: HP from 4 on, the tooltip, Str |
| `dwarf_fighter` | species effects (Dwarven Toughness, resistance, conditional advantage, darkvision), Defense, weapon attacks |
| `spellcasting` | class save DC / attack / slots, cantrips as picks at each level (one extra at 4), the prepared limit over, Magic Initiate via the background, High Elf lineage spells by level, choice points |
| `wizard_spellbook` | the spellbook rule: spellbook picks at 1 and 2, a spell copied in, a prepared spell outside the spellbook, too few prepared at 2 |
| `monk` | Unarmored Defense, Unarmored Movement from the class table, Martial Arts (Dex, the Martial Arts die once it beats the weapon's), a carried but unused shield |
| `bard` | Jack of All Trades (skills and Initiative), Expertise picks, College of Lore's skills, tool picks from the class, background and Musician; spells learned as picks at each level |

The catalog parts of the newer fixtures (`feat_tough*`, `dwarf_fighter`, `spellcasting`) are copied
from the ETL's processed data, so they use the atoms as the catalog stores them.
