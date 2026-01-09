-- Create schemas
CREATE SCHEMA IF NOT EXISTS sprint_management;
CREATE SCHEMA IF NOT EXISTS auth;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Set timezone to UTC for all connections
SET timezone = 'UTC';
