-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — durable очередь задач/jobs.
-- Yugabyte НЕ поддерживает LISTEN/NOTIFY — wakeup через Redis Pub/Sub,
-- забор работы через polling + SELECT ... FOR UPDATE SKIP LOCKED (Yugabyte 2.18+).
--
-- kind:
--   step_req  — пнуть Orchestrator.Step для задачи (Router решит что дальше)
--   agent_job — запустить конкретного агента с заданным input
--
-- payload (для agent_job):
--   { "agent": "developer",
--     "input": { "target_artifact_id": "uuid", "instructions": "..." },
--     "worktree_id": "uuid" }   -- nil для llm-агентов
--
-- Retry-семантика: при error worker делает UPDATE attempts=attempts+1, last_error=..,
-- scheduled_at = now() + backoff. После max_attempts событие "умирает" —
-- остаётся в таблице (для аудита), но больше не забирается. Следующий step_req
-- даст Router'у шанс обработать ситуацию.

CREATE TABLE task_events (
    id            BIGSERIAL PRIMARY KEY,
    task_id       UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind          VARCHAR(32) NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    scheduled_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    locked_by     VARCHAR(255),
    locked_at     TIMESTAMP WITH TIME ZONE,
    attempts      INTEGER NOT NULL DEFAULT 0,
    max_attempts  INTEGER NOT NULL DEFAULT 3,
    last_error    TEXT,
    completed_at  TIMESTAMP WITH TIME ZONE,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_task_events_kind
        CHECK (kind IN ('step_req', 'agent_job')),
    CONSTRAINT chk_task_events_attempts_non_negative
        CHECK (attempts >= 0),
    CONSTRAINT chk_task_events_max_attempts_positive
        CHECK (max_attempts > 0),
    CONSTRAINT chk_task_events_lock_consistency
        CHECK (
            (locked_by IS NULL AND locked_at IS NULL) OR
            (locked_by IS NOT NULL AND locked_at IS NOT NULL)
        )
);

-- Главный poll-индекс: воркер берёт ближайшее по времени НЕ-залоченное событие
-- нужного типа, у которого ещё есть попытки и оно не завершено.
--
-- ВАЖНО: условие `attempts < max_attempts` — критично для долгосрочной
-- производительности. Без него "мёртвые" события (attempts == max_attempts,
-- completed_at IS NULL) копились бы в индексе вечно и заставляли воркер
-- сканировать растущий мусор при каждом polling-цикле.
CREATE INDEX idx_task_events_pollable ON task_events(kind, scheduled_at)
    WHERE locked_by IS NULL AND completed_at IS NULL AND attempts < max_attempts;

-- Для observability и cron-операций по освобождению "застрявших" locks.
CREATE INDEX idx_task_events_task      ON task_events(task_id);
CREATE INDEX idx_task_events_locked    ON task_events(locked_at) WHERE locked_by IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
DROP TABLE IF EXISTS task_events;

-- +goose StatementEnd
