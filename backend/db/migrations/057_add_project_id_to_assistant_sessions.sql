-- +goose Up
-- +goose StatementBegin
ALTER TABLE assistant_sessions ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_assistant_sessions_project ON assistant_sessions(project_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE assistant_sessions DROP COLUMN IF EXISTS project_id;
-- +goose StatementEnd
