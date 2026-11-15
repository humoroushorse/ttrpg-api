package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
)

// Target is a migrations directory and where its version table lives.
type Target struct {
	Path string
	// Schema holds the version table (schema_migrations) and is created if it doesn't exist, so one
	// database can carry several migration histories (dnd_vtt beside Alembic's dnd). Empty: the
	// connection's current schema.
	Schema string
}

func open(ctx context.Context, db *sql.DB, t Target) (*migrate.Migrate, error) {
	if t.Schema != "" {
		if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+pq.QuoteIdentifier(t.Schema)); err != nil {
			return nil, fmt.Errorf("failed to create schema %s: %w", t.Schema, err)
		}
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{SchemaName: t.Schema})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", t.Path), "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}
	return m, nil
}

// MigrateUp applies every migration not yet applied.
func MigrateUp(ctx context.Context, db *sql.DB, t Target) error {
	m, err := open(ctx, db, t)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// MigrateDown rolls back the last applied migration only.
func MigrateDown(ctx context.Context, db *sql.DB, t Target) error {
	m, err := open(ctx, db, t)
	if err != nil {
		return err
	}
	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}
	return nil
}

// MigrateVersion reports the applied version and whether the last migration failed part-way.
func MigrateVersion(ctx context.Context, db *sql.DB, t Target) (uint, bool, error) {
	m, err := open(ctx, db, t)
	if err != nil {
		return 0, false, err
	}
	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}
