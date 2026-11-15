package characters

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
)

// Point buy costs by score (8-15).
var pointBuyCost = map[int]int{8: 0, 9: 1, 10: 2, 11: 3, 12: 4, 13: 5, 14: 7, 15: 9}

var standardArray = []int{15, 14, 13, 12, 10, 8}

// die rolls 1dN on the server (clients never roll dice that count).
func die(sides int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		panic(err) // the system's random source failed
	}
	return int(n.Int64()) + 1
}

// RolledSet is one 4d6-drop-lowest roll of six scores.
type RolledSet struct {
	Dice   [][]int `json:"dice"`   // each score's four dice, highest first
	Totals []int   `json:"totals"` // each score: the three highest dice
	At     string  `json:"at"`
	Origin string  `json:"origin"` // server | offline
}

func rollSet() RolledSet {
	set := RolledSet{At: httpx.Timestamp(time.Now()), Origin: "server"}
	for range 6 {
		dice := []int{die(6), die(6), die(6), die(6)}
		slices.SortFunc(dice, func(a, b int) int { return b - a })
		set.Dice = append(set.Dice, dice)
		set.Totals = append(set.Totals, dice[0]+dice[1]+dice[2])
	}
	return set
}

// RollCharacterAbilities is POST /characters/{entity_id}/ability-rolls: roll a new set of six
// scores. Rerolls are allowed and counted; the character keeps the last rules.rolled_sets_kept sets.
func (h *CharacterRoutes) RollCharacterAbilities(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.RollCharacterAbilitiesParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		if !slices.Contains(st.rules.AbilityScoreMethods, "rolled") {
			return change{}, httpx.Unprocessable("the rules don't allow rolled ability scores")
		}
		var sets []RolledSet
		_ = json.Unmarshal(st.row.AbilityRolls, &sets)
		set := rollSet()
		sets = append(sets, set)
		if keep := st.rules.RolledSetsKept; len(sets) > keep {
			sets = sets[len(sets)-keep:]
		}
		stored, _ := json.Marshal(sets)
		count := st.row.AbilityRollCount + 1
		if err := q.SetAbilityRolls(ctx, db.SetAbilityRollsParams{ID: id, AbilityRolls: stored, AbilityRollCount: count}); err != nil {
			return change{}, err
		}
		return change{action: "abilities.roll", entity: "character", entityID: &st.row.ID,
			after: map[string]any{"set": set, "roll_count": count}}, nil
	})
}

// SetCharacterAbilityScores is PUT /characters/{entity_id}/ability-scores: the method and the six
// base scores (before any bonus). The scores must be possible for the method:
//
//	point_buy       8-15 each, costing at most rules.point_buy_budget
//	standard_array  15, 14, 13, 12, 10, 8, each once
//	rolled          the totals of one kept set (rolled_set: its index in ability_rolls), each once
//	manual          3-18 each
func (h *CharacterRoutes) SetCharacterAbilityScores(w http.ResponseWriter, r *http.Request, id api.EntityId, _ api.SetCharacterAbilityScoresParams) {
	f, err := readFields(r, "method", "scores", "rolled_set")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		var method string
		if present, err := f.decode("method", &method, false); err != nil {
			return change{}, err
		} else if !present {
			return change{}, httpx.Unprocessable("method: field required")
		}
		if !slices.Contains(st.rules.AbilityScoreMethods, method) {
			return change{}, httpx.Unprocessable("method: the rules allow %s", strings.Join(st.rules.AbilityScoreMethods, ", "))
		}
		var scores map[string]int
		if present, err := f.decode("scores", &scores, false); err != nil {
			return change{}, err
		} else if !present {
			return change{}, httpx.Unprocessable("scores: field required")
		}
		values := make([]int, 0, 6)
		for _, a := range abilities {
			v, ok := scores[a]
			if !ok {
				return change{}, httpx.Unprocessable("scores: needs all six of %s", strings.Join(abilities, ", "))
			}
			values = append(values, v)
		}
		if len(scores) != 6 {
			return change{}, httpx.Unprocessable("scores: only %s", strings.Join(abilities, ", "))
		}
		var rolledSet *int
		if _, err := f.decode("rolled_set", &rolledSet, true); err != nil {
			return change{}, err
		}
		if err := checkScores(method, values, st, rolledSet); err != nil {
			return change{}, err
		}
		stored, _ := json.Marshal(scores)
		if err := q.SetAbilityScores(ctx, db.SetAbilityScoresParams{ID: id, AbilityMethod: method, BaseScores: stored}); err != nil {
			return change{}, err
		}
		return change{action: "scores.set", entity: "character", entityID: &st.row.ID,
			before: map[string]any{"method": st.row.AbilityMethod, "scores": json.RawMessage(st.row.BaseScores)},
			after:  map[string]any{"method": method, "scores": scores, "rolled_set": rolledSet}}, nil
	})
}

func checkScores(method string, values []int, st charState, rolledSet *int) error {
	switch method {
	case "point_buy":
		cost := 0
		for _, v := range values {
			c, ok := pointBuyCost[v]
			if !ok {
				return httpx.Unprocessable("scores: point buy scores are 8 to 15")
			}
			cost += c
		}
		if cost > st.rules.PointBuyBudget {
			return httpx.Unprocessable("scores: those cost %d points; the budget is %d", cost, st.rules.PointBuyBudget)
		}
	case "standard_array":
		if !samePool(values, standardArray) {
			return httpx.Unprocessable("scores: the standard array is 15, 14, 13, 12, 10, 8, each used once")
		}
	case "rolled":
		var sets []RolledSet
		_ = json.Unmarshal(st.row.AbilityRolls, &sets)
		if rolledSet == nil || *rolledSet < 0 || *rolledSet >= len(sets) {
			return httpx.Unprocessable("rolled_set: the index of a kept set in ability_rolls (0 to %d)", len(sets)-1)
		}
		if !samePool(values, sets[*rolledSet].Totals) {
			return httpx.Unprocessable("scores: use set %d's totals %v, each once", *rolledSet, sets[*rolledSet].Totals)
		}
	case "manual":
		for _, v := range values {
			if v < 3 || v > 18 {
				return httpx.Unprocessable("scores: manual base scores are 3 to 18")
			}
		}
	}
	return nil
}

// samePool reports whether a and b hold the same numbers, each as often.
func samePool(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

// HPRoll is one hit point roll for a level.
type HPRoll struct {
	Value  int    `json:"value"`
	Die    int    `json:"die"`
	At     string `json:"at"`
	Origin string `json:"origin"`
}

// RollCharacterLevelHP is POST /characters/{entity_id}/levels/{level}/hp-roll: roll the class's hit
// die for a planned level (2 or higher; level 1 is always the maximum). Rerolls are kept in hp_rolls;
// the newest is the level's value.
func (h *CharacterRoutes) RollCharacterLevelHP(w http.ResponseWriter, r *http.Request, id api.EntityId, level int, _ api.RollCharacterLevelHPParams) {
	h.write(w, r, id, access.EditBuild, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		if level < 2 || level > 20 {
			return change{}, httpx.Unprocessable("level: HP is rolled for levels 2 to 20 (level 1 is the die's maximum)")
		}
		st, err := loadState(ctx, q, id)
		if err != nil {
			return change{}, err
		}
		if !slices.Contains(st.rules.HPMethods, "rolled") {
			return change{}, httpx.Unprocessable("the rules don't allow rolled HP")
		}
		existing, err := q.GetLevel(ctx, db.GetLevelParams{CharacterID: id, Level: int32(level)})
		if errors.Is(err, pgx.ErrNoRows) {
			return change{}, httpx.NotFound("Level %d isn't planned", level)
		}
		if err != nil {
			return change{}, err
		}
		class, err := q.CharacterClass(ctx, id)
		if err != nil {
			return change{}, err
		}
		if class.HitDieSize == nil {
			return change{}, httpx.Unprocessable("%s has no hit die size in the catalog", class.Name)
		}
		roll := HPRoll{Die: int(*class.HitDieSize), At: httpx.Timestamp(time.Now()), Origin: "server"}
		roll.Value = die(roll.Die)
		var rolls []HPRoll
		_ = json.Unmarshal(existing.HpRolls, &rolls)
		rolls = append(rolls, roll)
		stored, _ := json.Marshal(rolls)
		value := int32(roll.Value)
		levelID, err := q.SetLevelHPRoll(ctx, db.SetLevelHPRollParams{CharacterID: id, Level: int32(level), HpValue: &value, HpRolls: stored, UserID: &userID})
		if err != nil {
			return change{}, err
		}
		return change{action: "hp.roll", entity: "character_level", entityID: &levelID,
			before: map[string]any{"hp_method": existing.HpMethod, "hp_value": existing.HpValue},
			after:  map[string]any{"hp_method": "rolled", "roll": roll}}, nil
	})
}
