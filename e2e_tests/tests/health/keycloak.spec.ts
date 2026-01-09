/**
 * Keycloak connectivity health check tests
 * Validates that Keycloak is accessible and authentication works correctly
 * 
 * @tags @health
 * Requirements: 6.1, 6.2, 6.3, 6.4
 */

import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loadCredentials } from '../../lib/config/credentials';
import { loginAsUser, decodeToken } from '../../lib/helpers/auth';

test.describe('Keycloak Health Checks', () => {
  test('should successfully connect to Keycloak and authenticate test user @health', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    // Feature: playwright-api-testing, Property 6: Health Check Prerequisite
    
    const config = getConfig();
    const credentials = loadCredentials();
    const apiClient = new APIClient(config);

    // Attempt to authenticate with test user
    let authenticatedUser;
    try {
      authenticatedUser = await loginAsUser(apiClient);
    } catch (error) {
      throw new Error(`Failed to authenticate with Keycloak: ${error}`);
    }

    // Verify we received a token
    expect(authenticatedUser.token).toBeTruthy();
    expect(authenticatedUser.token.length).toBeGreaterThan(0);

    // Verify username matches
    expect(authenticatedUser.username).toBe(credentials.testUserUsername);
  });

  test('should return valid JWT token with expected claims @health', async () => {
    // Feature: playwright-api-testing, Property 1: Authentication Token Validity
    
    const config = getConfig();
    const credentials = loadCredentials();
    const apiClient = new APIClient(config);

    // Authenticate
    const authenticatedUser = await loginAsUser(apiClient);

    // Decode the JWT token
    let tokenPayload;
    try {
      tokenPayload = decodeToken(authenticatedUser.token);
    } catch (error) {
      throw new Error(`Failed to decode JWT token: ${error}`);
    }

    // Verify token structure
    expect(tokenPayload).toBeTruthy();

    // Verify expected claims exist
    expect(tokenPayload.sub).toBeTruthy(); // Subject (user ID)
    expect(tokenPayload.exp).toBeTruthy(); // Expiration time
    expect(tokenPayload.iat).toBeTruthy(); // Issued at time

    // Verify user-specific claims
    // Note: Keycloak typically includes preferred_username or username
    const hasUsername = tokenPayload.preferred_username || tokenPayload.username || tokenPayload.email;
    expect(hasUsername).toBeTruthy();

    // Verify token is not expired
    const currentTime = Math.floor(Date.now() / 1000);
    expect(tokenPayload.exp).toBeGreaterThan(currentTime);
  });

  test('should fail authentication with invalid credentials @health', async () => {
    // Feature: playwright-api-testing, Property 9: Authenticated Request Authorization
    
    const config = getConfig();
    const apiClient = new APIClient(config);

    // Attempt to login with invalid credentials
    let loginError;
    try {
      await apiClient.login('invalid_user', 'invalid_password');
    } catch (error) {
      loginError = error;
    }

    // Verify authentication failed
    expect(loginError).toBeTruthy();
  });

  test('should include refresh token in authentication response @health', async () => {
    const config = getConfig();
    const apiClient = new APIClient(config);

    // Authenticate
    const authenticatedUser = await loginAsUser(apiClient);

    // Verify refresh token is present
    expect(authenticatedUser.refreshToken).toBeTruthy();
    expect(authenticatedUser.refreshToken!.length).toBeGreaterThan(0);
  });

  test('should verify Keycloak realm configuration @health', async () => {
    const config = getConfig();

    // Verify Keycloak configuration is present
    expect(config.services.keycloak).toBeTruthy();
    expect(config.services.keycloak.url).toBeTruthy();
    expect(config.services.keycloak.realm).toBeTruthy();
    expect(config.services.keycloak.clientId).toBeTruthy();

    // Verify URLs are properly formatted
    expect(config.services.keycloak.url).toMatch(/^https?:\/\//);
  });
});
