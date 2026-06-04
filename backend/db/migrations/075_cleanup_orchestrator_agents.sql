-- +goose Up
-- +goose StatementBegin

-- Оркестратор — это Go-движок (orchestrator_v2.go), а не LLM-агент: на каждом шаге
-- он зовёт агента role='router', а сам никакую LLM не вызывает. Раньше для каждой
-- команды сидился агент role='orchestrator', которого никто не загружал (мёртвый
-- сид). Новые проекты его уже не создают (см. CreateDefaultProjectAgents) — здесь
-- подчищаем существующие записи.

-- execution_steps.agent_id ссылается на agents без ON DELETE (старый workflow-движок),
-- поэтому обнуляем вручную, иначе DELETE упрётся в FK. Остальные ссылки безопасны:
-- agent_secrets / agent_skills / agent_* (016) — ON DELETE CASCADE; tasks.assigned_agent_id
-- и llm_logs.agent_id — ON DELETE SET NULL.
UPDATE execution_steps
SET agent_id = NULL
WHERE agent_id IN (SELECT id FROM agents WHERE role = 'orchestrator');

DELETE FROM agents WHERE role = 'orchestrator';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Необратимо: удалённые orchestrator-агенты не восстанавливаются (это был мёртвый
-- сид без потребителей). При необходимости они пересоздаются заново при пересоздании
-- команды. No-op, чтобы goose down не падал.
SELECT 1;
-- +goose StatementEnd
