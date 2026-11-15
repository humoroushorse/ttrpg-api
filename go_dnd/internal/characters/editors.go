package characters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/characters"
)

// Editors are the people the owner lets edit the character's build (another player, the DM). The
// list is the owner's to change; an editor can take themselves off it. Neither bumps the character's
// version (it's not a change to the build), and both go in its change log.

// Editor is someone on a character's allow list.
type Editor struct {
	UserID            uuid.UUID `json:"user_id"`
	Username          *string   `json:"username"`
	ProfilePictureURL *string   `json:"profile_picture_url"`
	AddedAt           string    `json:"added_at"`
	// the caller
	Me bool `json:"me"`
}

type editorList struct {
	Entities []Editor `json:"entities"`
}

func editors(ctx context.Context, q *db.Queries, id, userID uuid.UUID) (editorList, error) {
	rows, err := q.ListEditors(ctx, id)
	if err != nil {
		return editorList{}, err
	}
	out := editorList{Entities: make([]Editor, len(rows))}
	for i, row := range rows {
		out.Entities[i] = Editor{UserID: row.UserID, Username: row.Username, ProfilePictureURL: row.ProfilePictureUrl,
			AddedAt: httpx.Timestamp(row.CreatedAt.Time), Me: row.UserID == userID}
	}
	return out, nil
}

// someone is a user named in a request body by user_id or username: someone who has signed in.
type someone struct {
	UserID   *uuid.UUID `json:"user_id"`
	Username *string    `json:"username"`
}

func (s someone) find(ctx context.Context, q *db.Queries) (uuid.UUID, string, error) {
	switch {
	case s.UserID != nil:
		u, err := q.UserByID(ctx, *s.UserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", httpx.NotFound("No user %s", *s.UserID)
		}
		return u.ID, deref(u.Username), err
	case s.Username != nil && strings.TrimSpace(*s.Username) != "":
		u, err := q.UserByUsername(ctx, strings.TrimSpace(*s.Username))
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", httpx.NotFound("No user named %q has signed in yet", *s.Username)
		}
		return u.ID, deref(u.Username), err
	}
	return uuid.Nil, "", httpx.Unprocessable("name the user with user_id or username")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ListCharacterEditors is GET /characters/{entity_id}/editors: the owner and its editors see the list.
func (h *CharacterRoutes) ListCharacterEditors(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	if _, err := requireCharacter(ctx, q, id, userID, access.EditBuild); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := editors(ctx, q, id, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// AddCharacterEditor is POST /characters/{entity_id}/editors: the owner lets someone edit the build.
func (h *CharacterRoutes) AddCharacterEditor(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	var body someone
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.Unprocessable(`the body must be {"user_id": ...} or {"username": ...}: %v`, err))
		return
	}
	h.listWrite(w, r, id, func(ctx context.Context, q *db.Queries, who access.Character, userID uuid.UUID) error {
		if err := who.Require(access.ManageEditors); err != nil {
			return err
		}
		editorID, name, err := body.find(ctx, q)
		if err != nil {
			return err
		}
		if editorID == who.OwnerID {
			return httpx.Unprocessable("%s owns the character; owners can always edit it", name)
		}
		n, err := q.AddEditor(ctx, db.AddEditorParams{CharacterID: id, UserID: editorID, By: &userID})
		if err != nil || n == 0 {
			return err // already an editor: nothing to log
		}
		return logChange(ctx, q, id, userID, nil, change{action: "editor.add", entity: "user", entityID: &editorID,
			after: map[string]any{"username": name}})
	})
}

// RemoveCharacterEditor is DELETE /characters/{entity_id}/editors/{user_id}: the owner takes someone off
// the list, or an editor takes themselves off.
func (h *CharacterRoutes) RemoveCharacterEditor(w http.ResponseWriter, r *http.Request, id api.EntityId, editorID api.UserId) {
	h.listWrite(w, r, id, func(ctx context.Context, q *db.Queries, who access.Character, userID uuid.UUID) error {
		if !(editorID == userID && who.Editor) {
			if err := who.Require(access.ManageEditors); err != nil {
				return err
			}
		}
		n, err := q.RemoveEditor(ctx, db.RemoveEditorParams{CharacterID: id, UserID: editorID})
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("User %s isn't an editor of this character", editorID)
		}
		eid := uuid.UUID(editorID)
		return logChange(ctx, q, id, userID, nil, change{action: "editor.remove", entity: "user", entityID: &eid})
	})
}

// listWrite runs fn in a transaction for someone who can at least see the character, and answers with
// its editors after (an editor who took themselves off gets an empty list).
func (h *CharacterRoutes) listWrite(w http.ResponseWriter, r *http.Request, id uuid.UUID,
	fn func(ctx context.Context, q *db.Queries, who access.Character, userID uuid.UUID) error) {
	ctx := r.Context()
	out, err := func() (editorList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return editorList{}, err
		}
		tx, err := h.begin(ctx)
		if err != nil {
			return editorList{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		who, err := requireCharacter(ctx, q, id, userID, access.EditBuild)
		if err != nil {
			return editorList{}, err
		}
		if err := fn(ctx, q, who, userID); err != nil {
			return editorList{}, err
		}
		out := editorList{Entities: []Editor{}}
		if who.Owner || editorStill(ctx, q, id, userID) {
			if out, err = editors(ctx, q, id, userID); err != nil {
				return editorList{}, err
			}
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func editorStill(ctx context.Context, q *db.Queries, id, userID uuid.UUID) bool {
	row, err := q.CharacterAccess(ctx, db.CharacterAccessParams{UserID: userID, CharacterID: id})
	return err == nil && row.Editor
}

// --- transfers: the owner offers the character to someone, who accepts before it expires ---

var transferLifetimes = map[string]time.Duration{"1h": time.Hour, "1d": 24 * time.Hour, "7d": 7 * 24 * time.Hour}

// Transfer is a pending hand-over of a character.
type Transfer struct {
	ToUserID   uuid.UUID `json:"to_user_id"`
	ToUsername *string   `json:"to_username"`
	ExpiresAt  string    `json:"expires_at"`
	CreatedAt  string    `json:"created_at"`
}

type transferResponse struct {
	// null when there's none (or it expired)
	Transfer *Transfer `json:"transfer"`
}

func pendingTransfer(ctx context.Context, q *db.Queries, id uuid.UUID) (transferResponse, error) {
	t, err := q.GetTransfer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !t.ExpiresAt.Time.After(time.Now())) {
		return transferResponse{}, nil
	}
	if err != nil {
		return transferResponse{}, err
	}
	return transferResponse{Transfer: &Transfer{ToUserID: t.ToUserID, ToUsername: t.ToUsername,
		ExpiresAt: httpx.Timestamp(t.ExpiresAt.Time), CreatedAt: httpx.Timestamp(t.CreatedAt.Time)}}, nil
}

// ReadCharacterTransfer is GET /characters/{entity_id}/transfer: the owner's pending offer, if any.
func (h *CharacterRoutes) ReadCharacterTransfer(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := db.New(h.replica)
	if _, err := requireCharacter(ctx, q, id, userID, access.TransferCharacter); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := pendingTransfer(ctx, q, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// OfferCharacterTransfer is PUT /characters/{entity_id}/transfer: the owner offers the character to
// someone (replacing any earlier offer). A character in a campaign goes only to one of its members.
func (h *CharacterRoutes) OfferCharacterTransfer(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	var body struct {
		someone
		ExpiresIn *string `json:"expires_in"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.Unprocessable("the body must be {user_id or username, expires_in}: %v", err))
		return
	}
	lifetime := transferLifetimes["7d"]
	if body.ExpiresIn != nil {
		var ok bool
		if lifetime, ok = transferLifetimes[*body.ExpiresIn]; !ok {
			httpx.WriteError(w, r, httpx.Unprocessable("expires_in is 1h, 1d or 7d"))
			return
		}
	}
	h.transferWrite(w, r, id, func(ctx context.Context, q *db.Queries, who access.Character, userID uuid.UUID) error {
		to, name, err := body.find(ctx, q)
		if err != nil {
			return err
		}
		if to == who.OwnerID {
			return httpx.Unprocessable("%s already owns the character", name)
		}
		if err := inItsCampaign(ctx, q, who, to, name); err != nil {
			return err
		}
		if err := q.OfferTransfer(ctx, db.OfferTransferParams{CharacterID: id, ToUserID: to, By: &userID,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(lifetime), Valid: true}}); err != nil {
			return err
		}
		return logChange(ctx, q, id, userID, nil, change{action: "transfer.offer", entity: "user", entityID: &to,
			after: map[string]any{"username": name}})
	})
}

// CancelCharacterTransfer is DELETE /characters/{entity_id}/transfer: the owner takes the offer back.
func (h *CharacterRoutes) CancelCharacterTransfer(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	h.transferWrite(w, r, id, func(ctx context.Context, q *db.Queries, _ access.Character, userID uuid.UUID) error {
		n, err := q.CancelTransfer(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("No offer to hand character %s over", id)
		}
		return logChange(ctx, q, id, userID, nil, change{action: "transfer.cancel"})
	})
}

// inItsCampaign: a character in a campaign is handed over only to one of the campaign's members.
func inItsCampaign(ctx context.Context, q *db.Queries, who access.Character, to uuid.UUID, name string) error {
	if who.CampaignID == nil {
		return nil
	}
	row, err := q.CampaignAccess(ctx, db.CampaignAccessParams{UserID: to, CampaignID: *who.CampaignID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) || access.CampaignFrom(*who.CampaignID, to, row.OwnerID, row.Role, row.Settings).Role == access.None {
		return httpx.Unprocessable("%s isn't in the character's campaign; invite them, or take the character out of it first", name)
	}
	return nil
}

func (h *CharacterRoutes) transferWrite(w http.ResponseWriter, r *http.Request, id uuid.UUID,
	fn func(ctx context.Context, q *db.Queries, who access.Character, userID uuid.UUID) error) {
	ctx := r.Context()
	out, err := func() (transferResponse, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return transferResponse{}, err
		}
		tx, err := h.begin(ctx)
		if err != nil {
			return transferResponse{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		who, err := requireCharacter(ctx, q, id, userID, access.TransferCharacter)
		if err != nil {
			return transferResponse{}, err
		}
		if err := fn(ctx, q, who, userID); err != nil {
			return transferResponse{}, err
		}
		out, err := pendingTransfer(ctx, q, id)
		if err != nil {
			return transferResponse{}, err
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// IncomingTransfer is a character offered to the caller.
type IncomingTransfer struct {
	CharacterID   uuid.UUID  `json:"character_id"`
	CharacterName string     `json:"character_name"`
	CurrentLevel  int32      `json:"current_level"`
	FromUsername  *string    `json:"from_username"`
	CampaignID    *uuid.UUID `json:"campaign_id"`
	CampaignName  *string    `json:"campaign_name"`
	ExpiresAt     string     `json:"expires_at"`
	CreatedAt     string     `json:"created_at"`
}

type incomingList struct {
	Entities []IncomingTransfer `json:"entities"`
}

func incoming(ctx context.Context, q *db.Queries, userID uuid.UUID) (incomingList, error) {
	rows, err := q.IncomingTransfers(ctx, userID)
	if err != nil {
		return incomingList{}, err
	}
	out := incomingList{Entities: make([]IncomingTransfer, len(rows))}
	for i, row := range rows {
		out.Entities[i] = IncomingTransfer{CharacterID: row.CharacterID, CharacterName: row.CharacterName,
			CurrentLevel: row.CurrentLevel, FromUsername: row.FromUsername, CampaignID: row.CampaignID,
			CampaignName: row.CampaignName, ExpiresAt: httpx.Timestamp(row.ExpiresAt.Time),
			CreatedAt: httpx.Timestamp(row.CreatedAt.Time)}
	}
	return out, nil
}

// ListIncomingTransfers is GET /character-transfers: characters offered to the caller.
func (h *CharacterRoutes) ListIncomingTransfers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := incoming(ctx, db.New(h.replica), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// AcceptCharacterTransfer is POST /character-transfers/{character_id}/accept: the caller becomes the
// owner. The editors list is cleared (the new owner chooses); the campaign link and DM gifts stay,
// and the change log keeps who built it.
func (h *CharacterRoutes) AcceptCharacterTransfer(w http.ResponseWriter, r *http.Request, characterID api.CharacterId) {
	ctx := r.Context()
	out, err := func() (Detail, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return Detail{}, err
		}
		tx, err := h.begin(ctx)
		if err != nil {
			return Detail{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		t, err := q.LockTransfer(ctx, db.LockTransferParams{CharacterID: characterID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, httpx.NotFound("No offer of character %s for you", characterID)
		}
		if err != nil {
			return Detail{}, err
		}
		if !t.ExpiresAt.Time.After(time.Now()) {
			return Detail{}, httpx.Gone("This offer expired")
		}
		// someone who left the character's campaign since the offer can't take it
		who, err := requireCharacter(ctx, q, characterID, t.OwnerID, access.TransferCharacter)
		if err != nil {
			return Detail{}, err
		}
		if err := inItsCampaign(ctx, q, who, userID, "you"); err != nil {
			return Detail{}, err
		}
		if err := q.SetOwner(ctx, db.SetOwnerParams{ID: characterID, OwnerID: userID}); err != nil {
			return Detail{}, err
		}
		if err := q.ClearEditors(ctx, characterID); err != nil {
			return Detail{}, err
		}
		if _, err := q.CancelTransfer(ctx, characterID); err != nil {
			return Detail{}, err
		}
		if err := logChange(ctx, q, characterID, userID, nil, change{action: "character.transfer", entity: "user",
			entityID: &userID, before: map[string]any{"owner_id": t.OwnerID}, after: map[string]any{"owner_id": userID}}); err != nil {
			return Detail{}, err
		}
		out, err := h.load(ctx, q, characterID)
		if err != nil {
			return Detail{}, err
		}
		out.Viewer = &Viewer{Owner: true, CampaignRole: string(who.Campaign.Role), CanEdit: true, CanManage: true}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// DeclineCharacterTransfer is POST /character-transfers/{character_id}/decline.
func (h *CharacterRoutes) DeclineCharacterTransfer(w http.ResponseWriter, r *http.Request, characterID api.CharacterId) {
	ctx := r.Context()
	out, err := func() (incomingList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return incomingList{}, err
		}
		tx, err := h.begin(ctx)
		if err != nil {
			return incomingList{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
		q := db.New(tx)
		if _, err := q.LockTransfer(ctx, db.LockTransferParams{CharacterID: characterID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
			return incomingList{}, httpx.NotFound("No offer of character %s for you", characterID)
		} else if err != nil {
			return incomingList{}, err
		}
		if _, err := q.CancelTransfer(ctx, characterID); err != nil {
			return incomingList{}, err
		}
		if err := logChange(ctx, q, characterID, userID, nil, change{action: "transfer.decline"}); err != nil {
			return incomingList{}, err
		}
		out, err := incoming(ctx, q, userID)
		if err != nil {
			return incomingList{}, err
		}
		return out, tx.Commit(ctx)
	}()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
