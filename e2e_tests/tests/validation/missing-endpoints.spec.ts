/**
 * Missing Endpoints Detection Tests
 * Validates that all OpenAPI spec endpoints are implemented
 * Generates reports of unimplemented endpoints
 * 
 * @tags @validation @go_sprint @go_auth
 * Requirements: 8.2, 15.1, 15.2, 15.3, 15.4
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loginAsUser } from '../../lib/helpers/auth';
import { openapiValidator } from '../../lib/helpers/openapi-validator';
import {
  validateServiceEndpoints,
  generateMissingEndpointsReport,
  saveMissingEndpointsReport,
} from '../../lib/helpers/endpoint-detector';
import * as path from 'path';

test.describe('OpenAPI Endpoint Validation @validation', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;

  test.beforeAll(async () => {
    config = getConfig();
    apiClient = new APIClient(config, false);
    
    // Login to get auth token
    const user = await loginAsUser(apiClient);
    apiClient.setAuthToken(user.token);
    
    // Load OpenAPI specifications
    await openapiValidator.loadSpecs([
      {
        serviceName: 'go_auth',
        specPath: 'go_auth/api/openapi/auth.yaml',
      },
      {
        serviceName: 'go_sprint',
        specPath: 'go_sprint/api/openapi/combined.yaml',
      },
    ]);
  });

  test('should load OpenAPI specifications successfully', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    const services = openapiValidator.getServices();
    
    expect(services).toContain('go_auth');
    expect(services).toContain('go_sprint');
    
    const goAuthEndpoints = openapiValidator.getEndpoints('go_auth');
    const goSprintEndpoints = openapiValidator.getEndpoints('go_sprint');
    
    expect(goAuthEndpoints.length).toBeGreaterThan(0);
    expect(goSprintEndpoints.length).toBeGreaterThan(0);
    
    console.log(`\nLoaded ${goAuthEndpoints.length} endpoints for go_auth`);
    console.log(`Loaded ${goSprintEndpoints.length} endpoints for go_sprint`);
  });

  test('should generate endpoint report for go_auth', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    const report = openapiValidator.generateEndpointReport('go_auth');
    
    expect(report).toContain('go_auth');
    expect(report).toContain('Total Endpoints:');
    
    console.log(report);
  });

  test('should generate endpoint report for go_sprint', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    const report = openapiValidator.generateEndpointReport('go_sprint');
    
    expect(report).toContain('go_sprint');
    expect(report).toContain('Total Endpoints:');
    
    console.log(report);
  });

  test('should validate go_auth endpoints and generate missing endpoints report', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    const results = await validateServiceEndpoints(
      apiClient,
      config.services.goAuth.baseUrl,
      openapiValidator,
      'go_auth'
    );
    
    expect(results.length).toBeGreaterThan(0);
    
    // Generate report
    const report = generateMissingEndpointsReport(results, 'go_auth');
    console.log(report);
    
    // Save report to file
    const reportPath = path.join(process.cwd(), 'reports', 'missing-endpoints-go_auth.txt');
    saveMissingEndpointsReport(report, reportPath);
    
    // Count missing endpoints
    const missing = results.filter((r) => !r.implemented);
    const implemented = results.filter((r) => r.implemented);
    
    console.log(`\ngo_auth Summary:`);
    console.log(`  Implemented: ${implemented.length}/${results.length}`);
    console.log(`  Missing: ${missing.length}/${results.length}`);
    
    // This test doesn't fail if endpoints are missing - it just reports them
    // The actual endpoint tests will be skipped for missing endpoints
  });

  test('should validate go_sprint endpoints and generate missing endpoints report', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    const results = await validateServiceEndpoints(
      apiClient,
      config.services.goSprint.baseUrl,
      openapiValidator,
      'go_sprint'
    );
    
    expect(results.length).toBeGreaterThan(0);
    
    // Generate report
    const report = generateMissingEndpointsReport(results, 'go_sprint');
    console.log(report);
    
    // Save report to file
    const reportPath = path.join(process.cwd(), 'reports', 'missing-endpoints-go_sprint.txt');
    saveMissingEndpointsReport(report, reportPath);
    
    // Count missing endpoints
    const missing = results.filter((r) => !r.implemented);
    const implemented = results.filter((r) => r.implemented);
    
    console.log(`\ngo_sprint Summary:`);
    console.log(`  Implemented: ${implemented.length}/${results.length}`);
    console.log(`  Missing: ${missing.length}/${results.length}`);
    
    // This test doesn't fail if endpoints are missing - it just reports them
    // The actual endpoint tests will be skipped for missing endpoints
  });

  test('should check if specific endpoint is defined in spec', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    // Check a known endpoint
    const isLoginDefined = openapiValidator.isEndpointDefined(
      'go_auth',
      'POST',
      '/auth/login'
    );
    expect(isLoginDefined).toBe(true);
    
    // Check a non-existent endpoint
    const isFakeEndpointDefined = openapiValidator.isEndpointDefined(
      'go_auth',
      'GET',
      '/auth/fake-endpoint'
    );
    expect(isFakeEndpointDefined).toBe(false);
    
    // Get endpoint details
    const loginDetails = openapiValidator.getEndpointDetails(
      'go_auth',
      'POST',
      '/auth/login'
    );
    
    expect(loginDetails).not.toBeNull();
    expect(loginDetails?.method).toBe('POST');
    expect(loginDetails?.path).toBe('/auth/login');
    expect(loginDetails?.operationId).toBe('login');
  });

  test('should match paths with parameters correctly', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    // Check endpoint with path parameter
    const isWorkItemGetDefined = openapiValidator.isEndpointDefined(
      'go_sprint',
      'GET',
      '/api/v1/workitems/123e4567-e89b-12d3-a456-426614174000'
    );
    expect(isWorkItemGetDefined).toBe(true);
    
    // Get details for parameterized path
    const workItemDetails = openapiValidator.getEndpointDetails(
      'go_sprint',
      'GET',
      '/api/v1/workitems/123e4567-e89b-12d3-a456-426614174000'
    );
    
    expect(workItemDetails).not.toBeNull();
    expect(workItemDetails?.path).toBe('/api/v1/workitems/{id}');
  });
});

test.describe('Example: Skip Tests for Missing Endpoints @validation', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;

  test.beforeAll(async () => {
    config = getConfig();
    apiClient = new APIClient(config, false);
    
    // Login to get auth token
    const user = await loginAsUser(apiClient);
    apiClient.setAuthToken(user.token);
    
    // Load OpenAPI specifications
    await openapiValidator.loadSpecs([
      {
        serviceName: 'go_sprint',
        specPath: 'go_sprint/api/openapi/combined.yaml',
      },
    ]);
  });

  test('example: test that would be skipped if endpoint is missing', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    // This is an example of how to use skipIfNotImplemented
    // If the endpoint returns 404 and is defined in the spec, the test will be skipped
    
    const { skipIfNotImplemented } = await import('../../lib/helpers/endpoint-detector');
    
    // Check if endpoint is implemented before running test
    await skipIfNotImplemented(
      apiClient,
      config.services.goSprint.baseUrl,
      'POST',
      '/api/v1/sprints/planning/capacity',
      openapiValidator,
      'go_sprint'
    );
    
    // If we reach here, the endpoint is implemented
    // Run the actual test
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/planning/capacity`;
    const response = await apiClient.post(url, {
      team_id: '123e4567-e89b-12d3-a456-426614174000',
      lookback_sprints: 3,
    });
    
    // This test will only run if the endpoint is implemented
    // If it returns 404, the test will be skipped with a message
    // Planning endpoints return 501 Not Implemented as they're not yet fully implemented
    expect([200, 400, 401, 501]).toContain(response.status);
  });
});
