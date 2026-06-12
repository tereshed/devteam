-- +goose Up
-- +goose StatementBegin

-- enhancer_runs — журнал прогонов энхансера. Один прогон = один вызов
-- LLM-агент-петли (agentloop) с read-инструментами по истории проекта и
-- write-инструментом enhancer_propose_change. Итоговый отчёт агента — в report.
CREATE TABLE IF NOT EXISTS enhancer_runs (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- config_id — NULL после удаления конфига; история прогонов остаётся.
    config_id    UUID         REFERENCES enhancer_configs(id) ON DELETE SET NULL,
    trigger_kind VARCHAR(16)  NOT NULL DEFAULT 'manual',
    status       VARCHAR(16)  NOT NULL DEFAULT 'running',
    -- report — итоговый отчёт агента (markdown): что проанализировано, какие
    -- проблемы найдены, почему предложены (или не предложены) изменения.
    report       TEXT         NOT NULL DEFAULT '',
    error        TEXT         NOT NULL DEFAULT '',
    started_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ,
    CONSTRAINT chk_enhancer_run_trigger CHECK (trigger_kind IN ('manual', 'cron')),
    CONSTRAINT chk_enhancer_run_status CHECK (status IN ('running', 'done', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_enhancer_runs_project ON enhancer_runs(project_id, started_at DESC);

-- enhancer_changes — предложения изменений, рождённые прогоном. Полный жизненный
-- цикл статусов заложен сразу (фаза 2 добавит apply/reject без правки схемы):
-- proposed → approved → applied | rejected | rolled_back. В фазе 1 всё остаётся
-- в proposed. payload — самодостаточный дифф {old, new, ...} под target_kind.
CREATE TABLE IF NOT EXISTS enhancer_changes (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id          UUID         NOT NULL REFERENCES enhancer_runs(id) ON DELETE CASCADE,
    project_id      UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- target_kind: agent_override — проектный оверрайд промпта/настроек агента
    -- (project_agent_overrides появятся в фазе 2; предложения копим уже сейчас);
    -- project_description / project_settings — правки самого проекта.
    target_kind     VARCHAR(32)  NOT NULL,
    -- target_agent_id — только для target_kind=agent_override.
    target_agent_id UUID         REFERENCES agents(id) ON DELETE CASCADE,
    payload         JSONB        NOT NULL DEFAULT '{}',
    -- reason — на каких наблюдениях основано предложение (с ссылками на задачи).
    reason          TEXT         NOT NULL DEFAULT '',
    -- expected_effect — какой измеримый эффект ожидается (для замера в фазе 3).
    expected_effect TEXT         NOT NULL DEFAULT '',
    status          VARCHAR(16)  NOT NULL DEFAULT 'proposed',
    decided_by      UUID         REFERENCES users(id) ON DELETE SET NULL,
    decided_at      TIMESTAMPTZ,
    applied_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_enhancer_change_kind CHECK (target_kind IN ('agent_override', 'project_description', 'project_settings')),
    CONSTRAINT chk_enhancer_change_status CHECK (status IN ('proposed', 'approved', 'applied', 'rejected', 'rolled_back'))
);

CREATE INDEX IF NOT EXISTS idx_enhancer_changes_run ON enhancer_changes(run_id);
CREATE INDEX IF NOT EXISTS idx_enhancer_changes_project ON enhancer_changes(project_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS enhancer_changes;
DROP TABLE IF EXISTS enhancer_runs;

-- +goose StatementEnd
