-- +goose Up
-- +goose StatementBegin
ALTER TABLE projects ADD COLUMN last_indexed_commit VARCHAR(255) NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE projects DROP COLUMN IF EXISTS last_indexed_commit;
-- +goose StatementEnd
