-- +goose Up
-- +goose StatementBegin

-- === 1. ALTER agents: новые колонки ===
ALTER TABLE agents ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;
ALTER TABLE agents ADD COLUMN model VARCHAR(255);
ALTER TABLE agents ADD COLUMN skills JSONB NOT NULL DEFAULT '[]';
ALTER TABLE agents ADD COLUMN code_backend VARCHAR(50);
ALTER TABLE agents ADD COLUMN settings JSONB NOT NULL DEFAULT '{}';

ALTER TABLE agents ADD CONSTRAINT chk_agents_code_backend
    CHECK (code_backend IS NULL OR code_backend IN ('claude-code', 'aider', 'custom'));

ALTER TABLE agents ADD CONSTRAINT chk_agents_role
    CHECK (role IN ('worker', 'supervisor', 'orchestrator', 'planner', 'developer', 'reviewer', 'tester', 'devops'));

-- Заменяем глобальный UNIQUE(name) на partial index (team_id, name)
ALTER TABLE agents DROP CONSTRAINT agents_name_key;
CREATE UNIQUE INDEX idx_agents_team_name ON agents(team_id, name) WHERE team_id IS NOT NULL;

CREATE INDEX idx_agents_team_id ON agents(team_id);
CREATE INDEX idx_agents_role ON agents(role);

-- === 2. tool_definitions ===
CREATE TABLE tool_definitions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL UNIQUE,
    description       TEXT NOT NULL,
    category          VARCHAR(100) NOT NULL,
    parameters_schema JSONB NOT NULL DEFAULT '{}',
    is_builtin        BOOLEAN NOT NULL DEFAULT true,
    is_active         BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_tool_definitions_category ON tool_definitions(category);
CREATE INDEX idx_tool_definitions_is_active ON tool_definitions(is_active) WHERE is_active = true;

-- === 3. agent_tool_bindings ===
CREATE TABLE agent_tool_bindings (
    agent_id           UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_definition_id UUID NOT NULL REFERENCES tool_definitions(id) ON DELETE CASCADE,
    config             JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (agent_id, tool_definition_id)
);

-- === 4. mcp_server_configs ===
CREATE TABLE mcp_server_configs (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id            UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                  VARCHAR(255) NOT NULL,
    url                   VARCHAR(1024) NOT NULL,
    auth_type             VARCHAR(50) NOT NULL DEFAULT 'none',
    encrypted_credentials BYTEA,
    settings              JSONB NOT NULL DEFAULT '{}',
    is_active             BOOLEAN NOT NULL DEFAULT true,
    created_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT chk_mcp_auth_type CHECK (auth_type IN ('none', 'api_key', 'oauth', 'bearer')),
    CONSTRAINT uq_mcp_project_name UNIQUE (project_id, name)
);

CREATE INDEX idx_mcp_server_configs_project_id ON mcp_server_configs(project_id);
CREATE INDEX idx_mcp_server_configs_is_active ON mcp_server_configs(is_active) WHERE is_active = true;

-- === 5. agent_mcp_bindings ===
CREATE TABLE agent_mcp_bindings (
    agent_id             UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_config_id UUID NOT NULL REFERENCES mcp_server_configs(id) ON DELETE CASCADE,
    settings             JSONB NOT NULL DEFAULT '{}',
    created_at           TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (agent_id, mcp_server_config_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS agent_mcp_bindings;
DROP TABLE IF EXISTS mcp_server_configs;
DROP TABLE IF EXISTS agent_tool_bindings;
DROP TABLE IF EXISTS tool_definitions;

DROP INDEX IF EXISTS idx_agents_role;
DROP INDEX IF EXISTS idx_agents_team_id;
DROP INDEX IF EXISTS idx_agents_team_name;

ALTER TABLE agents ADD CONSTRAINT agents_name_key UNIQUE (name);

ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_role;
ALTER TABLE agents DROP CONSTRAINT IF EXISTS chk_agents_code_backend;

ALTER TABLE agents DROP COLUMN IF EXISTS settings;
ALTER TABLE agents DROP COLUMN IF EXISTS code_backend;
ALTER TABLE agents DROP COLUMN IF EXISTS skills;
ALTER TABLE agents DROP COLUMN IF EXISTS model;
ALTER TABLE agents DROP COLUMN IF EXISTS team_id;

-- +goose StatementEnd
