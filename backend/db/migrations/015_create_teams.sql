-- +goose Up
-- +goose StatementBegin
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    project_id  UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    type        VARCHAR(50) NOT NULL DEFAULT 'development',
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_teams_type CHECK (type IN ('development'))
);

CREATE INDEX idx_teams_project_id ON teams(project_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_teams_project_id;
DROP TABLE IF EXISTS teams;
-- +goose StatementEnd
