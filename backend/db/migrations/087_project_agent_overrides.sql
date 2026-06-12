-- +goose Up
-- +goose StatementBegin

-- project_agent_overrides — проектные оверрайды промптов агентов (фаза 2
-- энхансера). Строка = материализованная свёртка ВСЕХ применённых
-- enhancer_changes вида agent_override для пары (проект, агент): apply и
-- rollback пересобирают prompt_addendum заново из applied-предложений
-- (конкатенация по applied_at). ContextBuilder дописывает активный addendum
-- к системному промпту агента при исполнении задач ЭТОГО проекта; глобальный
-- промпт агента не меняется — blast radius ограничен проектом, откат =
-- пересборка без отозванного предложения.
CREATE TABLE IF NOT EXISTS project_agent_overrides (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    agent_id        UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    prompt_addendum TEXT        NOT NULL DEFAULT '',
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    updated_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_project_agent_override UNIQUE (project_id, agent_id)
);

-- Покрывает lookup в ContextBuilder на каждом исполнении агента.
CREATE INDEX IF NOT EXISTS idx_project_agent_overrides_lookup
    ON project_agent_overrides(project_id, agent_id) WHERE is_active;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS project_agent_overrides;

-- +goose StatementEnd
