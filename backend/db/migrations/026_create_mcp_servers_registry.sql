-- +goose Up
-- +goose StatementBegin

-- mcp_servers_registry — глобальный каталог MCP-серверов (Sprint 15.4).
-- В отличие от mcp_server_configs (привязан к project_id), registry хранит шаблоны/глобальные серверы,
-- доступные на разных scope: global | project | agent.
CREATE TABLE mcp_servers_registry (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(255) NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    transport     VARCHAR(16)  NOT NULL,
    command       VARCHAR(1024) NOT NULL DEFAULT '',
    args          JSONB        NOT NULL DEFAULT '[]',
    url           VARCHAR(1024) NOT NULL DEFAULT '',
    env_template  JSONB        NOT NULL DEFAULT '{}',
    scope         VARCHAR(16)  NOT NULL DEFAULT 'global',
    is_active     BOOLEAN      NOT NULL DEFAULT true,
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_mcp_registry_transport CHECK (transport IN ('stdio', 'http', 'sse')),
    CONSTRAINT chk_mcp_registry_scope     CHECK (scope IN ('global', 'project', 'agent')),
    CONSTRAINT uq_mcp_registry_name UNIQUE (name)
);

CREATE INDEX idx_mcp_servers_registry_scope     ON mcp_servers_registry(scope);
CREATE INDEX idx_mcp_servers_registry_is_active ON mcp_servers_registry(is_active) WHERE is_active = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_mcp_servers_registry_is_active;
DROP INDEX IF EXISTS idx_mcp_servers_registry_scope;
DROP TABLE IF EXISTS mcp_servers_registry;

-- +goose StatementEnd
