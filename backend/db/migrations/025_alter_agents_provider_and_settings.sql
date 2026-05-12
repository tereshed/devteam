-- +goose Up
-- +goose StatementBegin

-- 15.3: расширяем agents под Sprint 15 (LLM-провайдер + per-agent code_backend настройки + sandbox permissions).
ALTER TABLE agents
    ADD COLUMN llm_provider_id        UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
    ADD COLUMN code_backend_settings  JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN sandbox_permissions    JSONB NOT NULL DEFAULT '{}';

-- расширяем допустимые значения code_backend новым вариантом для прокси.
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'claude-code-via-proxy', 'aider', 'custom'
    ));

CREATE INDEX idx_agents_llm_provider_id ON agents(llm_provider_id) WHERE llm_provider_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_agents_llm_provider_id;

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN ('claude-code', 'aider', 'custom'));

ALTER TABLE agents
    DROP COLUMN IF EXISTS sandbox_permissions,
    DROP COLUMN IF EXISTS code_backend_settings,
    DROP COLUMN IF EXISTS llm_provider_id;

-- +goose StatementEnd
