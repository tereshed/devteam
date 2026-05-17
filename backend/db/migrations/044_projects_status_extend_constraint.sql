-- +goose Up
-- +goose StatementBegin

-- Расширяем chk_projects_status: исходная миграция 014 разрешала только
-- 'active','paused','archived', но models.ProjectStatus (и project_service.Create
-- при importDir != "") использует 'indexing', 'indexing_failed', 'ready' —
-- создание любого remote-проекта падало с CHECK violation → HTTP 500.

ALTER TABLE projects DROP CONSTRAINT IF EXISTS chk_projects_status;
ALTER TABLE projects ADD CONSTRAINT chk_projects_status
    CHECK (status IN ('active', 'paused', 'archived', 'indexing', 'indexing_failed', 'ready'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Сначала переводим проекты с новыми статусами в безопасный 'paused',
-- иначе ADD CONSTRAINT упадёт из-за нарушения проверки на существующих строках.
UPDATE projects
   SET status = 'paused'
 WHERE status IN ('indexing', 'indexing_failed', 'ready');

ALTER TABLE projects DROP CONSTRAINT IF EXISTS chk_projects_status;
ALTER TABLE projects ADD CONSTRAINT chk_projects_status
    CHECK (status IN ('active', 'paused', 'archived'));

-- +goose StatementEnd
