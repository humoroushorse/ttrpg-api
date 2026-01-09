-- Enable pg_trgm extension for better text search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create full-text search index for work items
CREATE INDEX idx_work_items_search ON sprint_management.work_items 
USING gin(to_tsvector('english', title || ' ' || COALESCE(description, '')));

-- Create trigram indexes for fuzzy search
CREATE INDEX idx_work_items_title_trgm ON sprint_management.work_items USING gin(title gin_trgm_ops);
CREATE INDEX idx_work_items_description_trgm ON sprint_management.work_items USING gin(description gin_trgm_ops);

-- Create cursor pagination indexes (created_at, id for stable ordering)
CREATE INDEX idx_work_items_cursor ON sprint_management.work_items(created_at DESC, id);
CREATE INDEX idx_sprints_cursor ON sprint_management.sprints(created_at DESC, id);
CREATE INDEX idx_comments_cursor ON sprint_management.comments(created_at DESC, id);

-- Create composite indexes for common query patterns
CREATE INDEX idx_work_items_sprint_status ON sprint_management.work_items(sprint_id, status) 
WHERE deleted_at IS NULL;

CREATE INDEX idx_work_items_assignee_status ON sprint_management.work_items(assignee_id, status) 
WHERE deleted_at IS NULL;

CREATE INDEX idx_work_items_type_status ON sprint_management.work_items(type, status) 
WHERE deleted_at IS NULL;

CREATE INDEX idx_sprints_status_dates ON sprint_management.sprints(status, start_date, end_date) 
WHERE deleted_at IS NULL;

-- Create index for active sprints query
CREATE INDEX idx_sprints_active ON sprint_management.sprints(status, start_date, end_date) 
WHERE status = 'active' AND deleted_at IS NULL;

-- Create index for work items in active sprints
CREATE INDEX idx_work_items_active_sprint ON sprint_management.work_items(sprint_id, status) 
WHERE deleted_at IS NULL AND sprint_id IS NOT NULL;

-- Create BRIN index for time-series data (activity_logs)
CREATE INDEX idx_activity_logs_created_brin ON sprint_management.activity_logs 
USING brin(created_at) WITH (pages_per_range = 128);

-- Create partial indexes for soft-deleted items (for recovery queries)
CREATE INDEX idx_work_items_soft_deleted ON sprint_management.work_items(deleted_at, deleted_by) 
WHERE deleted_at IS NOT NULL;

CREATE INDEX idx_sprints_soft_deleted ON sprint_management.sprints(deleted_at, deleted_by) 
WHERE deleted_at IS NOT NULL;

CREATE INDEX idx_comments_soft_deleted ON sprint_management.comments(deleted_at, deleted_by) 
WHERE deleted_at IS NOT NULL;
