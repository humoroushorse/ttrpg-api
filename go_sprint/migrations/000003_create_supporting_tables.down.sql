-- Drop triggers
DROP TRIGGER IF EXISTS update_comments_updated_at ON sprint_management.comments;
DROP TRIGGER IF EXISTS update_users_updated_at ON auth.users;

-- Drop trigger function
DROP FUNCTION IF EXISTS auth.update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS sprint_management.activity_logs CASCADE;
DROP TABLE IF EXISTS sprint_management.comments CASCADE;
DROP TABLE IF EXISTS auth.users CASCADE;
