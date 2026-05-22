-- +goose Up
-- +goose StatementBegin

CREATE TABLE project_sync_states (
    project_id     UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    active_run_id  VARCHAR(64) NOT NULL DEFAULT '',
    current_state  VARCHAR(20) NOT NULL DEFAULT 'idle',
    progress       DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    start_time     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_error     TEXT NOT NULL DEFAULT '',
    updated_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE file_sync_states (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    file_path    VARCHAR(1024) NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    last_indexed BIGINT NOT NULL,
    CONSTRAINT uq_file_sync_project_path UNIQUE (project_id, file_path)
);

CREATE TABLE failed_operations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    operation   VARCHAR(50) NOT NULL,
    entity_id   VARCHAR(1024) NOT NULL,
    last_error  TEXT NOT NULL DEFAULT '',
    retry_count INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_file_sync_states_project_id ON file_sync_states(project_id);
CREATE INDEX idx_failed_operations_project_id ON failed_operations(project_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS failed_operations;
DROP TABLE IF EXISTS file_sync_states;
DROP TABLE IF EXISTS project_sync_states;

-- +goose StatementEnd
