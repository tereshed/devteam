-- +goose Up
-- +goose StatementBegin

-- Sprint 15.e2e refactor: убираем shared-resource free-claude-proxy, переходим
-- на native Anthropic endpoint провайдеров + per-user creds.
--
-- 1) Добавляем agent.provider_kind — какой провайдер использует агент.
--    Резолвер по этому полю выбирает base_url и берёт ключ из user_llm_credentials
--    (или OAuth-токен из claude_code_subscriptions для anthropic_oauth).
--
-- 2) Схлопываем code_backend: claude-code-via-proxy больше не нужен.
--    Прокси-логика была эквивалентна установке ANTHROPIC_BASE_URL по non-anthropic
--    провайдеру — это теперь делает резолвер по provider_kind.
--
-- 3) Расширяем чек на user_llm_credentials — добавляем zhipu.

-- (1) provider_kind на agents
ALTER TABLE agents
    ADD COLUMN provider_kind VARCHAR(32);

ALTER TABLE agents ADD CONSTRAINT chk_agents_provider_kind
    CHECK (provider_kind IS NULL OR provider_kind IN (
        'anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'openrouter'
    ));

CREATE INDEX idx_agents_provider_kind ON agents(provider_kind) WHERE provider_kind IS NOT NULL;

-- (2) Мигрируем существующие записи и сужаем enum
UPDATE agents SET code_backend = 'claude-code'
    WHERE code_backend = 'claude-code-via-proxy';

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'custom'
    ));

-- (3) Добавляем zhipu в чек user_llm_credentials и audit-таблицы
ALTER TABLE user_llm_credentials DROP CONSTRAINT IF EXISTS chk_user_llm_credentials_provider;
ALTER TABLE user_llm_credentials ADD CONSTRAINT chk_user_llm_credentials_provider
    CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu'
    ));

ALTER TABLE user_llm_credential_audit DROP CONSTRAINT IF EXISTS chk_user_llm_credential_audit_provider;
ALTER TABLE user_llm_credential_audit ADD CONSTRAINT chk_user_llm_credential_audit_provider
    CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- (3-down) Сужаем чек user_llm_credentials обратно — удалив сначала строки с zhipu.
DELETE FROM user_llm_credential_audit WHERE provider = 'zhipu';
DELETE FROM user_llm_credentials WHERE provider = 'zhipu';

ALTER TABLE user_llm_credentials DROP CONSTRAINT IF EXISTS chk_user_llm_credentials_provider;
ALTER TABLE user_llm_credentials ADD CONSTRAINT chk_user_llm_credentials_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter'));

ALTER TABLE user_llm_credential_audit DROP CONSTRAINT IF EXISTS chk_user_llm_credential_audit_provider;
ALTER TABLE user_llm_credential_audit ADD CONSTRAINT chk_user_llm_credential_audit_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter'));

-- (2-down) Возвращаем claude-code-via-proxy в enum code_backend.
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'claude-code-via-proxy', 'aider', 'custom'
    ));

-- (1-down) Удаляем колонку provider_kind.
DROP INDEX IF EXISTS idx_agents_provider_kind;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_provider_kind;
ALTER TABLE agents DROP COLUMN IF EXISTS provider_kind;

-- +goose StatementEnd
