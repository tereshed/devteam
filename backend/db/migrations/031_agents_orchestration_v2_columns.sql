-- +goose Up

-- Sprint 17 / Orchestration v2 — расширение agents под LLM-driven Router.
-- Добавляем поля, нужные новой модели:
--   * execution_kind — явное разделение llm vs sandbox runtime.
--   * role_description — текст для промпта Router'а ("кому я могу делегировать что").
--   * system_prompt — системный промпт самого агента (inline; PromptID остаётся для версионированных промптов).
--   * temperature, max_tokens — параметры LLM для llm-агентов.
--
-- НЕ дублируем существующие колонки: model, code_backend, code_backend_settings,
-- sandbox_permissions, provider_kind, is_active (= enabled). Эти уже есть.
--
-- ─────────────────────────────────────────────────────────────────────────────
-- Миграция разбита на отдельные `StatementBegin/End` блоки.
-- Причина: в YugabyteDB DDL автокоммитится и плохо смешивается с DML в одном
-- Exec — UPDATE мог не успеть закоммититься до последующего `SET NOT NULL`,
-- из-за чего проверка падала с "column contains null values".
-- Раздельные блоки → goose отправляет каждый отдельным запросом → DDL и DML
-- идут разными транзакциями, видимость гарантирована.
-- ─────────────────────────────────────────────────────────────────────────────

-- 1. Добавляем колонки (идемпотентно — на случай частично применённой миграции).
-- +goose StatementBegin
ALTER TABLE agents ADD COLUMN IF NOT EXISTS execution_kind   VARCHAR(16);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS role_description TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS system_prompt    TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS temperature      NUMERIC(4, 3);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_tokens       INTEGER;
-- +goose StatementEnd

-- 2. Backfill execution_kind по уже существующим агентам.
--    Если code_backend задан — это sandbox-агент, иначе llm.
-- +goose StatementBegin
UPDATE agents
   SET execution_kind = CASE
           WHEN code_backend IS NOT NULL THEN 'sandbox'
           ELSE 'llm'
       END
 WHERE execution_kind IS NULL;
-- +goose StatementEnd

-- 3. Backfill для CHECK chk_agents_kind_requirements:
--    sandbox-агент обязан иметь model=NULL (модель определяется внутри backend).
--    Существующие записи могли быть созданы до этого правила и иметь оба
--    поля заполненными — обнуляем model, чтобы CHECK прошёл.
-- +goose StatementBegin
UPDATE agents
   SET model = NULL
 WHERE execution_kind = 'sandbox' AND model IS NOT NULL;
-- +goose StatementEnd

-- 4. Делаем execution_kind NOT NULL (после полного backfill).
-- +goose StatementBegin
ALTER TABLE agents ALTER COLUMN execution_kind SET NOT NULL;
-- +goose StatementEnd

-- 5. CHECK-ограничения. Добавляем идемпотентно через DO-блок,
--    т.к. PG/YB не поддерживают `ADD CONSTRAINT IF NOT EXISTS`.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_agents_execution_kind') THEN
        ALTER TABLE agents
            ADD CONSTRAINT chk_agents_execution_kind
                CHECK (execution_kind IN ('llm', 'sandbox'));
    END IF;

    -- Композитный CHECK: строгая взаимная исключительность.
    -- llm-агент:     model NOT NULL, code_backend NULL.
    -- sandbox-агент: code_backend NOT NULL, model NULL (sandbox сам выбирает
    -- модель через свой backend, провайдер и креденшелы).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_agents_kind_requirements') THEN
        ALTER TABLE agents
            ADD CONSTRAINT chk_agents_kind_requirements
                CHECK (
                    (execution_kind = 'llm'     AND model IS NOT NULL AND code_backend IS NULL) OR
                    (execution_kind = 'sandbox' AND code_backend IS NOT NULL AND model IS NULL)
                );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_agents_temperature_range') THEN
        ALTER TABLE agents
            ADD CONSTRAINT chk_agents_temperature_range
                CHECK (temperature IS NULL OR (temperature >= 0 AND temperature <= 2));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_agents_max_tokens_positive') THEN
        ALTER TABLE agents
            ADD CONSTRAINT chk_agents_max_tokens_positive
                CHECK (max_tokens IS NULL OR max_tokens > 0);
    END IF;
END$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_max_tokens_positive;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_temperature_range;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_kind_requirements;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_execution_kind;

ALTER TABLE agents
    DROP COLUMN IF EXISTS max_tokens,
    DROP COLUMN IF EXISTS temperature,
    DROP COLUMN IF EXISTS system_prompt,
    DROP COLUMN IF EXISTS role_description,
    DROP COLUMN IF EXISTS execution_kind;

-- +goose StatementEnd
