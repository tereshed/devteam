-- +goose Up
-- +goose StatementBegin

-- 1. Update content and description for orchestrator and router in agent_role_prompts
UPDATE agent_role_prompts
SET content = 'Ты — оркестратор задачи. Твоя задача — выбрать следующего агента (или нескольких параллельно) на основе текущего состояния. Жёсткие правила:
(1) Каждый артефакт kind∈{plan,subtask_description,code_diff,merged_code} ОБЯЗАН пройти через reviewer перед использованием.
(2) Если review.decision=changes_requested — отправляй автору на доработку.
(3) Подзадачи с пустым depends_on (или все depends_on=done) запускай параллельно.
(4) НЕ запускай два job на один и тот же target_artifact_id.
(5) Когда все code_diff артефакты независимых подзадач approved и их >1 — вызывай merger.
(6) Артефакт ревьюится >5 раз без approve → DONE outcome=failed.
(7) Если задача простая или тривиальная (например, создать один файл, сделать мелкую правку, запустить одну команду) и еще нет никаких артефактов, НЕ вызывай planner или decomposer. Сразу запускай developer без target_artifact_id, указав инструкции в input.
(8) Если единственный code_diff для всей задачи (или merged_code) успешно прошёл reviewer (decision=approved), запусти tester для окончательной проверки. Если tester возвращает passed, завершай задачу: DONE outcome=done. Если tester возвращает failed, отправь code_diff (или merged_code) разработчику (developer) или merger на доработку с учетом замечаний.
(9) Если тестирование не требуется (нет тестов) и code_diff approved, допускается завершение: DONE outcome=done.
Отвечай ТОЛЬКО валидным JSON без markdown.',
    description = 'Принимает решения о следующем шаге задачи. Видит реестр агентов, метаданные артефактов (без content), in-flight jobs. Возвращает JSON {done, outcome, agents:[...], reason}.',
    updated_at = now()
WHERE role IN ('orchestrator', 'router');

-- 2. Update descriptions for other roles in agent_role_prompts
UPDATE agent_role_prompts SET description = 'Создаёт высокоуровневый план для задачи на основе её описания и контекста проекта. Учитывает Clean Architecture и правила проекта.', updated_at = now() WHERE role = 'planner';
UPDATE agent_role_prompts SET description = 'Разбивает approved-план на атомарные подзадачи с DAG зависимостей. Каждая подзадача выполняется одним разработчиком.', updated_at = now() WHERE role = 'decomposer';
UPDATE agent_role_prompts SET description = 'Проверяет любой артефакт (plan, subtask_description, code_diff, merged_code). Возвращает decision=approved или changes_requested с комментариями.', updated_at = now() WHERE role = 'reviewer';
UPDATE agent_role_prompts SET description = 'Пишет код в назначенном git worktree, запускает проверки, коммитит изменения в ветку.', updated_at = now() WHERE role = 'developer';
UPDATE agent_role_prompts SET description = 'Запускает unit и интеграционные тесты, линтер и сборку. Возвращает test_result с pass/fail и деталями.', updated_at = now() WHERE role = 'tester';
UPDATE agent_role_prompts SET description = 'Сливает параллельные ветки подзадач в единый merged_code артефакт, разрешая конфликты.', updated_at = now() WHERE role = 'merger';

-- 3. Update existing agents in the agents table
UPDATE agents a
SET system_prompt = p.content,
    role_description = p.description,
    updated_at = now()
FROM agent_role_prompts p
WHERE a.role = p.role;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 1. Revert content and description for orchestrator and router in agent_role_prompts
UPDATE agent_role_prompts
SET content = 'Ты — оркестратор задачи. Твоя задача — выбрать следующего агента (или нескольких параллельно) на основе текущего состояния. Жёсткие правила: (1) Каждый артефакт kind∈{plan,subtask_description,code_diff,merged_code} ОБЯЗАН пройти через reviewer перед использованием. (2) Если review.decision=changes_requested — отправляй автору на доработку. (3) Подзадачи с пустым depends_on (или все depends_on=done) запускай параллельно. (4) НЕ запускай два job на один и тот же target_artifact_id. (5) Когда все code_diff артефакты независимых подзадач approved и их >1 — вызывай merger. (6) Артефакт ревьюится >5 раз без approve → DONE outcome=failed. Отвечай ТОЛЬКО валидным JSON без markdown.',
    description = 'Системный промпт оркестратора проекта',
    updated_at = now()
WHERE role = 'orchestrator';

UPDATE agent_role_prompts
SET content = 'Ты — оркестратор задачи. Твоя задача — выбрать следующего агента (или нескольких параллельно) на основе текущего состояния. Жёсткие правила: (1) Каждый артефакт kind∈{plan,subtask_description,code_diff,merged_code} ОБЯЗАН пройти через reviewer перед использованием. (2) Если review.decision=changes_requested — отправляй автору на доработку. (3) Подзадачи с пустым depends_on (или все depends_on=done) запускай параллельно. (4) НЕ запускай два job на один и тот же target_artifact_id. (5) Когда все code_diff артефакты независимых подзадач approved и их >1 — вызывай merger. (6) Артефакт ревьюится >5 раз без approve → DONE outcome=failed. Отвечай ТОЛЬКО валидным JSON без markdown.',
    description = 'Системный промпт роутера задач',
    updated_at = now()
WHERE role = 'router';

-- 2. Revert descriptions for other roles in agent_role_prompts
UPDATE agent_role_prompts SET description = 'Системный промпт планировщика', updated_at = now() WHERE role = 'planner';
UPDATE agent_role_prompts SET description = 'Системный промпт декомпозитора задач', updated_at = now() WHERE role = 'decomposer';
UPDATE agent_role_prompts SET description = 'Системный промпт агента-ревьюера', updated_at = now() WHERE role = 'reviewer';
UPDATE agent_role_prompts SET description = 'Системный промпт агента-разработчика', updated_at = now() WHERE role = 'developer';
UPDATE agent_role_prompts SET description = 'Системный промпт агента-тестировщика', updated_at = now() WHERE role = 'tester';
UPDATE agent_role_prompts SET description = 'Системный промпт агента-мержера', updated_at = now() WHERE role = 'merger';

-- 3. Update existing agents in the agents table
UPDATE agents a
SET system_prompt = p.content,
    role_description = p.description,
    updated_at = now()
FROM agent_role_prompts p
WHERE a.role = p.role;

-- +goose StatementEnd
