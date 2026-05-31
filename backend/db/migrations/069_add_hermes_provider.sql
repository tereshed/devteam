-- +goose Up
-- +goose StatementBegin

ALTER TABLE user_llm_credentials DROP CONSTRAINT IF EXISTS chk_user_llm_credentials_provider;
ALTER TABLE user_llm_credentials ADD CONSTRAINT chk_user_llm_credentials_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity', 'hermes'));

ALTER TABLE user_llm_credential_audit DROP CONSTRAINT IF EXISTS chk_user_llm_credential_audit_provider;
ALTER TABLE user_llm_credential_audit ADD CONSTRAINT chk_user_llm_credential_audit_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity', 'hermes'));

ALTER TABLE llm_providers DROP CONSTRAINT IF EXISTS chk_llm_providers_kind;
ALTER TABLE llm_providers ADD CONSTRAINT chk_llm_providers_kind
    CHECK (kind IN ('anthropic', 'anthropic_oauth', 'openai', 'gemini', 'deepseek', 'qwen', 'openrouter', 'moonshot', 'ollama', 'zhipu', 'free_claude_proxy', 'antigravity', 'antigravity_oauth', 'hermes'));

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_provider_kind;
ALTER TABLE agents ADD CONSTRAINT chk_agents_provider_kind
    CHECK (provider_kind IS NULL OR provider_kind IN (
        'anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'openrouter', 'antigravity', 'antigravity_oauth', 'hermes'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE agents SET provider_kind = NULL WHERE provider_kind = 'hermes';
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_provider_kind;
ALTER TABLE agents ADD CONSTRAINT chk_agents_provider_kind
    CHECK (provider_kind IS NULL OR provider_kind IN (
        'anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'openrouter', 'antigravity', 'antigravity_oauth'
    ));

DELETE FROM llm_providers WHERE kind = 'hermes';
ALTER TABLE llm_providers DROP CONSTRAINT IF EXISTS chk_llm_providers_kind;
ALTER TABLE llm_providers ADD CONSTRAINT chk_llm_providers_kind
    CHECK (kind IN ('anthropic', 'anthropic_oauth', 'openai', 'gemini', 'deepseek', 'qwen', 'openrouter', 'moonshot', 'ollama', 'zhipu', 'free_claude_proxy', 'antigravity', 'antigravity_oauth'));

DELETE FROM user_llm_credential_audit WHERE provider = 'hermes';
DELETE FROM user_llm_credentials WHERE provider = 'hermes';

ALTER TABLE user_llm_credentials DROP CONSTRAINT IF EXISTS chk_user_llm_credentials_provider;
ALTER TABLE user_llm_credentials ADD CONSTRAINT chk_user_llm_credentials_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity'));

ALTER TABLE user_llm_credential_audit DROP CONSTRAINT IF EXISTS chk_user_llm_credential_audit_provider;
ALTER TABLE user_llm_credential_audit ADD CONSTRAINT chk_user_llm_credential_audit_provider
    CHECK (provider IN ('openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter', 'zhipu', 'antigravity'));

-- +goose StatementEnd
