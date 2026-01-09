package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/database"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// getTestDatabaseURL returns the test database URL from environment or default
func getTestDatabaseURL() string {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/sprint_management_test?sslmode=disable"
	}
	return url
}

// setupTestDB creates a test database connection and runs migrations
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	dbURL := getTestDatabaseURL()
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("test database not available: %v (set TEST_DATABASE_URL or start postgres)", err)
	}

	// Run migrations
	migrationsPath := "../../migrations"
	if err := database.MigrateUp(db, migrationsPath); err != nil {
		db.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		// Clean up test data
		db.Exec("TRUNCATE sprint_management.work_items, sprint_management.sprints, sprint_management.comments, sprint_management.activity_logs, sprint_management.work_item_dependencies CASCADE")
		db.Exec("TRUNCATE auth.users CASCADE")
		db.Close()
	}

	return db, cleanup
}

// TestDatabaseManager tests the database manager functionality
func TestDatabaseManager(t *testing.T) {
	dbURL := getTestDatabaseURL()

	cfg := database.Config{
		MasterURL:       dbURL,
		ReplicaURL:      dbURL, // Same as master for testing
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	manager, err := database.NewManager(cfg)
	if err != nil {
		t.Skipf("failed to create database manager: %v (test database not available)", err)
	}
	defer manager.Close()

	// Test health check
	ctx := context.Background()
	if err := manager.HealthCheck(ctx); err != nil {
		t.Errorf("health check failed: %v", err)
	}

	// Test master connection
	if manager.Master() == nil {
		t.Error("master connection is nil")
	}

	// Test replica connection
	if manager.Replica() == nil {
		t.Error("replica connection is nil")
	}
}

// TestProperty12_UTCTimestampStorage tests that all timestamps are stored in UTC
// Property 12: UTC Timestamp Storage
// For any entity creation or update, all timestamps should be stored in UTC timezone
// regardless of client timezone.
// Validates: Requirements 3.7
func TestProperty12_UTCTimestampStorage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for random timestamps
	minTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	genTimestamp := gen.Int64Range(minTime.Unix(), maxTime.Unix()).Map(func(ts int64) time.Time {
		return time.Unix(ts, 0)
	})

	// Test work_items table
	properties.Property("Feature: go-sprint-management, Property 12: Work items timestamps are stored in UTC",
		prop.ForAll(
			func(createdAt time.Time) bool {
				ctx := context.Background()

				// Create a work item with a specific timestamp
				workItemID := uuid.New()
				reporterID := uuid.New()

				// Insert work item with explicit created_at
				_, err := db.ExecContext(ctx, `
					INSERT INTO sprint_management.work_items 
					(id, type, title, description, status, priority, reporter_id, created_at, updated_at)
					VALUES ($1, 'story', 'Test Item', 'Test Description', 'todo', 'medium', $2, $3, $3)
				`, workItemID, reporterID, createdAt)

				if err != nil {
					t.Logf("failed to insert work item: %v", err)
					return false
				}

				// Retrieve the work item and check timezone
				var retrievedCreatedAt time.Time
				err = db.QueryRowContext(ctx, `
					SELECT created_at FROM sprint_management.work_items WHERE id = $1
				`, workItemID).Scan(&retrievedCreatedAt)

				if err != nil {
					t.Logf("failed to retrieve work item: %v", err)
					return false
				}

				// Verify the timestamp is in UTC (PostgreSQL may return Etc/UTC which is equivalent)
				loc := retrievedCreatedAt.Location()
				if loc.String() != "UTC" && loc.String() != "Etc/UTC" {
					t.Logf("timestamp not in UTC: got %v, location: %v", retrievedCreatedAt, loc)
					return false
				}

				// Verify the timestamp value is preserved (within 1 second tolerance for rounding)
				diff := retrievedCreatedAt.Sub(createdAt.UTC())
				if diff < -time.Second || diff > time.Second {
					t.Logf("timestamp value not preserved: expected %v, got %v, diff: %v",
						createdAt.UTC(), retrievedCreatedAt, diff)
					return false
				}

				return true
			},
			genTimestamp,
		))

	// Test sprints table
	properties.Property("Feature: go-sprint-management, Property 12: Sprint timestamps are stored in UTC",
		prop.ForAll(
			func(createdAt time.Time) bool {
				ctx := context.Background()

				// Create a sprint with a specific timestamp
				sprintID := uuid.New()
				creatorID := uuid.New()
				startDate := time.Now().AddDate(0, 0, 1)
				endDate := time.Now().AddDate(0, 0, 15)

				// Insert sprint with explicit created_at
				_, err := db.ExecContext(ctx, `
					INSERT INTO sprint_management.sprints 
					(id, name, status, start_date, end_date, created_by, created_at, updated_at)
					VALUES ($1, 'Test Sprint', 'planned', $2, $3, $4, $5, $5)
				`, sprintID, startDate, endDate, creatorID, createdAt)

				if err != nil {
					t.Logf("failed to insert sprint: %v", err)
					return false
				}

				// Retrieve the sprint and check timezone
				var retrievedCreatedAt time.Time
				err = db.QueryRowContext(ctx, `
					SELECT created_at FROM sprint_management.sprints WHERE id = $1
				`, sprintID).Scan(&retrievedCreatedAt)

				if err != nil {
					t.Logf("failed to retrieve sprint: %v", err)
					return false
				}

				// Verify the timestamp is in UTC (PostgreSQL may return Etc/UTC which is equivalent)
				loc := retrievedCreatedAt.Location()
				if loc.String() != "UTC" && loc.String() != "Etc/UTC" {
					t.Logf("timestamp not in UTC: got %v, location: %v", retrievedCreatedAt, loc)
					return false
				}

				// Verify the timestamp value is preserved
				diff := retrievedCreatedAt.Sub(createdAt.UTC())
				if diff < -time.Second || diff > time.Second {
					t.Logf("timestamp value not preserved: expected %v, got %v, diff: %v",
						createdAt.UTC(), retrievedCreatedAt, diff)
					return false
				}

				return true
			},
			genTimestamp,
		))

	// Test comments table
	properties.Property("Feature: go-sprint-management, Property 12: Comment timestamps are stored in UTC",
		prop.ForAll(
			func(createdAt time.Time) bool {
				ctx := context.Background()

				// First create a work item
				workItemID := uuid.New()
				reporterID := uuid.New()
				_, err := db.ExecContext(ctx, `
					INSERT INTO sprint_management.work_items 
					(id, type, title, description, status, priority, reporter_id)
					VALUES ($1, 'story', 'Test Item', 'Test Description', 'todo', 'medium', $2)
				`, workItemID, reporterID)

				if err != nil {
					t.Logf("failed to insert work item: %v", err)
					return false
				}

				// Create a comment with a specific timestamp
				commentID := uuid.New()
				authorID := uuid.New()

				_, err = db.ExecContext(ctx, `
					INSERT INTO sprint_management.comments 
					(id, work_item_id, author_id, content, created_at, updated_at)
					VALUES ($1, $2, $3, 'Test comment', $4, $4)
				`, commentID, workItemID, authorID, createdAt)

				if err != nil {
					t.Logf("failed to insert comment: %v", err)
					return false
				}

				// Retrieve the comment and check timezone
				var retrievedCreatedAt time.Time
				err = db.QueryRowContext(ctx, `
					SELECT created_at FROM sprint_management.comments WHERE id = $1
				`, commentID).Scan(&retrievedCreatedAt)

				if err != nil {
					t.Logf("failed to retrieve comment: %v", err)
					return false
				}

				// Verify the timestamp is in UTC (PostgreSQL may return Etc/UTC which is equivalent)
				loc := retrievedCreatedAt.Location()
				if loc.String() != "UTC" && loc.String() != "Etc/UTC" {
					t.Logf("timestamp not in UTC: got %v, location: %v", retrievedCreatedAt, loc)
					return false
				}

				// Verify the timestamp value is preserved
				diff := retrievedCreatedAt.Sub(createdAt.UTC())
				if diff < -time.Second || diff > time.Second {
					t.Logf("timestamp value not preserved: expected %v, got %v, diff: %v",
						createdAt.UTC(), retrievedCreatedAt, diff)
					return false
				}

				return true
			},
			genTimestamp,
		))

	// Test activity_logs table
	properties.Property("Feature: go-sprint-management, Property 12: Activity log timestamps are stored in UTC",
		prop.ForAll(
			func(createdAt time.Time) bool {
				ctx := context.Background()

				// Create an activity log with a specific timestamp
				logID := uuid.New()
				entityID := uuid.New()
				userID := uuid.New()

				_, err := db.ExecContext(ctx, `
					INSERT INTO sprint_management.activity_logs 
					(id, entity_type, entity_id, action, user_id, created_at)
					VALUES ($1, 'work_item', $2, 'created', $3, $4)
				`, logID, entityID, userID, createdAt)

				if err != nil {
					t.Logf("failed to insert activity log: %v", err)
					return false
				}

				// Retrieve the activity log and check timezone
				var retrievedCreatedAt time.Time
				err = db.QueryRowContext(ctx, `
					SELECT created_at FROM sprint_management.activity_logs WHERE id = $1
				`, logID).Scan(&retrievedCreatedAt)

				if err != nil {
					t.Logf("failed to retrieve activity log: %v", err)
					return false
				}

				// Verify the timestamp is in UTC (PostgreSQL may return Etc/UTC which is equivalent)
				loc := retrievedCreatedAt.Location()
				if loc.String() != "UTC" && loc.String() != "Etc/UTC" {
					t.Logf("timestamp not in UTC: got %v, location: %v", retrievedCreatedAt, loc)
					return false
				}

				// Verify the timestamp value is preserved
				diff := retrievedCreatedAt.Sub(createdAt.UTC())
				if diff < -time.Second || diff > time.Second {
					t.Logf("timestamp value not preserved: expected %v, got %v, diff: %v",
						createdAt.UTC(), retrievedCreatedAt, diff)
					return false
				}

				return true
			},
			genTimestamp,
		))

	// Test auth.users table
	properties.Property("Feature: go-sprint-management, Property 12: User timestamps are stored in UTC",
		prop.ForAll(
			func(createdAt time.Time) bool {
				ctx := context.Background()

				// Create a user with a specific timestamp
				userID := uuid.New()
				keycloakID := fmt.Sprintf("keycloak-%s", uuid.New().String())
				username := fmt.Sprintf("user-%s", uuid.New().String())
				email := fmt.Sprintf("%s@example.com", username)

				_, err := db.ExecContext(ctx, `
					INSERT INTO auth.users 
					(id, keycloak_id, username, email, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $5)
				`, userID, keycloakID, username, email, createdAt)

				if err != nil {
					t.Logf("failed to insert user: %v", err)
					return false
				}

				// Retrieve the user and check timezone
				var retrievedCreatedAt time.Time
				err = db.QueryRowContext(ctx, `
					SELECT created_at FROM auth.users WHERE id = $1
				`, userID).Scan(&retrievedCreatedAt)

				if err != nil {
					t.Logf("failed to retrieve user: %v", err)
					return false
				}

				// Verify the timestamp is in UTC (PostgreSQL may return Etc/UTC which is equivalent)
				loc := retrievedCreatedAt.Location()
				if loc.String() != "UTC" && loc.String() != "Etc/UTC" {
					t.Logf("timestamp not in UTC: got %v, location: %v", retrievedCreatedAt, loc)
					return false
				}

				// Verify the timestamp value is preserved
				diff := retrievedCreatedAt.Sub(createdAt.UTC())
				if diff < -time.Second || diff > time.Second {
					t.Logf("timestamp value not preserved: expected %v, got %v, diff: %v",
						createdAt.UTC(), retrievedCreatedAt, diff)
					return false
				}

				return true
			},
			genTimestamp,
		))

	properties.TestingRun(t)
}
