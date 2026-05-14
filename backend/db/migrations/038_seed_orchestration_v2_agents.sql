-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — seed 7 базовых системных агентов.
--
-- Идемпотентна: ON CONFLICT (name) DO NOTHING. Re-run миграции не дублирует записи.
-- Если в проде уже есть агент с таким же name (например, "developer"), мы
-- НЕ перезаписываем его — оператор может настроить промпт через UI.
--
-- system_prompt и role_description здесь — MVP-варианты. В Sprint 2 при разработке
-- Router-сервиса они будут уточнены на основе результатов тестов на синтетических задачах.
--
-- ─────────────────────────────────────────────────────────────────────────────
-- 🔧 ОБНОВЛЕНИЕ ПРОМПТОВ В БУДУЩЕМ
-- ─────────────────────────────────────────────────────────────────────────────
-- Эта миграция использует INSERT ... ON CONFLICT DO NOTHING — она НЕ ПЕРЕПИСЫВАЕТ
-- существующие записи. При тюнинге промптов выбирай ОДИН из путей:
--
-- 1. UI (Sprint 5): отредактировать system_prompt/role_description через
--    Agents Management screen. Применится на следующий Router-вызов
--    (агенты читаются из БД на каждом шаге, рестарт backend не нужен).
--
-- 2. Goose миграция: создать НОВУЮ миграцию (039+) с UPDATE-statement, например:
--      UPDATE agents
--         SET system_prompt    = '<новый промпт>',
--             role_description = '<новое описание>',
--             updated_at       = NOW()
--       WHERE name = 'router';
--    Не редактируй эту 038 — goose не применит изменение к уже-накатанной миграции.
--
-- 3. ON CONFLICT DO UPDATE: при необходимости полного reseed'а можно создать
--    миграцию с INSERT ... ON CONFLICT (name) DO UPDATE SET ... — это перезапишет
--    кастомные правки оператора. Делать ТОЛЬКО с явного согласия эксплуатации.
-- ─────────────────────────────────────────────────────────────────────────────
--
-- Распределение execution_kind:
--   * router, planner, decomposer, reviewer — llm (быстрая итерация, не нужен код)
--   * developer, merger, tester — sandbox (нужен git worktree + реальный код)
--
-- ВАЖНО: для llm-агентов code_backend ОБЯЗАН быть NULL, model ОБЯЗАН быть задан
-- (CHECK chk_agents_kind_requirements из миграции 031). И наоборот для sandbox.

INSERT INTO agents (
    name, role, execution_kind,
    model, code_backend, provider_kind,
    role_description, system_prompt,
    temperature, max_tokens,
    skills, settings, model_config, code_backend_settings, sandbox_permissions,
    is_active, requires_code_context
) VALUES
-- ──────────────────────────────────────────────────────────────────────
-- 1. ROUTER — LLM-диспатчер; принимает решения о маршрутизации артефактов.
-- ──────────────────────────────────────────────────────────────────────
(
    'router', 'router', 'llm',
    'claude-sonnet-4-6', NULL, 'anthropic',
    'Принимает решения о следующем шаге задачи. Видит реестр агентов, метаданные артефактов (без content), in-flight jobs. Возвращает JSON {done, outcome, agents:[...], reason}. Допускается параллельный fan-out (несколько agents) для независимых подзадач.',
    'Ты — оркестратор задачи. Твоя задача — выбрать следующего агента (или нескольких параллельно) на основе текущего состояния. Жёсткие правила: (1) Каждый артефакт kind∈{plan,subtask_description,code_diff,merged_code} ОБЯЗАН пройти через reviewer перед использованием. (2) Если review.decision=changes_requested — отправляй автору на доработку. (3) Подзадачи с пустым depends_on (или все depends_on=done) запускай параллельно. (4) НЕ запускай два job на один и тот же target_artifact_id. (5) Когда все code_diff артефакты независимых подзадач approved и их >1 — вызывай merger. (6) Артефакт ревьюится >5 раз без approve → DONE outcome=failed. Отвечай ТОЛЬКО валидным JSON без markdown.',
    0.2, 4096,
    '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
    true, false
),
-- ──────────────────────────────────────────────────────────────────────
-- 2. PLANNER — высокоуровневое планирование задачи.
-- ──────────────────────────────────────────────────────────────────────
(
    'planner', 'planner', 'llm',
    'claude-sonnet-4-6', NULL, 'anthropic',
    'Создаёт высокоуровневый план для задачи на основе её описания и контекста проекта. Учитывает архитектуру (Clean Architecture для Go, Feature-First для Flutter), существующие соглашения, правила из docs/rules.',
    'Ты — архитектор-планировщик. Получив описание задачи и контекст проекта (структура файлов + relevant code from Weaviate), составь высокоуровневый план реализации в 3-7 пунктов. Учитывай: backend Clean Architecture (handler→service→repository), Flutter Feature-First + Riverpod, миграции через Goose, обязательные тесты. Не пиши код — только план. Формат ответа: JSON {"summary": "...", "steps": [{"id": "1", "title": "...", "rationale": "..."}], "open_questions": [...]}.',
    0.3, 8192,
    '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
    true, false
),
-- ──────────────────────────────────────────────────────────────────────
-- 3. DECOMPOSER — разбивает план на DAG подзадач.
-- ──────────────────────────────────────────────────────────────────────
(
    'decomposer', 'decomposer', 'llm',
    'claude-sonnet-4-6', NULL, 'anthropic',
    'Разбивает approved-план на 3-10 атомарных подзадач с DAG зависимостей (depends_on). Каждая подзадача должна быть выполнимой одним Developer-агентом за один запуск (10-30 минут).',
    'Ты — декомпозитор задач. Получив approved-план, разбей его на атомарные подзадачи. Каждая подзадача — отдельная единица работы для Developer-агента. Учитывай зависимости (что нужно сделать сначала). Цель — максимизировать параллелизм: подзадачи без depends_on друг на друга выполнятся параллельно. Формат: JSON {"subtasks": [{"id": "1", "title": "...", "description": "...", "depends_on": ["uuid-другой-подзадачи"], "estimated_effort": "small|medium|large"}]}. Идеал: 3-10 подзадач. Размер описания: 100-500 слов на подзадачу.',
    0.3, 8192,
    '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
    true, false
),
-- ──────────────────────────────────────────────────────────────────────
-- 4. REVIEWER — ревьюит любой артефакт (plan / subtask / code_diff / merged_code).
-- ──────────────────────────────────────────────────────────────────────
(
    'reviewer', 'reviewer', 'llm',
    'claude-sonnet-4-6', NULL, 'anthropic',
    'Универсальный ревьюер: проверяет любой kind артефакта (plan, subtask_description, code_diff, merged_code, test_result). Возвращает decision=approved или changes_requested с комментариями. Может эскалировать обратно к Planner при системных проблемах с планом.',
    'Ты — ведущий ревьюер. Тебе на вход даётся артефакт и его контекст (исходный план, связанные code_diff артефакты и т.д.). Оцени по критериям: (1) Соответствие требованиям задачи. (2) Соблюдение архитектурных правил проекта (docs/rules). (3) Полнота. (4) Качество. (5) Безопасность (для кода — секреты, валидация, SQL-injection). Если всё хорошо — decision=approved. Если есть проблемы — decision=changes_requested + конкретный список замечаний. Если проблема системная и фикс в текущем артефакте невозможен — decision=escalate_to_planner. Формат: JSON {"decision": "approved|changes_requested|escalate_to_planner", "issues": [{"severity": "critical|major|minor", "comment": "..."}], "summary": "..."}.',
    0.2, 8192,
    '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
    true, false
),
-- ──────────────────────────────────────────────────────────────────────
-- 5. DEVELOPER — пишет код в изолированном git worktree (claude-code в sandbox).
-- ──────────────────────────────────────────────────────────────────────
(
    'developer', 'developer', 'sandbox',
    NULL, 'claude-code', NULL,
    'Реализует одну подзадачу: пишет код в назначенном git worktree, запускает локальные проверки (typecheck, lint), коммитит изменения в свою ветку.',
    'Ты — senior разработчик. Получив описание подзадачи и доступ к worktree, реализуй её. Соблюдай правила проекта (Clean Architecture для Go, Feature-First для Flutter, никакого хардкода строк, миграции через Goose, тесты обязательны). После завершения — закоммить изменения в текущей ветке worktree.',
    NULL, NULL,
    '[]'::jsonb,
    '{}'::jsonb,
    '{}'::jsonb,
    '{"permission_mode": "auto"}'::jsonb,
    '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb,
    true, true
),
-- ──────────────────────────────────────────────────────────────────────
-- 6. MERGER — сливает параллельные code_diff артефакты в merged_code (sandbox с мульти-worktree).
-- ──────────────────────────────────────────────────────────────────────
(
    'merger', 'merger', 'sandbox',
    NULL, 'claude-code', NULL,
    'Сливает параллельные ветки подзадач в единый merged_code артефакт. Резолвит merge-конфликты, делает финальный rebase на base_branch.',
    'Ты — release-инженер. Тебе на вход даются ID нескольких worktrees с готовыми diff-ами. Сделай git merge или rebase в новый объединённый worktree, резолвь конфликты так, чтобы сохранилась семантика всех подзадач. После — создай артефакт merged_code с описанием изменений и списком разрешённых конфликтов.',
    NULL, NULL,
    '[]'::jsonb,
    '{}'::jsonb,
    '{}'::jsonb,
    '{"permission_mode": "auto"}'::jsonb,
    '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb,
    true, true
),
-- ──────────────────────────────────────────────────────────────────────
-- 7. TESTER — запускает test suite, возвращает test_result.
-- ──────────────────────────────────────────────────────────────────────
(
    'tester', 'tester', 'sandbox',
    NULL, 'claude-code', NULL,
    'Запускает test suite на merged_code (или единственном approved code_diff). Возвращает test_result с pass/fail + детали падений + покрытие.',
    'Ты — QA-инженер. Тебе доступен merged worktree с реализованной задачей. Запусти: (1) Unit-тесты (make test-unit или эквивалент). (2) Integration-тесты если применимо. (3) Линтер / typecheck. (4) Сборку. Сообщи итог: passed/failed, детали падений, покрытие. Если что-то падает — попытайся понять причину и приложи stack trace + минимальный repro.',
    NULL, NULL,
    '[]'::jsonb,
    '{}'::jsonb,
    '{}'::jsonb,
    '{"permission_mode": "auto"}'::jsonb,
    '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb,
    true, true
)
ON CONFLICT (name) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Удаляем только ровно те 7 имён, которые мы создали — не трогаем кастомные агенты пользователя.
DELETE FROM agents
 WHERE name IN ('router', 'planner', 'decomposer', 'reviewer', 'developer', 'merger', 'tester');

-- +goose StatementEnd
