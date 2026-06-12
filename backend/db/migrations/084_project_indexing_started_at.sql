-- +goose Up
-- +goose StatementBegin

-- Маркер старта индексации для recovery осиротевшего status='indexing' (процесс
-- умер посреди индексации — финальный CAS indexing→ready/failed не выполнился).
-- Отдельная колонка, а не updated_at: updated_at освежается любым full-row
-- Update настроек проекта и потому непригодна как мера давности индексации.
-- NULL = legacy-строка (переход в indexing был до этой миграции) — recovery
-- для таких падает обратно на updated_at.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS indexing_started_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS indexing_started_at;

-- +goose StatementEnd
