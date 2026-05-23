-- +goose Up
-- +goose StatementBegin
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_project_id_key;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS chk_teams_type;
ALTER TABLE teams ADD CONSTRAINT chk_teams_type CHECK (type IN ('development', 'research', 'analytics'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Note: Re-adding the UNIQUE constraint might fail if there are duplicate teams, but this is standard Down migration.
ALTER TABLE teams ADD CONSTRAINT teams_project_id_key UNIQUE (project_id);
ALTER TABLE teams DROP CONSTRAINT IF EXISTS chk_teams_type;
ALTER TABLE teams ADD CONSTRAINT chk_teams_type CHECK (type IN ('development'));
-- +goose StatementEnd
