-- +goose Up
-- +goose StatementBegin

-- Поддержка НЕСКОЛЬКИХ env-файлов на один репозиторий: снимаем ограничение
-- «один файл на репо» и заменяем уникальностью по пути назначения
-- (project_repository_id, target_dir, file_name) — два инжекта не могут писать в
-- один и тот же файл рабочей копии (иначе «последний победит» молча).
ALTER TABLE repository_env_files DROP CONSTRAINT IF EXISTS uq_repository_env_files_repo;
ALTER TABLE repository_env_files
    ADD CONSTRAINT uq_repository_env_files_path UNIQUE (project_repository_id, target_dir, file_name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE repository_env_files DROP CONSTRAINT IF EXISTS uq_repository_env_files_path;
ALTER TABLE repository_env_files
    ADD CONSTRAINT uq_repository_env_files_repo UNIQUE (project_repository_id);

-- +goose StatementEnd
