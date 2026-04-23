-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents ADD COLUMN requires_code_context BOOLEAN DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents DROP COLUMN requires_code_context;
-- +goose StatementEnd
