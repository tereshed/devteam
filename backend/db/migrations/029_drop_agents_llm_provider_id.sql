-- +goose Up
-- +goose StatementBegin

-- Sprint 15.e2e (post-rewrite): убираем колонку agents.llm_provider_id.
-- После переключения резолвера на agent.provider_kind + user_llm_credentials
-- колонка llm_provider_id потеряла смысл: системный каталог llm_providers
-- больше не нужен для маршрутизации (kind зашит прямо в агенте).
--
-- Таблица llm_providers остаётся как admin-only справочник (UI list / health-check),
-- но её записи никак не связаны с агентами.

DROP INDEX IF EXISTS idx_agents_llm_provider_id;

ALTER TABLE agents DROP COLUMN IF EXISTS llm_provider_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Восстанавливаем колонку и FK (см. миграцию 025).
-- ВНИМАНИЕ: data loss — потерянные значения llm_provider_id не восстанавливаются.
ALTER TABLE agents
    ADD COLUMN llm_provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL;

CREATE INDEX idx_agents_llm_provider_id ON agents(llm_provider_id) WHERE llm_provider_id IS NOT NULL;

-- +goose StatementEnd
