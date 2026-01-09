import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';

// Load environment variables from .env file
dotenv.config();

/**
 * Playwright configuration for multi-project API testing
 * 
 * This configuration supports:
 * - Multiple projects (go_auth, go_sprint) with independent execution
 * - Test categorization via tags (@go_auth, @go_sprint, @health, @stress)
 * - Parallel execution with configurable workers
 * - Multiple report formats (HTML, JSON, JUnit)
 * - Global setup/teardown for database initialization
 */
export default defineConfig({
  // Test directory
  testDir: './tests',
  
  // Maximum time one test can run
  timeout: 30 * 1000, // 30 seconds
  
  // Test execution settings
  fullyParallel: true,
  forbidOnly: !!process.env.CI, // Fail if test.only is left in CI
  retries: process.env.CI ? 2 : 0, // Retry failed tests in CI
  workers: process.env.CI ? 2 : undefined, // Limit workers in CI, use all cores locally
  
  // Reporter configuration
  reporter: [
    ['html', { outputFolder: 'reports/html', open: 'never' }],
    ['json', { outputFile: 'reports/results.json' }],
    ['junit', { outputFile: 'reports/junit.xml' }],
    ['list'], // Console output
  ],
  
  // Global setup and teardown
  globalSetup: require.resolve('./global-setup.ts'),
  globalTeardown: require.resolve('./global-teardown.ts'),
  
  // Shared settings for all projects
  use: {
    // Base URL for API requests (can be overridden per project)
    baseURL: process.env.BASE_URL || 'http://localhost:8080',
    
    // Collect trace on failure for debugging
    trace: 'retain-on-failure',
    
    // API request settings
    extraHTTPHeaders: {
      'Accept': 'application/json',
      'Content-Type': 'application/json',
    },
    
    // Ignore HTTPS errors in local/test environments
    ignoreHTTPSErrors: true,
  },
  
  // Project definitions for multi-project testing
  projects: [
    {
      name: 'health-checks',
      testMatch: /.*health.*\.spec\.ts/,
      grep: /@health/,
      retries: 3, // Health checks are critical, retry more
      timeout: 15 * 1000, // Shorter timeout for health checks
    },
    
    {
      name: 'validation',
      testMatch: /.*validation.*\.spec\.ts/,
      grep: /@validation/,
      timeout: 60 * 1000, // Longer timeout for validation tests
    },
    
    {
      name: 'go_auth',
      testMatch: /.*go_auth.*\.spec\.ts/,
      grep: /@go_auth/,
      dependencies: ['health-checks'], // Run health checks first
      use: {
        baseURL: process.env.GO_AUTH_BASE_URL || 'http://localhost:8081',
      },
    },
    
    {
      name: 'go_sprint',
      testMatch: /.*go_sprint.*\.spec\.ts/,
      grep: /@go_sprint/,
      dependencies: ['health-checks'], // Run health checks first
      use: {
        baseURL: process.env.GO_SPRINT_BASE_URL || 'http://localhost:8080',
      },
    },
  ],
  
  // Grep configuration for excluding tests in CI
  // Exclude @stress tests in CI/CD pipelines
  grepInvert: process.env.CI ? /@stress/ : undefined,
});
