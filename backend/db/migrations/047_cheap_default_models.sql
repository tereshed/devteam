-- +goose Up
-- +goose StatementBegin

-- Перевод seed'ных pipeline-агентов с Sonnet 4.6 на Haiku 4.5.
-- Sonnet 4.6 = $3/$15 per MTok input/output. Haiku 4.5 = $1/$5. Экономия 3×.
--
-- См. cost-leak audit Phase 2 (docs/integration-tests-plan.md):
--   - 5,271 LLM-calls / 15.3M токенов за день → ~$69 на Sonnet
--   - те же вызовы на Haiku → ~$23. Экономия $46/день только за этот класс
--     задач (router/planner/decomposer/reviewer).
--
-- Затрагиваем ТОЛЬКО seed-агентов из миграции 038 (где model='claude-sonnet-4-6'
-- AND team_id IS NULL AND name IN (...)). Пользовательских кастомных агентов,
-- даже если у них Sonnet — НЕ ТРОГАЕМ (они явно выбраны человеком).
UPDATE agents
   SET model = 'claude-haiku-4-5-20251001',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name IN ('router', 'planner', 'decomposer', 'reviewer')
   AND model = 'claude-sonnet-4-6';

-- Assistant — отдельно (он попал из миграции 046). Та же логика.
UPDATE agents
   SET model = 'claude-haiku-4-5-20251001',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name = 'assistant'
   AND model = 'claude-sonnet-4-6';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Откат: возвращаем Sonnet 4.6 ТОЛЬКО тем seed-агентам, которые сейчас на Haiku 4.5.
-- Кастомные пользовательские агенты, у которых Haiku по их выбору, не трогаем.
UPDATE agents
   SET model = 'claude-sonnet-4-6',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name IN ('router', 'planner', 'decomposer', 'reviewer', 'assistant')
   AND model = 'claude-haiku-4-5-20251001';

-- +goose StatementEnd
