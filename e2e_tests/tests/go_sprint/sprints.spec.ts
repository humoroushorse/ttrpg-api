/**
 * Sprints endpoint tests for go_sprint service
 * Tests sprint list, filtering, pagination, and CRUD operations
 * 
 * @tags @go_sprint
 * Requirements: 11.1, 11.2, 11.3, 11.4, 11.5
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loginAsUser } from '../../lib/helpers/auth';
import { TestDataHelper, Sprint } from '../../lib/helpers/test-data';

// Response types
interface SprintListResponse {
  items: Sprint[];
  pagination: {
    next_cursor?: string | null;
    has_more: boolean;
    total_count?: number | null;
  };
}

interface ErrorResponse {
  error: {
    code: string;
    message: string;
    details?: any;
  };
  trace_id?: string;
  timestamp?: string;
}

test.describe('Sprints List Endpoint Tests @go_sprint', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;
  let testDataHelper: TestDataHelper;

  test.beforeEach(async () => {
    config = getConfig();
    apiClient = new APIClient(config, false);
    
    // Login and set auth token
    const user = await loginAsUser(apiClient);
    apiClient.setAuthToken(user.token);
    
    testDataHelper = new TestDataHelper(apiClient);
  });

  test.afterEach(async () => {
    // Cleanup created test data
    await testDataHelper.cleanupAll();
  });

  test('should list sprints without filters', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create some test sprints
    await testDataHelper.createSprints(3);
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await apiClient.get<SprintListResponse>(url);

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify pagination structure
    expect(response.data.pagination).toHaveProperty('has_more');
    expect(typeof response.data.pagination.has_more).toBe('boolean');

    // Verify at least our created sprints are present
    expect(response.data.items.length).toBeGreaterThanOrEqual(3);

    // Verify sprint structure
    if (response.data.items.length > 0) {
      const sprint = response.data.items[0];
      expect(sprint).toHaveProperty('id');
      expect(sprint).toHaveProperty('name');
      expect(sprint).toHaveProperty('status');
      expect(sprint).toHaveProperty('start_date');
      expect(sprint).toHaveProperty('end_date');
      expect(sprint).toHaveProperty('committed_points');
      expect(sprint).toHaveProperty('completed_points');
      expect(sprint).toHaveProperty('created_by');
      expect(sprint).toHaveProperty('created_at');
      expect(sprint).toHaveProperty('updated_at');
    }
  });

  test('should filter sprints by status', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Create sprints with different statuses
    // Note: Sprints are created with 'planned' status by default
    await testDataHelper.createSprint({ name: 'Planned Sprint 1' });
    await testDataHelper.createSprint({ name: 'Planned Sprint 2' });
    await testDataHelper.createSprint({ name: 'Planned Sprint 3' });
    
    // Filter by status 'planned'
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await apiClient.get<SprintListResponse>(url, { params: { status: 'planned' } });

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify at least our created sprints are present
    expect(response.data.items.length).toBeGreaterThanOrEqual(3);
    
    // Verify all returned sprints have status 'planned'
    const plannedSprints = response.data.items.filter((sprint: Sprint) => sprint.status === 'planned');
    expect(plannedSprints.length).toBe(response.data.items.length);
  });

  test('should paginate sprints with limit parameter', async () => {
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create multiple sprints
    await testDataHelper.createSprints(5);
    
    // Request with limit of 2
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await apiClient.get<SprintListResponse>(url, { params: { limit: 2 } });

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');

    // Verify limit is respected (should return at most 2 items)
    expect(response.data.items.length).toBeLessThanOrEqual(2);

    // Verify pagination metadata
    expect(response.data.pagination).toHaveProperty('has_more');
    
    // If there are more sprints, has_more should be true
    if (response.data.items.length === 2) {
      // We created 5 sprints, so there should be more
      expect(response.data.pagination.has_more).toBe(true);
    }
  });

  test('should support cursor-based pagination', async () => {
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create multiple sprints
    await testDataHelper.createSprints(5);
    
    // Get first page
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const firstPageResponse = await apiClient.get<SprintListResponse>(url, { params: { limit: 2 } });

    expect(firstPageResponse.status).toBe(200);
    expect(firstPageResponse.data.items.length).toBeGreaterThan(0);

    // If there's a next cursor, fetch the next page
    if (firstPageResponse.data.pagination.next_cursor) {
      const secondPageResponse = await apiClient.get<SprintListResponse>(url, {
        params: {
          limit: 2,
          cursor: firstPageResponse.data.pagination.next_cursor,
        },
      });

      expect(secondPageResponse.status).toBe(200);
      expect(secondPageResponse.data).toHaveProperty('items');
      
      // Verify we got different sprints (no duplicates)
      const firstPageIds = firstPageResponse.data.items.map((sprint: Sprint) => sprint.id);
      const secondPageIds = secondPageResponse.data.items.map((sprint: Sprint) => sprint.id);
      
      const duplicates = firstPageIds.filter((id: string) => secondPageIds.includes(id));
      expect(duplicates.length).toBe(0);
    }
  });

  test('should validate response schema for sprints list', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    await testDataHelper.createSprint();
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await apiClient.get<SprintListResponse>(url);

    expect(response.status).toBe(200);

    // Verify top-level structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');

    // Verify items array
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify each sprint has required fields
    response.data.items.forEach((sprint: any) => {
      // Required fields
      expect(sprint).toHaveProperty('id');
      expect(sprint).toHaveProperty('name');
      expect(sprint).toHaveProperty('status');
      expect(sprint).toHaveProperty('start_date');
      expect(sprint).toHaveProperty('end_date');
      expect(sprint).toHaveProperty('committed_points');
      expect(sprint).toHaveProperty('completed_points');
      expect(sprint).toHaveProperty('created_by');
      expect(sprint).toHaveProperty('created_at');
      expect(sprint).toHaveProperty('updated_at');

      // Verify types
      expect(typeof sprint.id).toBe('string');
      expect(typeof sprint.name).toBe('string');
      expect(typeof sprint.status).toBe('string');
      expect(typeof sprint.start_date).toBe('string');
      expect(typeof sprint.end_date).toBe('string');
      expect(typeof sprint.committed_points).toBe('number');
      expect(typeof sprint.completed_points).toBe('number');
      expect(typeof sprint.created_by).toBe('string');
      expect(typeof sprint.created_at).toBe('string');
      expect(typeof sprint.updated_at).toBe('string');

      // Verify enum values
      expect(['planned', 'active', 'completed', 'cancelled']).toContain(sprint.status);
    });

    // Verify pagination structure
    expect(response.data.pagination).toHaveProperty('has_more');
    expect(typeof response.data.pagination.has_more).toBe('boolean');
  });

  test('should handle empty results gracefully', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Query with a filter that should return no results
    // Note: Using a status filter that won't match any of our test data
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await apiClient.get<SprintListResponse>(url, {
      params: { status: 'cancelled' },
    });

    expect(response.status).toBe(200);

    // Verify response structure even with empty results
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');
    expect(Array.isArray(response.data.items)).toBe(true);
    // Note: May have items if there are cancelled sprints in the database
    expect(response.data.pagination.has_more).toBeDefined();
  });
});

test.describe('Sprints CRUD Endpoint Tests @go_sprint', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;
  let testDataHelper: TestDataHelper;

  test.beforeEach(async () => {
    config = getConfig();
    apiClient = new APIClient(config, false);
    
    // Login and set auth token
    const user = await loginAsUser(apiClient);
    apiClient.setAuthToken(user.token);
    
    testDataHelper = new TestDataHelper(apiClient);
  });

  test.afterEach(async () => {
    // Cleanup created test data
    await testDataHelper.cleanupAll();
  });

  test('should create sprint with valid payload', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    // Feature: playwright-api-testing, Property 7: Test Data Cleanup
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    
    // Calculate dates (start tomorrow, end in 2 weeks)
    const startDate = new Date();
    startDate.setDate(startDate.getDate() + 1);
    const endDate = new Date(startDate);
    endDate.setDate(endDate.getDate() + 14);
    
    const payload = {
      name: 'Sprint 2024-Q1',
      description: 'First quarter sprint for 2024',
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      capacity_points: 50,
    };

    const response = await apiClient.post<Sprint>(url, payload);

    // Verify response status
    expect(response.status).toBe(201);

    // Verify response contains created sprint
    expect(response.data).toHaveProperty('id');
    expect(response.data.name).toBe(payload.name);
    expect(response.data.description).toBe(payload.description);
    expect(response.data.start_date).toBe(payload.start_date);
    expect(response.data.end_date).toBe(payload.end_date);
    expect(response.data.capacity_points).toBe(payload.capacity_points);

    // Verify auto-generated fields
    expect(response.data).toHaveProperty('status');
    expect(response.data).toHaveProperty('committed_points');
    expect(response.data).toHaveProperty('completed_points');
    expect(response.data).toHaveProperty('created_by');
    expect(response.data).toHaveProperty('created_at');
    expect(response.data).toHaveProperty('updated_at');

    // Track for cleanup
    testDataHelper.getCreatedSprintIds().push(response.data.id);
  });

  test('should fail to create sprint with invalid dates - end before start', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    
    // Create invalid dates (end date before start date)
    const startDate = new Date();
    startDate.setDate(startDate.getDate() + 14);
    const endDate = new Date();
    endDate.setDate(endDate.getDate() + 1);
    
    const payload = {
      name: 'Invalid Sprint',
      description: 'Sprint with invalid dates',
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      capacity_points: 40,
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data.error).toHaveProperty('code');
    expect(response.data.error).toHaveProperty('message');
  });

  test('should fail to create sprint with missing required fields', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const payload = {
      name: 'Incomplete Sprint',
      // Missing start_date and end_date
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data.error).toHaveProperty('code');
    expect(response.data.error).toHaveProperty('message');
  });

  test('should fail to create sprint with past dates', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    
    // Create dates in the past
    const startDate = new Date();
    startDate.setDate(startDate.getDate() - 14);
    const endDate = new Date();
    endDate.setDate(endDate.getDate() - 1);
    
    const payload = {
      name: 'Past Sprint',
      description: 'Sprint with past dates',
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      capacity_points: 40,
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
  });

  test('should get sprint by ID for existing sprint', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Create a sprint first
    const createdSprint = await testDataHelper.createSprint({
      name: 'Test Sprint for Retrieval',
      description: 'This sprint will be retrieved by ID',
    });

    // Get the sprint by ID
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/${createdSprint.id}`;
    const response = await apiClient.get<Sprint>(url);

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response contains the sprint
    expect(response.data.id).toBe(createdSprint.id);
    expect(response.data.name).toBe(createdSprint.name);
    expect(response.data.description).toBe(createdSprint.description);
    expect(response.data.start_date).toBe(createdSprint.start_date);
    expect(response.data.end_date).toBe(createdSprint.end_date);
    expect(response.data.status).toBe(createdSprint.status);
  });

  test('should return 404 for non-existing sprint ID', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    // Use a UUID that doesn't exist
    const nonExistentId = '00000000-0000-0000-0000-000000000000';
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/${nonExistentId}`;
    const response = await apiClient.get<ErrorResponse>(url);

    // Verify response status is 404 Not Found
    expect(response.status).toBe(404);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data.error).toHaveProperty('code');
    expect(response.data.error).toHaveProperty('message');
  });

  test('should validate created sprint matches request payload', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    
    const startDate = new Date();
    startDate.setDate(startDate.getDate() + 7);
    const endDate = new Date(startDate);
    endDate.setDate(endDate.getDate() + 21);
    
    const payload = {
      name: 'Sprint 2024-Q2',
      description: 'Second quarter sprint with extended duration',
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      capacity_points: 60,
    };

    const response = await apiClient.post<Sprint>(url, payload);

    expect(response.status).toBe(201);

    // Verify all payload fields match
    expect(response.data.name).toBe(payload.name);
    expect(response.data.description).toBe(payload.description);
    expect(response.data.start_date).toBe(payload.start_date);
    expect(response.data.end_date).toBe(payload.end_date);
    expect(response.data.capacity_points).toBe(payload.capacity_points);

    // Track for cleanup
    testDataHelper.getCreatedSprintIds().push(response.data.id);
  });

  test('should require authentication for sprints endpoints', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    // Create a new client without authentication
    const unauthClient = new APIClient(config, false);
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    const response = await unauthClient.get<ErrorResponse>(url);

    // Verify response status is 401 Unauthorized
    expect(response.status).toBe(401);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
  });

  test('should create sprint with optional fields', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints`;
    
    const startDate = new Date();
    startDate.setDate(startDate.getDate() + 1);
    const endDate = new Date(startDate);
    endDate.setDate(endDate.getDate() + 14);
    
    const payload = {
      name: 'Minimal Sprint',
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      // Optional fields omitted
    };

    const response = await apiClient.post<Sprint>(url, payload);

    expect(response.status).toBe(201);

    // Verify response
    expect(response.data.name).toBe(payload.name);
    expect(response.data.start_date).toBe(payload.start_date);
    expect(response.data.end_date).toBe(payload.end_date);

    // Track for cleanup
    testDataHelper.getCreatedSprintIds().push(response.data.id);
  });
});
