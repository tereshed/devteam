-- +goose Up
-- +goose StatementBegin
ALTER TABLE projects ADD CONSTRAINT uq_projects_user_id_name UNIQUE (user_id, name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE projects DROP CONSTRAINT IF EXISTS uq_projects_user_id_name;
-- +goose StatementEnd
