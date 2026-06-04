-- +goose Up
-- +goose StatementBegin

ALTER TABLE webhook_triggers ADD COLUMN task_title_template TEXT NOT NULL DEFAULT '';
ALTER TABLE webhook_triggers ADD COLUMN task_description_template TEXT NOT NULL DEFAULT '';
ALTER TABLE webhook_triggers ADD COLUMN task_priority_template TEXT NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS task_priority_template;
ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS task_description_template;
ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS task_title_template;

-- +goose StatementEnd
