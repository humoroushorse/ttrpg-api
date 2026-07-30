package handlers

import (
	"encoding/csv"
	"encoding/json"
	"net/http"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/middleware"
	svcsources "github.com/humoroushorse/go_dnd/internal/service/sources"
	"github.com/humoroushorse/go_dnd/pkg/logging"
	"github.com/humoroushorse/go_dnd/pkg/models"
)

type SourceHandler struct {
	svc *svcsources.Service
}

func NewSourceHandler(svc *svcsources.Service) *SourceHandler {
	return &SourceHandler{svc: svc}
}

func (h *SourceHandler) QuerySources(w http.ResponseWriter, r *http.Request, params api.QuerySourcesParams) {
	ctx := r.Context()

	sources, err := h.svc.QuerySources(ctx, params.DndVersion)
	if err != nil {
		logging.FromContext(ctx).Error("query sources failed", "error", err)
		respondInternalError(w, r)
		return
	}

	filters := map[string]any{}
	if params.DndVersion != nil {
		filters["dnd_version"] = *params.DndVersion
	}

	respondJSON(w, http.StatusOK, models.GenericListResponse[any]{
		TotalEntitiesCount: len(sources),
		Limit:              100,
		Offset:             0,
		Filters:            filters,
		Entities:           toAnySlice(sources),
		EntitiesCount:      len(sources),
	})
}

func (h *SourceHandler) BulkLoadSources(w http.ResponseWriter, r *http.Request) {
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

	var records []svcsources.CreateSourceInput
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		logging.FromContext(ctx).Error("json decode failed", "error", err)
		file.Seek(0, 0)
		reader := csv.NewReader(file)
		rows, csvErr := reader.ReadAll()
		if csvErr != nil {
			respondBadRequest(w, r, "failed to parse file as JSON or CSV: "+err.Error())
			return
		}
		records, err = parseSourceCSV(rows)
		if err != nil {
			respondBadRequest(w, r, err.Error())
			return
		}
	}

	result, err := h.svc.BulkLoadSources(ctx, records, user.ID)
	if err != nil {
		logging.FromContext(ctx).Error("bulk load sources failed", "error", err)
		respondInternalError(w, r)
		return
	}

	respondJSON(w, http.StatusOK, result)
}
