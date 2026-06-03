-- +goose Up
-- +goose StatementBegin

-- 1. Убираем NOT NULL у workflow_name (сохраняя существующие данные)
ALTER TABLE webhook_triggers ALTER COLUMN workflow_name DROP NOT NULL;

-- 2. Добавляем новые колонки в webhook_triggers
ALTER TABLE webhook_triggers ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE webhook_triggers ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE webhook_triggers ADD COLUMN instructions TEXT NOT NULL DEFAULT '';

-- 3. Добавляем conversation_id в webhook_logs (для связи с созданным чатом)
ALTER TABLE webhook_logs ADD COLUMN conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL;

-- 4. Добавляем check constraint (должен быть либо workflow_name, либо project_id)
ALTER TABLE webhook_triggers ADD CONSTRAINT chk_webhook_target CHECK (
    (workflow_name IS NOT NULL AND workflow_name != '') OR project_id IS NOT NULL
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE webhook_triggers DROP CONSTRAINT IF EXISTS chk_webhook_target;

ALTER TABLE webhook_logs DROP COLUMN IF EXISTS conversation_id;

ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS instructions;
ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS team_id;
ALTER TABLE webhook_triggers DROP COLUMN IF EXISTS project_id;

-- Восстанавливаем NOT NULL. Важно: если есть записи с NULL, это упадет.
-- Перед этим нужно либо удалить записи с NULL, либо заполнить их дефолтом.
UPDATE webhook_triggers SET workflow_name = 'default' WHERE workflow_name IS NULL;
ALTER TABLE webhook_triggers ALTER COLUMN workflow_name SET NOT NULL;

-- +goose StatementEnd
