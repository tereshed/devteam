-- +goose Up
-- +goose StatementBegin

-- UI Refactoring Stage 3 (Git Integrations).
-- Хранение OAuth-токенов GitHub / GitLab.com / self-hosted GitLab (BYO).
--
-- Шифрование:
--   access_token / refresh_token / byo_client_secret — AES-256-GCM blob
--   (формат pkg/crypto.AESEncryptor: 0x01 || nonce(12) || sealed). AAD = id записи
--   (UUID PK) — защита от cross-row substitution. См. docs/rules/main.md §2.3 п.5.
--   byo_client_id — plain VARCHAR (public по спеке OAuth 2.0).
--
-- Уникальность (user_id, provider): на одного пользователя одна запись на провайдер.
-- Для self-hosted GitLab используется provider='gitlab' с непустым host (BYO).

CREATE TABLE git_integration_credentials (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider                 VARCHAR(32) NOT NULL,
    host                     VARCHAR(255) NOT NULL DEFAULT '',
    byo_client_id            VARCHAR(255) NOT NULL DEFAULT '',
    byo_client_secret_enc    BYTEA,
    access_token_enc         BYTEA NOT NULL,
    refresh_token_enc        BYTEA,
    token_type               VARCHAR(32) NOT NULL DEFAULT 'Bearer',
    scopes                   TEXT NOT NULL DEFAULT '',
    account_login            VARCHAR(255) NOT NULL DEFAULT '',
    expires_at               TIMESTAMP WITH TIME ZONE,
    last_refreshed_at        TIMESTAMP WITH TIME ZONE,
    created_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_git_integration_credentials_user_provider UNIQUE (user_id, provider),
    CONSTRAINT chk_git_integration_credentials_provider
        CHECK (provider IN ('github', 'gitlab')),
    -- 29 байт = минимальный blob pkg/crypto (1 версия + 12 nonce + 16 GCM tag).
    CONSTRAINT chk_git_integration_credentials_access_minlen
        CHECK (octet_length(access_token_enc) >= 29),
    CONSTRAINT chk_git_integration_credentials_refresh_minlen
        CHECK (refresh_token_enc IS NULL OR octet_length(refresh_token_enc) >= 29),
    CONSTRAINT chk_git_integration_credentials_byo_secret_minlen
        CHECK (byo_client_secret_enc IS NULL OR octet_length(byo_client_secret_enc) >= 29)
);

-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_git_integration_credentials_user_id
    ON git_integration_credentials(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS git_integration_credentials;

-- +goose StatementEnd
