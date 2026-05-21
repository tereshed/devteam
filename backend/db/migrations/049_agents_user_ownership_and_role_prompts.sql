-- +goose Up

-- Phase 1: Agents Refactor — user ownership, relaxed kind requirements, role prompts registry.
--
-- Содержание:
--   1.1 — user_id + CHECK chk_agents_ownership + singleton/multi-instance индексы
--   1.2 — chk_agents_role уже содержит assistant (миграция 046) — OK
--   1.3 — ослабляем chk_agents_kind_requirements (разрешаем model IS NULL для llm)
--   1.4 — таблица agent_role_prompts (реестр дефолтных промптов по ролям)
--
-- Миграция разбита на отдельные StatementBegin/End блоки (Yugabyte DDL-safety).

-- ═══════════════════════════════════════════════════════════════════════════════
-- 1.1  user_id + ownership constraint + indexes
-- ═══════════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE agents ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
-- +goose StatementEnd

-- Ownership: агент принадлежит ОДНОМУ из: user, team, или никому (системный seed).
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_agents_ownership') THEN
        ALTER TABLE agents ADD CONSTRAINT chk_agents_ownership
            CHECK (
                (user_id IS NOT NULL AND team_id IS NULL) OR   -- user-level (assistant)
                (user_id IS NULL AND team_id IS NOT NULL) OR   -- team-level (orchestrator, router, developers...)
                (user_id IS NULL AND team_id IS NULL)          -- system-level (legacy seed)
            );
    END IF;
END$$;
-- +goose StatementEnd

-- Синглтон-роли: один assistant на пользователя.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_user_singleton
    ON agents (user_id, role)
    WHERE user_id IS NOT NULL AND role IN ('assistant');
-- +goose StatementEnd

-- Синглтон-роли: один orchestrator/router на команду.
-- Мульти-инстанс роли (developer, reviewer, ...) НЕ попадают в этот индекс.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_team_singleton
    ON agents (team_id, role)
    WHERE team_id IS NOT NULL AND role IN ('orchestrator', 'router');
-- +goose StatementEnd

-- Уникальность имени для user-level агентов.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_user_name
    ON agents (user_id, name)
    WHERE user_id IS NOT NULL;
-- +goose StatementEnd

-- B-Tree индекс для GET /me/agents (фильтрация по user_id).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_agents_user_id
    ON agents (user_id) WHERE user_id IS NOT NULL;
-- +goose StatementEnd

-- Исправление уникального индекса для глобальных системных агентов:
-- Он должен распространяться только на агентов, у которых нет ни user_id, ни team_id.
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_global_name;
CREATE UNIQUE INDEX idx_agents_global_name
    ON agents(name) WHERE team_id IS NULL AND user_id IS NULL;
-- +goose StatementEnd

-- ═══════════════════════════════════════════════════════════════════════════════
-- 1.3  Ослабляем chk_agents_kind_requirements
-- ═══════════════════════════════════════════════════════════════════════════════
-- Разрешаем LLM-агентам model IS NULL (состояние "не сконфигурирован").
-- Валидация полноты настроек — ответственность сервисного слоя при запуске.

-- +goose StatementBegin
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_kind_requirements;
ALTER TABLE agents ADD CONSTRAINT chk_agents_kind_requirements CHECK (
    (execution_kind = 'llm' AND code_backend IS NULL)
    OR (execution_kind = 'sandbox' AND code_backend IS NOT NULL AND model IS NULL)
);
-- +goose StatementEnd

-- ═══════════════════════════════════════════════════════════════════════════════
-- 1.4  Таблица agent_role_prompts — реестр дефолтных промптов по ролям
-- ═══════════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS agent_role_prompts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role        VARCHAR(50) NOT NULL UNIQUE,
    content     TEXT NOT NULL,
    description TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_agent_role_prompts_role
    ON agent_role_prompts (role);
-- +goose StatementEnd


-- +goose Down

-- +goose StatementBegin
DROP TABLE IF EXISTS agent_role_prompts;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_user_id;
DROP INDEX IF EXISTS idx_agents_user_name;
DROP INDEX IF EXISTS idx_agents_team_singleton;
DROP INDEX IF EXISTS idx_agents_user_singleton;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_ownership;
-- +goose StatementEnd

-- Восстанавливаем строгий CHECK из миграции 031.
-- +goose StatementBegin
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_kind_requirements;
ALTER TABLE agents ADD CONSTRAINT chk_agents_kind_requirements CHECK (
    (execution_kind = 'llm' AND model IS NOT NULL AND code_backend IS NULL)
    OR (execution_kind = 'sandbox' AND code_backend IS NOT NULL AND model IS NULL)
);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE agents DROP COLUMN IF EXISTS user_id;
-- +goose StatementEnd

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_global_name;
CREATE UNIQUE INDEX idx_agents_global_name
    ON agents(name) WHERE team_id IS NULL;
-- +goose StatementEnd
