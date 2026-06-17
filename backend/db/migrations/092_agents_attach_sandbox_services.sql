-- +goose Up
-- +goose StatementBegin

-- attach_sandbox_services — подключать ли к sandbox-прогонам этого агента
-- эфемерные сервис-сайдкары проекта (sandbox_service_configs), например postgres
-- для интеграционных тестов с БД (Sprint 22). Типично включается у tester-агента.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS attach_sandbox_services BOOLEAN NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE agents DROP COLUMN IF EXISTS attach_sandbox_services;

-- +goose StatementEnd
