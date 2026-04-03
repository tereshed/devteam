-- +goose Up
-- +goose StatementBegin

-- 1. Таблица git_credentials (создаётся первой, т.к. projects ссылается на неё)
CREATE TABLE git_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(50) NOT NULL,
    auth_type       VARCHAR(50) NOT NULL DEFAULT 'token',
    encrypted_value BYTEA NOT NULL,
    label           VARCHAR(255) NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_git_credentials_provider CHECK (provider IN ('github', 'gitlab', 'bitbucket')),
    CONSTRAINT chk_git_credentials_auth_type CHECK (auth_type IN ('token', 'ssh_key', 'oauth'))
);

CREATE INDEX idx_git_credentials_user_id ON git_credentials(user_id);
CREATE INDEX idx_git_credentials_provider ON git_credentials(user_id, provider);

-- 2. Таблица projects
CREATE TABLE projects (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    git_provider        VARCHAR(50) NOT NULL DEFAULT 'local',
    git_url             VARCHAR(1024),
    git_default_branch  VARCHAR(255) NOT NULL DEFAULT 'main',
    git_credentials_id  UUID REFERENCES git_credentials(id) ON DELETE SET NULL,
    vector_collection   VARCHAR(255),
    tech_stack          JSONB DEFAULT '{}',
    status              VARCHAR(50) NOT NULL DEFAULT 'active',
    settings            JSONB DEFAULT '{}',
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_projects_git_provider CHECK (git_provider IN ('github', 'gitlab', 'bitbucket', 'local')),
    CONSTRAINT chk_projects_status CHECK (status IN ('active', 'paused', 'archived'))
);

CREATE INDEX idx_projects_user_id ON projects(user_id);
CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_git_provider ON projects(git_provider);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_projects_git_provider;
DROP INDEX IF EXISTS idx_projects_status;
DROP INDEX IF EXISTS idx_projects_user_id;
DROP TABLE IF EXISTS projects;

DROP INDEX IF EXISTS idx_git_credentials_provider;
DROP INDEX IF EXISTS idx_git_credentials_user_id;
DROP TABLE IF EXISTS git_credentials;

-- +goose StatementEnd
