/**
 * OpenAPI Validator Unit Tests
 * Tests the OpenAPI validator functionality without requiring running services
 * 
 * @tags @validation @unit
 * Requirements: 8.2, 15.2
 */

import { test, expect } from '@playwright/test';
import { OpenAPIValidator } from '../../lib/helpers/openapi-validator';
import * as path from 'path';

test.describe('OpenAPI Validator Unit Tests @validation @unit', () => {
  let validator: OpenAPIValidator;

  test.beforeEach(() => {
    validator = new OpenAPIValidator();
  });

  test('should load go_auth OpenAPI specification', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_auth', 'go_auth/api/openapi/auth.yaml');
    
    const services = validator.getServices();
    expect(services).toContain('go_auth');
    
    const endpoints = validator.getEndpoints('go_auth');
    expect(endpoints.length).toBeGreaterThan(0);
    
    console.log(`Loaded ${endpoints.length} endpoints for go_auth`);
  });

  test('should load go_sprint OpenAPI specification', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_sprint', 'go_sprint/api/openapi/combined.yaml');
    
    const services = validator.getServices();
    expect(services).toContain('go_sprint');
    
    const endpoints = validator.getEndpoints('go_sprint');
    expect(endpoints.length).toBeGreaterThan(0);
    
    console.log(`Loaded ${endpoints.length} endpoints for go_sprint`);
  });

  test('should extract endpoints from go_auth spec', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_auth', 'go_auth/api/openapi/auth.yaml');
    
    const endpoints = validator.getEndpoints('go_auth');
    
    // Verify we have the expected endpoints
    const loginEndpoint = endpoints.find(
      (e) => e.method === 'POST' && e.path === '/auth/login'
    );
    expect(loginEndpoint).toBeDefined();
    expect(loginEndpoint?.operationId).toBe('login');
    
    const registerEndpoint = endpoints.find(
      (e) => e.method === 'POST' && e.path === '/auth/register'
    );
    expect(registerEndpoint).toBeDefined();
    expect(registerEndpoint?.operationId).toBe('registerUser');
    
    const refreshEndpoint = endpoints.find(
      (e) => e.method === 'POST' && e.path === '/auth/refresh'
    );
    expect(refreshEndpoint).toBeDefined();
    expect(refreshEndpoint?.operationId).toBe('refreshToken');
  });

  test('should check if endpoint is defined in spec', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_auth', 'go_auth/api/openapi/auth.yaml');
    
    // Check existing endpoint
    const isLoginDefined = validator.isEndpointDefined('go_auth', 'POST', '/auth/login');
    expect(isLoginDefined).toBe(true);
    
    // Check non-existent endpoint
    const isFakeDefined = validator.isEndpointDefined('go_auth', 'GET', '/auth/fake');
    expect(isFakeDefined).toBe(false);
  });

  test('should get endpoint details', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_auth', 'go_auth/api/openapi/auth.yaml');
    
    const details = validator.getEndpointDetails('go_auth', 'POST', '/auth/login');
    
    expect(details).not.toBeNull();
    expect(details?.method).toBe('POST');
    expect(details?.path).toBe('/auth/login');
    expect(details?.operationId).toBe('login');
    expect(details?.summary).toContain('Login');
  });

  test('should match paths with parameters', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_sprint', 'go_sprint/api/openapi/combined.yaml');
    
    // Check endpoint with path parameter
    const isWorkItemDefined = validator.isEndpointDefined(
      'go_sprint',
      'GET',
      '/api/v1/workitems/123e4567-e89b-12d3-a456-426614174000'
    );
    expect(isWorkItemDefined).toBe(true);
    
    // Get details for parameterized path
    const details = validator.getEndpointDetails(
      'go_sprint',
      'GET',
      '/api/v1/workitems/123e4567-e89b-12d3-a456-426614174000'
    );
    
    expect(details).not.toBeNull();
    expect(details?.path).toBe('/api/v1/workitems/{id}');
  });

  test('should generate endpoint report', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_auth', 'go_auth/api/openapi/auth.yaml');
    
    const report = validator.generateEndpointReport('go_auth');
    
    expect(report).toContain('go_auth');
    expect(report).toContain('Total Endpoints:');
    expect(report).toContain('POST');
    expect(report).toContain('/auth/login');
    
    console.log(report);
  });

  test('should load multiple specs', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpecs([
      { serviceName: 'go_auth', specPath: 'go_auth/api/openapi/auth.yaml' },
      { serviceName: 'go_sprint', specPath: 'go_sprint/api/openapi/combined.yaml' },
    ]);
    
    const services = validator.getServices();
    expect(services).toContain('go_auth');
    expect(services).toContain('go_sprint');
    
    const goAuthEndpoints = validator.getEndpoints('go_auth');
    const goSprintEndpoints = validator.getEndpoints('go_sprint');
    
    expect(goAuthEndpoints.length).toBeGreaterThan(0);
    expect(goSprintEndpoints.length).toBeGreaterThan(0);
    
    console.log(`Loaded ${goAuthEndpoints.length} go_auth endpoints`);
    console.log(`Loaded ${goSprintEndpoints.length} go_sprint endpoints`);
  });

  test('should handle query parameters in paths', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_sprint', 'go_sprint/api/openapi/combined.yaml');
    
    // Path with query parameters should match
    const isDefined = validator.isEndpointDefined(
      'go_sprint',
      'GET',
      '/api/v1/workitems?type=story&status=todo'
    );
    expect(isDefined).toBe(true);
  });

  test('should extract all HTTP methods', async () => {
    // Feature: playwright-api-testing, Property 8: Missing Endpoint Detection
    
    await validator.loadSpec('go_sprint', 'go_sprint/api/openapi/combined.yaml');
    
    const endpoints = validator.getEndpoints('go_sprint');
    
    // Check we have different HTTP methods
    const methods = new Set(endpoints.map((e) => e.method));
    
    // The go_sprint API currently has GET and POST methods
    expect(methods.has('GET')).toBe(true);
    expect(methods.has('POST')).toBe(true);
    
    // Verify we extracted multiple endpoints
    expect(endpoints.length).toBeGreaterThan(0);
    
    console.log(`Found HTTP methods: ${Array.from(methods).join(', ')}`);
    console.log(`Total endpoints: ${endpoints.length}`);
  });
});
