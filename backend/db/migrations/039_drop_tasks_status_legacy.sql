-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — финальное удаление legacy-колонки tasks.status.
--
-- К моменту применения этой миграции:
--   * tasks.state (миграция 037) — populated, единственный источник правды.
--   * orchestrator_pipeline.go удалён (миграция кода).
--   * TaskStatus enum + Task.Status поле удалены из Go-моделей.
--   * Все consumers (task_service, handler, mcp, ws, indexer, eventbus, dto) переведены на TaskState.

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_status;
DROP INDEX IF EXISTS idx_tasks_project_status;
DROP INDEX IF EXISTS idx_tasks_status;
ALTER TABLE tasks DROP COLUMN IF EXISTS status;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Воссоздаём legacy-колонку с маппингом из state. Полный rollback невозможен
-- (planning/in_progress/review/testing все сходятся в state='active' → status='pending').
ALTER TABLE tasks ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'pending';

UPDATE tasks
   SET status = CASE
           WHEN state = 'done'        THEN 'completed'
           WHEN state = 'failed'      THEN 'failed'
           WHEN state = 'cancelled'   THEN 'cancelled'
           WHEN state = 'needs_human' THEN 'paused'
           ELSE 'pending'
       END;

ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_status CHECK (status IN (
        'pending', 'planning', 'in_progress', 'review',
        'changes_requested', 'testing', 'completed',
        'failed', 'cancelled', 'paused'
    ));

CREATE INDEX idx_tasks_status         ON tasks(status);
CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);

-- +goose StatementEnd
