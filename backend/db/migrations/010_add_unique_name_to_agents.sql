-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents ADD CONSTRAINT agents_name_key UNIQUE (name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents DROP CONSTRAINT agents_name_key;
-- +goose StatementEnd

