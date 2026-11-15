package tabletop

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

var sceneKeys = []string{"name", "parent_id", "sort_order", "background_sources", "grid_type", "width_cells",
	"height_cells", "cell_px", "grid_offset_x", "grid_offset_y", "grid_distance", "grid_units", "settings"}

// sceneInput is every editable column of a scene.
type sceneInput struct {
	db.UpdateSceneParams
}

// sceneInput applies the body to base: a create's defaults, or the current row for a PATCH.
func (f fields) sceneInput(base db.DndVttScene, create bool) (sceneInput, error) {
	in := sceneInput{db.UpdateSceneParams{ParentID: base.ParentID, SortOrder: base.SortOrder, Name: base.Name,
		BackgroundSources: base.BackgroundSources, GridType: base.GridType, WidthCells: base.WidthCells,
		HeightCells: base.HeightCells, CellPx: base.CellPx, GridOffsetX: base.GridOffsetX, GridOffsetY: base.GridOffsetY,
		GridDistance: base.GridDistance, GridUnits: base.GridUnits, Settings: base.Settings}}
	if create {
		in.BackgroundSources, in.GridType, in.CellPx, in.GridDistance, in.GridUnits = []byte(`[]`), "square", 70, 5, "ft"
	}
	if name, present, err := f.text("name", 200, false, false); err != nil {
		return in, err
	} else if present {
		in.Name = *name
	} else if create {
		return in, httpx.Unprocessable("name: field required")
	}
	if _, err := f.decode("parent_id", &in.ParentID, true); err != nil {
		return in, err
	}
	if _, err := f.decode("sort_order", &in.SortOrder, false); err != nil {
		return in, err
	}
	var err error
	if in.BackgroundSources, err = f.backgroundSources(in.BackgroundSources); err != nil {
		return in, err
	}
	if _, err := f.decode("grid_type", &in.GridType, false); err != nil {
		return in, err
	}
	if err := oneOf("grid_type", in.GridType, "square", "none"); err != nil {
		return in, err
	}
	for _, size := range []struct {
		key string
		dst *int32
	}{{"width_cells", &in.WidthCells}, {"height_cells", &in.HeightCells}} {
		present, err := f.decode(size.key, size.dst, false)
		if err != nil {
			return in, err
		}
		if !present && create {
			return in, httpx.Unprocessable("%s: field required", size.key)
		}
		if *size.dst < 1 || *size.dst > 500 {
			return in, httpx.Unprocessable("%s must be 1 to 500", size.key)
		}
	}
	for _, n := range []struct {
		key      string
		dst      *float64
		min, max float64
	}{
		{"cell_px", &in.CellPx, 8, 1024},
		{"grid_offset_x", &in.GridOffsetX, -100_000, 100_000},
		{"grid_offset_y", &in.GridOffsetY, -100_000, 100_000},
		{"grid_distance", &in.GridDistance, 1e-6, 1_000_000},
	} {
		if _, err := f.decode(n.key, n.dst, false); err != nil {
			return in, err
		}
		if err := finite(n.key, *n.dst, n.min, n.max); err != nil {
			return in, err
		}
	}
	if units, present, err := f.text("grid_units", 10, false, false); err != nil {
		return in, err
	} else if present {
		in.GridUnits = *units
	}
	if in.Settings, err = f.settings(in.Settings); err != nil {
		return in, err
	}
	return in, nil
}

// requireManage: DMs may change scenes; a player gets a 404 for a scene they can't see, a 403 for
// the one they're on.
func requireManage(who access.Campaign, sceneID uuid.UUID, st db.LockTableStateRow) error {
	if who.Can(access.ManageScenes) {
		return nil
	}
	if err := who.RequireScene(sceneID, current(st)); err != nil {
		return err
	}
	return who.Require(access.ManageScenes)
}

// sceneCampaign finds which campaign a scene is in, for the /vtt/scenes/{id} routes. The caller's
// access is checked after, and a scene in someone else's campaign is a 404 either way.
func (h *TableRoutes) sceneCampaign(ctx context.Context, sceneID uuid.UUID) (uuid.UUID, error) {
	s, err := db.New(h.master).Scene(ctx, sceneID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, sceneNotFound(sceneID)
	}
	return s.CampaignID, err
}

// asSceneNotFound keeps a scene route from naming the campaign to someone who isn't in it.
func asSceneNotFound(err error, sceneID uuid.UUID) error {
	var e *httpx.Error
	if errors.As(err, &e) && e.Status == http.StatusNotFound {
		return sceneNotFound(sceneID)
	}
	return err
}

type sceneList struct {
	ActiveSceneID *uuid.UUID  `json:"active_scene_id"`
	Entities      []SceneView `json:"entities"`
}

// ListVttScenes is GET /vtt/campaigns/{entity_id}/scenes.
func (h *TableRoutes) ListVttScenes(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	out, err := func() (sceneList, error) {
		userID, err := h.users.CurrentUserID(ctx)
		if err != nil {
			return sceneList{}, err
		}
		q := db.New(h.replica)
		who, err := require(ctx, q, id, userID, access.ViewTable)
		if err != nil {
			return sceneList{}, err
		}
		st, err := tableState(ctx, q, id, userID)
		if err != nil {
			return sceneList{}, err
		}
		out := sceneList{Entities: []SceneView{}}
		if who.Role == access.DM {
			rows, err := q.Scenes(ctx, id)
			if err != nil {
				return sceneList{}, err
			}
			for _, row := range rows {
				out.Entities = append(out.Entities, sceneView(row))
			}
			out.ActiveSceneID = st.ActiveSceneID
			return out, nil
		}
		viewer := Viewer{Campaign: who, Active: st.ActiveSceneID, Moved: st.MemberSceneID}
		if cur := viewer.Current(); cur != nil {
			s, err := liveScene(ctx, q, id, *cur)
			if err != nil {
				return sceneList{}, err
			}
			out.Entities = append(out.Entities, viewer.Scene(sceneView(s)))
		}
		return out, nil
	}()
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// CreateVttScene is POST /vtt/campaigns/{entity_id}/scenes.
func (h *TableRoutes) CreateVttScene(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	f, err := readFields(r, sceneKeys...)
	if err != nil {
		writeError(w, r, err)
		return
	}
	in, err := f.sceneInput(db.DndVttScene{}, true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var created db.DndVttScene
	err = h.write(ctx, id, access.ManageScenes, func(q *db.Queries, _ access.Campaign, _ db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
		if in.ParentID != nil {
			if _, err := liveScene(ctx, q, id, *in.ParentID); err != nil {
				return nil, httpx.Unprocessable("parent_id: no such scene in this campaign")
			}
		}
		row, err := q.CreateScene(ctx, db.CreateSceneParams{CampaignID: id, ParentID: in.ParentID, Name: in.Name,
			BackgroundSources: in.BackgroundSources, GridType: in.GridType, WidthCells: in.WidthCells,
			HeightCells: in.HeightCells, CellPx: in.CellPx, GridOffsetX: in.GridOffsetX, GridOffsetY: in.GridOffsetY,
			GridDistance: in.GridDistance, GridUnits: in.GridUnits, Settings: in.Settings, By: userID})
		if err != nil {
			return nil, err
		}
		created = row
		e, err := appendEvent(ctx, q, id, userID, nil, draft{typ: SceneCreated, audience: AudienceDMs, sceneID: &row.ID,
			payload: scenePayload{Scene: sceneView(row)}})
		return []Event{e}, err
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sceneView(created))
}

// UpdateVttScene is PATCH /vtt/scenes/{entity_id}.
func (h *TableRoutes) UpdateVttScene(w http.ResponseWriter, r *http.Request, sceneID api.EntityId) {
	ctx := r.Context()
	f, err := readFields(r, sceneKeys...)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var updated db.DndVttScene
	err = func() error {
		campaignID, err := h.sceneCampaign(ctx, sceneID)
		if err != nil {
			return err
		}
		return h.write(ctx, campaignID, access.ViewTable, func(q *db.Queries, who access.Campaign, st db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
			if err := requireManage(who, sceneID, st); err != nil {
				return nil, err
			}
			cur, err := liveScene(ctx, q, campaignID, sceneID)
			if err != nil {
				return nil, err
			}
			in, err := f.sceneInput(cur, false)
			if err != nil {
				return nil, err
			}
			if f.has("parent_id") && in.ParentID != nil {
				if _, err := liveScene(ctx, q, campaignID, *in.ParentID); err != nil {
					return nil, httpx.Unprocessable("parent_id: no such scene in this campaign")
				}
				under, err := q.SceneSubtree(ctx, db.SceneSubtreeParams{CampaignID: campaignID, ID: sceneID})
				if err != nil {
					return nil, err
				}
				if slices.Contains(under, *in.ParentID) {
					return nil, httpx.Unprocessable("parent_id: a scene can't go under itself or a scene inside it")
				}
			}
			in.ID, in.CampaignID, in.By = sceneID, campaignID, userID
			row, err := q.UpdateScene(ctx, in.UpdateSceneParams)
			if err != nil {
				return nil, err
			}
			updated = row
			// players on the scene see its new map or grid; the tree is in DMs' copies only
			e, err := appendEvent(ctx, q, campaignID, userID, nil, draft{typ: SceneUpdated, audience: AudienceAll,
				sceneID: &row.ID, payload: scenePayload{Scene: sceneView(row)}})
			return []Event{e}, err
		})
	}()
	if err != nil {
		writeError(w, r, asSceneNotFound(err, sceneID))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sceneView(updated))
}

// DeleteVttScene is DELETE /vtt/scenes/{entity_id}: the scene and everything under it go to the
// trash together, with their tokens, so a restore brings it all back.
func (h *TableRoutes) DeleteVttScene(w http.ResponseWriter, r *http.Request, sceneID api.EntityId) {
	ctx := r.Context()
	var trashed []uuid.UUID
	err := func() error {
		campaignID, err := h.sceneCampaign(ctx, sceneID)
		if err != nil {
			return err
		}
		return h.write(ctx, campaignID, access.ViewTable, func(q *db.Queries, who access.Campaign, st db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
			if err := requireManage(who, sceneID, st); err != nil {
				return nil, err
			}
			ids, err := q.SceneSubtree(ctx, db.SceneSubtreeParams{CampaignID: campaignID, ID: sceneID})
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				return nil, sceneNotFound(sceneID)
			}
			if st.ActiveSceneID != nil && slices.Contains(ids, *st.ActiveSceneID) {
				return nil, httpx.Conflict("the active scene is in it; switch to another scene first")
			}
			moved, err := q.MemberScenes(ctx, campaignID)
			if err != nil {
				return nil, err
			}
			for _, m := range moved {
				if slices.Contains(ids, m.SceneID) {
					return nil, httpx.Conflict("a player is on a scene in it; move them first")
				}
			}
			if err := q.TrashScenes(ctx, db.TrashScenesParams{CampaignID: campaignID, Ids: ids, By: userID}); err != nil {
				return nil, err
			}
			trashed = ids
			e, err := appendEvent(ctx, q, campaignID, userID, nil, draft{typ: SceneTrashed, audience: AudienceDMs,
				payload: trashPayload{SceneIDs: ids}})
			return []Event{e}, err
		})
	}()
	if err != nil {
		writeError(w, r, asSceneNotFound(err, sceneID))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, trashed)
}

// sceneChoice reads {"scene_id": uuid | null}.
func sceneChoice(r *http.Request) (*uuid.UUID, error) {
	f, err := readFields(r, "scene_id")
	if err != nil {
		return nil, err
	}
	var sceneID *uuid.UUID
	present, err := f.decode("scene_id", &sceneID, true)
	if err == nil && !present {
		err = httpx.Unprocessable("scene_id: field required (null for none)")
	}
	return sceneID, err
}

type seqResult struct {
	Seq   *int64     `json:"seq"`
	Token *TokenView `json:"token,omitempty"`
}

// SetVttActiveScene is PUT /vtt/campaigns/{entity_id}/active-scene: everyone goes to the scene.
func (h *TableRoutes) SetVttActiveScene(w http.ResponseWriter, r *http.Request, id api.EntityId) {
	ctx := r.Context()
	sceneID, err := sceneChoice(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var out seqResult
	err = h.write(ctx, id, access.ManageScenes, func(q *db.Queries, _ access.Campaign, st db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
		if sceneID != nil {
			if _, err := liveScene(ctx, q, id, *sceneID); err != nil {
				return nil, httpx.Unprocessable("scene_id: no such scene in this campaign")
			}
		}
		if err := q.SetActiveScene(ctx, db.SetActiveSceneParams{CampaignID: id, SceneID: sceneID, UserID: userID}); err != nil {
			return nil, err
		}
		movedBack, err := q.ClearMemberScenes(ctx, db.ClearMemberScenesParams{CampaignID: id, By: userID})
		if err != nil {
			return nil, err
		}
		e, err := appendEvent(ctx, q, id, userID, nil, draft{typ: SceneActivated, audience: AudienceAll,
			payload: activatedPayload{SceneID: sceneID, PreviousSceneID: st.ActiveSceneID, MovedBack: movedBack}})
		out.Seq = &e.Seq
		return []Event{e}, err
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SetVttMemberScene is PUT /vtt/campaigns/{entity_id}/members/{user_id}/scene: one player goes to a
// scene (a split party), or back to the active one.
func (h *TableRoutes) SetVttMemberScene(w http.ResponseWriter, r *http.Request, id api.EntityId, memberID api.UserId) {
	ctx := r.Context()
	sceneID, err := sceneChoice(r)
	if err != nil {
		writeError(w, r, err)
		return
	}
	var out seqResult
	err = h.write(ctx, id, access.ManageScenes, func(q *db.Queries, _ access.Campaign, _ db.LockTableStateRow, userID uuid.UUID) ([]Event, error) {
		row, err := q.CampaignAccess(ctx, db.CampaignAccessParams{UserID: memberID, CampaignID: id})
		if err != nil {
			return nil, err
		}
		target := access.CampaignFrom(id, memberID, row.OwnerID, row.Role, row.Settings)
		switch target.Role {
		case access.None:
			return nil, httpx.NotFound("User %s isn't in this campaign", memberID)
		case access.DM:
			return nil, httpx.Unprocessable("DMs can open any scene; this moves players")
		}
		if sceneID != nil {
			if _, err := liveScene(ctx, q, id, *sceneID); err != nil {
				return nil, httpx.Unprocessable("scene_id: no such scene in this campaign")
			}
		}
		if err := q.SetMemberScene(ctx, db.SetMemberSceneParams{CampaignID: id, UserID: memberID, SceneID: sceneID, By: userID}); err != nil {
			return nil, err
		}
		e, err := appendEvent(ctx, q, id, userID, nil, draft{typ: MemberMoved, audience: AudienceDMs,
			payload: memberMovedPayload{UserID: memberID, SceneID: sceneID}})
		out.Seq = &e.Seq
		return []Event{e}, err
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
