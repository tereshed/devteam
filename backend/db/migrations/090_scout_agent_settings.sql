-- +goose Up
-- +goose StatementBegin

-- Разведчик настраивается как агент в команде (provider/model/temperature +
-- code_backend_settings c MCP/скиллами/hermes-блоком + sandbox_permissions),
-- отличие — всегда sandbox. Поля зеркалят models.Agent: при диспатче из
-- scout_configs собирается временный Agent и прогоняется через тот же
-- SandboxAuthEnvResolver + AgentSettingsService.BuildSandboxBundle, что и
-- dev-агенты. Модель живёт внутри code_backend_settings.model (как у
-- sandbox-агента), отдельной колонки нет.
ALTER TABLE scout_configs
    ADD COLUMN IF NOT EXISTS provider_kind         VARCHAR(32),
    ADD COLUMN IF NOT EXISTS temperature           NUMERIC(4,3),
    ADD COLUMN IF NOT EXISTS code_backend_settings JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS sandbox_permissions   JSONB NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE scout_configs
    DROP COLUMN IF EXISTS provider_kind,
    DROP COLUMN IF EXISTS temperature,
    DROP COLUMN IF EXISTS code_backend_settings,
    DROP COLUMN IF EXISTS sandbox_permissions;

-- +goose StatementEnd
