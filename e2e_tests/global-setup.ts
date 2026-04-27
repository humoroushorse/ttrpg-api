import * as dotenv from 'dotenv';
import { DatabaseHelper } from './lib/helpers/database';

// Load environment variables
dotenv.config();

/**
 * Global setup hook for Playwright test suite
 * 
 * This runs once before all tests and handles:
 * - Environment validation
 * - Database initialization
 * - Database migrations
 * - Test fixture loading
 * 
 * Requirements: 5.3 (Database initialization before tests), 5.5, 12.5 (Fixture loading)
 */
async function globalSetup() {
  console.log('🚀 Starting global setup...');
  
  const db = new DatabaseHelper();
  
  try {
    // Validate required environment variables
    validateEnvironment();
    
    // Initialize test database connection
    console.log('📦 Connecting to test database...');
    await db.connect();
    
    // Check if migrations are needed (check if tables exist)
    const tablesExist = await checkIfTablesExist(db);
    
    if (!tablesExist) {
      // Run database migrations only if tables don't exist
      console.log('🔄 Running database migrations...');
      await db.runMigrations();
    } else {
      console.log('✅ Database schema already exists, skipping migrations');
    }
    
    // Clean existing data
    console.log('🧹 Cleaning existing test data...');
    await db.truncateAllTables();
    await db.resetSequences();
    
    // Load test fixtures
    console.log('📥 Loading test fixtures...');
    await loadFixtures(db);
    
    // Disconnect from database
    await db.disconnect();
    
    console.log('✅ Global setup completed successfully');
  } catch (error) {
    console.error('❌ Global setup failed:', error);
    await db.disconnect();
    throw error;
  }
}

/**
 * Load test fixtures into the database
 * Requirements: 5.5, 12.5
 */
async function loadFixtures(db: DatabaseHelper): Promise<void> {
  try {
    // Load fixtures in order (sprints first, then work items that reference them)
    const fixtureNames = [
      'sprints',
      'work-items',
      // Note: users.json is for reference only, actual users are managed by Keycloak
    ];
    
    await db.loadFixtures(fixtureNames);
    
    console.log('✅ All fixtures loaded successfully');
  } catch (error) {
    console.error('❌ Failed to load fixtures:', error);
    throw error;
  }
}

/**
 * Check if database tables already exist
 * Returns true if sprint_management schema has tables
 */
async function checkIfTablesExist(db: DatabaseHelper): Promise<boolean> {
  try {
    const result = await db.query(`
      SELECT COUNT(*) as count
      FROM information_schema.tables
      WHERE table_schema = 'sprint_management'
    `);
    
    return result[0]?.count > 0;
  } catch (error) {
    // If query fails, assume tables don't exist
    return false;
  }
}

/**
 * Validate that required environment variables are present
 * Fails fast with clear error messages if credentials are missing
 */
function validateEnvironment() {
  console.log('🔍 Validating environment configuration...');
  
  const requiredVars = [
    'GO_AUTH_BASE_URL',
    'GO_SPRINT_BASE_URL',
  ];
  
  const missingVars: string[] = [];
  
  for (const varName of requiredVars) {
    if (!process.env[varName]) {
      missingVars.push(varName);
    }
  }
  
  if (missingVars.length > 0) {
    const errorMessage = `
❌ Missing required environment variables:
${missingVars.map(v => `  - ${v}`).join('\n')}

Please create a .env file based on .env.example and set these variables.
See .env.example for reference values.
    `.trim();
    
    throw new Error(errorMessage);
  }
  
  console.log('✅ Environment validation passed');
}

export default globalSetup;
