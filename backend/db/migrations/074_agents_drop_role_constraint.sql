-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_role;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents ADD CONSTRAINT chk_agents_role CHECK (role::text = ANY (ARRAY['worker'::character varying, 'supervisor'::character varying, 'orchestrator'::character varying, 'planner'::character varying, 'developer'::character varying, 'reviewer'::character varying, 'tester'::character varying, 'devops'::character varying, 'router'::character varying, 'decomposer'::character varying, 'merger'::character varying, 'assistant'::character varying]::text[]));
-- +goose StatementEnd

