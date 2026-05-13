-- +goose Up
-- +goose StatementBegin

-- Sprint 16: Hermes Agent (Nous Research) как третий code_backend.
-- Расширяем CHECK ограничение enum'а code_backend.
-- Никаких ALTER COLUMN data — это только enum extension.

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'hermes', 'custom'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Возвращаем CHECK без 'hermes'. ВНИМАНИЕ: data loss — все строки с
-- code_backend='hermes' нужно перевести во что-то другое перед откатом.
UPDATE agents SET code_backend = NULL WHERE code_backend = 'hermes';

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'custom'
    ));

-- +goose StatementEnd
