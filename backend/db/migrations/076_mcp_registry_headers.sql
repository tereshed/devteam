-- +goose Up
-- +goose StatementBegin

-- headers_template — шаблон HTTP-заголовков для remote (sse/http) MCP-серверов.
-- Значения могут содержать ${secret:NAME}; при сборке .mcp.json секрет резолвится
-- в env-переменную sandbox (MCP_*), а в файл пишется ${VAR} — токен не утекает в репо.
ALTER TABLE mcp_servers_registry ADD COLUMN headers_template jsonb NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE mcp_servers_registry DROP COLUMN IF EXISTS headers_template;
-- +goose StatementEnd
