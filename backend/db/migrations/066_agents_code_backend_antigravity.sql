-- +goose Up
-- +goose StatementBegin

-- Sprint 21: Add 'antigravity' as a fifth code_backend.
-- Expand check constraint on agents table.
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'hermes', 'custom', 'antigravity'
    ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove 'antigravity' from the check constraint.
-- Deactivate and set code_backend = NULL for any antigravity agents.
DO $$
DECLARE
    affected INT;
BEGIN
    UPDATE agents
       SET code_backend = NULL,
           is_active    = false
     WHERE code_backend = 'antigravity';
    GET DIAGNOSTICS affected = ROW_COUNT;
    IF affected > 0 THEN
        RAISE NOTICE 'sprint_antigravity down: % agents had code_backend=antigravity; set to NULL + is_active=false.', affected;
    END IF;
END $$;

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;
ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN (
        'claude-code', 'aider', 'hermes', 'custom'
    ));

-- +goose StatementEnd
