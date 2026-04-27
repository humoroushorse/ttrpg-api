-- Rollback: Remove parent_work_item_id constraints

-- Drop the check constraint
ALTER TABLE sprint_management.work_items
DROP CONSTRAINT IF EXISTS work_item_parent_rules;

-- Drop the index
DROP INDEX IF EXISTS sprint_management.idx_work_items_parent_id;
