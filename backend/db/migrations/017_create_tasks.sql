-- +goose Up
-- +goose StatementBegin

CREATE TABLE tasks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id        UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_task_id    UUID REFERENCES tasks(id) ON DELETE SET NULL,
    title             VARCHAR(500) NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    status            VARCHAR(50) NOT NULL DEFAULT 'pending',
    priority          VARCHAR(50) NOT NULL DEFAULT 'medium',
    assigned_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    created_by_type   VARCHAR(50) NOT NULL,
    created_by_id     UUID NOT NULL,
    context           JSONB NOT NULL DEFAULT '{}',
    result            TEXT,
    artifacts         JSONB NOT NULL DEFAULT '{}',
    branch_name       VARCHAR(255),
    error_message     TEXT,
    started_at        TIMESTAMP WITH TIME ZONE,
    completed_at      TIMESTAMP WITH TIME ZONE,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_tasks_status CHECK (status IN (
        'pending', 'planning', 'in_progress', 'review',
        'changes_requested', 'testing', 'completed',
        'failed', 'cancelled', 'paused'
    )),
    CONSTRAINT chk_tasks_priority CHECK (priority IN (
        'critical', 'high', 'medium', 'low'
    )),
    CONSTRAINT chk_tasks_created_by_type CHECK (created_by_type IN (
        'user', 'agent'
    ))
);

CREATE INDEX idx_tasks_project_id ON tasks(project_id);
CREATE INDEX idx_tasks_parent_task_id ON tasks(parent_task_id);
CREATE INDEX idx_tasks_assigned_agent_id ON tasks(assigned_agent_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_priority ON tasks(priority);
CREATE INDEX idx_tasks_created_by ON tasks(created_by_type, created_by_id);
CREATE INDEX idx_tasks_branch_name ON tasks(branch_name) WHERE branch_name IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tasks_branch_name;
DROP INDEX IF EXISTS idx_tasks_created_by;
DROP INDEX IF EXISTS idx_tasks_priority;
DROP INDEX IF EXISTS idx_tasks_project_status;
DROP INDEX IF EXISTS idx_tasks_status;
DROP INDEX IF EXISTS idx_tasks_assigned_agent_id;
DROP INDEX IF EXISTS idx_tasks_parent_task_id;
DROP INDEX IF EXISTS idx_tasks_project_id;
DROP TABLE IF EXISTS tasks;

-- +goose StatementEnd
