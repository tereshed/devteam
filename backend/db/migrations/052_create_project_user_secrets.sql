-- +goose Up

CREATE TABLE project_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key_name        VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_project_secrets_key UNIQUE (project_id, key_name),
    CONSTRAINT chk_project_secrets_key_format CHECK (key_name ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    CONSTRAINT chk_project_secrets_min_len CHECK (octet_length(encrypted_value) >= 29)
);

CREATE TABLE user_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_name        VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_secrets_key UNIQUE (user_id, key_name),
    CONSTRAINT chk_user_secrets_key_format CHECK (key_name ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    CONSTRAINT chk_user_secrets_min_len CHECK (octet_length(encrypted_value) >= 29)
);

-- +goose Down
DROP TABLE IF EXISTS user_secrets;
DROP TABLE IF EXISTS project_secrets;
