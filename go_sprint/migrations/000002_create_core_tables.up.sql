-- Create custom types for sprint_management schema
CREATE TYPE sprint_management.work_item_type AS ENUM ('epic', 'story', 'defect');
CREATE TYPE sprint_management.work_item_status AS ENUM ('todo', 'in_progress', 'in_review', 'done', 'blocked');
CREATE TYPE sprint_management.priority_level AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE sprint_management.sprint_status AS ENUM ('planned', 'active', 'completed', 'cancelled');
CREATE TYPE sprint_management.dependency_type AS ENUM ('blocks', 'is_blocked_by', 'relates_to', 'duplicates');

-- Create sprints table
CREATE TABLE sprint_management.sprints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status sprint_management.sprint_status NOT NULL DEFAULT 'planned',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    capacity_points INTEGER,
    committed_points INTEGER DEFAULT 0,
    completed_points INTEGER DEFAULT 0,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    
    CONSTRAINT valid_date_range CHECK (end_date > start_date),
    CONSTRAINT valid_capacity CHECK (capacity_points IS NULL OR capacity_points >= 0),
    CONSTRAINT valid_committed_points CHECK (committed_points >= 0),
    CONSTRAINT valid_completed_points CHECK (completed_points >= 0)
);

-- Create indexes for sprints
CREATE INDEX idx_sprints_status ON sprint_management.sprints(status);
CREATE INDEX idx_sprints_dates ON sprint_management.sprints(start_date, end_date);
CREATE INDEX idx_sprints_deleted ON sprint_management.sprints(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_sprints_created_by ON sprint_management.sprints(created_by);

-- Create work_items table
CREATE TABLE sprint_management.work_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type sprint_management.work_item_type NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status sprint_management.work_item_status NOT NULL DEFAULT 'todo',
    priority sprint_management.priority_level NOT NULL DEFAULT 'medium',
    story_points INTEGER,
    assignee_id UUID,
    reporter_id UUID NOT NULL,
    parent_id UUID REFERENCES sprint_management.work_items(id) ON DELETE SET NULL,
    sprint_id UUID REFERENCES sprint_management.sprints(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    
    CONSTRAINT valid_story_points CHECK (story_points IS NULL OR story_points > 0),
    CONSTRAINT no_self_parent CHECK (parent_id IS NULL OR parent_id != id)
);

-- Create indexes for work_items
CREATE INDEX idx_work_items_type ON sprint_management.work_items(type);
CREATE INDEX idx_work_items_status ON sprint_management.work_items(status);
CREATE INDEX idx_work_items_priority ON sprint_management.work_items(priority);
CREATE INDEX idx_work_items_assignee ON sprint_management.work_items(assignee_id);
CREATE INDEX idx_work_items_reporter ON sprint_management.work_items(reporter_id);
CREATE INDEX idx_work_items_sprint ON sprint_management.work_items(sprint_id);
CREATE INDEX idx_work_items_parent ON sprint_management.work_items(parent_id);
CREATE INDEX idx_work_items_deleted ON sprint_management.work_items(deleted_at) WHERE deleted_at IS NULL;

-- Create work_item_dependencies table
CREATE TABLE sprint_management.work_item_dependencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sprint_management.work_items(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES sprint_management.work_items(id) ON DELETE CASCADE,
    dependency_type sprint_management.dependency_type NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    created_by UUID NOT NULL,
    
    CONSTRAINT no_self_dependency CHECK (source_id != target_id),
    CONSTRAINT unique_dependency UNIQUE(source_id, target_id, dependency_type)
);

-- Create indexes for work_item_dependencies
CREATE INDEX idx_dependencies_source ON sprint_management.work_item_dependencies(source_id);
CREATE INDEX idx_dependencies_target ON sprint_management.work_item_dependencies(target_id);
CREATE INDEX idx_dependencies_type ON sprint_management.work_item_dependencies(dependency_type);

-- Create trigger function to update updated_at timestamp
CREATE OR REPLACE FUNCTION sprint_management.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW() AT TIME ZONE 'UTC';
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_sprints_updated_at BEFORE UPDATE ON sprint_management.sprints
    FOR EACH ROW EXECUTE FUNCTION sprint_management.update_updated_at_column();

CREATE TRIGGER update_work_items_updated_at BEFORE UPDATE ON sprint_management.work_items
    FOR EACH ROW EXECUTE FUNCTION sprint_management.update_updated_at_column();
