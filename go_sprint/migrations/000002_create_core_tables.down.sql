-- Drop triggers
DROP TRIGGER IF EXISTS update_work_items_updated_at ON sprint_management.work_items;
DROP TRIGGER IF EXISTS update_sprints_updated_at ON sprint_management.sprints;

-- Drop trigger function
DROP FUNCTION IF EXISTS sprint_management.update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS sprint_management.work_item_dependencies CASCADE;
DROP TABLE IF EXISTS sprint_management.work_items CASCADE;
DROP TABLE IF EXISTS sprint_management.sprints CASCADE;

-- Drop custom types
DROP TYPE IF EXISTS sprint_management.dependency_type;
DROP TYPE IF EXISTS sprint_management.sprint_status;
DROP TYPE IF EXISTS sprint_management.priority_level;
DROP TYPE IF EXISTS sprint_management.work_item_status;
DROP TYPE IF EXISTS sprint_management.work_item_type;
