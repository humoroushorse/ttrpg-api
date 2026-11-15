// Package identity maps a signed-in Keycloak user to their dnd.user id, the id every
// created_by / updated_by / deleted_by column points at.
package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	authmodels "github.com/humoroushorse/go_auth/pkg/auth/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/middleware"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

// Resolver finds or creates dnd.user rows. Users made straight in Keycloak (the bulk-upload admin)
// have no row until their first write. A user's id never changes, so lookups are cached.
type Resolver struct {
	master *pgxpool.Pool
	ids    sync.Map // Keycloak id (uuid.UUID) -> dnd.user id (uuid.UUID)
}

// NewResolver returns a Resolver that writes through master.
func NewResolver(master *pgxpool.Pool) *Resolver {
	return &Resolver{master: master}
}

// The live row, or a new one; ON CONFLICT covers a concurrent first write.
const findOrCreate = `
WITH found AS (
    SELECT id FROM dnd."user" WHERE keycloak_id = $1 AND deleted_at IS NULL
), created AS (
    INSERT INTO dnd."user" (keycloak_id, username)
    SELECT $1, $2 WHERE NOT EXISTS (SELECT 1 FROM found)
    ON CONFLICT (keycloak_id) DO NOTHING
    RETURNING id
)
SELECT id FROM found UNION ALL SELECT id FROM created`

// CurrentUserID is the signed-in user's dnd.user id, or a 401 when nobody is signed in.
func (r *Resolver) CurrentUserID(ctx context.Context) (uuid.UUID, error) {
	user, ok := middleware.GetUserFromContext(ctx)
	if !ok || user == nil {
		return uuid.Nil, httpx.Unauthorized()
	}
	return r.UserID(ctx, user)
}

// UserID is user's dnd.user id, creating the row on the first write.
func (r *Resolver) UserID(ctx context.Context, user *authmodels.User) (uuid.UUID, error) {
	keycloakID, err := uuid.Parse(user.KeycloakID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("keycloak id %q isn't a uuid: %w", user.KeycloakID, err)
	}
	if id, ok := r.ids.Load(keycloakID); ok {
		return id.(uuid.UUID), nil
	}
	var id uuid.UUID
	err = r.master.QueryRow(ctx, findOrCreate, keycloakID, user.Username).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// the only row is soft-deleted, or a concurrent request created it after our snapshot
		err = r.master.QueryRow(ctx, `SELECT id FROM dnd."user" WHERE keycloak_id = $1`, keycloakID).Scan(&id)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("dnd.user for keycloak user %s: %w", keycloakID, err)
	}
	logging.FromContext(ctx).Debug("resolved dnd.user", "keycloak_id", keycloakID, "user_id", id)
	r.ids.Store(keycloakID, id)
	return id, nil
}
