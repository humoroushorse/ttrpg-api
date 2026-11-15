package campaigns

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/campaigns"
	"github.com/humoroushorse/go_dnd/internal/tabletop"
)

// Member is someone in a campaign.
type Member struct {
	UserID            uuid.UUID `json:"user_id"`
	Username          *string   `json:"username"`
	ProfilePictureURL *string   `json:"profile_picture_url"`
	Role              string    `json:"role"`
	// the permanent DM
	Owner bool `json:"owner"`
	// the caller
	Me       bool   `json:"me"`
	JoinedAt string `json:"joined_at"`
}

type memberList struct {
	Entities []Member `json:"entities"`
}

func (h *CampaignRoutes) members(r *http.Request, q *db.Queries, who access.Campaign, userID uuid.UUID) (memberList, error) {
	rows, err := q.ListMembers(r.Context(), who.ID)
	if err != nil {
		return memberList{}, err
	}
	out := memberList{Entities: make([]Member, len(rows))}
	for i, row := range rows {
		out.Entities[i] = Member{UserID: row.UserID, Username: row.Username, ProfilePictureURL: row.ProfilePictureUrl,
			Role: row.Role, Owner: row.UserID == who.OwnerID, Me: row.UserID == userID, JoinedAt: httpx.Timestamp(row.CreatedAt.Time)}
	}
	return out, nil
}

// ListCampaignMembers is GET /campaigns/{entity_id}/members: any member sees who's in it.
func (h *CampaignRoutes) ListCampaignMembers(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	who, err := require(ctx, q, id, userID, access.ViewCampaign)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := h.members(r, q, who, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// UpdateCampaignMember is PATCH /campaigns/{entity_id}/members/{user_id}: a DM makes a member a DM or a
// player. The owner is always a DM.
func (h *CampaignRoutes) UpdateCampaignMember(w http.ResponseWriter, r *http.Request, id api.EntityId, memberID api.UserId) {
	var body struct {
		Role string `json:"role"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || (body.Role != string(access.DM) && body.Role != string(access.Player)) {
		httpx.WriteError(w, r, httpx.Unprocessable(`the body must be {"role": "dm" | "player"}`))
		return
	}
	h.memberWrite(w, r, id, access.ManageDMs, func(tx pgx.Tx, q *db.Queries, who access.Campaign, userID uuid.UUID) ([]tabletop.Event, error) {
		if memberID == who.OwnerID {
			return nil, httpx.Forbidden("the owner is always a DM")
		}
		if err := isMember(r, q, id, memberID); err != nil {
			return nil, err
		}
		// the table first: its lock comes before any row it touches, as in every table write
		events, err := h.table.MemberUpdated(r.Context(), tx, id, memberID, body.Role, userID)
		if err != nil {
			return nil, err
		}
		return events, q.SetMemberRole(r.Context(), db.SetMemberRoleParams{CampaignID: id, UserID: memberID, Role: body.Role, By: &userID})
	})
}

// RemoveCampaignMember is DELETE /campaigns/{entity_id}/members/{user_id}: leaving (the caller's own id)
// or a DM removing someone. Never the owner. Their characters leave the campaign with them and go
// back to their own rules; anyone's editor rights on them stay (they're the character's).
func (h *CampaignRoutes) RemoveCampaignMember(w http.ResponseWriter, r *http.Request, id api.EntityId, memberID api.UserId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	action := access.RemoveMember
	if memberID == userID {
		action = access.Leave
	}
	h.memberWrite(w, r, id, action, func(tx pgx.Tx, q *db.Queries, who access.Campaign, userID uuid.UUID) ([]tabletop.Event, error) {
		if memberID == who.OwnerID {
			return nil, httpx.Forbidden("the owner can't leave or be removed: they're the campaign's permanent DM")
		}
		if err := isMember(r, q, id, memberID); err != nil {
			return nil, err
		}
		// before the deletes: unlinking their characters deletes their tokens, so the table logs them first
		events, err := h.table.MemberLeaving(ctx, tx, id, memberID, userID)
		if err != nil {
			return nil, err
		}
		if err := q.RemoveMember(ctx, db.RemoveMemberParams{CampaignID: id, UserID: memberID}); err != nil {
			return nil, err
		}
		left, err := q.UnlinkMemberCharacters(ctx, db.UnlinkMemberCharactersParams{CampaignID: id, UserID: memberID})
		if err != nil {
			return nil, err
		}
		for _, characterID := range left {
			if err := q.LogCampaignLeave(ctx, db.LogCampaignLeaveParams{CharacterID: characterID, By: &userID, CampaignID: id}); err != nil {
				return nil, err
			}
		}
		return events, nil
	})
}

func isMember(r *http.Request, q *db.Queries, campaignID, userID uuid.UUID) error {
	_, err := q.MemberRole(r.Context(), db.MemberRoleParams{CampaignID: campaignID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("User %s isn't in this campaign", userID)
	}
	return err
}

// memberWrite runs fn in a transaction once the caller may take the action, publishes the table events
// fn logged once it commits, and answers with the members after.
func (h *CampaignRoutes) memberWrite(w http.ResponseWriter, r *http.Request, id uuid.UUID, a access.CampaignAction,
	fn func(tx pgx.Tx, q *db.Queries, who access.Campaign, userID uuid.UUID) ([]tabletop.Event, error)) {
	ctx := r.Context()
	var events []tabletop.Event
	out, err := func() (memberList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return memberList{}, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return memberList{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		who, err := require(ctx, q, id, userID, a)
		if err != nil {
			return memberList{}, err
		}
		if events, err = fn(tx, q, who, userID); err != nil {
			return memberList{}, err
		}
		out, err := h.members(r, q, who, userID)
		if err != nil {
			return memberList{}, err
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	h.table.Publish(ctx, events)
	httpx.WriteJSON(w, http.StatusOK, out)
}
