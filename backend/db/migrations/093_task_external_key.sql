-- +goose Up
-- +goose StatementBegin

-- external_key — внешний ключ тикета задачи (напр. DEV-123 из трекера). Опционален;
-- становится обязательным только если шаблон имени ветки проекта содержит {ticket}
-- без fallback (проверяется на уровне приложения, fail-loud в task_service.Create).
-- Используется плейсхолдером {ticket} при генерации git-ветки.
-- CHECK — безопасный floor (git-ref/shell-safe, без ведущего '-'); строгая конвенция
-- формата валидируется в Go (ValidateExternalKey), чтобы её можно было эволюционировать.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS external_key VARCHAR(64);

ALTER TABLE tasks ADD CONSTRAINT chk_tasks_external_key_safe
    CHECK (external_key IS NULL OR external_key ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$');

CREATE INDEX IF NOT EXISTS idx_tasks_external_key ON tasks(external_key) WHERE external_key IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tasks_external_key;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_external_key_safe;
ALTER TABLE tasks DROP COLUMN IF EXISTS external_key;

-- +goose StatementEnd
