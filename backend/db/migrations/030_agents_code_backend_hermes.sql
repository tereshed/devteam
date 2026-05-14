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

-- Возвращаем CHECK без 'hermes'. ВНИМАНИЕ: data loss — Hermes-агенты
-- получают code_backend=NULL И is_active=false, чтобы оркестратор не пытался
-- их рутить и оператор видел, какие записи деактивированы. Полный rollback
-- (восстановление code_backend='hermes') невозможен после повторного up.
DO $$
DECLARE
    affected INT;
BEGIN
    UPDATE agents
       SET code_backend = NULL,
           is_active    = false
     WHERE code_backend = 'hermes';
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected > 0 THEN
        RAISE NOTICE 'sprint16 down: % agents had code_backend=hermes; set to NULL + is_active=false. Reactivate manually after re-routing.', affected;
    END IF;
END $$;

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'custom'
    ));

-- +goose StatementEnd
