-- +goose Up
-- +goose StatementBegin

-- scout_runs — один прогон разведчика: headless sandbox-исполнение Claude Code
-- CLI на подписке, которое читает репозитории проекта и собирает досье контекста
-- (dossier). session_id/tool_call_id связывают прогон с распарканной сессией
-- ассистента для wake-up (фаза 2; в фазе 1 NULL — ручной/API-запуск). dossier
-- хранится в строке (артефакты в проекте task-scoped, у скаута задачи нет).
CREATE TABLE IF NOT EXISTS scout_runs (
    id                  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- created_by — кто инициировал прогон; от его имени резолвится подписка.
    created_by          UUID         REFERENCES users(id) ON DELETE SET NULL,
    -- session_id / tool_call_id — фаза 2: распарканная сессия ассистента и
    -- tool_call, который закрывается досье при завершении прогона (wake-up).
    session_id          UUID,
    tool_call_id        VARCHAR(128),
    status              VARCHAR(16)  NOT NULL DEFAULT 'running',
    code_backend        VARCHAR(32)  NOT NULL DEFAULT 'claude-code',
    -- problem — постановка проблемы, с которой пришёл пользователь (вход разведки).
    problem             TEXT         NOT NULL DEFAULT '',
    -- dossier — собранное досье (выход разведки); пусто пока running/failed.
    dossier             TEXT         NOT NULL DEFAULT '',
    error               TEXT         NOT NULL DEFAULT '',
    sandbox_instance_id VARCHAR(128) NOT NULL DEFAULT '',
    started_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_scout_run_status CHECK (status IN ('running', 'done', 'failed'))
);

-- Лента прогонов проекта (новые сверху).
CREATE INDEX IF NOT EXISTS idx_scout_runs_project ON scout_runs(project_id, started_at DESC);
-- Фаза 2: поиск распарканного прогона по сессии ассистента для wake-up.
CREATE INDEX IF NOT EXISTS idx_scout_runs_session ON scout_runs(session_id) WHERE session_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS scout_runs;

-- +goose StatementEnd
