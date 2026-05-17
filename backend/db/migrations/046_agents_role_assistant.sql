-- +goose Up
-- +goose StatementBegin

-- Sprint 21 §6 — расширяем chk_agents_role, чтобы пропустить 'assistant'.
--
-- Глобальный assistant-агент создаётся Go-seed'ом (internal/seed/assistant.go)
-- при старте бэкенда: ON CONFLICT (name) DO NOTHING — оператор может править
-- system_prompt через UI редактирования агентов, seed не перетрёт.
--
-- Конкретный INSERT в этой миграции НЕ делаем: seed-функция работает с
-- актуальным дефолтом из кода (промпт может меняться между релизами без
-- новой миграции).

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_role;
ALTER TABLE agents ADD CONSTRAINT chk_agents_role
    CHECK (role IN (
        'worker', 'supervisor', 'orchestrator', 'planner',
        'developer', 'reviewer', 'tester', 'devops',
        'router', 'decomposer', 'merger',
        'assistant'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Откатываем к набору ролей из миграции 038. Если в БД остались записи
-- role='assistant', сначала удалим их — иначе DROP/ADD CONSTRAINT упадёт.
DELETE FROM agents WHERE role = 'assistant';

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_role;
ALTER TABLE agents ADD CONSTRAINT chk_agents_role
    CHECK (role IN (
        'worker', 'supervisor', 'orchestrator', 'planner',
        'developer', 'reviewer', 'tester', 'devops',
        'router', 'decomposer', 'merger'
    ));

-- +goose StatementEnd
