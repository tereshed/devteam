-- +goose Up
-- +goose StatementBegin

-- Мульти-аккаунт OAuth: разрешаем несколько подключений на один провайдер (например
-- два GitHub-аккаунта). Различаем по (host, account_login). Прежний UNIQUE(user_id, provider)
-- допускал ровно одну связку на провайдера.
ALTER TABLE git_integration_credentials
    DROP CONSTRAINT IF EXISTS uq_git_integration_credentials_user_provider;

-- Новый ключ уникальности: один аккаунт = (user, provider, host, account_login).
-- host='' для shared github.com/gitlab.com; account_login='' у legacy-строк до захвата логина.
ALTER TABLE git_integration_credentials
    ADD CONSTRAINT uq_git_integration_credentials_user_provider_host_login
    UNIQUE (user_id, provider, host, account_login);

-- Привязка «какой аккаунт использовать» на уровне проекта и репозитория (мульти-репо).
-- NULL → прежнее поведение (фолбэк на первый аккаунт провайдера).
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS git_integration_credential_id UUID
    REFERENCES git_integration_credentials(id) ON DELETE SET NULL;

ALTER TABLE project_repositories
    ADD COLUMN IF NOT EXISTS git_integration_credential_id UUID
    REFERENCES git_integration_credentials(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_projects_git_integration_credential
    ON projects(git_integration_credential_id);
CREATE INDEX IF NOT EXISTS idx_project_repositories_git_integration_credential
    ON project_repositories(git_integration_credential_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_project_repositories_git_integration_credential;
DROP INDEX IF EXISTS idx_projects_git_integration_credential;
ALTER TABLE project_repositories DROP COLUMN IF EXISTS git_integration_credential_id;
ALTER TABLE projects DROP COLUMN IF EXISTS git_integration_credential_id;

ALTER TABLE git_integration_credentials
    DROP CONSTRAINT IF EXISTS uq_git_integration_credentials_user_provider_host_login;
ALTER TABLE git_integration_credentials
    ADD CONSTRAINT uq_git_integration_credentials_user_provider
    UNIQUE (user_id, provider);

-- +goose StatementEnd
