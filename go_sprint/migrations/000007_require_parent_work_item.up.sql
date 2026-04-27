-- Migration: Require parent_id for Story and Defect types
-- This enforces the hierarchy: Epic (no parent) -> Story/Defect (parent=Epic)

-- First, delete all existing work items since we're changing the schema requirements
TRUNCATE TABLE sprint_management.work_items CASCADE;

-- Add check constraint to enforce parent rules based on work item type
-- Epic: parent_id must be NULL
-- Story: parent_id must NOT be NULL (will validate parent is Epic in application)
-- Defect: parent_id must NOT be NULL (will validate parent is Epic in application)
ALTER TABLE sprint_management.work_items
ADD CONSTRAINT work_item_parent_rules CHECK (
    (type = 'epic' AND parent_id IS NULL) OR
    (type IN ('story', 'defect') AND parent_id IS NOT NULL)
);

-- Add index on parent_id for faster queries (if not exists)
CREATE INDEX IF NOT EXISTS idx_work_items_parent_id ON sprint_management.work_items(parent_id);

-- Add comment explaining the hierarchy
COMMENT ON CONSTRAINT work_item_parent_rules ON sprint_management.work_items IS 
'Enforces work item hierarchy: Epic (no parent) -> Story/Defect (parent=Epic)';
