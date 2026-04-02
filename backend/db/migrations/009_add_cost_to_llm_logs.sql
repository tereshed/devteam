-- +goose Up
-- +goose StatementBegin
ALTER TABLE llm_logs ADD COLUMN cost NUMERIC(20, 10) DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE llm_logs DROP COLUMN cost;
-- +goose StatementEnd

