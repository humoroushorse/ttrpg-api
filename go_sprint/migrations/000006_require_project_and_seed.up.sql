-- Delete all existing data
TRUNCATE TABLE sprint_management.work_item_dependencies CASCADE;
TRUNCATE TABLE sprint_management.work_items CASCADE;
TRUNCATE TABLE sprint_management.sprints CASCADE;

-- Seed default projects
INSERT INTO sprint_management.projects (key, name, description, created_by, updated_by)
VALUES 
    ('SPRINT', 'Sprint Management', 'General sprint management work items', 'system', 'system'),
    ('DND', 'D&D Campaign', 'Dungeons & Dragons campaign management', 'system', 'system')
ON CONFLICT (key) DO NOTHING;

-- Make project_id required for new work items
ALTER TABLE sprint_management.work_items 
ALTER COLUMN project_id SET NOT NULL;
