package tabletop

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

type memberScene struct {
	UserID  uuid.UUID `json:"user_id"`
	SceneID uuid.UUID `json:"scene_id"`
}

type stateView struct {
	Role          string        `json:"role"`
	Seq           int64         `json:"seq"`
	ActiveSceneID *uuid.UUID    `json:"active_scene_id"`
	Scene         *SceneView    `json:"scene"`
	Tokens        []TokenView   `json:"tokens"`
	Members       []memberScene `json:"members,omitempty"`
}

// ReadVttState is GET /vtt/campaigns/{entity_id}/state: one snapshot (repeatable read on the replica),
// so the tokens and the seq to stream from agree. A replica a little behind only means the stream
// replays a little more.
func (h *TableRoutes) ReadVttState(w http.ResponseWriter, r *http.Request, id api.EntityId, p api.ReadVttStateParams) {
	ctx := r.Context()
	out, err := func() (stateView, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return stateView{}, err
		}
		tx, err := h.replica.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return stateView{}, err
		}
		defer tx.Rollback(ctx) //nolint:errcheck // read-only
		q := db.New(tx)
		who, err := require(ctx, q, id, userID, access.ViewTable)
		if err != nil {
			return stateView{}, err
		}
		st, err := tableState(ctx, q, id, userID)
		if err != nil {
			return stateView{}, err
		}
		characters, err := q.CampaignCharacterEditors(ctx, db.CampaignCharacterEditorsParams{CampaignID: id, UserID: userID})
		if err != nil {
			return stateView{}, err
		}
		viewer := NewViewer(userID, who, st.ActiveSceneID, st.MemberSceneID, characters)
		out := stateView{Role: string(who.Role), Seq: st.LastSeq, Tokens: []TokenView{}}
		if viewer.DM() {
			out.ActiveSceneID = st.ActiveSceneID
			moved, err := q.MemberScenes(ctx, id)
			if err != nil {
				return stateView{}, err
			}
			for _, m := range moved {
				out.Members = append(out.Members, memberScene{UserID: m.UserID, SceneID: m.SceneID})
			}
		}
		sceneID := viewer.Current()
		if p.Scene != nil {
			sceneID = p.Scene
		}
		if sceneID == nil {
			return out, nil // no active scene yet: the table waits for the DM
		}
		if err := who.RequireScene(*sceneID, viewer.Current()); err != nil {
			return stateView{}, err
		}
		s, err := liveScene(ctx, q, id, *sceneID)
		if err != nil {
			return stateView{}, err
		}
		scene := viewer.Scene(sceneView(s))
		out.Scene = &scene
		rows, err := q.SceneTokens(ctx, *sceneID)
		if err != nil {
			return stateView{}, err
		}
		for _, row := range rows {
			if t, ok := viewer.Token(tokenRecordFromList(row)); ok {
				out.Tokens = append(out.Tokens, t)
			}
		}
		return out, nil
	}()
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
