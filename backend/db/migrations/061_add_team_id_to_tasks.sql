-- +goose Up
-- +goose StatementBegin
ALTER TABLE tasks ADD COLUMN team_id UUID;
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_team FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE SET NULL;
CREATE INDEX idx_tasks_team_id ON tasks(team_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS fk_tasks_team;
ALTER TABLE tasks DROP COLUMN IF EXISTS team_id;
-- +goose StatementEnd
