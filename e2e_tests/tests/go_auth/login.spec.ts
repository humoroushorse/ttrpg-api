/**
 * Login endpoint tests for go_auth service
 * Tests authentication functionality including valid/invalid credentials
 * 
 * @tags @go_auth
 * Requirements: 16.1, 16.6
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loadCredentials } from '../../lib/config/credentials';
import { decodeToken } from '../../lib/helpers/auth';

test.describe('Login Endpoint Tests @go_auth', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;
  let credentials: ReturnType<typeof loadCredentials>;

  test.beforeEach(() => {
    config = getConfig();
    credentials = loadCredentials();
    apiClient = new APIClient(config, false); // Disable logging for cleaner test output
  });

  test('should successfully login with valid credentials', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    // Feature: playwright-api-testing, Property 2: Endpoint Response Schema Compliance
    
    // Use the login method which handles form-urlencoded format
    const tokenResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    // Verify response contains required fields
    expect(tokenResponse).toHaveProperty('access_token');
    expect(tokenResponse).toHaveProperty('token_type');
    expect(tokenResponse).toHaveProperty('expires_in');

    // Verify token is a valid JWT
    expect(tokenResponse.access_token).toBeTruthy();
    expect(typeof tokenResponse.access_token).toBe('string');
    expect(tokenResponse.access_token.split('.')).toHaveLength(3);

    // Verify token type
    expect(tokenResponse.token_type).toBe('Bearer');

    // Verify expires_in is a positive number
    expect(tokenResponse.expires_in).toBeGreaterThan(0);

    // Decode and verify token claims
    const tokenPayload = decodeToken(tokenResponse.access_token);
    expect(tokenPayload.sub).toBeTruthy();
    expect(tokenPayload.exp).toBeTruthy();
    expect(tokenPayload.iat).toBeTruthy();

    // Verify token is not expired
    const currentTime = Math.floor(Date.now() / 1000);
    expect(tokenPayload.exp).toBeGreaterThan(currentTime);
  });

  test('should fail login with invalid username', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    // Attempt login with invalid username - should throw error
    await expect(async () => {
      await apiClient.login('nonexistent_user_12345', credentials.testUserPassword);
    }).rejects.toThrow();
  });

  test('should fail login with invalid password', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    // Attempt login with invalid password - should throw error
    await expect(async () => {
      await apiClient.login(credentials.testUserUsername, 'wrong_password_12345');
    }).rejects.toThrow();
  });

  test('should fail login with missing username field', async () => {
    const loginUrl = `${config.services.goAuth.baseUrl}/auth/login`;
    
    const formData = new URLSearchParams();
    formData.append('password', credentials.testUserPassword);
    
    const response = await apiClient.postForm(loginUrl, formData);

    // Verify response status is 400 Bad Request or 401 Unauthorized
    expect([400, 401]).toContain(response.status);
  });

  test('should fail login with missing password field', async () => {
    const loginUrl = `${config.services.goAuth.baseUrl}/auth/login`;
    
    const formData = new URLSearchParams();
    formData.append('username', credentials.testUserUsername);
    
    const response = await apiClient.postForm(loginUrl, formData);

    // Verify response status is 400 Bad Request or 401 Unauthorized
    expect([400, 401]).toContain(response.status);
  });

  test('should fail login with empty credentials', async () => {
    // Attempt login with empty credentials - should throw error
    await expect(async () => {
      await apiClient.login('', '');
    }).rejects.toThrow();
  });

  test('should include refresh token in successful login response', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    
    const tokenResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    // Verify refresh token is present
    expect(tokenResponse).toHaveProperty('refresh_token');
    expect(tokenResponse.refresh_token).toBeTruthy();
    expect(typeof tokenResponse.refresh_token).toBe('string');
  });

  test('should validate JWT token structure and claims', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    
    const tokenResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    // Decode token
    const tokenPayload = decodeToken(tokenResponse.access_token);

    // Verify required JWT claims
    expect(tokenPayload).toHaveProperty('sub'); // Subject (user ID)
    expect(tokenPayload).toHaveProperty('exp'); // Expiration time
    expect(tokenPayload).toHaveProperty('iat'); // Issued at time

    // Verify user-specific claims (Keycloak typically includes these)
    const hasUserIdentifier = 
      tokenPayload.preferred_username || 
      tokenPayload.username || 
      tokenPayload.email;
    expect(hasUserIdentifier).toBeTruthy();

    // Verify expiration is in the future
    const currentTime = Math.floor(Date.now() / 1000);
    expect(tokenPayload.exp).toBeGreaterThan(currentTime);

    // Verify issued at is in the past
    expect(tokenPayload.iat).toBeLessThanOrEqual(currentTime);
  });
});
