-- +goose Up
-- +goose StatementBegin

-- claude_code_subscriptions — OAuth-токены Claude Code-подписки (Sprint 15.2).
-- access/refresh-токены шифруются AES-256-GCM (см. backend/pkg/crypto).
-- AAD при шифровании: "claude_code_subscription:" || user_id (детали — в сервисе).
CREATE TABLE claude_code_subscriptions (
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
    CONSTRAINT uq_claude_code_subscriptions_user UNIQUE (user_id),
    -- Sprint 15.m7: token_type ограничен Bearer (Anthropic OAuth не использует другие схемы).
    -- Защищает от мусора в БД, который мог бы оказаться в Authorization-заголовке sandbox-а.
    CONSTRAINT chk_claude_code_subscriptions_token_type CHECK (token_type IN ('Bearer'))
);

CREATE INDEX idx_claude_code_subscriptions_expires_at
    ON claude_code_subscriptions(expires_at)
    WHERE expires_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_claude_code_subscriptions_expires_at;
DROP TABLE IF EXISTS claude_code_subscriptions;

-- +goose StatementEnd
