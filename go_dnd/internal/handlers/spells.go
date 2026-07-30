package handlers

import (
	"encoding/csv"
	"encoding/json"
	"net/http"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/middleware"
	svcspells "github.com/humoroushorse/go_dnd/internal/service/spells"
	"github.com/humoroushorse/go_dnd/pkg/logging"
	"github.com/humoroushorse/go_dnd/pkg/models"
)

type SpellHandler struct {
	svc *svcspells.Service
}

func NewSpellHandler(svc *svcspells.Service) *SpellHandler {
	return &SpellHandler{svc: svc}
}

func (h *SpellHandler) ListSpells(w http.ResponseWriter, r *http.Request, params api.ListSpellsParams) {
	ctx := r.Context()
	limit := int32(100)
	offset := int32(0)
	if params.Limit != nil {
		limit = int32(*params.Limit)
	}
	if params.Offset != nil {
		offset = int32(*params.Offset)
	}

	spells, total, err := h.svc.ListSpells(ctx, limit, offset)
	if err != nil {
		logging.FromContext(ctx).Error("list spells failed", "error", err)
		respondInternalError(w, r)
		return
	}

	respondJSON(w, http.StatusOK, models.GenericListResponse[any]{
		TotalEntitiesCount: int(total),
		Limit:              int(limit),
		Offset:             int(offset),
		Filters:            map[string]any{},
		Entities:           toAnySlice(spells),
		EntitiesCount:      len(spells),
	})
}

func (h *SpellHandler) QuerySpells(w http.ResponseWriter, r *http.Request, params api.QuerySpellsParams) {
	ctx := r.Context()
	limit := int32(100)
	offset := int32(0)
	if params.Limit != nil {
		limit = int32(*params.Limit)
	}
	if params.Offset != nil {
		offset = int32(*params.Offset)
	}

	var school *string
	if params.School != nil {
		s := string(*params.School)
		school = &s
	}

	var level *int32
	if params.Level != nil {
		l := int32(*params.Level)
		level = &l
	}

	spells, total, err := h.svc.QuerySpells(ctx, params.Name, level, school, limit, offset)
	if err != nil {
		logging.FromContext(ctx).Error("query spells failed", "error", err)
		respondInternalError(w, r)
		return
	}

	filters := map[string]any{}
	if params.Name != nil {
		filters["name"] = *params.Name
	}
	if params.Level != nil {
		filters["level"] = *params.Level
	}
	if params.School != nil {
		filters["school"] = *params.School
	}

	respondJSON(w, http.StatusOK, models.GenericListResponse[any]{
		TotalEntitiesCount: int(total),
		Limit:              int(limit),
		Offset:             int(offset),
		Filters:            filters,
		Entities:           toAnySlice(spells),
		EntitiesCount:      len(spells),
	})
}

func (h *SpellHandler) CreateSpell(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := middleware.GetUserFromContext(ctx)
	if !ok || user == nil {
		respondUnauthorized(w, r)
		return
	}

	var req api.CreateSpellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, r, "invalid request body")
		return
	}

	spell, err := h.svc.CreateSpell(ctx, mapCreateSpellRequest(req), user.ID)
	if err != nil {
		logging.FromContext(ctx).Error("create spell failed", "error", err)
		respondBadRequest(w, r, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, spell)
}

func (h *SpellHandler) BulkLoadSpells(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := middleware.GetUserFromContext(ctx)
	if !ok || user == nil {
		respondUnauthorized(w, r)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondBadRequest(w, r, "failed to parse multipart form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		respondBadRequest(w, r, "file field is required")
		return
	}
	defer file.Close()

	var records []svcspells.CreateSpellInput
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		logging.FromContext(ctx).Error("json decode failed", "error", err)
		file.Seek(0, 0)
		reader := csv.NewReader(file)
		rows, csvErr := reader.ReadAll()
		if csvErr != nil {
			respondBadRequest(w, r, "failed to parse file as JSON or CSV: "+err.Error())
			return
		}
		records, err = parseSpellCSV(rows)
		if err != nil {
			respondBadRequest(w, r, err.Error())
			return
		}
	}

	result, err := h.svc.BulkLoadSpells(ctx, records, user.ID)
	if err != nil {
		logging.FromContext(ctx).Error("bulk load spells failed", "error", err)
		respondInternalError(w, r)
		return
	}

	respondJSON(w, http.StatusOK, result)
}
