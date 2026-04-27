/**
 * Registration endpoint tests for go_auth service
 * Tests user registration functionality with various validation scenarios
 * 
 * @tags @go_auth
 * Requirements: 16.2
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';

test.describe('Registration Endpoint Tests @go_auth', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;

  test.beforeEach(() => {
    config = getConfig();
    apiClient = new APIClient(config, false); // Disable logging for cleaner test output
  });

  test('should successfully register a new user with valid data', async () => {
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    // Generate unique username and email to avoid conflicts
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      email: uniqueEmail,
      password: 'SecurePassword123!',
      first_name: 'Test',
      last_name: 'User',
    });

    // Verify response status (201 Created or 200 OK)
    expect([200, 201]).toContain(response.status);

    // Verify response contains user information
    expect(response.data).toBeTruthy();
    
    // Response should contain either user info or success message
    const hasUserInfo = response.data.sub || response.data.user_id || response.data.message;
    expect(hasUserInfo).toBeTruthy();
  });

  test('should fail registration with invalid email format', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      email: 'invalid-email-format', // Invalid email
      password: 'SecurePassword123!',
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
    
    // Error message should mention email validation
    expect(response.data.message.toLowerCase()).toMatch(/email|invalid/);
  });

  test('should fail registration with missing email', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      password: 'SecurePassword123!',
      // email is missing
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail registration with weak password (too short)', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      email: uniqueEmail,
      password: 'weak', // Too short (less than 8 characters)
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
    
    // Error message should mention password requirements
    expect(response.data.message.toLowerCase()).toMatch(/password|length|characters/);
  });

  test('should fail registration with missing password', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      email: uniqueEmail,
      // password is missing
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail registration with duplicate username', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const duplicateUsername = `duplicate_user_${timestamp}`;
    const firstEmail = `first_${timestamp}@example.com`;
    const secondEmail = `second_${timestamp}@example.com`;
    
    // First registration should succeed
    const firstResponse = await apiClient.post(registerUrl, {
      username: duplicateUsername,
      email: firstEmail,
      password: 'SecurePassword123!',
    });

    expect([200, 201]).toContain(firstResponse.status);

    // Second registration with same username should fail
    const secondResponse = await apiClient.post(registerUrl, {
      username: duplicateUsername, // Same username
      email: secondEmail, // Different email
      password: 'SecurePassword123!',
    });

    // Verify response status is 400 Bad Request or 409 Conflict
    expect([400, 409]).toContain(secondResponse.status);

    // Verify error response structure
    expect(secondResponse.data).toHaveProperty('error');
    expect(secondResponse.data).toHaveProperty('message');
    
    // Error message should mention duplicate or existing user
    expect(secondResponse.data.message.toLowerCase()).toMatch(/exists|duplicate|already/);
  });

  test('should fail registration with duplicate email', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const duplicateEmail = `duplicate_${timestamp}@example.com`;
    const firstUsername = `first_user_${timestamp}`;
    const secondUsername = `second_user_${timestamp}`;
    
    // First registration should succeed
    const firstResponse = await apiClient.post(registerUrl, {
      username: firstUsername,
      email: duplicateEmail,
      password: 'SecurePassword123!',
    });

    expect([200, 201]).toContain(firstResponse.status);

    // Second registration with same email should fail
    const secondResponse = await apiClient.post(registerUrl, {
      username: secondUsername, // Different username
      email: duplicateEmail, // Same email
      password: 'SecurePassword123!',
    });

    // Verify response status is 400 Bad Request or 409 Conflict
    expect([400, 409]).toContain(secondResponse.status);

    // Verify error response structure
    expect(secondResponse.data).toHaveProperty('error');
    expect(secondResponse.data).toHaveProperty('message');
    
    // Error message should mention duplicate or existing email
    expect(secondResponse.data.message.toLowerCase()).toMatch(/email|exists|duplicate|already/);
  });

  test('should fail registration with username that is too short', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      username: 'ab', // Too short (less than 3 characters per OpenAPI spec)
      email: uniqueEmail,
      password: 'SecurePassword123!',
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail registration with missing username', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      // username is missing
      email: uniqueEmail,
      password: 'SecurePassword123!',
    });

    // Verify response status is 400 Bad Request
    expect(response.status).toBe(400);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should accept registration with optional first_name and last_name', async () => {
    const registerUrl = `${config.services.goAuth.baseUrl}/auth/register`;
    
    const timestamp = Date.now();
    const uniqueUsername = `testuser_${timestamp}`;
    const uniqueEmail = `testuser_${timestamp}@example.com`;
    
    const response = await apiClient.post(registerUrl, {
      username: uniqueUsername,
      email: uniqueEmail,
      password: 'SecurePassword123!',
      first_name: 'John',
      last_name: 'Doe',
    });

    // Verify response status (201 Created or 200 OK)
    expect([200, 201]).toContain(response.status);

    // Verify response contains user information
    expect(response.data).toBeTruthy();
  });
});
