-- Make project_id optional again
ALTER TABLE sprint_management.work_items 
ALTER COLUMN project_id DROP NOT NULL;

-- Remove seeded projects
DELETE FROM sprint_management.projects 
WHERE key IN ('SPRINT', 'DND') AND created_by = 'system';
