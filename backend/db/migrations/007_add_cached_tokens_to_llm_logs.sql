-- +goose Up
-- +goose StatementBegin
ALTER TABLE llm_logs ADD COLUMN cached_tokens INT DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE llm_logs DROP COLUMN cached_tokens;
-- +goose StatementEnd

