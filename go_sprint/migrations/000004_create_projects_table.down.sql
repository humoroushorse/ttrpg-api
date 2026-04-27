-- Drop trigger and function
DROP TRIGGER IF EXISTS work_items_assign_ticket_number ON work_items;
DROP FUNCTION IF EXISTS assign_ticket_number();
DROP FUNCTION IF EXISTS get_next_ticket_number(UUID);

-- Remove columns from work_items
DROP INDEX IF EXISTS idx_work_items_project_ticket;
DROP INDEX IF EXISTS idx_work_items_project_id;
ALTER TABLE work_items 
DROP COLUMN IF EXISTS ticket_number,
DROP COLUMN IF EXISTS project_id;

-- Drop projects table
DROP INDEX IF EXISTS idx_projects_key;
DROP TABLE IF EXISTS projects;
