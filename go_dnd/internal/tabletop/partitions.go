package tabletop

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/humoroushorse/go_dnd/internal/repository/tabletop"
)

const (
	// the log keeps 90 days, dropped a month at a time, so 90 to 120 days (DECISIONS.md)
	logKeepDays = 90
	// partitions made ahead of time, so a write never lands in event_default
	monthsAhead = 3
	// the advisory lock every pod's partition job tries; one gets it
	partitionJobLock int64 = 0x76747470_61727473 // "vttparts"
)

// RunPartitions keeps the event log's monthly partitions: at startup, then daily. Every pod runs it;
// the one that gets the advisory lock does the work.
func RunPartitions(ctx context.Context, master *pgxpool.Pool, logger *slog.Logger) {
	run := func() {
		created, dropped, defaultRows, ran, err := partitionsOnce(ctx, master)
		switch {
		case err != nil:
			logger.Error("event log partitions: failed (retried tomorrow, or at the next start)", "error", err)
		case !ran:
			logger.Debug("event log partitions: another pod has the job")
		default:
			logger.Info("event log partitions", "created", created, "dropped", dropped)
		}
		if defaultRows {
			logger.Error("dnd_vtt.event_default has rows: the partition job fell behind; create the missing months and move them")
		}
	}
	run()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func partitionsOnce(ctx context.Context, master *pgxpool.Pool) (created, dropped []string, defaultRows, ran bool, err error) {
	tx, err := master.Begin(ctx)
	if err != nil {
		return nil, nil, false, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	q := db.New(tx)
	if ran, err = q.TryJobLock(ctx, partitionJobLock); err != nil || !ran {
		return nil, nil, false, false, err
	}
	if err = q.SetLockTimeout(ctx, "5s"); err != nil {
		return nil, nil, false, true, err
	}
	if created, err = q.CreateEventPartitions(ctx, monthsAhead); err != nil {
		return nil, nil, false, true, err
	}
	if dropped, err = q.DropEventPartitions(ctx, logKeepDays); err != nil {
		return nil, nil, false, true, err
	}
	if defaultRows, err = q.DefaultPartitionHasRows(ctx); err != nil {
		return nil, nil, false, true, err
	}
	return created, dropped, defaultRows, true, tx.Commit(ctx)
}
