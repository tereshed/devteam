-- +goose Up

-- Phase 2 §2.4: Remove the legacy global assistant agent (user_id IS NULL, team_id IS NULL).
-- Per-user assistants are now auto-created on registration via AgentService.CreateDefaultAssistant.
-- The global agent is no longer needed after Phase 2.

-- +goose StatementBegin
DELETE FROM agents
WHERE name = 'assistant'
  AND role = 'assistant'
  AND user_id IS NULL
  AND team_id IS NULL;
-- +goose StatementEnd


-- +goose Down

-- Re-creating the global assistant is handled by SeedAssistantAgent at startup.
-- No-op: the seed function will recreate it on next startup if needed.
