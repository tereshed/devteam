-- +goose Up
-- +goose StatementBegin
UPDATE agents SET
    execution_kind = 'sandbox',
    code_backend = 'claude-code',
    model = NULL,
    provider_kind = NULL,
    temperature = NULL,
    max_tokens = NULL,
    code_backend_settings = '{"permission_mode": "auto"}'::jsonb,
    sandbox_permissions = '{"env_secret_keys": ["ANTHROPIC_API_KEY"]}'::jsonb,
    requires_code_context = true,
    updated_at = NOW()
WHERE role = 'reviewer';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE agents SET
    execution_kind = 'llm',
    code_backend = NULL,
    provider_kind = 'anthropic',
    model = 'claude-haiku-4-5-20251001',
    temperature = 0.2,
    max_tokens = 8192,
    code_backend_settings = '{}'::jsonb,
    sandbox_permissions = '{}'::jsonb,
    requires_code_context = false,
    updated_at = NOW()
WHERE role = 'reviewer';
-- +goose StatementEnd
