-- Create auth.users table
CREATE TABLE auth.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    keycloak_id VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    profile_picture_url TEXT,
    roles TEXT[] DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC')
);

-- Create indexes for auth.users
CREATE INDEX idx_users_keycloak ON auth.users(keycloak_id);
CREATE INDEX idx_users_username ON auth.users(username);
CREATE INDEX idx_users_email ON auth.users(email);
CREATE INDEX idx_users_is_active ON auth.users(is_active);

-- Create comments table
CREATE TABLE sprint_management.comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    work_item_id UUID NOT NULL REFERENCES sprint_management.work_items(id) ON DELETE CASCADE,
    author_id UUID NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    
    CONSTRAINT valid_content CHECK (LENGTH(TRIM(content)) > 0)
);

-- Create indexes for comments
CREATE INDEX idx_comments_work_item ON sprint_management.comments(work_item_id);
CREATE INDEX idx_comments_author ON sprint_management.comments(author_id);
CREATE INDEX idx_comments_deleted ON sprint_management.comments(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_comments_created_at ON sprint_management.comments(created_at DESC);

-- Create activity_logs table
CREATE TABLE sprint_management.activity_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL,
    changes JSONB,
    trace_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() AT TIME ZONE 'UTC'),
    
    CONSTRAINT valid_entity_type CHECK (entity_type IN ('work_item', 'sprint', 'comment', 'dependency')),
    CONSTRAINT valid_action CHECK (action IN ('created', 'updated', 'deleted', 'status_changed', 'assigned', 'unassigned', 'commented', 'restored'))
);

-- Create indexes for activity_logs
CREATE INDEX idx_activity_entity ON sprint_management.activity_logs(entity_type, entity_id);
CREATE INDEX idx_activity_user ON sprint_management.activity_logs(user_id);
CREATE INDEX idx_activity_trace ON sprint_management.activity_logs(trace_id);
CREATE INDEX idx_activity_created ON sprint_management.activity_logs(created_at DESC);
CREATE INDEX idx_activity_action ON sprint_management.activity_logs(action);

-- Create trigger function for auth.users updated_at
CREATE OR REPLACE FUNCTION auth.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW() AT TIME ZONE 'UTC';
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for auth.users
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON auth.users
    FOR EACH ROW EXECUTE FUNCTION auth.update_updated_at_column();

-- Create trigger for comments
CREATE TRIGGER update_comments_updated_at BEFORE UPDATE ON sprint_management.comments
    FOR EACH ROW EXECUTE FUNCTION sprint_management.update_updated_at_column();
