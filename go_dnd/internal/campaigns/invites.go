package campaigns

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/campaigns"
)

// How long an invite is open for; 1d when the request doesn't say.
var inviteLifetimes = map[string]time.Duration{"1h": time.Hour, "1d": 24 * time.Hour, "7d": 7 * 24 * time.Hour}

// Invite is one of a campaign's open invites.
type Invite struct {
	ID uuid.UUID `json:"id"`
	// link or user
	Kind string `json:"kind"`
	// a user invite's invitee
	Username  *string `json:"username"`
	InvitedBy *string `json:"invited_by"`
	ExpiresAt string  `json:"expires_at"`
	MaxUses   *int32  `json:"max_uses"`
	Uses      int32   `json:"uses"`
	CreatedAt string  `json:"created_at"`
}

type inviteList struct {
	Entities []Invite `json:"entities"`
}

func (h *CampaignRoutes) invites(ctx context.Context, q *db.Queries, campaignID uuid.UUID) (inviteList, error) {
	rows, err := q.ListInvites(ctx, campaignID)
	if err != nil {
		return inviteList{}, err
	}
	out := inviteList{Entities: make([]Invite, len(rows))}
	for i, row := range rows {
		kind := "user"
		if row.IsLink {
			kind = "link"
		}
		out.Entities[i] = Invite{ID: row.ID, Kind: kind, Username: row.InvitedUsername, InvitedBy: row.InvitedByUsername,
			ExpiresAt: httpx.Timestamp(row.ExpiresAt.Time), MaxUses: row.MaxUses, Uses: row.Uses,
			CreatedAt: httpx.Timestamp(row.CreatedAt.Time)}
	}
	return out, nil
}

// newToken is a link's secret (32 random bytes, URL-safe) and the hash stored in its place.
func newToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// usable is nil while an invite can still be used, else why not (a 410).
func usable(revokedAt, expiresAt pgtype.Timestamptz, maxUses *int32, uses int32, campaignDeleted bool) error {
	switch {
	case campaignDeleted:
		return httpx.Gone("This campaign was deleted")
	case revokedAt.Valid:
		return httpx.Gone("This invite was revoked or declined")
	case !expiresAt.Time.After(time.Now()):
		return httpx.Gone("This invite expired")
	case maxUses != nil && uses >= *maxUses:
		return httpx.Gone("This invite has been used up")
	}
	return nil
}

// ListCampaignInvites is GET /campaigns/{entity_id}/invites: a DM sees the open ones.
func (h *CampaignRoutes) ListCampaignInvites(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	if _, err := require(ctx, q, id, userID, access.ManageInvites); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := h.invites(ctx, q, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type inviteCreated struct {
	Invite Invite `json:"invite"`
	// a link's token, shown only now
	Token *string `json:"token"`
}

// CreateCampaignInvite is POST /campaigns/{entity_id}/invites: a link anyone signed in can use until it
// expires, or an invite for one user by username, which waits for them to accept.
func (h *CampaignRoutes) CreateCampaignInvite(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	var body struct {
		ExpiresIn *string `json:"expires_in"`
		MaxUses   *int32  `json:"max_uses"`
		Username  *string `json:"username"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.Unprocessable("the body must be {expires_in, max_uses, username}: %v", err))
		return
	}
	lifetime := inviteLifetimes["1d"]
	if body.ExpiresIn != nil {
		var ok bool
		if lifetime, ok = inviteLifetimes[*body.ExpiresIn]; !ok {
			httpx.WriteError(w, r, httpx.Unprocessable("expires_in is 1h, 1d or 7d"))
			return
		}
	}
	if body.MaxUses != nil && *body.MaxUses < 1 {
		httpx.WriteError(w, r, httpx.Unprocessable("max_uses is at least 1"))
		return
	}
	ctx := r.Context()
	out, err := func() (inviteCreated, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return inviteCreated{}, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return inviteCreated{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		if _, err := require(ctx, q, id, userID, access.Invite); err != nil {
			return inviteCreated{}, err
		}
		params := db.CreateInviteParams{CampaignID: id, By: &userID,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(lifetime), Valid: true}}
		var token *string
		if body.Username != nil {
			if body.MaxUses != nil {
				return inviteCreated{}, httpx.Unprocessable("max_uses is for links; a user's invite is used once")
			}
			invitee, err := h.invitee(ctx, q, id, *body.Username)
			if err != nil {
				return inviteCreated{}, err
			}
			one := int32(1)
			params.UserID, params.MaxUses = &invitee, &one
		} else {
			secret, hash, err := newToken()
			if err != nil {
				return inviteCreated{}, err
			}
			token, params.TokenHash, params.MaxUses = &secret, &hash, body.MaxUses
		}
		inviteID, err := q.CreateInvite(ctx, params)
		if err != nil {
			return inviteCreated{}, err
		}
		list, err := h.invites(ctx, q, id)
		if err != nil {
			return inviteCreated{}, err
		}
		for _, inv := range list.Entities {
			if inv.ID == inviteID {
				return inviteCreated{Invite: inv, Token: token}, tx.Commit(ctx)
			}
		}
		return inviteCreated{}, errors.New("the new invite isn't in the open list")
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// invitee is the user a username invite is for: someone who has signed in, isn't in the campaign, and
// has no open invite to it yet.
func (h *CampaignRoutes) invitee(ctx context.Context, q *db.Queries, campaignID uuid.UUID, username string) (uuid.UUID, error) {
	user, err := q.UserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, httpx.NotFound("No user named %q has signed in yet", username)
	}
	if err != nil {
		return uuid.Nil, err
	}
	_, err = q.MemberRole(ctx, db.MemberRoleParams{CampaignID: campaignID, UserID: user.ID})
	if err == nil {
		return uuid.Nil, httpx.Conflict("%s is already in this campaign", username)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if _, err := q.PendingUserInvite(ctx, db.PendingUserInviteParams{CampaignID: campaignID, UserID: &user.ID}); err == nil {
		return uuid.Nil, httpx.Conflict("%s already has an open invite to this campaign", username)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	return user.ID, nil
}

// RevokeCampaignInvite is DELETE /campaigns/{entity_id}/invites/{invite_id}: a DM closes an invite.
func (h *CampaignRoutes) RevokeCampaignInvite(w http.ResponseWriter, r *http.Request, id api.EntityId, inviteID api.InviteId) {
	ctx := r.Context()
	out, err := func() (inviteList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return inviteList{}, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return inviteList{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		if _, err := require(ctx, q, id, userID, access.ManageInvites); err != nil {
			return inviteList{}, err
		}
		n, err := q.RevokeInvite(ctx, db.RevokeInviteParams{ID: inviteID, CampaignID: id, By: &userID})
		if err != nil {
			return inviteList{}, err
		}
		if n == 0 {
			return inviteList{}, httpx.NotFound("No open invite %s in this campaign", inviteID)
		}
		out, err := h.invites(ctx, q, id)
		if err != nil {
			return inviteList{}, err
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type linkPreview struct {
	CampaignID   uuid.UUID `json:"campaign_id"`
	CampaignName string    `json:"campaign_name"`
	InvitedBy    *string   `json:"invited_by"`
	ExpiresAt    string    `json:"expires_at"`
	// the caller is already in it
	Member bool `json:"member"`
}

// ReadInviteLink is GET /invite-links/{token}: what the link is for, so the join page can say so.
func (h *CampaignRoutes) ReadInviteLink(w http.ResponseWriter, r *http.Request, token api.InviteToken) {
	ctx := r.Context()
	out, err := func() (linkPreview, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return linkPreview{}, err
		}
		// the master: the lookup locks the row (released at once here)
		q := db.New(h.master)
		hash := hashToken(token)
		inv, err := q.InviteByToken(ctx, &hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return linkPreview{}, httpx.NotFound("This invite link isn't valid")
		}
		if err != nil {
			return linkPreview{}, err
		}
		if err := usable(inv.RevokedAt, inv.ExpiresAt, inv.MaxUses, inv.Uses, inv.CampaignDeleted); err != nil {
			return linkPreview{}, err
		}
		_, err = q.MemberRole(ctx, db.MemberRoleParams{CampaignID: inv.CampaignID, UserID: userID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return linkPreview{}, err
		}
		return linkPreview{CampaignID: inv.CampaignID, CampaignName: inv.CampaignName, InvitedBy: inv.InvitedByUsername,
			ExpiresAt: httpx.Timestamp(inv.ExpiresAt.Time), Member: err == nil}, nil
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// AcceptInviteLink is POST /invite-links/{token}/accept: join as a player. Already a member, nothing
// changes and the link's use isn't counted.
func (h *CampaignRoutes) AcceptInviteLink(w http.ResponseWriter, r *http.Request, token api.InviteToken) {
	h.join(w, r, func(ctx context.Context, q *db.Queries, _ uuid.UUID) (joinable, error) {
		hash := hashToken(token)
		inv, err := q.InviteByToken(ctx, &hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return joinable{}, httpx.NotFound("This invite link isn't valid")
		}
		return joinable{inv.ID, inv.CampaignID, inv.RevokedAt, inv.ExpiresAt, inv.MaxUses, inv.Uses, inv.CampaignDeleted}, err
	})
}

// AcceptInvite is POST /invites/{invite_id}/accept: accept an invite sent to the caller.
func (h *CampaignRoutes) AcceptInvite(w http.ResponseWriter, r *http.Request, inviteID api.InviteId) {
	h.join(w, r, func(ctx context.Context, q *db.Queries, userID uuid.UUID) (joinable, error) {
		inv, err := q.UserInvite(ctx, db.UserInviteParams{ID: inviteID, UserID: &userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return joinable{}, httpx.NotFound("No invite %s for you", inviteID)
		}
		return joinable{inv.ID, inv.CampaignID, inv.RevokedAt, inv.ExpiresAt, inv.MaxUses, inv.Uses, inv.CampaignDeleted}, err
	})
}

// joinable is a locked invite, either kind.
type joinable struct {
	id, campaignID       uuid.UUID
	revokedAt, expiresAt pgtype.Timestamptz
	maxUses              *int32
	uses                 int32
	campaignDeleted      bool
}

// join adds the caller as a player through the invite find locks, and answers with the campaign.
func (h *CampaignRoutes) join(w http.ResponseWriter, r *http.Request,
	find func(ctx context.Context, q *db.Queries, userID uuid.UUID) (joinable, error)) {
	ctx := r.Context()
	out, err := func() (Campaign, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return Campaign{}, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return Campaign{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		inv, err := find(ctx, q, userID)
		if err != nil {
			return Campaign{}, err
		}
		role, err := q.MemberRole(ctx, db.MemberRoleParams{CampaignID: inv.campaignID, UserID: userID})
		switch {
		case err == nil: // already in it
		case errors.Is(err, pgx.ErrNoRows):
			if err := usable(inv.revokedAt, inv.expiresAt, inv.maxUses, inv.uses, inv.campaignDeleted); err != nil {
				return Campaign{}, err
			}
			role = string(access.Player)
			if err := q.AddMember(ctx, db.AddMemberParams{CampaignID: inv.campaignID, UserID: userID, Role: role, By: &userID}); err != nil {
				return Campaign{}, err
			}
			if err := q.UseInvite(ctx, db.UseInviteParams{ID: inv.id, By: &userID}); err != nil {
				return Campaign{}, err
			}
		default:
			return Campaign{}, err
		}
		row, err := q.GetCampaign(ctx, inv.campaignID)
		if err != nil {
			return Campaign{}, err
		}
		return toCampaign(row, access.Role(role)), tx.Commit(ctx)
	}()
	if errors.Is(err, pgx.ErrNoRows) {
		err = httpx.Gone("This campaign was deleted")
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// MyInvite is an invite sent to the caller.
type MyInvite struct {
	ID           uuid.UUID `json:"id"`
	CampaignID   uuid.UUID `json:"campaign_id"`
	CampaignName string    `json:"campaign_name"`
	InvitedBy    *string   `json:"invited_by"`
	ExpiresAt    string    `json:"expires_at"`
	CreatedAt    string    `json:"created_at"`
}

type myInviteList struct {
	Entities []MyInvite `json:"entities"`
}

func myInvites(ctx context.Context, q *db.Queries, userID uuid.UUID) (myInviteList, error) {
	rows, err := q.MyPendingInvites(ctx, &userID)
	if err != nil {
		return myInviteList{}, err
	}
	out := myInviteList{Entities: make([]MyInvite, len(rows))}
	for i, row := range rows {
		out.Entities[i] = MyInvite{ID: row.ID, CampaignID: row.CampaignID, CampaignName: row.CampaignName,
			InvitedBy: row.InvitedByUsername, ExpiresAt: httpx.Timestamp(row.ExpiresAt.Time), CreatedAt: httpx.Timestamp(row.CreatedAt.Time)}
	}
	return out, nil
}

// ListMyInvites is GET /invites: the caller's open invites to campaigns they aren't in.
func (h *CampaignRoutes) ListMyInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := myInvites(ctx, db.New(h.replica), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// DeclineInvite is POST /invites/{invite_id}/decline.
func (h *CampaignRoutes) DeclineInvite(w http.ResponseWriter, r *http.Request, inviteID api.InviteId) {
	ctx := r.Context()
	out, err := func() (myInviteList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return myInviteList{}, err
		}
		tx, err := h.master.Begin(ctx)
		if err != nil {
			return myInviteList{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		inv, err := q.UserInvite(ctx, db.UserInviteParams{ID: inviteID, UserID: &userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return myInviteList{}, httpx.NotFound("No invite %s for you", inviteID)
		}
		if err != nil {
			return myInviteList{}, err
		}
		if !inv.RevokedAt.Valid && inv.Uses == 0 {
			if err := q.DeclineInvite(ctx, db.DeclineInviteParams{ID: inviteID, By: &userID}); err != nil {
				return myInviteList{}, err
			}
		}
		out, err := myInvites(ctx, q, userID)
		if err != nil {
			return myInviteList{}, err
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
