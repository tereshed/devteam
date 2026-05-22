-- +goose Up
-- +goose StatementBegin

-- 1) Добавляем 'antigravity' в чек-констреинты user_llm_credentials
ALTER TABLE user_llm_credentials DROP CONSTRAINT IF EXISTS chk_user_llm_credentials_provider;
ALTER TABLE user_llm_credentials ADD CONSTRAINT chk_user_llm_credentials_provider
    CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity'
    ));

ALTER TABLE user_llm_credential_audit DROP CONSTRAINT IF EXISTS chk_user_llm_credential_audit_provider;
ALTER TABLE user_llm_credential_audit ADD CONSTRAINT chk_user_llm_credential_audit_provider
    CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity'
    ));

-- 2) Добавляем 'antigravity' и 'antigravity_oauth' в чек-констреинт llm_providers
ALTER TABLE llm_providers DROP CONSTRAINT IF EXISTS chk_llm_providers_kind;
ALTER TABLE llm_providers ADD CONSTRAINT chk_llm_providers_kind
    CHECK (kind IN (
        'anthropic', 'anthropic_oauth', 'openai', 'gemini', 'deepseek', 'qwen', 'openrouter', 'moonshot', 'ollama', 'zhipu', 'free_claude_proxy', 'antigravity', 'antigravity_oauth'
    ));

-- 3) Добавляем 'antigravity' и 'antigravity_oauth' в чек-констреинт agents
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_provider_kind;
ALTER TABLE agents ADD CONSTRAINT chk_agents_provider_kind
    CHECK (provider_kind IS NULL OR provider_kind IN (
        'anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'openrouter', 'antigravity', 'antigravity_oauth'
    ));

-- 4) Создаем таблицу antigravity_subscriptions
CREATE TABLE antigravity_subscriptions (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    oauth_access_token_enc   BYTEA NOT NULL,
    oauth_refresh_token_enc  BYTEA,
    token_type               VARCHAR(32) NOT NULL DEFAULT 'Bearer',
    scopes                   TEXT NOT NULL DEFAULT '',
    expires_at               TIMESTAMP WITH TIME ZONE,
    last_refreshed_at        TIMESTAMP WITH TIME ZONE,
    created_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_antigravity_subscriptions_user UNIQUE (user_id),
    CONSTRAINT chk_antigravity_subscriptions_token_type CHECK (token_type IN ('Bearer'))
);

CREATE INDEX idx_antigravity_subscriptions_expires_at
    ON antigravity_subscriptions(expires_at)
    WHERE expires_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_antigravity_subscriptions_expires_at;
DROP TABLE IF EXISTS antigravity_subscriptions;

-- Восстанавливаем старые констреинты для agents (удаляя записи с antigravity сначала)
UPDATE agents SET provider_kind = NULL WHERE provider_kind IN ('antigravity', 'antigravity_oauth');
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_provider_kind;
ALTER TABLE agents ADD CONSTRAINT chk_agents_provider_kind
    CHECK (provider_kind IS NULL OR provider_kind IN (
        'anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'openrouter'
    ));

-- Восстанавливаем старые констреинты для llm_providers
DELETE FROM llm_providers WHERE kind IN ('antigravity', 'antigravity_oauth');
ALTER TABLE llm_providers DROP CONSTRAINT IF EXISTS chk_llm_providers_kind;
ALTER TABLE llm_providers ADD CONSTRAINT chk_llm_providers_kind
    CHECK (kind IN (
        'anthropic', 'anthropic_oauth', 'openai', 'gemini', 'deepseek', 'qwen', 'openrouter', 'moonshot', 'ollama', 'zhipu', 'free_claude_proxy'
    ));

-- Восстанавливаем старые констреинты для user_llm_credentials / audit
DELETE FROM user_llm_credential_audit WHERE provider = 'antigravity';
DELETE FROM user_llm_credentials WHERE provider = 'antigravity';

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
