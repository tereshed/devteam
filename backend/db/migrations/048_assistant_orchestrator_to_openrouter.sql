-- +goose Up
-- +goose StatementBegin

-- Phase 5 review: перевод assistant + global pipeline-агентов с
-- anthropic/claude-haiku-4-5 на openrouter/deepseek-v4-flash.
--
-- Почему:
--   - v4-flash на OpenRouter — ~$0.0000001/M токенов (заметно дешевле haiku $1/$5)
--     и в разы быстрее (latency-bound assistant и router-decisions).
--   - Качество tool-calling сопоставимо для управляющих задач (assistant,
--     router/planner/decomposer).
--   - Phase 5 цель — сократить время nightly e2e_real pipeline с 15 минут.
--
-- Скоуп:
--   - assistant (seed/SeedAssistantAgent, миграция 046).
--   - router, planner, decomposer (seed из миграции 038, LLM-only).
--   - orchestrator: добавлен в IN для согласованности с именем файла и
--     будущей совместимости. На текущий момент глобального агента с
--     name='orchestrator' нет (миграция 038 создаёт router/planner/
--     decomposer/reviewer/developer/merger/tester), поэтому UPDATE 0 rows.
--     Если кто-то когда-нибудь сидит global orchestrator — эта миграция
--     автоматически приведёт его к v4-flash.
--
-- ВАЖНО (Phase 5 review #2): reviewer ИСКЛЮЧЁН из этой миграции.
-- backend/agents/reviewer.yaml сейчас имеет model='claude-haiku-4-5-20251001'
-- (haiku), и resolveDefaultAgentConfig матчит YAML по ROLE (см.
-- orchestrator_context_builder.go:330) для ВСЕХ агентов с role=reviewer,
-- включая per-team reviewer'ов в e2e_real_test (claude-code+deepseek+sandbox).
-- Для не-hermes агентов yamlModel > dbModel (resolveInputModel:472), поэтому:
--   - Если бы мы здесь обновили DB.Model на v4-flash, sandbox-ревьюер всё
--     равно бы получил DEVTEAM_AGENT_MODEL='claude-haiku-4-5' из YAML.
--   - А если БЫ обновили и reviewer.yaml — sandbox получил бы v4-flash, но
--     SandboxAuthEnvResolver редиректит на api.deepseek.com/anthropic, где
--     openrouter-style имена 'deepseek/...' неизвестны → 404 в контейнере.
-- Согласованный переход reviewer на v4-flash требует одновременно: эту
-- миграцию, reviewer.yaml И правки в e2e_real_test seedAgentsForTeam. Это
-- отдельная задача, не в скоупе текущего PR.
--
-- Затрагиваем ТОЛЬКО seed-агентов с team_id IS NULL И текущей моделью =
-- 'claude-haiku-4-5-20251001' (защита от случайного перетирания кастомных
-- пользовательских настроек, выставленных через UI/PATCH).
--
-- Идемпотентно: повторный goose up — no-op (модель уже openrouter/v4-flash).

UPDATE agents
   SET model = 'deepseek/deepseek-v4-flash',
       provider_kind = 'openrouter',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name = 'assistant'
   AND model = 'claude-haiku-4-5-20251001'
   AND provider_kind = 'anthropic';

UPDATE agents
   SET model = 'deepseek/deepseek-v4-flash',
       provider_kind = 'openrouter',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name IN ('orchestrator', 'router', 'planner', 'decomposer')
   AND model = 'claude-haiku-4-5-20251001'
   AND provider_kind = 'anthropic';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Откат: возвращаем haiku ТОЛЬКО тем seed-агентам, которые СЕЙЧАС на v4-flash
-- (мы их и переводили). Если кто-то уже перепереключил через UI на другую
-- модель — не трогаем, оставляем его выбор.

UPDATE agents
   SET model = 'claude-haiku-4-5-20251001',
       provider_kind = 'anthropic',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name = 'assistant'
   AND model = 'deepseek/deepseek-v4-flash'
   AND provider_kind = 'openrouter';

UPDATE agents
   SET model = 'claude-haiku-4-5-20251001',
       provider_kind = 'anthropic',
       updated_at = NOW()
 WHERE team_id IS NULL
   AND name IN ('orchestrator', 'router', 'planner', 'decomposer')
   AND model = 'deepseek/deepseek-v4-flash'
   AND provider_kind = 'openrouter';

-- +goose StatementEnd
