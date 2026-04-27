/**
 * Token refresh endpoint tests for go_auth service
 * Tests token refresh functionality with valid/invalid/expired tokens
 * 
 * @tags @go_auth
 * Requirements: 16.3, 16.7
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loadCredentials } from '../../lib/config/credentials';
import { decodeToken } from '../../lib/helpers/auth';

test.describe('Token Refresh Endpoint Tests @go_auth', () => {
  let apiClient: APIClient;
  let config: ReturnType<typeof getConfig>;
  let credentials: ReturnType<typeof loadCredentials>;

  test.beforeEach(() => {
    config = getConfig();
    credentials = loadCredentials();
    apiClient = new APIClient(config, false); // Disable logging for cleaner test output
  });

  test('should successfully refresh token with valid refresh token', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    // First, login to get a refresh token
    const loginResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    expect(loginResponse.refresh_token).toBeTruthy();

    const originalRefreshToken = loginResponse.refresh_token;
    const originalAccessToken = loginResponse.access_token;

    // Now use the refresh token to get a new access token
    const refreshResponse = await apiClient.post(refreshUrl, {
      refresh_token: originalRefreshToken,
    });

    // Verify response status
    expect(refreshResponse.status).toBe(200);

    // Verify response contains new tokens
    expect(refreshResponse.data).toHaveProperty('access_token');
    expect(refreshResponse.data).toHaveProperty('token_type');
    expect(refreshResponse.data).toHaveProperty('expires_in');

    // Verify new access token is different from original
    expect(refreshResponse.data.access_token).not.toBe(originalAccessToken);

    // Verify new access token is a valid JWT
    expect(refreshResponse.data.access_token).toBeTruthy();
    expect(typeof refreshResponse.data.access_token).toBe('string');
    expect(refreshResponse.data.access_token.split('.')).toHaveLength(3);

    // Decode and verify new token claims
    const newTokenPayload = decodeToken(refreshResponse.data.access_token);
    expect(newTokenPayload.sub).toBeTruthy();
    expect(newTokenPayload.exp).toBeTruthy();
    expect(newTokenPayload.iat).toBeTruthy();

    // Verify new token is not expired
    const currentTime = Math.floor(Date.now() / 1000);
    expect(newTokenPayload.exp).toBeGreaterThan(currentTime);
  });

  test('should fail refresh with invalid refresh token', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    const response = await apiClient.post(refreshUrl, {
      refresh_token: 'invalid_refresh_token_12345',
    });

    // Verify response status is 401 Unauthorized
    expect(response.status).toBe(401);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail refresh with malformed refresh token', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    const response = await apiClient.post(refreshUrl, {
      refresh_token: 'not.a.valid.jwt.token',
    });

    // Verify response status is 401 Unauthorized
    expect(response.status).toBe(401);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail refresh with missing refresh token', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    const response = await apiClient.post(refreshUrl, {
      // refresh_token is missing
    });

    // Verify response status is 400 Bad Request or 404 Not Found (per OpenAPI spec)
    expect([400, 404]).toContain(response.status);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should fail refresh with empty refresh token', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    const response = await apiClient.post(refreshUrl, {
      refresh_token: '',
    });

    // Verify response status is 400 Bad Request, 401 Unauthorized, or 404 Not Found
    expect([400, 401, 404]).toContain(response.status);

    // Verify error response structure
    expect(response.data).toHaveProperty('error');
    expect(response.data).toHaveProperty('message');
  });

  test('should return new refresh token in response', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    // First, login to get a refresh token
    const loginResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    const originalRefreshToken = loginResponse.refresh_token;

    // Refresh the token
    const refreshResponse = await apiClient.post(refreshUrl, {
      refresh_token: originalRefreshToken,
    });

    expect(refreshResponse.status).toBe(200);

    // Verify new refresh token is present
    expect(refreshResponse.data).toHaveProperty('refresh_token');
    expect(refreshResponse.data.refresh_token).toBeTruthy();
    expect(typeof refreshResponse.data.refresh_token).toBe('string');
  });

  test('should maintain user identity after token refresh', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    // Login to get tokens
    const loginResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    // Decode original token to get user ID
    const originalTokenPayload = decodeToken(loginResponse.access_token);
    const originalUserId = originalTokenPayload.sub;

    // Refresh the token
    const refreshResponse = await apiClient.post(refreshUrl, {
      refresh_token: loginResponse.refresh_token,
    });

    expect(refreshResponse.status).toBe(200);

    // Decode new token and verify user ID is the same
    const newTokenPayload = decodeToken(refreshResponse.data.access_token);
    const newUserId = newTokenPayload.sub;

    expect(newUserId).toBe(originalUserId);
  });

  test('should fail refresh with access token instead of refresh token', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    // Login to get tokens
    const loginResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    // Try to use access token as refresh token (should fail)
    const refreshResponse = await apiClient.post(refreshUrl, {
      refresh_token: loginResponse.access_token, // Wrong token type
    });

    // Verify response status is 401 Unauthorized
    expect(refreshResponse.status).toBe(401);

    // Verify error response structure
    expect(refreshResponse.data).toHaveProperty('error');
    expect(refreshResponse.data).toHaveProperty('message');
  });

  test('should have valid expiration time in refreshed token', async () => {
    const refreshUrl = `${config.services.goAuth.baseUrl}/auth/refresh`;
    
    // Login and refresh
    const loginResponse = await apiClient.login(
      credentials.testUserUsername,
      credentials.testUserPassword
    );

    const refreshResponse = await apiClient.post(refreshUrl, {
      refresh_token: loginResponse.refresh_token,
    });

    expect(refreshResponse.status).toBe(200);

    // Verify expires_in is present and positive
    expect(refreshResponse.data.expires_in).toBeGreaterThan(0);

    // Decode token and verify expiration
    const tokenPayload = decodeToken(refreshResponse.data.access_token);
    const currentTime = Math.floor(Date.now() / 1000);
    
    // Token should expire in the future
    expect(tokenPayload.exp).toBeGreaterThan(currentTime);
    
    // Token expiration should be reasonable (not too far in the future)
    const maxExpiration = currentTime + (24 * 60 * 60); // 24 hours
    expect(tokenPayload.exp).toBeLessThan(maxExpiration);
  });
});
