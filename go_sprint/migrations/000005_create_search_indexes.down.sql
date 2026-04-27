-- Drop partial indexes for soft-deleted items
DROP INDEX IF EXISTS sprint_management.idx_comments_soft_deleted;
DROP INDEX IF EXISTS sprint_management.idx_sprints_soft_deleted;
DROP INDEX IF EXISTS sprint_management.idx_work_items_soft_deleted;

-- Drop BRIN index
DROP INDEX IF EXISTS sprint_management.idx_activity_logs_created_brin;

-- Drop composite indexes
DROP INDEX IF EXISTS sprint_management.idx_work_items_active_sprint;
DROP INDEX IF EXISTS sprint_management.idx_sprints_active;
DROP INDEX IF EXISTS sprint_management.idx_sprints_status_dates;
DROP INDEX IF EXISTS sprint_management.idx_work_items_type_status;
DROP INDEX IF EXISTS sprint_management.idx_work_items_assignee_status;
DROP INDEX IF EXISTS sprint_management.idx_work_items_sprint_status;

-- Drop cursor pagination indexes
DROP INDEX IF EXISTS sprint_management.idx_comments_cursor;
DROP INDEX IF EXISTS sprint_management.idx_sprints_cursor;
DROP INDEX IF EXISTS sprint_management.idx_work_items_cursor;

-- Drop trigram indexes
DROP INDEX IF EXISTS sprint_management.idx_work_items_description_trgm;
DROP INDEX IF EXISTS sprint_management.idx_work_items_title_trgm;

-- Drop full-text search index
DROP INDEX IF EXISTS sprint_management.idx_work_items_search;
