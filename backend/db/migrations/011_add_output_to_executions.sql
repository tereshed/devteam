-- +goose Up
-- +goose StatementBegin
ALTER TABLE executions ADD COLUMN output_data TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE executions DROP COLUMN output_data;
-- +goose StatementEnd

