-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — секреты агентов отдельной таблицей,
-- AES-256-GCM шифрование через pkg/crypto.AESEncryptor.
--
-- В sandbox_permissions (или code_backend_settings) у агента хранятся ТОЛЬКО
-- ссылки на ключи по key_name: { "env_secret_keys": ["GITHUB_TOKEN", ...] }.
-- Сами секреты — никогда в открытом JSONB.
--
-- ВАЖНО: encrypted_value — это весь blob формата pkg/crypto (одна колонка):
--   [version 1b = 0x01][nonce 12b][sealed = AES-256-GCM(plaintext) + tag 16b]
-- Минимальная длина: 1 + 12 + 16 = 29 байт (MinCiphertextBlobLen).
-- Отдельная колонка `nonce` НЕ нужна — он внутри blob.
-- Паттерн идентичен user_llm_credentials.encrypted_key.
--
-- AAD при шифровании = id записи (как в user_llm_credential_service.GetPlaintext).

CREATE TABLE agent_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    key_name        VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT uq_agent_secrets_agent_key UNIQUE (agent_id, key_name),
    CONSTRAINT chk_agent_secrets_key_name_format
        CHECK (key_name ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    -- 29 байт = минимальный blob pkg/crypto (1 версия + 12 nonce + 16 GCM tag).
    CONSTRAINT chk_agent_secrets_value_minlen
        CHECK (octet_length(encrypted_value) >= 29)
);

CREATE INDEX idx_agent_secrets_agent_id ON agent_secrets(agent_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
DROP TABLE IF EXISTS agent_secrets;

-- +goose StatementEnd
