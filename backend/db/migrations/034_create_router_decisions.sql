-- +goose Up
-- +goose StatementBegin

-- Sprint 17 / Orchestration v2 — лог решений Router-агента.
-- encrypted_raw_response — blob pkg/crypto (AES-256-GCM, AAD = id записи).
-- Формат: [version 1b][nonce 12b][sealed]; nonce внутри blob, отдельная колонка не нужна.
-- Минимум 29 байт (MinCiphertextBlobLen).
-- Retention: 30 дней через cron-job, см. internal/service/router_decisions_retention.go.
--
-- reason — короткое объяснение (1-2 предложения), НЕ-sensitive, в открытом виде:
-- удобно для UI/grep при дебаге, не содержит полного промпта или ответа.
-- chosen_agents — массив для аналитики "что обычно запускают параллельно".

CREATE TABLE router_decisions (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id                  UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    step_no                  INTEGER NOT NULL,
    chosen_agents            TEXT[] NOT NULL DEFAULT '{}',
    outcome                  VARCHAR(32),
    reason                   TEXT NOT NULL,
    encrypted_raw_response   BYTEA,
    created_at               TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT chk_router_decisions_step_non_negative
        CHECK (step_no >= 0),
    CONSTRAINT chk_router_decisions_outcome
        CHECK (outcome IS NULL OR outcome IN ('done', 'failed', 'needs_human', 'cancelled')),
    -- Если raw_response сохраняем — он обязан быть валидным blob pkg/crypto (≥ 29 байт).
    CONSTRAINT chk_router_decisions_cipher_minlen
        CHECK (encrypted_raw_response IS NULL OR octet_length(encrypted_raw_response) >= 29)
);

CREATE INDEX idx_router_decisions_task_step ON router_decisions(task_id, step_no);
CREATE INDEX idx_router_decisions_created   ON router_decisions(created_at);  -- для retention cron

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
DROP TABLE IF EXISTS router_decisions;

-- +goose StatementEnd
