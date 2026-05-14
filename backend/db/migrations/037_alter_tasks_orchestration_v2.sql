-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — добавляем новые поля задачи под LLM-driven orchestrator.
--
-- ВАЖНО: эта миграция НЕ дропает legacy-колонку `status`. Параллельная жизнь:
--   * `state` — новый источник правды (active|done|failed|cancelled|needs_human),
--     заполняется backfill'ом по существующим status.
--   * `status` — остаётся для обратной совместимости с orchestrator_pipeline.go,
--     handle*, task_repository.go и т.д. Удалится в Sprint 3 (миграция 038)
--     одним PR-ом вместе с удалением legacy-оркестратора.
--
-- Это даёт чистый чек-пойнт: Sprint 1 оставляет проект компилирующимся,
-- старая система работает, новая инфраструктура (artifacts, task_events, worktrees)
-- готовится параллельно. Sprint 3 переключает рубильник.
--
-- Поля для новой модели:
--   state             — упрощённое жизненное состояние задачи (5 значений).
--   cancel_requested  — кооперативная отмена; Step и Agent Worker'ы поллят её.
--   current_step_no   — счётчик шагов оркестратора (для max_steps_per_task).
--   custom_timeout    — override дефолтного task_timeout (per-task).
--   locked_by/locked_at — observability + детект "застрявших" Step-обработок.

ALTER TABLE tasks
    ADD COLUMN state             VARCHAR(32),
    ADD COLUMN cancel_requested  BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN current_step_no   INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN custom_timeout    INTERVAL,
    ADD COLUMN locked_by         VARCHAR(255),
    ADD COLUMN locked_at         TIMESTAMP WITH TIME ZONE;

-- Backfill: маппинг старых status → новых state.
UPDATE tasks
   SET state = CASE
           WHEN status = 'completed' THEN 'done'
           WHEN status = 'failed'    THEN 'failed'
           WHEN status = 'cancelled' THEN 'cancelled'
           WHEN status = 'paused'    THEN 'needs_human'
           ELSE 'active'  -- pending|planning|in_progress|review|changes_requested|testing
       END
 WHERE state IS NULL;

ALTER TABLE tasks ALTER COLUMN state SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN state SET DEFAULT 'active';

ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_state
        CHECK (state IN ('active', 'done', 'failed', 'cancelled', 'needs_human'));

ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_current_step_non_negative
        CHECK (current_step_no >= 0);

ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_custom_timeout_positive
        CHECK (custom_timeout IS NULL OR custom_timeout > interval '0');

ALTER TABLE tasks
    ADD CONSTRAINT chk_tasks_lock_consistency
        CHECK (
            (locked_by IS NULL AND locked_at IS NULL) OR
            (locked_by IS NOT NULL AND locked_at IS NOT NULL)
        );

CREATE INDEX idx_tasks_state          ON tasks(state);
CREATE INDEX idx_tasks_project_state  ON tasks(project_id, state);
CREATE INDEX idx_tasks_active_cancel  ON tasks(cancel_requested)
    WHERE state = 'active' AND cancel_requested = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_tasks_active_cancel;
DROP INDEX IF EXISTS idx_tasks_project_state;
DROP INDEX IF EXISTS idx_tasks_state;

ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_lock_consistency;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_custom_timeout_positive;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_current_step_non_negative;
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_tasks_state;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS locked_at,
    DROP COLUMN IF EXISTS locked_by,
    DROP COLUMN IF EXISTS custom_timeout,
    DROP COLUMN IF EXISTS current_step_no,
    DROP COLUMN IF EXISTS cancel_requested,
    DROP COLUMN IF EXISTS state;

-- +goose StatementEnd
