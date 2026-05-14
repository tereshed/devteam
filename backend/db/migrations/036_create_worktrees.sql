-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — git worktree-изоляция для параллельных sandbox-агентов.
--
-- БЕЗОПАСНОСТЬ:
--   * Колонка path НАМЕРЕННО ОТСУТСТВУЕТ. Путь к worktree всегда вычисляется в Go
--     через filepath.Join(cfg.WorktreesRoot, task_id.String(), id.String()).
--     Это исключает path traversal через подмену БД (UUID.String() формирует
--     строго "8-4-4-4-12" без / и .. ).
--   * branch_name формируется backend'ом по шаблону "task-<task_uuid>-wt-<wt_uuid>",
--     валидируется regex'ом перед записью; не приходит от пользователя/LLM.
--   * base_branch берётся из конфига проекта, валидируется branch_validator.go
--     (regex + отказ при ведущем "-" для git-injection защиты).

CREATE TABLE worktrees (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id       UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    subtask_id    UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    base_branch   VARCHAR(128) NOT NULL,
    branch_name   VARCHAR(128) NOT NULL,
    state         VARCHAR(16) NOT NULL DEFAULT 'allocated',
    agent_job_id  BIGINT REFERENCES task_events(id) ON DELETE SET NULL,
    allocated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    released_at   TIMESTAMP WITH TIME ZONE,

    CONSTRAINT chk_worktrees_state
        CHECK (state IN ('allocated', 'in_use', 'released')),

    -- Формат имени ветки строго детерминирован backend'ом — никаких LLM-данных.
    CONSTRAINT chk_worktrees_branch_name_format
        CHECK (branch_name ~ '^task-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-wt-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'),

    -- base_branch не должен начинаться с '-' (защита от git-флаг-injection)
    -- и должен содержать только безопасные символы.
    CONSTRAINT chk_worktrees_base_branch_safe
        CHECK (base_branch ~ '^[a-zA-Z0-9._/][a-zA-Z0-9._/-]{0,127}$'),

    CONSTRAINT chk_worktrees_released_after_allocated
        CHECK (released_at IS NULL OR released_at >= allocated_at)
);

CREATE INDEX idx_worktrees_task              ON worktrees(task_id);
CREATE INDEX idx_worktrees_state             ON worktrees(state);
CREATE INDEX idx_worktrees_released_cleanup  ON worktrees(released_at)
    WHERE state = 'released' AND released_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
DROP TABLE IF EXISTS worktrees;

-- +goose StatementEnd
