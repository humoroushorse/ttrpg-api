package characters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
)

// Starting equipment: the sheet's level-1 equipment picks. Saving one adds what its package gives
// (items linked to the pick by granted_by_id, and coins), and records that on the pick's value
// ("granted"). Changing or removing the pick takes back what's untouched, from that record: items
// changed since (quantity, equipped, attuned, a note) and items already gone stay as they are, and
// coins move by the difference, so coins spent since stay spent.

var (
	equipmentSlot     = regexp.MustCompile(`^(class|background):[a-z0-9_-]+:equipment$`)
	equipmentPickSlot = regexp.MustCompile(`^(class|background):[a-z0-9_-]+:equipment:pick(:[0-9]+)?$`)
	coinOrder         = []string{"pp", "gp", "ep", "sp", "cp"}
)

// catalogEquipmentOption is a package as the catalog stores it (the ETL's EquipmentOption).
type catalogEquipmentOption struct {
	Label  *string        `json:"label"`
	Coins  map[string]int `json:"coins"`
	Grants []struct {
		Name        string  `json:"name"`
		Quantity    int     `json:"quantity"`
		DisplayName *string `json:"display_name"`
	} `json:"grants"`
	Picks []struct {
		Categories []string `json:"categories"`
		Count      int      `json:"count"`
	} `json:"picks"`
	Custom []string `json:"custom"`
}

// equipmentGrant is what a pick added, kept on its value as "granted".
type equipmentGrant struct {
	Items []grantedItem  `json:"items"`
	Coins map[string]int `json:"coins,omitempty"`
}

type grantedItem struct {
	ID       uuid.UUID `json:"id"` // the character_item row
	Quantity int32     `json:"quantity"`
}

func coinText(coins map[string]int) string {
	var parts []string
	for _, c := range coinOrder {
		if n := coins[c]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToUpper(c)))
		}
	}
	return strings.Join(parts, ", ")
}

func isEquipmentSlot(slot string) bool {
	return equipmentSlot.MatchString(slot) || equipmentPickSlot.MatchString(slot)
}

// pickedOption reads a pick's value: {"option": "A"} (a package) or {"option": "Lute"} (a pick's item).
func pickedOption(value []byte) string {
	var v struct {
		Option string `json:"option"`
	}
	_ = json.Unmarshal(value, &v)
	return strings.TrimSpace(v.Option)
}

func recordedGrant(value []byte) equipmentGrant {
	var v struct {
		Granted equipmentGrant `json:"granted"`
	}
	_ = json.Unmarshal(value, &v)
	return v.Granted
}

// grantEquipment adds what the pick gives and returns its value with the record of it.
func grantEquipment(ctx context.Context, q *db.Queries, st charState, characterID, choiceID uuid.UUID, slot string,
	value []byte, userID uuid.UUID) ([]byte, error) {
	option := pickedOption(value)
	if option == "" {
		return value, nil
	}
	var (
		grant equipmentGrant
		year  int32
		pkg   *catalogEquipmentOption
	)
	if equipmentSlot.MatchString(slot) {
		var row struct {
			year    int32
			options []byte
		}
		if strings.HasPrefix(slot, "class:") {
			r, err := q.ClassEquipmentOptions(ctx, characterID)
			if err != nil {
				return nil, notPicked(err, "plan level 1 (the class) before its starting equipment")
			}
			row.year, row.options = r.DndVersionYear, r.Options
		} else {
			r, err := q.BackgroundEquipmentOptions(ctx, characterID)
			if err != nil {
				return nil, notPicked(err, "choose a background before its starting equipment")
			}
			row.year, row.options = r.DndVersionYear, r.Options
		}
		year = row.year
		var list []catalogEquipmentOption
		_ = json.Unmarshal(row.options, &list)
		for i := range list {
			if list[i].Label != nil && strings.EqualFold(*list[i].Label, option) {
				pkg = &list[i]
			}
		}
		if pkg == nil {
			return nil, httpx.Unprocessable("value: there's no starting equipment option %q", option)
		}
		grant.Coins = pkg.Coins
	} else {
		year = int32(st.rules.Edition)
	}
	add := func(itemID *uuid.UUID, customName *string, qty int32) error {
		saved, err := q.InsertItem(ctx, db.InsertItemParams{CharacterID: characterID, ItemID: itemID, CustomName: customName,
			Quantity: qty, GrantedByID: &choiceID, UserID: &userID})
		if err != nil {
			return err
		}
		grant.Items = append(grant.Items, grantedItem{ID: saved.ID, Quantity: qty})
		return nil
	}
	byName := func(name string, qty int32) error {
		id, err := q.ItemByName(ctx, db.ItemByNameParams{Name: name, Year: year})
		if errors.Is(err, pgx.ErrNoRows) {
			return add(nil, &name, qty) // not in the catalog: a custom item of that name
		}
		if err != nil {
			return err
		}
		return add(&id, nil, qty)
	}
	if pkg == nil { // a pick's item, by name
		if err := byName(option, 1); err != nil {
			return nil, err
		}
	} else {
		for _, g := range pkg.Grants {
			if err := byName(g.Name, int32(max(g.Quantity, 1))); err != nil {
				return nil, err
			}
		}
		for _, name := range pkg.Custom {
			if err := add(nil, &name, 1); err != nil {
				return nil, err
			}
		}
	}
	if err := moveCoins(ctx, q, st, characterID, nil, grant.Coins, userID); err != nil {
		return nil, err
	}
	var v map[string]any
	_ = json.Unmarshal(value, &v)
	if v == nil {
		v = map[string]any{}
	}
	v["granted"] = grant
	return json.Marshal(v)
}

func notPicked(err error, msg string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Unprocessable("%s", msg)
	}
	return err
}

// ungrantEquipment takes back what a pick added: its untouched items, and its coins (kept at 0 or more).
func ungrantEquipment(ctx context.Context, q *db.Queries, st charState, characterID uuid.UUID, value []byte, userID uuid.UUID) error {
	grant := recordedGrant(value)
	for _, g := range grant.Items {
		it, err := q.GetItem(ctx, db.GetItemParams{ID: g.ID, CharacterID: characterID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue // gone already: used up, sold, removed
		}
		if err != nil {
			return err
		}
		if it.Quantity != g.Quantity || it.Equipped || it.Attuned || it.Note != nil {
			continue // changed since: the player's now
		}
		if err := q.DeleteItem(ctx, db.DeleteItemParams{ID: g.ID, CharacterID: characterID}); err != nil {
			return err
		}
	}
	return moveCoins(ctx, q, st, characterID, grant.Coins, nil, userID)
}

// moveCoins takes `from` off the currency and adds `to`, each coin kept at 0 or more.
func moveCoins(ctx context.Context, q *db.Queries, st charState, characterID uuid.UUID, from, to map[string]int, userID uuid.UUID) error {
	if len(from) == 0 && len(to) == 0 {
		return nil
	}
	c, err := q.GetCharacter(ctx, characterID)
	if err != nil {
		return err
	}
	currency := map[string]int{}
	_ = json.Unmarshal(c.DndCharacter.Currency, &currency)
	for _, coin := range coinOrder {
		currency[coin] = max(0, currency[coin]-from[coin]+to[coin])
	}
	raw, _ := json.Marshal(currency)
	return q.SetCurrency(ctx, db.SetCurrencyParams{Currency: raw, UserID: &userID, ID: characterID})
}
