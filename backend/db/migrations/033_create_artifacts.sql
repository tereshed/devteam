-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — однородная таблица артефактов.
-- Заменяет: tasks.artifacts (JSONB-bag) + task_messages для промежуточных результатов.
--
-- Артефакты — это ВСЁ, что производят агенты: план, описание подзадачи (с DAG depends_on),
-- code_diff, review-вердикт, test_result, merged_code, и т.д.
--
-- kind намеренно НЕ enum — новые типы добавляются без миграции.
-- summary — обязательный краткий текст ≤500 chars, идёт в промпт Router'у (бюджет контекста).
-- content — полный JSON, Router его НЕ читает; загружают только специалисты по artifact_id.

CREATE TABLE artifacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    parent_id       UUID REFERENCES artifacts(id) ON DELETE SET NULL,
    producer_agent  VARCHAR(255) NOT NULL,
    kind            VARCHAR(64) NOT NULL,
    summary         VARCHAR(500) NOT NULL,
    content         JSONB NOT NULL DEFAULT '{}',
    status          VARCHAR(32) NOT NULL DEFAULT 'ready',
    iteration       INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_artifacts_status
        CHECK (status IN ('ready', 'superseded')),
    CONSTRAINT chk_artifacts_iteration_non_negative
        CHECK (iteration >= 0),
    CONSTRAINT chk_artifacts_summary_not_empty
        CHECK (length(trim(summary)) > 0)
);

CREATE INDEX idx_artifacts_task_created    ON artifacts(task_id, created_at);
CREATE INDEX idx_artifacts_parent          ON artifacts(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_artifacts_task_kind       ON artifacts(task_id, kind);
CREATE INDEX idx_artifacts_task_ready      ON artifacts(task_id) WHERE status = 'ready';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
DROP TABLE IF EXISTS artifacts;

-- +goose StatementEnd
