-- Initialize test database for sprint management system

-- Create sprint_management schema
CREATE SCHEMA IF NOT EXISTS sprint_management;

-- Create auth schema (for authentication-related tables if needed)
CREATE SCHEMA IF NOT EXISTS auth;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set timezone to UTC for all connections
SET timezone = 'UTC';

-- Grant permissions
GRANT ALL PRIVILEGES ON SCHEMA sprint_management TO postgres;
GRANT ALL PRIVILEGES ON SCHEMA auth TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA sprint_management TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA auth TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA sprint_management TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA auth TO postgres;
