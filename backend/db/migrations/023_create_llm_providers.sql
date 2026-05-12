-- +goose Up
-- +goose StatementBegin

-- llm_providers — каталог LLM-провайдеров (Sprint 15.1).
-- Кредсы хранятся зашифрованными (AES-256-GCM) в credentials_encrypted (см. backend/pkg/crypto).
CREATE TABLE llm_providers (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  VARCHAR(255) NOT NULL,
    kind                  VARCHAR(32)  NOT NULL,
    base_url              VARCHAR(1024) NOT NULL DEFAULT '',
    auth_type             VARCHAR(32)  NOT NULL DEFAULT 'api_key',
    credentials_encrypted BYTEA,
    default_model         VARCHAR(255) NOT NULL DEFAULT '',
    settings              JSONB        NOT NULL DEFAULT '{}',
    enabled               BOOLEAN      NOT NULL DEFAULT true,
    created_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_llm_providers_kind CHECK (kind IN (
        'anthropic',
        'anthropic_oauth',
        'openai',
        'gemini',
        'deepseek',
        'qwen',
        'openrouter',
        'moonshot',
        'ollama',
        'zhipu',
        'free_claude_proxy'
    )),
    CONSTRAINT chk_llm_providers_auth_type CHECK (auth_type IN (
        'api_key', 'oauth', 'bearer', 'none'
    )),
    CONSTRAINT uq_llm_providers_name UNIQUE (name)
);

CREATE INDEX idx_llm_providers_kind    ON llm_providers(kind);
CREATE INDEX idx_llm_providers_enabled ON llm_providers(enabled) WHERE enabled = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_llm_providers_enabled;
DROP INDEX IF EXISTS idx_llm_providers_kind;
DROP TABLE IF EXISTS llm_providers;

-- +goose StatementEnd
