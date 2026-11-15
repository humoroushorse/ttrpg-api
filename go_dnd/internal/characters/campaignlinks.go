package characters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
	"github.com/humoroushorse/go_dnd/internal/rules"
)

// CampaignCharacter is a character on a campaign's party list, with what breaks the campaign's rules
// (for its DMs and its own player; empty for the other players).
type CampaignCharacter struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	CurrentLevel  int32     `json:"current_level"`
	OwnerID       uuid.UUID `json:"owner_id"`
	OwnerUsername *string   `json:"owner_username"`
	ClassName     string    `json:"class_name"`
	SpeciesName   string    `json:"species_name"`
	Issues        []Issue   `json:"issues"`
}

// campaignRules is the campaign's rules, once the caller may take the action on it.
func (h *CharacterRoutes) campaignRules(ctx context.Context, q *db.Queries, campaignID, userID uuid.UUID,
	a access.CampaignAction) (rules.Rules, access.Campaign, error) {
	who, err := requireCampaign(ctx, q, campaignID, userID, a)
	if err != nil {
		return rules.Rules{}, who, err
	}
	r, err := h.rulesOf(ctx, q, campaignID)
	return r, who, err
}

func (h *CharacterRoutes) rulesOf(ctx context.Context, q *db.Queries, campaignID uuid.UUID) (rules.Rules, error) {
	raw, err := q.CampaignRules(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rules.Rules{}, httpx.NotFound("Campaign %s not found", campaignID)
	}
	if err != nil {
		return rules.Rules{}, err
	}
	r, err := rules.Parse(raw, rules.Default())
	if err != nil {
		r = rules.Default()
	}
	return r, nil
}

// campaignList is the party list as the viewer may see it: a player sees only their own characters'
// issues.
func (h *CharacterRoutes) campaignList(ctx context.Context, q *db.Queries, campaignID uuid.UUID, viewer access.Campaign,
	userID uuid.UUID, ruleSet rules.Rules) ([]CampaignCharacter, error) {
	rows, err := q.CampaignCharacters(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	out := make([]CampaignCharacter, len(rows))
	for i, row := range rows {
		out[i] = CampaignCharacter{ID: row.ID, Name: row.Name, CurrentLevel: row.CurrentLevel, OwnerID: row.OwnerID,
			OwnerUsername: row.OwnerUsername, ClassName: row.ClassName, SpeciesName: row.SpeciesName, Issues: []Issue{}}
		if viewer.Role != access.DM && row.OwnerID != userID {
			continue
		}
		issues, err := Violations(ctx, q, row.ID, ruleSet)
		if err != nil {
			return nil, err
		}
		out[i].Issues = markApproved(issues, row.ApprovedExceptions)
	}
	return out, nil
}

// approvedException is one entry of campaign_character.approved_exceptions: a catalog entry the DM
// accepted for this character although it breaks the campaign's rules.
type approvedException struct {
	Category   string    `json:"category"`
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Reason     string    `json:"reason"` // why it broke the rules when it was approved
	Note       *string   `json:"note,omitempty"`
	ApprovedAt string    `json:"approved_at"`
	ApprovedBy uuid.UUID `json:"approved_by"`
}

func readApproved(raw []byte) []approvedException {
	list := []approvedException{}
	_ = json.Unmarshal(raw, &list)
	return list
}

// markApproved flags the issues the DM approved. An approval for something that no longer breaks the
// rules is kept but has nothing to mark.
func markApproved(issues []Issue, raw []byte) []Issue {
	approved := readApproved(raw)
	for i := range issues {
		for _, a := range approved {
			if a.ID == issues[i].ID {
				issues[i].Approved, issues[i].Note = true, a.Note
			}
		}
	}
	return issues
}

// ApproveCampaignException is PUT /campaigns/{entity_id}/characters/{character_id}/exceptions/{ref_id}:
// the DM accepts a catalog entry the character uses that breaks the campaign's current rules.
func (h *CharacterRoutes) ApproveCampaignException(w http.ResponseWriter, r *http.Request, campaignID, characterID api.EntityId, refID api.RefId) {
	var body struct {
		Note *string `json:"note"`
	}
	if r.ContentLength != 0 {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			httpx.WriteError(w, r, httpx.Unprocessable("the body must be {\"note\": \"...\"} or empty: %v", err))
			return
		}
	}
	h.exceptionWrite(w, r, campaignID, characterID, func(ctx context.Context, q *db.Queries, userID uuid.UUID, ruleSet rules.Rules,
		list []approvedException) ([]approvedException, change, error) {
		issues, err := Violations(ctx, q, characterID, ruleSet)
		if err != nil {
			return nil, change{}, err
		}
		i := slices.IndexFunc(issues, func(is Issue) bool { return is.ID == refID })
		if i < 0 {
			return nil, change{}, httpx.Unprocessable("%s isn't outside this campaign's rules for this character; nothing to approve", refID)
		}
		is := issues[i]
		entry := approvedException{Category: is.Category, ID: is.ID, Name: is.Name, Reason: is.Reason, Note: body.Note,
			ApprovedAt: httpx.Timestamp(time.Now()), ApprovedBy: userID}
		list = slices.DeleteFunc(list, func(a approvedException) bool { return a.ID == refID })
		list = append(list, entry)
		id := uuid.UUID(refID)
		return list, change{action: "exception.approve", entity: is.Category, entityID: &id, after: entry}, nil
	})
}

// RevokeCampaignException is DELETE /campaigns/{entity_id}/characters/{character_id}/exceptions/{ref_id}.
func (h *CharacterRoutes) RevokeCampaignException(w http.ResponseWriter, r *http.Request, campaignID, characterID api.EntityId, refID api.RefId) {
	h.exceptionWrite(w, r, campaignID, characterID, func(_ context.Context, _ *db.Queries, _ uuid.UUID, _ rules.Rules,
		list []approvedException) ([]approvedException, change, error) {
		i := slices.IndexFunc(list, func(a approvedException) bool { return a.ID == refID })
		if i < 0 {
			return nil, change{}, httpx.NotFound("No approved exception for %s", refID)
		}
		before := list[i]
		id := uuid.UUID(refID)
		return slices.Delete(list, i, i+1), change{action: "exception.revoke", entity: before.Category, entityID: &id, before: before}, nil
	})
}

// exceptionWrite changes a campaign character's approved exceptions in one transaction and logs it in
// the character's change log. The character's version doesn't change: this is the DM's ruling on the
// campaign link, not an edit to the build (a player's offline edits shouldn't conflict with it).
func (h *CharacterRoutes) exceptionWrite(w http.ResponseWriter, r *http.Request, campaignID, characterID uuid.UUID,
	fn func(ctx context.Context, q *db.Queries, userID uuid.UUID, ruleSet rules.Rules, list []approvedException) ([]approvedException, change, error)) {
	ctx := r.Context()
	out, err := func() (CampaignCharacter, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return CampaignCharacter{}, err
		}
		tx, err := h.begin(ctx)
		if err != nil {
			return CampaignCharacter{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		ruleSet, who, err := h.campaignRules(ctx, q, campaignID, userID, access.ApproveExceptions)
		if err != nil {
			return CampaignCharacter{}, err
		}
		raw, err := q.CampaignLinkExceptions(ctx, db.CampaignLinkExceptionsParams{CampaignID: campaignID, CharacterID: characterID})
		if errors.Is(err, pgx.ErrNoRows) {
			return CampaignCharacter{}, httpx.NotFound("Character %s isn't in campaign %s", characterID, campaignID)
		}
		if err != nil {
			return CampaignCharacter{}, err
		}
		list, c, err := fn(ctx, q, userID, ruleSet, readApproved(raw))
		if err != nil {
			return CampaignCharacter{}, err
		}
		stored, _ := json.Marshal(list)
		if err := q.SetApprovedExceptions(ctx, db.SetApprovedExceptionsParams{ApprovedExceptions: stored, UserID: &userID,
			CampaignID: campaignID, CharacterID: characterID}); err != nil {
			return CampaignCharacter{}, err
		}
		if err := logChange(ctx, q, characterID, userID, nil, c); err != nil {
			return CampaignCharacter{}, err
		}
		all, err := h.campaignList(ctx, q, campaignID, who, userID, ruleSet)
		if err != nil {
			return CampaignCharacter{}, err
		}
		i := slices.IndexFunc(all, func(cc CampaignCharacter) bool { return cc.ID == characterID })
		if i < 0 {
			return CampaignCharacter{}, httpx.NotFound("Character %s isn't in campaign %s", characterID, campaignID)
		}
		return all[i], tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type campaignCharactersResponse struct {
	TotalEntitiesCount int                 `json:"total_entities_count"`
	Limit              int                 `json:"limit"`
	Offset             int                 `json:"offset"`
	Filters            map[string]any      `json:"filters"`
	EntitiesCount      int                 `json:"entities_count"`
	Entities           []CampaignCharacter `json:"entities"`
}

func listOf(items []CampaignCharacter) campaignCharactersResponse {
	return campaignCharactersResponse{TotalEntitiesCount: len(items), Filters: map[string]any{}, EntitiesCount: len(items), Entities: items}
}

// ReadCampaignCharacters is GET /campaigns/{entity_id}/characters: each with what breaks the
// campaign's current rules (rules tightened after it joined).
func (h *CharacterRoutes) ReadCampaignCharacters(w http.ResponseWriter, r *http.Request, campaignID api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	ruleSet, who, err := h.campaignRules(ctx, q, campaignID, userID, access.ViewCampaign)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	list, err := h.campaignList(ctx, q, campaignID, who, userID, ruleSet)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listOf(list))
}

// PreviewCampaignRules is POST /campaigns/{entity_id}/rules/preview: the campaign's characters as a
// rules change (merged into the current rules, as a PATCH would) would flag them. Saves nothing.
func (h *CharacterRoutes) PreviewCampaignRules(w http.ResponseWriter, r *http.Request, campaignID api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	current, who, err := h.campaignRules(ctx, q, campaignID, userID, access.EditCampaign)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	proposed, err := rules.Parse(raw, current)
	if err != nil {
		httpx.WriteError(w, r, httpx.Unprocessable("%v", err))
		return
	}
	list, err := h.campaignList(ctx, q, campaignID, who, userID, proposed)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listOf(list))
}

// AddCampaignCharacter is POST /campaigns/{entity_id}/characters/{character_id}: a member adds one of
// their own characters. It's checked against the campaign's rules first: anything that breaks them is
// a 409 listing what to fix, and nothing is linked.
func (h *CharacterRoutes) AddCampaignCharacter(w http.ResponseWriter, r *http.Request, campaignID, characterID api.EntityId) {
	h.write(w, r, characterID, access.JoinCampaign, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		ruleSet, _, err := h.campaignRules(ctx, q, campaignID, userID, access.AddOwnCharacter)
		if err != nil {
			return change{}, err
		}
		current, err := q.CharacterCampaign(ctx, characterID)
		switch {
		case err == nil && current == campaignID:
			return change{}, nil // already in it
		case err == nil:
			return change{}, httpx.Conflict("the character is already in campaign %s; one campaign per character for now", current)
		case !errors.Is(err, pgx.ErrNoRows):
			return change{}, err
		}
		issues, err := Violations(ctx, q, characterID, ruleSet)
		if err != nil {
			return change{}, err
		}
		if len(issues) > 0 {
			conflict := httpx.Conflict("the character breaks this campaign's rules in %d place(s); fix them first (see issues)", len(issues))
			conflict.Issues = issues
			return change{}, conflict
		}
		if err := q.LinkCampaign(ctx, db.LinkCampaignParams{CampaignID: campaignID, CharacterID: characterID, UserID: &userID}); err != nil {
			return change{}, err
		}
		cid := uuid.UUID(campaignID)
		return change{action: "campaign.join", entity: "campaign", entityID: &cid}, nil
	})
}

// RemoveCampaignCharacter is DELETE /campaigns/{entity_id}/characters/{character_id}: its owner or
// the campaign's DMs take the character out, and it goes back to its own rules.
func (h *CharacterRoutes) RemoveCampaignCharacter(w http.ResponseWriter, r *http.Request, campaignID, characterID api.EntityId) {
	h.write(w, r, characterID, access.RemoveFromCampaign, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (change, error) {
		current, err := q.CharacterCampaign(ctx, characterID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && current != campaignID) {
			return change{}, httpx.NotFound("Character %s isn't in campaign %s", characterID, campaignID)
		}
		if err != nil {
			return change{}, err
		}
		// before the unlink, which deletes the character's tokens: the table logs them first
		events, err := h.table.CharacterLeaving(ctx, ctx.Value(writeTxKey{}).(pgx.Tx), campaignID, characterID, userID)
		if err != nil {
			return change{}, err
		}
		if _, err := q.UnlinkCampaign(ctx, db.UnlinkCampaignParams{CampaignID: campaignID, CharacterID: characterID}); err != nil {
			return change{}, err
		}
		cid := uuid.UUID(campaignID)
		return change{action: "campaign.leave", entity: "campaign", entityID: &cid, table: events}, nil
	})
}
