-- +goose Up
-- +goose StatementBegin

-- sandbox_service_configs — per-project декларация эфемерных сервис-сайдкаров,
-- которые поднимаются рядом с sandbox-агентом для интеграционных тестов с БД
-- (Sprint 22). Проект объявляет «доступен postgres: alias=db, образ, db/user, сид»;
-- агент через флаг agents.attach_sandbox_services включает их подключение к своим
-- прогонам. Runner поднимает по контейнеру на сервис в той же per-run bridge-сети
-- с alias-DNS, инжектит POSTGRES_*/DATABASE_URL в env агента; пароль НЕ хранится —
-- генерится случайно на каждый прогон.
CREATE TABLE IF NOT EXISTS sandbox_service_configs (
    id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- created_by — владелец конфига (как scout_configs.created_by).
    created_by           UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_enabled           BOOLEAN      NOT NULL DEFAULT false,
    -- kind — тип сервиса (пока только postgres).
    kind                 VARCHAR(32)  NOT NULL DEFAULT 'postgres',
    -- alias — сетевой alias/hostname в bridge-сети прогона (агент обращается как alias:port).
    alias                VARCHAR(63)  NOT NULL DEFAULT 'db',
    -- image — docker-образ сервиса; сверяется с allowlist раннера (allowedServiceImages).
    image                VARCHAR(255) NOT NULL DEFAULT 'postgres:16-alpine',
    -- db_name / db_user — имя БД и суперюзера сервис-контейнера (пароль НЕ хранится).
    db_name              VARCHAR(255) NOT NULL DEFAULT 'app',
    db_user              VARCHAR(255) NOT NULL DEFAULT 'postgres',
    port                 INTEGER      NOT NULL DEFAULT 5432,
    -- seed_kind — источник сида: none | repo_file (путь в репо) | inline (SQL в seed_value).
    seed_kind            VARCHAR(16)  NOT NULL DEFAULT 'none',
    seed_value           TEXT         NOT NULL DEFAULT '',
    ready_timeout_seconds INTEGER     NOT NULL DEFAULT 60,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_sandbox_service_project_alias UNIQUE (project_id, alias),
    CONSTRAINT chk_sandbox_service_kind CHECK (kind IN ('postgres')),
    CONSTRAINT chk_sandbox_service_seed_kind CHECK (seed_kind IN ('none', 'repo_file', 'inline')),
    CONSTRAINT chk_sandbox_service_port CHECK (port BETWEEN 1 AND 65535),
    CONSTRAINT chk_sandbox_service_ready_timeout CHECK (ready_timeout_seconds BETWEEN 10 AND 600)
);

CREATE INDEX IF NOT EXISTS idx_sandbox_service_configs_project ON sandbox_service_configs (project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS sandbox_service_configs;

-- +goose StatementEnd
