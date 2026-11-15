package tabletop

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/access"
	"github.com/humoroushorse/go_dnd/internal/httpx"
	"github.com/humoroushorse/go_dnd/internal/identity"
	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

// TableRoutes serves /api/v1/vtt.
type TableRoutes struct {
	master, replica *pgxpool.Pool
	users           *identity.Resolver
	table           *Table
	pings           *pingLimiter
}

// NewRoutes reads from replica, writes through master, and publishes through table.
func NewRoutes(master, replica *pgxpool.Pool, users *identity.Resolver, table *Table) *TableRoutes {
	return &TableRoutes{master: master, replica: replica, users: users, table: table,
		pings: &pingLimiter{buckets: map[uuid.UUID]*bucket{}}}
}

// require is who the caller is to the campaign, or the error when they may not take the action.
func require(ctx context.Context, q *db.Queries, campaignID, userID uuid.UUID, a access.CampaignAction) (access.Campaign, error) {
	row, err := q.CampaignAccess(ctx, db.CampaignAccessParams{UserID: userID, CampaignID: campaignID})
	if errors.Is(err, pgx.ErrNoRows) {
		return access.Campaign{}, httpx.NotFound("Campaign %s not found", campaignID)
	}
	if err != nil {
		return access.Campaign{}, err
	}
	who := access.CampaignFrom(campaignID, userID, row.OwnerID, row.Role, row.Settings)
	return who, who.Require(a)
}

func sceneNotFound(id uuid.UUID) error { return httpx.NotFound("Scene %s not found", id) }

// liveScene is a scene of this campaign that isn't in the trash.
func liveScene(ctx context.Context, q *db.Queries, campaignID, sceneID uuid.UUID) (db.DndVttScene, error) {
	s, err := q.Scene(ctx, sceneID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && s.CampaignID != campaignID) {
		return db.DndVttScene{}, sceneNotFound(sceneID)
	}
	return s, err
}

// tableState is the table's state for a read; a campaign whose table nobody set up yet has none.
func tableState(ctx context.Context, q *db.Queries, campaignID, userID uuid.UUID) (db.TableStateRow, error) {
	st, err := q.TableState(ctx, db.TableStateParams{CampaignID: campaignID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.TableStateRow{}, nil
	}
	return st, err
}

func current(st db.LockTableStateRow) *uuid.UUID {
	if st.MemberSceneID != nil {
		return st.MemberSceneID
	}
	return st.ActiveSceneID
}

// write runs fn in a transaction once the caller may take the action, with the table locked (lock),
// and publishes fn's events after the commit.
func (h *TableRoutes) write(ctx context.Context, campaignID uuid.UUID, a access.CampaignAction,
	fn func(q *db.Queries, who access.Campaign, st db.LockTableStateRow, userID uuid.UUID) ([]Event, error)) error {
	userID, err := h.users.CurrentUserID(ctx)
	if err != nil {
		return err
	}
	tx, err := h.master.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	who, err := require(ctx, q, campaignID, userID, a)
	if err != nil {
		return err
	}
	st, err := lock(ctx, q, campaignID, userID)
	if err != nil {
		return err
	}
	events, err := fn(q, who, st, userID)
	if err != nil {
		return constraintError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	h.table.Publish(ctx, events)
	return nil
}

// pingLimiter allows each user a burst of pings, refilling a few a second: a ping is one request
// fanned out to every viewer, so a stuck key mustn't flood the table.
type pingLimiter struct {
	mu      sync.Mutex
	buckets map[uuid.UUID]*bucket
}

type bucket struct {
	tokens float64
	at     time.Time
}

const (
	pingBurst  = 10
	pingPerSec = 5
)

func (l *pingLimiter) allow(userID uuid.UUID, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10_000 {
		for id, b := range l.buckets {
			if now.Sub(b.at) > time.Minute {
				delete(l.buckets, id)
			}
		}
	}
	b := l.buckets[userID]
	if b == nil {
		b = &bucket{tokens: pingBurst, at: now}
		l.buckets[userID] = b
	}
	b.tokens = min(pingBurst, b.tokens+now.Sub(b.at).Seconds()*pingPerSec)
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) { httpx.WriteError(w, r, err) }
