package characters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
)

// A DM gives a character in their campaign items, coins, boons and banes through one route, so the
// DM's permission is exactly that action, not part of the player's build routes. What's given is
// marked: an item's given_by and given_campaign_id, a modifier's campaign_id (and added_as dm).
//
//   - The player owns a gift once given: they can drop it, spend the coins, equip and attune it.
//   - A custom item's name and description stay the DMs' (the player can't rename a quest item).
//   - A DM boon or bane is the DMs' to change or remove, and counts only while the character is in
//     the campaign that gave it (it switches back on if the character rejoins).
//   - A DM takes back a gift item only while nobody has changed it.

var coins = []string{"cp", "sp", "ep", "gp", "pp"}

// itemDefinition are the fields of a custom item's definition, which a gift keeps the DMs'.
var itemDefinition = []string{"custom_name", "custom_description"}

// mayChangeItem: the player changes an item's state; a gift's definition is its campaign's DMs'.
func mayChangeItem(who access.Character, item db.DndCharacterItem, f fields) error {
	definition := slices.ContainsFunc(itemDefinition, func(k string) bool { _, ok := f[k]; return ok })
	state := len(f) > 0 && slices.ContainsFunc(keys(f), func(k string) bool { return !slices.Contains(itemDefinition, k) })
	gift := item.GivenBy != nil
	if state && !who.Can(access.EditBuild) {
		return httpx.Forbidden("only the character's owner and editors change an item's quantity, equipped, attuned and note")
	}
	if definition && gift && !who.DMOf(item.GivenCampaignID) {
		return httpx.Forbidden("a DM's gift: only its campaign's DMs change its name and description")
	}
	if definition && !gift && !who.Can(access.EditBuild) {
		return httpx.Forbidden("only the character's owner and editors change this item")
	}
	return nil
}

// mayChangeModifier: a DM boon or bane is its campaign's DMs'; the rest are the player's.
func mayChangeModifier(who access.Character, m db.DndCharacterModifier) error {
	if m.CampaignID != nil {
		if !who.DMOf(m.CampaignID) {
			return httpx.Forbidden("a DM's boon or bane: only its campaign's DMs change or remove it")
		}
		return nil
	}
	if !who.Can(access.EditBuild) {
		return httpx.Forbidden("only the character's owner and editors change their own boons and banes")
	}
	return nil
}

// addedAs is the added_as the body sets, if any.
func addedAs(f fields) (string, bool, error) {
	var v string
	ok, err := f.decode("added_as", &v, false)
	return v, ok, err
}

func keys(f fields) []string {
	out := make([]string, 0, len(f))
	for k := range f {
		out = append(out, k)
	}
	return out
}

type giftItem struct {
	ItemID            *uuid.UUID `json:"item_id"`
	CustomName        *string    `json:"custom_name"`
	CustomDescription *string    `json:"custom_description"`
	Quantity          *int32     `json:"quantity"`
	Note              *string    `json:"note"`
}

// CreateCampaignGift is POST /campaigns/{entity_id}/characters/{character_id}/gifts: a DM of the
// character's campaign gives it items, coins (added to what it has) and boons or banes, in one write.
func (h *CharacterRoutes) CreateCampaignGift(w http.ResponseWriter, r *http.Request, campaignID api.EntityId, characterID api.CharacterId, _ api.CreateCampaignGiftParams) {
	var body struct {
		Items     []giftItem                   `json:"items"`
		Currency  map[string]int               `json:"currency"`
		Modifiers []map[string]json.RawMessage `json:"modifiers"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.Unprocessable("the body must be {items, currency, modifiers}: %v", err))
		return
	}
	if len(body.Items) == 0 && len(body.Currency) == 0 && len(body.Modifiers) == 0 {
		httpx.WriteError(w, r, httpx.Unprocessable("a gift needs items, currency or modifiers"))
		return
	}
	h.write(w, r, characterID, access.GiveTo, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		cid := uuid.UUID(campaignID)
		if who := caller(ctx); who.CampaignID == nil || *who.CampaignID != cid {
			return change{}, httpx.NotFound("Character %s isn't in campaign %s", characterID, campaignID)
		}
		given := map[string]any{}
		var items []map[string]any
		for i, it := range body.Items {
			saved, err := giveItem(ctx, q, characterID, cid, userID, it)
			if err != nil {
				return change{}, httpx.Unprocessable("items[%d]: %v", i, err)
			}
			items = append(items, itemChange(saved))
		}
		if len(items) > 0 {
			given["items"] = items
		}
		if len(body.Currency) > 0 {
			added, err := giveCoins(ctx, q, characterID, userID, body.Currency)
			if err != nil {
				return change{}, err
			}
			given["currency"] = added
		}
		var mods []Modifier
		for i, raw := range body.Modifiers {
			f := fields(raw)
			if v, ok, err := addedAs(f); err != nil || (ok && v != "dm") {
				return change{}, httpx.Unprocessable("modifiers[%d].added_as: a gift's is dm", i)
			}
			if _, ok := f["active"]; ok {
				return change{}, httpx.Unprocessable("modifiers[%d].active: a gift starts on", i)
			}
			v, err := readModifier(f, modifierValues{active: true, addedAs: "dm"}, true)
			if err != nil {
				return change{}, httpx.Unprocessable("modifiers[%d]: %v", i, err)
			}
			saved, err := q.InsertModifier(ctx, db.InsertModifierParams{CharacterID: characterID, Label: v.label, Note: v.note,
				Target: v.target, Op: v.op, Value: v.value, FromLevel: v.fromLevel, Active: true, AddedAs: "dm",
				CampaignID: &cid, UserID: &userID})
			if err != nil {
				return change{}, err
			}
			mods = append(mods, toModifier(saved))
		}
		if len(mods) > 0 {
			given["modifiers"] = mods
		}
		return change{action: "gift", entity: "campaign", entityID: &cid, after: given}, nil
	})
}

func giveItem(ctx context.Context, q *db.Queries, characterID, campaignID, userID uuid.UUID, it giftItem) (db.DndCharacterItem, error) {
	quantity := int32(1)
	if it.Quantity != nil {
		quantity = *it.Quantity
	}
	if quantity < 1 {
		return db.DndCharacterItem{}, errors.New("quantity is at least 1")
	}
	if it.ItemID == nil && (it.CustomName == nil || *it.CustomName == "") {
		return db.DndCharacterItem{}, errors.New("an item needs item_id (from the catalog) or custom_name")
	}
	if it.ItemID != nil {
		if live, err := q.CatalogItemLive(ctx, *it.ItemID); err != nil {
			return db.DndCharacterItem{}, err
		} else if !live {
			return db.DndCharacterItem{}, errors.New("item_id: no such catalog item")
		}
	}
	return q.InsertItem(ctx, db.InsertItemParams{CharacterID: characterID, ItemID: it.ItemID, CustomName: it.CustomName,
		CustomDescription: it.CustomDescription, Quantity: quantity, Note: it.Note, GivenBy: &userID,
		GivenCampaignID: &campaignID, UserID: &userID})
}

// giveCoins adds to the character's coins and answers with what was added.
func giveCoins(ctx context.Context, q *db.Queries, characterID, userID uuid.UUID, add map[string]int) (map[string]int, error) {
	for coin, n := range add {
		if !slices.Contains(coins, coin) {
			return nil, httpx.Unprocessable("currency.%s: coins are %v", coin, coins)
		}
		if n < 0 {
			return nil, httpx.Unprocessable("currency.%s: a gift adds coins (0 or more)", coin)
		}
	}
	row, err := q.GetCharacter(ctx, characterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound(characterID)
	}
	if err != nil {
		return nil, err
	}
	current := map[string]int{}
	_ = json.Unmarshal(row.DndCharacter.Currency, &current)
	for coin, n := range add {
		current[coin] += n
	}
	stored, _ := json.Marshal(current)
	if err := q.SetCurrency(ctx, db.SetCurrencyParams{ID: characterID, Currency: stored, UserID: &userID}); err != nil {
		return nil, err
	}
	return add, nil
}
