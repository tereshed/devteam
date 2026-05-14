-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — расширение agents под LLM-driven Router.
-- Добавляем поля, нужные новой модели:
--   * execution_kind — явное разделение llm vs sandbox runtime.
--   * role_description — текст для промпта Router'а ("кому я могу делегировать что").
--   * system_prompt — системный промпт самого агента (inline; PromptID остаётся для версионированных промптов).
--   * temperature, max_tokens — параметры LLM для llm-агентов.
--
-- НЕ дублируем существующие колонки: model, code_backend, code_backend_settings,
-- sandbox_permissions, provider_kind, is_active (= enabled). Эти уже есть.

ALTER TABLE agents
    ADD COLUMN execution_kind   VARCHAR(16),
    ADD COLUMN role_description TEXT,
    ADD COLUMN system_prompt    TEXT,
    ADD COLUMN temperature      NUMERIC(4, 3),
    ADD COLUMN max_tokens       INTEGER;

-- Backfill для уже существующих агентов:
-- если code_backend задан — это sandbox-агент, иначе llm.
UPDATE agents
   SET execution_kind = CASE
           WHEN code_backend IS NOT NULL THEN 'sandbox'
           ELSE 'llm'
       END
 WHERE execution_kind IS NULL;

-- После backfill — делаем NOT NULL.
ALTER TABLE agents ALTER COLUMN execution_kind SET NOT NULL;

-- CHECK на допустимые значения.
ALTER TABLE agents
    ADD CONSTRAINT chk_agents_execution_kind
        CHECK (execution_kind IN ('llm', 'sandbox'));

-- Композитный CHECK: строгая взаимная исключительность.
-- llm-агент: model NOT NULL, code_backend NULL (sandbox-параметры запрещены).
-- sandbox-агент: code_backend NOT NULL, model NULL (sandbox сам выбирает модель
-- через свой backend, провайдер и креденшелы; иметь model в agents.model
-- внесло бы путаницу с реальным выбранным экземпляром).
-- Цель: исключить "мусор" в неактуальных колонках при переключении типа агента.
ALTER TABLE agents
    ADD CONSTRAINT chk_agents_kind_requirements
        CHECK (
            (execution_kind = 'llm'     AND model IS NOT NULL AND code_backend IS NULL) OR
            (execution_kind = 'sandbox' AND code_backend IS NOT NULL AND model IS NULL)
        );

-- Семантическая валидация диапазонов (мягкая, не сломает существующие записи).
ALTER TABLE agents
    ADD CONSTRAINT chk_agents_temperature_range
        CHECK (temperature IS NULL OR (temperature >= 0 AND temperature <= 2));
ALTER TABLE agents
    ADD CONSTRAINT chk_agents_max_tokens_positive
        CHECK (max_tokens IS NULL OR max_tokens > 0);

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
