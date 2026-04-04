-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX uq_projects_user_id_name ON projects (user_id, name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uq_projects_user_id_name;
-- +goose StatementEnd
