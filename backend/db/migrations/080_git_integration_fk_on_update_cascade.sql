-- +goose Up
-- +goose StatementBegin

-- Фикс: при повторной OAuth-авторизации сервис меняет PK строки git_integration_credentials
-- (id пересоздаётся, т.к. AAD шифрования = id; ON CONFLICT DO UPDATE SET id=excluded.id).
-- FK projects/project_repositories.git_integration_credential_id без ON UPDATE CASCADE ловили
-- это как нарушение (SQLSTATE 23503) → callback падал, подключение «зависало». Добавляем
-- ON UPDATE CASCADE (смена id → ссылка следует за тем же аккаунтом), ON DELETE SET NULL сохраняем.

ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_git_integration_credential_id_fkey;
ALTER TABLE projects
    ADD CONSTRAINT projects_git_integration_credential_id_fkey
    FOREIGN KEY (git_integration_credential_id)
    REFERENCES git_integration_credentials(id)
    ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE project_repositories
    DROP CONSTRAINT IF EXISTS project_repositories_git_integration_credential_id_fkey;
ALTER TABLE project_repositories
    ADD CONSTRAINT project_repositories_git_integration_credential_id_fkey
    FOREIGN KEY (git_integration_credential_id)
    REFERENCES git_integration_credentials(id)
    ON UPDATE CASCADE ON DELETE SET NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_git_integration_credential_id_fkey;
ALTER TABLE projects
    ADD CONSTRAINT projects_git_integration_credential_id_fkey
    FOREIGN KEY (git_integration_credential_id)
    REFERENCES git_integration_credentials(id)
    ON DELETE SET NULL;

ALTER TABLE project_repositories
    DROP CONSTRAINT IF EXISTS project_repositories_git_integration_credential_id_fkey;
ALTER TABLE project_repositories
    ADD CONSTRAINT project_repositories_git_integration_credential_id_fkey
    FOREIGN KEY (git_integration_credential_id)
    REFERENCES git_integration_credentials(id)
    ON DELETE SET NULL;

-- +goose StatementEnd
