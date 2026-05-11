-- +goose Up
-- +goose StatementBegin

CREATE TABLE user_llm_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(32) NOT NULL,
    encrypted_key   BYTEA NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_user_llm_credentials_provider CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter'
    )),
    CONSTRAINT uq_user_llm_credentials_user_provider UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_llm_credentials_user_id ON user_llm_credentials(user_id);

CREATE TABLE user_llm_credential_audit (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    VARCHAR(32) NOT NULL,
    action      VARCHAR(16) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip          VARCHAR(64) NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    CONSTRAINT chk_user_llm_credential_audit_provider CHECK (provider IN (
        'openai', 'anthropic', 'gemini', 'deepseek', 'qwen', 'openrouter'
    )),
    CONSTRAINT chk_user_llm_credential_audit_action CHECK (action IN ('set', 'clear'))
);

CREATE INDEX idx_user_llm_credential_audit_user_created ON user_llm_credential_audit(user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_user_llm_credential_audit_user_created;
DROP TABLE IF EXISTS user_llm_credential_audit;

DROP INDEX IF EXISTS idx_user_llm_credentials_user_id;
DROP TABLE IF EXISTS user_llm_credentials;

-- +goose StatementEnd
