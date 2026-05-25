-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    t RECORD;
    prompt_content TEXT;
    prompt_desc TEXT;
BEGIN
    FOR t IN SELECT id FROM teams LOOP
        -- 1. orchestrator
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'orchestrator') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'orchestrator';
            INSERT INTO agents (id, name, role, execution_kind, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active)
            VALUES (gen_random_uuid(), 'orchestrator', 'orchestrator', 'llm', prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, true);
        END IF;

        -- 2. router
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'router') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'router';
            INSERT INTO agents (id, name, role, execution_kind, provider_kind, model, temperature, max_tokens, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active)
            VALUES (gen_random_uuid(), 'router', 'router', 'llm', 'openrouter', 'deepseek/deepseek-v4-flash', 0.2, 4096, prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, true);
        END IF;

        -- 3. planner
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'planner') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'planner';
            INSERT INTO agents (id, name, role, execution_kind, provider_kind, model, temperature, max_tokens, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active)
            VALUES (gen_random_uuid(), 'planner', 'planner', 'llm', 'openrouter', 'deepseek/deepseek-v4-flash', 0.3, 8192, prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, true);
        END IF;

        -- 4. decomposer
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'decomposer') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'decomposer';
            INSERT INTO agents (id, name, role, execution_kind, provider_kind, model, temperature, max_tokens, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active)
            VALUES (gen_random_uuid(), 'decomposer', 'decomposer', 'llm', 'openrouter', 'deepseek/deepseek-v4-flash', 0.3, 8192, prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, true);
        END IF;

        -- 5. reviewer
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'reviewer') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'reviewer';
            INSERT INTO agents (id, name, role, execution_kind, provider_kind, model, temperature, max_tokens, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active)
            VALUES (gen_random_uuid(), 'reviewer', 'reviewer', 'llm', 'anthropic', 'claude-haiku-4-5-20251001', 0.2, 8192, prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, true);
        END IF;

        -- 6. developer
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'developer') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'developer';
            INSERT INTO agents (id, name, role, execution_kind, code_backend, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active, requires_code_context)
            VALUES (gen_random_uuid(), 'developer', 'developer', 'sandbox', 'claude-code', prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"permission_mode": "auto"}'::jsonb, '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb, true, true);
        END IF;

        -- 7. tester
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'tester') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'tester';
            INSERT INTO agents (id, name, role, execution_kind, code_backend, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active, requires_code_context)
            VALUES (gen_random_uuid(), 'tester', 'tester', 'sandbox', 'claude-code', prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"permission_mode": "auto"}'::jsonb, '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb, true, true);
        END IF;

        -- 8. merger
        IF NOT EXISTS (SELECT 1 FROM agents WHERE team_id = t.id AND role = 'merger') THEN
            SELECT content, description INTO prompt_content, prompt_desc FROM agent_role_prompts WHERE role = 'merger';
            INSERT INTO agents (id, name, role, execution_kind, code_backend, system_prompt, role_description, team_id, skills, settings, model_config, code_backend_settings, sandbox_permissions, is_active, requires_code_context)
            VALUES (gen_random_uuid(), 'merger', 'merger', 'sandbox', 'claude-code', prompt_content, prompt_desc, t.id, '[]'::jsonb, '{}'::jsonb, '{}'::jsonb, '{"permission_mode": "auto"}'::jsonb, '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb, true, true);
        END IF;
    END LOOP;
END$$;
-- +goose StatementEnd

-- Delete old global system agents (where team_id IS NULL AND user_id IS NULL)
-- +goose StatementBegin
DELETE FROM agents
 WHERE team_id IS NULL
   AND user_id IS NULL
   AND name IN ('router', 'planner', 'decomposer', 'reviewer', 'developer', 'tester', 'merger');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- +goose StatementEnd
