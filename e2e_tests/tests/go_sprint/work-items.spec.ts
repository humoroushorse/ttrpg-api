/**
 * Work Items endpoint tests for go_sprint service
 * Tests work item list, filtering, pagination, and CRUD operations
 * 
 * @tags @go_sprint
 * Requirements: 10.1, 10.2, 10.3, 10.4, 10.5
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loginAsUser } from '../../lib/helpers/auth';
import { TestDataHelper, WorkItem } from '../../lib/helpers/test-data';

// Response types
interface WorkItemSummary {
  id: string;
  type: string;
  title: string;
  status: string;
  priority: string;
  story_points?: number | null;
}

interface WorkItemListResponse {
  items: WorkItemSummary[];
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

test.describe('Work Items List Endpoint Tests @go_sprint', () => {
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

  test('should list work items without filters', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create some test work items
    await testDataHelper.createWorkItems(3);
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url);

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify pagination structure
    expect(response.data.pagination).toHaveProperty('has_more');
    expect(typeof response.data.pagination.has_more).toBe('boolean');

    // Verify at least our created items are present
    expect(response.data.items.length).toBeGreaterThanOrEqual(3);

    // Verify work item structure (WorkItemSummary schema)
    if (response.data.items.length > 0) {
      const workItem = response.data.items[0];
      expect(workItem).toHaveProperty('id');
      expect(workItem).toHaveProperty('type');
      expect(workItem).toHaveProperty('title');
      expect(workItem).toHaveProperty('status');
      expect(workItem).toHaveProperty('priority');
      // story_points is optional
    }
  });

  test('should filter work items by type', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Create work items of different types
    await testDataHelper.createWorkItem({ type: 'epic', title: 'Test Epic' });
    await testDataHelper.createWorkItem({ type: 'story', title: 'Test Story' });
    await testDataHelper.createWorkItem({ type: 'defect', title: 'Test Defect' });
    
    // Filter by type 'story'
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url, { params: { type: 'story' } });

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify all returned items are of type 'story'
    const storyItems = response.data.items.filter((item: WorkItem) => item.type === 'story');
    expect(storyItems.length).toBeGreaterThanOrEqual(1);
    
    // Verify no items of other types are returned
    const nonStoryItems = response.data.items.filter((item: WorkItem) => item.type !== 'story');
    expect(nonStoryItems.length).toBe(0);
  });

  test('should filter work items by status', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Create work items with different statuses
    const todoItem = await testDataHelper.createWorkItem({ title: 'Todo Item' });
    
    // Note: Status is set to 'todo' by default in the backend
    // We would need to update the item to test other statuses
    
    // Filter by status 'todo'
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url, { params: { status: 'todo' } });

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify at least our created item is present
    expect(response.data.items.length).toBeGreaterThanOrEqual(1);
    
    // Verify all returned items have status 'todo'
    const todoItems = response.data.items.filter((item: WorkItem) => item.status === 'todo');
    expect(todoItems.length).toBe(response.data.items.length);
  });

  test('should paginate work items with limit parameter', async () => {
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create multiple work items
    await testDataHelper.createWorkItems(5);
    
    // Request with limit of 2
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url, { params: { limit: 2 } });

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');

    // Verify limit is respected (should return at most 2 items)
    expect(response.data.items.length).toBeLessThanOrEqual(2);

    // Verify pagination metadata
    expect(response.data.pagination).toHaveProperty('has_more');
    
    // If there are more items, has_more should be true
    if (response.data.items.length === 2) {
      // We created 5 items, so there should be more
      expect(response.data.pagination.has_more).toBe(true);
    }
  });

  test('should support cursor-based pagination', async () => {
    // Feature: playwright-api-testing, Property 10: Pagination Consistency
    
    // Create multiple work items
    await testDataHelper.createWorkItems(5);
    
    // Get first page
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const firstPageResponse = await apiClient.get<WorkItemListResponse>(url, { params: { limit: 2 } });

    expect(firstPageResponse.status).toBe(200);
    expect(firstPageResponse.data.items.length).toBeGreaterThan(0);

    // If there's a next cursor, fetch the next page
    if (firstPageResponse.data.pagination.next_cursor) {
      const secondPageResponse = await apiClient.get<WorkItemListResponse>(url, {
        params: {
          limit: 2,
          cursor: firstPageResponse.data.pagination.next_cursor,
        },
      });

      expect(secondPageResponse.status).toBe(200);
      expect(secondPageResponse.data).toHaveProperty('items');
      
      // Verify we got different items (no duplicates)
      const firstPageIds = firstPageResponse.data.items.map((item: WorkItemSummary) => item.id);
      const secondPageIds = secondPageResponse.data.items.map((item: WorkItemSummary) => item.id);
      
      const duplicates = firstPageIds.filter((id: string) => secondPageIds.includes(id));
      expect(duplicates.length).toBe(0);
    }
  });

  test('should validate response schema for work items list', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    await testDataHelper.createWorkItem();
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url);

    expect(response.status).toBe(200);

    // Verify top-level structure
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');

    // Verify items array
    expect(Array.isArray(response.data.items)).toBe(true);

    // Verify each work item has required fields (WorkItemSummary schema)
    response.data.items.forEach((item: any) => {
      // Required fields
      expect(item).toHaveProperty('id');
      expect(item).toHaveProperty('type');
      expect(item).toHaveProperty('title');
      expect(item).toHaveProperty('status');
      expect(item).toHaveProperty('priority');
      // story_points is optional

      // Verify types
      expect(typeof item.id).toBe('string');
      expect(typeof item.type).toBe('string');
      expect(typeof item.title).toBe('string');
      expect(typeof item.status).toBe('string');
      expect(typeof item.priority).toBe('string');
      if (item.story_points !== null && item.story_points !== undefined) {
        expect(typeof item.story_points).toBe('number');
      }

      // Verify enum values
      expect(['epic', 'story', 'defect']).toContain(item.type);
      expect(['todo', 'in_progress', 'in_review', 'done', 'blocked']).toContain(item.status);
      expect(['low', 'medium', 'high', 'critical']).toContain(item.priority);
    });

    // Verify pagination structure
    expect(response.data.pagination).toHaveProperty('has_more');
    expect(typeof response.data.pagination.has_more).toBe('boolean');
  });

  test('should handle empty results gracefully', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Query with a filter that should return no results
    // Note: Using a status filter that won't match any of our test data
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await apiClient.get<WorkItemListResponse>(url, {
      params: { status: 'blocked' },
    });

    expect(response.status).toBe(200);

    // Verify response structure even with empty results
    expect(response.data).toHaveProperty('items');
    expect(response.data).toHaveProperty('pagination');
    expect(Array.isArray(response.data.items)).toBe(true);
    // Note: May have items if there are blocked work items in the database
    expect(response.data.pagination.has_more).toBeDefined();
  });
});

test.describe('Work Items CRUD Endpoint Tests @go_sprint', () => {
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

  test('should create work item with valid payload', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    // Feature: playwright-api-testing, Property 7: Test Data Cleanup
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'story',
      title: 'Test Story for Creation',
      description: 'This is a test story created via API',
      priority: 'high',
      story_points: 5,
    };

    const response = await apiClient.post<WorkItem>(url, payload);

    // Verify response status
    expect(response.status).toBe(201);

    // Verify response contains created work item
    expect(response.data).toHaveProperty('id');
    expect(response.data.type).toBe(payload.type);
    expect(response.data.title).toBe(payload.title);
    expect(response.data.description).toBe(payload.description);
    expect(response.data.priority).toBe(payload.priority);
    expect(response.data.story_points).toBe(payload.story_points);

    // Verify auto-generated fields
    expect(response.data).toHaveProperty('status');
    expect(response.data).toHaveProperty('reporter_id');
    expect(response.data).toHaveProperty('created_at');
    expect(response.data).toHaveProperty('updated_at');

    // Track for cleanup
    testDataHelper.getCreatedWorkItemIds().push(response.data.id);
  });

  test('should fail to create work item with invalid payload - missing required fields', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'story',
      // Missing title, description, and priority
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data.error).toHaveProperty('code');
    expect(response.data.error).toHaveProperty('message');
  });

  test('should fail to create work item with invalid type', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'invalid_type',
      title: 'Test Story',
      description: 'Test description',
      priority: 'medium',
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
  });

  test('should fail to create work item with empty title', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'story',
      title: '',
      description: 'Test description',
      priority: 'medium',
    };

    const response = await apiClient.post<ErrorResponse>(url, payload);

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
  });

  test('should get work item by ID for existing item', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Create a work item first
    const createdItem = await testDataHelper.createWorkItem({
      title: 'Test Item for Retrieval',
      description: 'This item will be retrieved by ID',
    });

    // Get the work item by ID
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems/${createdItem.id}`;
    const response = await apiClient.get<WorkItem>(url);

    // Verify response status
    expect(response.status).toBe(200);

    // Verify response contains the work item
    expect(response.data.id).toBe(createdItem.id);
    expect(response.data.title).toBe(createdItem.title);
    expect(response.data.description).toBe(createdItem.description);
    expect(response.data.type).toBe(createdItem.type);
    expect(response.data.priority).toBe(createdItem.priority);
  });

  test('should return 404 for non-existing work item ID', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    // Use a UUID that doesn't exist
    const nonExistentId = '00000000-0000-0000-0000-000000000000';
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems/${nonExistentId}`;
    const response = await apiClient.get<ErrorResponse>(url);

    // Verify response status is 404 Not Found
    expect(response.status).toBe(404);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data.error).toHaveProperty('code');
    expect(response.data.error).toHaveProperty('message');
  });

  test('should validate created work item matches request payload', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'defect',
      title: 'Critical Bug in Login',
      description: 'Users cannot login with valid credentials',
      priority: 'critical',
      story_points: 8,
    };

    const response = await apiClient.post<WorkItem>(url, payload);

    expect(response.status).toBe(201);

    // Verify all payload fields match
    expect(response.data.type).toBe(payload.type);
    expect(response.data.title).toBe(payload.title);
    expect(response.data.description).toBe(payload.description);
    expect(response.data.priority).toBe(payload.priority);
    expect(response.data.story_points).toBe(payload.story_points);

    // Track for cleanup
    testDataHelper.getCreatedWorkItemIds().push(response.data.id);
  });

  test('should require authentication for work items endpoints', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    // Create a new client without authentication
    const unauthClient = new APIClient(config, false);
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const response = await unauthClient.get<ErrorResponse>(url);

    // Verify response status is 401 Unauthorized
    expect(response.status).toBe(401);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
  });

  test('should create work item with optional fields', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const url = `${config.services.goSprint.baseUrl}/api/v1/workitems`;
    const payload = {
      type: 'epic',
      title: 'Major Feature Epic',
      description: 'This epic encompasses multiple stories',
      priority: 'high',
      story_points: null, // Explicitly null
      assignee_id: null,
      parent_id: null,
      sprint_id: null,
    };

    const response = await apiClient.post<WorkItem>(url, payload);

    expect(response.status).toBe(201);

    // Verify response
    expect(response.data.type).toBe(payload.type);
    expect(response.data.title).toBe(payload.title);
    expect(response.data.description).toBe(payload.description);
    expect(response.data.priority).toBe(payload.priority);

    // Track for cleanup
    testDataHelper.getCreatedWorkItemIds().push(response.data.id);
  });
});
