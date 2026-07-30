package handlers

import (
	"encoding/json"
	"net/http"

	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/middleware"
	repoUsers "github.com/humoroushorse/go_dnd/internal/repository/users"
	svcusers "github.com/humoroushorse/go_dnd/internal/service/users"
	"github.com/humoroushorse/go_dnd/pkg/logging"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type UserHandler struct {
	svc *svcusers.Service
}

func NewUserHandler(svc *svcusers.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()

	pgID := pgtype.UUID{Bytes: id, Valid: true}
	user, err := h.svc.GetUser(ctx, pgID)
	if err != nil {
		respondNotFound(w, r, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	ctx := r.Context()

	authUser, ok := middleware.GetUserFromContext(ctx)
	if !ok || authUser == nil {
		respondUnauthorized(w, r)
		return
	}

	// users can only update their own profile
	if authUser.ID != id {
		respondForbidden(w, r)
		return
	}

	var req api.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, r, "invalid request body")
		return
	}

	pgID := pgtype.UUID{Bytes: id, Valid: true}
	user, err := h.svc.UpdateUser(ctx, repoUsers.UpdateUserParams{
		ID:                pgID,
		Username:          req.Username,
		ProfilePictureUrl: req.ProfilePictureUrl,
	})
	if err != nil {
		logging.FromContext(ctx).Error("update user failed", "error", err)
		respondInternalError(w, r)
		return
	}

	respondJSON(w, http.StatusOK, user)
}
