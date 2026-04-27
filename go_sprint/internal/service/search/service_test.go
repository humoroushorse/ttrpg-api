package search_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/repository/workitems"
	"github.com/humoroushorse/go_sprint/internal/service/search"
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

// createTestWorkItem creates a work item for testing
func createTestWorkItem(ctx context.Context, pool *pgxpool.Pool, title, description string, workItemType workitems.SprintManagementWorkItemType) (workitems.SprintManagementWorkItem, error) {
	repo := workitems.NewRepository(pool, pool)

	reporterID := uuidToPgtype(uuid.New())

	return repo.CreateWorkItem(ctx, workitems.CreateWorkItemParams{
		Type:        workItemType,
		Title:       title,
		Description: &description,
		Status:      workitems.SprintManagementWorkItemStatusTodo,
		Priority:    workitems.SprintManagementPriorityLevelMedium,
		ReporterID:  reporterID,
	})
}

// getDescription safely gets description string
func getDescription(desc *string) string {
	if desc == nil {
		return ""
	}
	return *desc
}

// TestSearchAccuracy tests that search returns relevant results
// Property: Search Accuracy
// For any search query, all returned results should contain the search terms in either title or description
// Validates: Requirements 31.1, 31.3
func TestSearchAccuracy(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	svc := search.NewService(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generator for search terms
	genSearchTerm := gen.OneConstOf(
		"authentication",
		"database",
		"user",
		"sprint",
		"bug",
		"feature",
		"login",
		"password",
		"api",
		"test",
	)

	properties.Property("Feature: go-sprint-management, Property: Search returns relevant results containing search terms",
		prop.ForAll(
			func(searchTerm string) bool {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Create test work items with the search term
				title1 := fmt.Sprintf("Implement %s functionality", searchTerm)
				desc1 := fmt.Sprintf("Add %s support to the system", searchTerm)

				title2 := fmt.Sprintf("Fix %s issue", searchTerm)
				desc2 := "General bug fix"

				title3 := "Unrelated work item"
				desc3 := "This should not match"

				_, err := createTestWorkItem(ctx, pool, title1, desc1, workitems.SprintManagementWorkItemTypeStory)
				if err != nil {
					t.Logf("Failed to create test work item 1: %v", err)
					return false
				}

				_, err = createTestWorkItem(ctx, pool, title2, desc2, workitems.SprintManagementWorkItemTypeDefect)
				if err != nil {
					t.Logf("Failed to create test work item 2: %v", err)
					return false
				}

				_, err = createTestWorkItem(ctx, pool, title3, desc3, workitems.SprintManagementWorkItemTypeStory)
				if err != nil {
					t.Logf("Failed to create test work item 3: %v", err)
					return false
				}

				// Perform search
				results, err := svc.Search(ctx, search.SearchRequest{
					Query:  searchTerm,
					Limit:  100,
					Offset: 0,
				})

				if err != nil {
					t.Logf("Search failed: %v", err)
					return false
				}

				// Verify all results contain the search term
				searchTermLower := strings.ToLower(searchTerm)
				for _, result := range results {
					titleLower := strings.ToLower(result.WorkItem.Title)
					descLower := strings.ToLower(getDescription(result.WorkItem.Description))

					if !strings.Contains(titleLower, searchTermLower) && !strings.Contains(descLower, searchTermLower) {
						t.Logf("Result does not contain search term '%s': title='%s', desc='%s'",
							searchTerm, result.WorkItem.Title, getDescription(result.WorkItem.Description))
						return false
					}
				}

				// Verify we got at least the 2 matching items
				if len(results) < 2 {
					t.Logf("Expected at least 2 results, got %d", len(results))
					return false
				}

				return true
			},
			genSearchTerm,
		))

	properties.TestingRun(t)
}

// TestSearchRanking tests that search results are ranked by relevance
// Property: Search Ranking
// For any search query, results should be ordered by relevance with higher ranks appearing first
// Validates: Requirements 31.3, 31.5
func TestSearchRanking(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	svc := search.NewService(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	genSearchTerm := gen.OneConstOf(
		"authentication",
		"database",
		"user",
		"sprint",
	)

	properties.Property("Feature: go-sprint-management, Property: Search results are ranked by relevance",
		prop.ForAll(
			func(searchTerm string) bool {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Create work items with varying relevance
				// High relevance: term in both title and description
				title1 := fmt.Sprintf("%s %s implementation", searchTerm, searchTerm)
				desc1 := fmt.Sprintf("Complete %s system with %s support", searchTerm, searchTerm)

				// Medium relevance: term only in title
				title2 := fmt.Sprintf("Implement %s", searchTerm)
				desc2 := "General implementation"

				// Low relevance: term only in description
				title3 := "General work item"
				desc3 := fmt.Sprintf("This involves %s", searchTerm)

				_, err := createTestWorkItem(ctx, pool, title1, desc1, workitems.SprintManagementWorkItemTypeStory)
				if err != nil {
					t.Logf("Failed to create test work item 1: %v", err)
					return false
				}

				_, err = createTestWorkItem(ctx, pool, title2, desc2, workitems.SprintManagementWorkItemTypeStory)
				if err != nil {
					t.Logf("Failed to create test work item 2: %v", err)
					return false
				}

				_, err = createTestWorkItem(ctx, pool, title3, desc3, workitems.SprintManagementWorkItemTypeStory)
				if err != nil {
					t.Logf("Failed to create test work item 3: %v", err)
					return false
				}

				// Perform search
				results, err := svc.Search(ctx, search.SearchRequest{
					Query:  searchTerm,
					Limit:  100,
					Offset: 0,
				})

				if err != nil {
					t.Logf("Search failed: %v", err)
					return false
				}

				if len(results) < 3 {
					t.Logf("Expected at least 3 results, got %d", len(results))
					return false
				}

				// Verify ranking is in descending order
				for i := 0; i < len(results)-1; i++ {
					if results[i].Rank == nil || results[i+1].Rank == nil {
						continue
					}

					// Convert ranks to float64 for comparison
					rank1, ok1 := results[i].Rank.(float64)
					rank2, ok2 := results[i+1].Rank.(float64)

					if !ok1 || !ok2 {
						continue
					}

					if rank1 < rank2 {
						t.Logf("Ranking order violated: rank[%d]=%.4f < rank[%d]=%.4f", i, rank1, i+1, rank2)
						return false
					}
				}

				return true
			},
			genSearchTerm,
		))

	properties.TestingRun(t)
}

// TestFilterCombinationLogic tests that multiple filters work correctly together
// Property: Filter Combination Logic
// For any combination of filters, all returned results should match ALL specified filter criteria
// Validates: Requirements 31.2, 31.4
func TestFilterCombinationLogic(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	svc := search.NewService(pool, pool)

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Generators for filter values
	genType := gen.OneConstOf(
		workitems.SprintManagementWorkItemTypeEpic,
		workitems.SprintManagementWorkItemTypeStory,
		workitems.SprintManagementWorkItemTypeDefect,
	)

	genStatus := gen.OneConstOf(
		workitems.SprintManagementWorkItemStatusTodo,
		workitems.SprintManagementWorkItemStatusInProgress,
		workitems.SprintManagementWorkItemStatusDone,
	)

	genPriority := gen.OneConstOf(
		workitems.SprintManagementPriorityLevelLow,
		workitems.SprintManagementPriorityLevelMedium,
		workitems.SprintManagementPriorityLevelHigh,
		workitems.SprintManagementPriorityLevelCritical,
	)

	properties.Property("Feature: go-sprint-management, Property: Filter combinations return only matching results",
		prop.ForAll(
			func(filterType workitems.SprintManagementWorkItemType, filterStatus workitems.SprintManagementWorkItemStatus, filterPriority workitems.SprintManagementPriorityLevel) bool {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Create work items with different combinations
				repo := workitems.NewRepository(pool, pool)
				reporterID := uuidToPgtype(uuid.New())

				desc1 := "This matches all filters"
				// Create matching work item
				matchingItem, err := repo.CreateWorkItem(ctx, workitems.CreateWorkItemParams{
					Type:        filterType,
					Title:       "Matching work item",
					Description: &desc1,
					Status:      filterStatus,
					Priority:    filterPriority,
					ReporterID:  reporterID,
				})
				if err != nil {
					t.Logf("Failed to create matching work item: %v", err)
					return false
				}

				// Create non-matching work items (different type)
				differentType := workitems.SprintManagementWorkItemTypeStory
				if filterType == workitems.SprintManagementWorkItemTypeStory {
					differentType = workitems.SprintManagementWorkItemTypeEpic
				}

				desc2 := "Different type"
				_, err = repo.CreateWorkItem(ctx, workitems.CreateWorkItemParams{
					Type:        differentType,
					Title:       "Non-matching work item",
					Description: &desc2,
					Status:      filterStatus,
					Priority:    filterPriority,
					ReporterID:  reporterID,
				})
				if err != nil {
					t.Logf("Failed to create non-matching work item: %v", err)
					return false
				}

				// Perform filtered search
				results, err := svc.Search(ctx, search.SearchRequest{
					Query:    "", // Empty query to test filter-only
					Type:     &filterType,
					Status:   &filterStatus,
					Priority: &filterPriority,
					Limit:    100,
					Offset:   0,
				})

				if err != nil {
					t.Logf("Filter search failed: %v", err)
					return false
				}

				// Verify all results match the filters
				for _, result := range results {
					if result.WorkItem.Type != filterType {
						t.Logf("Result type mismatch: expected %s, got %s", filterType, result.WorkItem.Type)
						return false
					}
					if result.WorkItem.Status != filterStatus {
						t.Logf("Result status mismatch: expected %s, got %s", filterStatus, result.WorkItem.Status)
						return false
					}
					if result.WorkItem.Priority != filterPriority {
						t.Logf("Result priority mismatch: expected %s, got %s", filterPriority, result.WorkItem.Priority)
						return false
					}
				}

				// Verify we found at least the matching item
				found := false
				for _, result := range results {
					if result.WorkItem.ID == matchingItem.ID {
						found = true
						break
					}
				}

				if !found {
					t.Logf("Matching work item not found in results")
					return false
				}

				return true
			},
			genType,
			genStatus,
			genPriority,
		))

	properties.TestingRun(t)
}

// TestBooleanSearchOperators tests that boolean search operators work correctly
// Property: Boolean Search Operators
// For any boolean search query with AND/OR operators, results should match the boolean logic
// Validates: Requirements 31.3
func TestBooleanSearchOperators(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	svc := search.NewService(pool, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test work items
	_, err := createTestWorkItem(ctx, pool, "User authentication system", "Implement user login", workitems.SprintManagementWorkItemTypeStory)
	if err != nil {
		t.Fatalf("Failed to create test work item: %v", err)
	}

	_, err = createTestWorkItem(ctx, pool, "Database migration", "Update database schema", workitems.SprintManagementWorkItemTypeStory)
	if err != nil {
		t.Fatalf("Failed to create test work item: %v", err)
	}

	_, err = createTestWorkItem(ctx, pool, "User profile page", "Create user profile UI", workitems.SprintManagementWorkItemTypeStory)
	if err != nil {
		t.Fatalf("Failed to create test work item: %v", err)
	}

	// Test AND operator
	t.Run("AND operator", func(t *testing.T) {
		results, err := svc.Search(ctx, search.SearchRequest{
			Query:      "user AND authentication",
			UseBoolean: true,
			Limit:      100,
			Offset:     0,
		})

		if err != nil {
			t.Fatalf("Boolean search failed: %v", err)
		}

		// Should only return items containing both "user" and "authentication"
		for _, result := range results {
			titleLower := strings.ToLower(result.WorkItem.Title)
			descLower := strings.ToLower(getDescription(result.WorkItem.Description))
			combined := titleLower + " " + descLower

			if !strings.Contains(combined, "user") || !strings.Contains(combined, "authentication") {
				t.Errorf("Result does not contain both terms: %s", result.WorkItem.Title)
			}
		}
	})

	// Test OR operator
	t.Run("OR operator", func(t *testing.T) {
		results, err := svc.Search(ctx, search.SearchRequest{
			Query:      "authentication OR database",
			UseBoolean: true,
			Limit:      100,
			Offset:     0,
		})

		if err != nil {
			t.Fatalf("Boolean search failed: %v", err)
		}

		// Should return items containing either "authentication" or "database"
		if len(results) < 2 {
			t.Errorf("Expected at least 2 results for OR query, got %d", len(results))
		}

		for _, result := range results {
			titleLower := strings.ToLower(result.WorkItem.Title)
			descLower := strings.ToLower(getDescription(result.WorkItem.Description))
			combined := titleLower + " " + descLower

			if !strings.Contains(combined, "authentication") && !strings.Contains(combined, "database") {
				t.Errorf("Result does not contain either term: %s", result.WorkItem.Title)
			}
		}
	})
}

// TestFieldSpecificSearch tests that field-specific search works correctly
// Property: Field-Specific Search
// For any field-specific search, results should only match the specified field
// Validates: Requirements 31.3
func TestFieldSpecificSearch(t *testing.T) {
	pool, cleanup := setupTestPool(t)
	defer cleanup()

	svc := search.NewService(pool, pool)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create test work items
	_, err := createTestWorkItem(ctx, pool, "Authentication feature", "Implement database connection", workitems.SprintManagementWorkItemTypeStory)
	if err != nil {
		t.Fatalf("Failed to create test work item: %v", err)
	}

	_, err = createTestWorkItem(ctx, pool, "Database migration", "Update authentication system", workitems.SprintManagementWorkItemTypeStory)
	if err != nil {
		t.Fatalf("Failed to create test work item: %v", err)
	}

	// Test title-only search
	t.Run("Title-only search", func(t *testing.T) {
		results, err := svc.Search(ctx, search.SearchRequest{
			Query:  "authentication",
			Field:  "title",
			Limit:  100,
			Offset: 0,
		})

		if err != nil {
			t.Fatalf("Title search failed: %v", err)
		}

		// Should only return items with "authentication" in title
		for _, result := range results {
			titleLower := strings.ToLower(result.WorkItem.Title)
			if !strings.Contains(titleLower, "authentication") {
				t.Errorf("Result title does not contain search term: %s", result.WorkItem.Title)
			}
		}

		// Should find the first item but not the second
		if len(results) != 1 {
			t.Errorf("Expected 1 result for title search, got %d", len(results))
		}
	})

	// Test description-only search
	t.Run("Description-only search", func(t *testing.T) {
		results, err := svc.Search(ctx, search.SearchRequest{
			Query:  "authentication",
			Field:  "description",
			Limit:  100,
			Offset: 0,
		})

		if err != nil {
			t.Fatalf("Description search failed: %v", err)
		}

		// Should only return items with "authentication" in description
		for _, result := range results {
			descLower := strings.ToLower(getDescription(result.WorkItem.Description))
			if !strings.Contains(descLower, "authentication") {
				t.Errorf("Result description does not contain search term: %s", getDescription(result.WorkItem.Description))
			}
		}

		// Should find the second item but not the first
		if len(results) != 1 {
			t.Errorf("Expected 1 result for description search, got %d", len(results))
		}
	})
}
