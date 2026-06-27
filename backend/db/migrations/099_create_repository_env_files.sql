-- +goose Up
-- +goose StatementBegin

-- «Инъекция env-файла» на уровне репозитория проекта. Пользователь задаёт для
-- конкретного project_repository содержимое файла, его имя и относительную папку
-- внутри репо (по умолчанию — корень). Перед запуском агента sandbox-entrypoint
-- пишет этот файл в рабочую копию репо и добавляет его в .git/info/exclude — файл
-- доступен агенту/тестам, но НЕ попадает в diff/commit/push (защита от утечки секретов).
--
-- Один файл на репозиторий (UNIQUE project_repository_id). Содержимое шифруется
-- AES-256-GCM тем же SecretService, что и project_secrets (encrypted_content — BYTEA,
-- минимум 29 байт после шифрования; guard в репозитории fail-loud'ит на «сырой» blob).
CREATE TABLE IF NOT EXISTS repository_env_files (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_repository_id UUID NOT NULL REFERENCES project_repositories(id) ON DELETE CASCADE,
    -- file_name — имя создаваемого файла (например ".env"). Без слешей и "..".
    file_name             VARCHAR(255) NOT NULL,
    -- target_dir — относительная папка внутри репо, куда положить файл. Пусто = корень.
    -- Без ведущего "/" и без "..".
    target_dir            VARCHAR(512) NOT NULL DEFAULT '',
    -- encrypted_content — содержимое файла, зашифрованное AES-256-GCM.
    encrypted_content     BYTEA NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_repository_env_files_repo UNIQUE (project_repository_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS repository_env_files;

-- +goose StatementEnd
