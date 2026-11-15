package table

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/humoroushorse/go_dnd/internal/httpx"
)

// Querier is a pool or a transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

var _ Querier = (*pgxpool.Pool)(nil)

// Insert creates a row from a Full record and returns it as JSON.
func (t *Table) Insert(ctx context.Context, db Querier, r Record, userID uuid.UUID) ([]byte, error) {
	cols := t.columnsOf(r)
	sql := fmt.Sprintf(`WITH t AS (
    INSERT INTO %[1]s (%[2]s, created_by, updated_by)
    SELECT %[3]s, $2::uuid, $2::uuid FROM jsonb_populate_record(NULL::%[1]s, $1::jsonb) r
    RETURNING *
) SELECT %[4]s FROM %[5]s`, t.Name, columnList("", cols), columnList("r.", cols), t.jsonFor(t.fields), t.from("t"))
	return t.writeOne(ctx, db, sql, r, userID)
}

// Update sets the record's columns on a live row and returns it as JSON, or nil when the row is
// missing or deleted. A Full record replaces every writable column; a Patch record only its own.
func (t *Table) Update(ctx context.Context, db Querier, id uuid.UUID, r Record, userID uuid.UUID) ([]byte, error) {
	cols := t.columnsOf(r)
	if len(cols) == 0 { // an empty PATCH changes nothing
		return t.getAny(ctx, db, id)
	}
	sql := fmt.Sprintf(`WITH t AS (
    UPDATE %[1]s AS t SET (%[2]s, updated_at, updated_by) =
        (SELECT %[3]s, now(), $2::uuid FROM jsonb_populate_record(NULL::%[1]s, $1::jsonb) r)
    WHERE t.id = $3 AND t.deleted_at IS NULL
    RETURNING t.*
) SELECT %[4]s FROM %[5]s`, t.Name, columnList("", cols), columnList("r.", cols), t.jsonFor(t.fields), t.from("t"))
	return t.writeOne(ctx, db, sql, r, userID, id)
}

func (t *Table) getAny(ctx context.Context, db Querier, id uuid.UUID) ([]byte, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE t.deleted_at IS NULL AND t.id = $1", t.jsonFor(t.fields), t.from(t.Name))
	var body []byte
	err := db.QueryRow(ctx, sql, id).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return body, err
}

func (t *Table) writeOne(ctx context.Context, db Querier, sql string, r Record, userID uuid.UUID, more ...any) ([]byte, error) {
	doc, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var body []byte
	err = db.QueryRow(ctx, sql, append([]any{string(doc), userID}, more...)...).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, t.constraintError(err)
	}
	return body, nil
}

// SoftDelete hides a live row from every read. False when there is no live row.
func (t *Table) SoftDelete(ctx context.Context, db Querier, id, userID uuid.UUID) (bool, error) {
	sql := fmt.Sprintf("UPDATE %s SET deleted_at = now(), deleted_by = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING id", t.Name)
	var deleted uuid.UUID
	err := db.QueryRow(ctx, sql, id, userID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// constraintError turns a unique or foreign key violation into the client's error.
func (t *Table) constraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505": // unique_violation
		return httpx.Conflict("A %s like that already exists (%s): %s", t.Label, pgErr.ConstraintName, pgErr.Detail)
	case "23503": // foreign_key_violation
		return httpx.Unprocessable("%s: %s", pgErr.ConstraintName, pgErr.Detail)
	case "23514", "22P02", "22003": // check, invalid text representation, out of range
		return httpx.Unprocessable("%s", pgErr.Message)
	}
	return err
}
