-- Create projects table
CREATE TABLE IF NOT EXISTS sprint_management.projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    current_counter INTEGER NOT NULL DEFAULT 0,
    starting_number INTEGER NOT NULL DEFAULT 1,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT projects_key_uppercase CHECK (key = UPPER(key)),
    CONSTRAINT projects_key_length CHECK (LENGTH(key) >= 3 AND LENGTH(key) <= 10),
    CONSTRAINT projects_key_format CHECK (key ~ '^[A-Z][A-Z0-9]*$'),
    CONSTRAINT projects_starting_number_positive CHECK (starting_number >= 1),
    CONSTRAINT projects_current_counter_valid CHECK (current_counter >= 0)
);

-- Create index on key for fast lookups
CREATE INDEX IF NOT EXISTS idx_projects_key ON sprint_management.projects(key);

-- Add project_id to work_items table
ALTER TABLE sprint_management.work_items 
ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES sprint_management.projects(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS ticket_number INTEGER;

-- Create unique constraint on project_id + ticket_number
CREATE UNIQUE INDEX IF NOT EXISTS idx_work_items_project_ticket ON sprint_management.work_items(project_id, ticket_number) 
WHERE project_id IS NOT NULL AND ticket_number IS NOT NULL;

-- Create index for faster queries
CREATE INDEX IF NOT EXISTS idx_work_items_project_id ON sprint_management.work_items(project_id);

-- Add function to get next ticket number for a project
CREATE OR REPLACE FUNCTION sprint_management.get_next_ticket_number(p_project_id UUID)
RETURNS INTEGER AS $$
DECLARE
    next_number INTEGER;
BEGIN
    UPDATE sprint_management.projects 
    SET current_counter = current_counter + 1,
        updated_at = NOW()
    WHERE id = p_project_id
    RETURNING current_counter INTO next_number;
    
    RETURN next_number;
END;
$$ LANGUAGE plpgsql;

-- Add trigger to auto-assign ticket numbers
CREATE OR REPLACE FUNCTION sprint_management.assign_ticket_number()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.project_id IS NOT NULL AND NEW.ticket_number IS NULL THEN
        NEW.ticket_number := sprint_management.get_next_ticket_number(NEW.project_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS work_items_assign_ticket_number ON sprint_management.work_items;
CREATE TRIGGER work_items_assign_ticket_number
BEFORE INSERT ON sprint_management.work_items
FOR EACH ROW
EXECUTE FUNCTION sprint_management.assign_ticket_number();

-- Add comments
COMMENT ON TABLE sprint_management.projects IS 'Projects for organizing work items';
COMMENT ON COLUMN sprint_management.projects.key IS 'Unique uppercase project key (e.g., DND, SPRINT)';
COMMENT ON COLUMN sprint_management.projects.current_counter IS 'Current ticket counter for sequential numbering';
COMMENT ON COLUMN sprint_management.projects.starting_number IS 'Starting number for ticket sequence';
COMMENT ON COLUMN sprint_management.work_items.project_id IS 'Reference to parent project';
COMMENT ON COLUMN sprint_management.work_items.ticket_number IS 'Sequential ticket number within project';
