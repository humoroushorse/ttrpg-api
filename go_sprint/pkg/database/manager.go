package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Config holds database configuration
type Config struct {
	MasterURL       string
	ReplicaURL      string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Manager manages master and replica database connections
type Manager struct {
	master  *sql.DB
	replica *sql.DB
}

// NewManager creates a new database manager with master/replica configuration
func NewManager(cfg Config) (*Manager, error) {
	// Connect to master database
	master, err := sql.Open("postgres", cfg.MasterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open master database: %w", err)
	}

	// Configure master connection pool
	master.SetMaxOpenConns(cfg.MaxOpenConns)
	master.SetMaxIdleConns(cfg.MaxIdleConns)
	master.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify master connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := master.PingContext(ctx); err != nil {
		master.Close()
		return nil, fmt.Errorf("failed to ping master database: %w", err)
	}

	// Connect to replica database (may be same as master for local dev)
	replica, err := sql.Open("postgres", cfg.ReplicaURL)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("failed to open replica database: %w", err)
	}

	// Configure replica connection pool
	replica.SetMaxOpenConns(cfg.MaxOpenConns)
	replica.SetMaxIdleConns(cfg.MaxIdleConns)
	replica.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify replica connection
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := replica.PingContext(ctx); err != nil {
		master.Close()
		replica.Close()
		return nil, fmt.Errorf("failed to ping replica database: %w", err)
	}

	return &Manager{
		master:  master,
		replica: replica,
	}, nil
}

// Master returns the master database connection for write operations
func (m *Manager) Master() *sql.DB {
	return m.master
}

// Replica returns the replica database connection for read operations
func (m *Manager) Replica() *sql.DB {
	return m.replica
}

// Close closes both master and replica connections
func (m *Manager) Close() error {
	var masterErr, replicaErr error

	if m.master != nil {
		masterErr = m.master.Close()
	}

	if m.replica != nil {
		replicaErr = m.replica.Close()
	}

	if masterErr != nil {
		return fmt.Errorf("failed to close master: %w", masterErr)
	}

	if replicaErr != nil {
		return fmt.Errorf("failed to close replica: %w", replicaErr)
	}

	return nil
}

// HealthCheck verifies both master and replica connections are healthy
func (m *Manager) HealthCheck(ctx context.Context) error {
	if err := m.master.PingContext(ctx); err != nil {
		return fmt.Errorf("master database unhealthy: %w", err)
	}

	if err := m.replica.PingContext(ctx); err != nil {
		return fmt.Errorf("replica database unhealthy: %w", err)
	}

	return nil
}
