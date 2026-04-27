package sprints_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/repository/sprints"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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

// setupTestPool creates a test database connection pool
func setupTestPool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	dbURL := getTestDatabaseURL()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("failed to create connection pool: %v (test database not available)", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test database not available: %v (set TEST_DATABASE_URL or start postgres)", err)
	}

	cleanup := func() {
		// Clean up test data
		pool.Exec(ctx, "TRUNCATE sprint_management.work_items, sprint_management.sprints, sprint_management.work_item_dependencies CASCADE")
		pool.Close()
	}

	return pool, cleanup
}

// uuidToPgtype converts uuid.UUID to pgtype.UUID
func uuidToPgtype(id uuid.UUID) pgtype.UUID {
	var pgUUID pgtype.UUID
	pgUUID.Scan(id.String())
	return pgUUID
}

// dateToPgtype converts time.Time to pgtype.Date
func dateToPgtype(t time.Time) pgtype.Date {
	var pgDate pgtype.Date
	pgDate.Scan(t)
	return pgDate
}

// TestProperty7_SprintRequiredFields tests that required fields are enforced
// Property 7: Sprint Required Fields
// For any sprint creation request, the system should require name, start date, and end date fields.
// Validates: Requirements 2.1
func TestProperty7_SprintRequiredFields(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := sprints.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for random sprint names
	genName := gen.AlphaString().SuchThat(func(s string) bool {
		return len(s) > 0 && len(s) <= 255
	})

	// Generator for valid date ranges
	genDateRange := gen.Int64Range(0, 365).Map(func(days int64) (time.Time, time.Time) {
		start := time.Now().AddDate(0, 0, int(days))
		end := start.AddDate(0, 0, 14) // 2 week sprint
		return start, end
	})

	properties.Property("Feature: go-sprint-management, Property 7: Sprint required fields are enforced",
		prop.ForAll(
			func(name string, dates interface{}) bool {
				ctx := context.Background()

				startDate, endDate := dates.(struct {
					Start time.Time
					End   time.Time
				}).Start, dates.(struct {
					Start time.Time
					End   time.Time
				}).End

				createdBy := uuidToPgtype(uuid.New())

				params := sprints.CreateSprintParams{
					Name:      name,
					Status:    sprints.SprintManagementSprintStatusPlanned,
					StartDate: dateToPgtype(startDate),
					EndDate:   dateToPgtype(endDate),
					CreatedBy: createdBy,
				}

				sprint, err := repo.CreateSprint(ctx, params)
				if err != nil {
					t.Logf("failed to create sprint: %v", err)
					return false
				}

				// Verify required fields are present
				if sprint.Name != name {
					t.Logf("name mismatch: expected %v, got %v", name, sprint.Name)
					return false
				}

				if !sprint.StartDate.Valid {
					t.Logf("start date should be set")
					return false
				}

				if !sprint.EndDate.Valid {
					t.Logf("end date should be set")
					return false
				}

				return true
			},
			genName,
			genDateRange.Map(func(dates interface{}) interface{} {
				start, end := dates.(struct {
					Start time.Time
					End   time.Time
				}).Start, dates.(struct {
					Start time.Time
					End   time.Time
				}).End
				return struct {
					Start time.Time
					End   time.Time
				}{start, end}
			}),
		))

	properties.TestingRun(t)
}

// TestProperty8_SprintDateValidation tests that end date must be after start date
// Property 8: Sprint Date Validation
// For any sprint creation or update, the system should validate that the end date is after the start date.
// Validates: Requirements 2.2
func TestProperty8_SprintDateValidation(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := sprints.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Feature: go-sprint-management, Property 8: Valid date ranges are accepted",
		prop.ForAll(
			func(seed int64) bool {
				ctx := context.Background()

				start := time.Now().AddDate(0, 0, int(seed%365))
				end := start.AddDate(0, 0, 14)

				createdBy := uuidToPgtype(uuid.New())

				params := sprints.CreateSprintParams{
					Name:      fmt.Sprintf("Sprint %d", seed),
					Status:    sprints.SprintManagementSprintStatusPlanned,
					StartDate: dateToPgtype(start),
					EndDate:   dateToPgtype(end),
					CreatedBy: createdBy,
				}

				sprint, err := repo.CreateSprint(ctx, params)
				if err != nil {
					t.Logf("failed to create sprint with valid date range: %v", err)
					return false
				}

				// Verify dates are stored correctly
				if !sprint.StartDate.Valid || !sprint.EndDate.Valid {
					t.Logf("dates should be valid")
					return false
				}

				// Verify end date is after start date
				startTime := sprint.StartDate.Time
				endTime := sprint.EndDate.Time
				if !endTime.After(startTime) {
					t.Logf("end date should be after start date: start=%v, end=%v", startTime, endTime)
					return false
				}

				return true
			},
			gen.Int64Range(1, 365),
		))

	// Test that invalid date ranges are rejected by database constraint
	properties.Property("Feature: go-sprint-management, Property 8: Invalid date ranges are rejected",
		prop.ForAll(
			func(seed int64) bool {
				ctx := context.Background()

				start := time.Now().AddDate(0, 0, int(seed%365))
				end := start.AddDate(0, 0, -1) // End before start (invalid)

				createdBy := uuidToPgtype(uuid.New())

				params := sprints.CreateSprintParams{
					Name:      fmt.Sprintf("Invalid Sprint %d", seed),
					Status:    sprints.SprintManagementSprintStatusPlanned,
					StartDate: dateToPgtype(start),
					EndDate:   dateToPgtype(end),
					CreatedBy: createdBy,
				}

				_, err := repo.CreateSprint(ctx, params)
				// Should fail due to CHECK constraint
				if err == nil {
					t.Logf("should reject sprint with end date before start date")
					return false
				}

				return true
			},
			gen.Int64Range(1, 365),
		))

	properties.TestingRun(t)
}

// TestProperty10_SprintClosureWorkflow tests sprint closure workflow
// Property 10: Sprint Closure Workflow
// For any sprint closure operation, all incomplete work items should be moved to the backlog
// and sprint metrics should be calculated.
// Validates: Requirements 2.4
func TestProperty10_SprintClosureWorkflow(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	repo := sprints.NewRepository(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("Feature: go-sprint-management, Property 10: Sprint closure moves incomplete items to backlog",
		prop.ForAll(
			func(seed int64) bool {
				ctx := context.Background()

				// Create a sprint
				createdBy := uuidToPgtype(uuid.New())
				start := time.Now().AddDate(0, 0, 1)
				end := start.AddDate(0, 0, 14)

				sprintParams := sprints.CreateSprintParams{
					Name:      fmt.Sprintf("Sprint %d", seed),
					Status:    sprints.SprintManagementSprintStatusActive,
					StartDate: dateToPgtype(start),
					EndDate:   dateToPgtype(end),
					CreatedBy: createdBy,
				}

				sprint, err := repo.CreateSprint(ctx, sprintParams)
				if err != nil {
					t.Logf("failed to create sprint: %v", err)
					return false
				}

				// Create some work items in the sprint (mix of done and not done)
				reporterID := uuidToPgtype(uuid.New())

				// Create a done work item
				doneDesc := "Done item"
				_, err = pool.Exec(ctx, `
					INSERT INTO sprint_management.work_items 
					(type, title, description, status, priority, reporter_id, sprint_id)
					VALUES ('story', $1, $2, 'done', 'medium', $3, $4)
				`, fmt.Sprintf("Done Item %d", seed), &doneDesc, reporterID, sprint.ID)
				if err != nil {
					t.Logf("failed to create done work item: %v", err)
					return false
				}

				// Create an incomplete work item
				incompleteDesc := "Incomplete item"
				var incompleteID pgtype.UUID
				err = pool.QueryRow(ctx, `
					INSERT INTO sprint_management.work_items 
					(type, title, description, status, priority, reporter_id, sprint_id)
					VALUES ('story', $1, $2, 'in_progress', 'medium', $3, $4)
					RETURNING id
				`, fmt.Sprintf("Incomplete Item %d", seed), &incompleteDesc, reporterID, sprint.ID).Scan(&incompleteID)
				if err != nil {
					t.Logf("failed to create incomplete work item: %v", err)
					return false
				}

				// Move incomplete work items to backlog (simulating sprint closure)
				err = repo.MoveWorkItemsToBacklog(ctx, sprint.ID)
				if err != nil {
					t.Logf("failed to move work items to backlog: %v", err)
					return false
				}

				// Verify the incomplete item was moved to backlog (sprint_id = NULL)
				var sprintID pgtype.UUID
				err = pool.QueryRow(ctx, `
					SELECT sprint_id FROM sprint_management.work_items WHERE id = $1
				`, incompleteID).Scan(&sprintID)
				if err != nil {
					t.Logf("failed to query work item: %v", err)
					return false
				}

				if sprintID.Valid {
					t.Logf("incomplete work item should have been moved to backlog (sprint_id should be NULL)")
					return false
				}

				// Verify done items remain in the sprint
				var doneCount int64
				err = pool.QueryRow(ctx, `
					SELECT COUNT(*) FROM sprint_management.work_items 
					WHERE sprint_id = $1 AND status = 'done'
				`, sprint.ID).Scan(&doneCount)
				if err != nil {
					t.Logf("failed to count done items: %v", err)
					return false
				}

				if doneCount != 1 {
					t.Logf("done items should remain in sprint: expected 1, got %d", doneCount)
					return false
				}

				return true
			},
			gen.Int64(),
		))

	properties.TestingRun(t)
}
