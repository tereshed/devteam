-- +goose Up
-- +goose StatementBegin

-- assistant_mcp_servers — per-project удалённые MCP-серверы для IN-PROCESS петли
-- ассистента (agentloop поверх OpenRouter/Gemini; инструменты отдаются как обычные
-- function-tools). Это ОТДЕЛЬНЫЙ путь от mcp_servers_registry (тот — для sandbox-
-- агентов через .mcp.json).
--
-- Remote-only по решению безопасности: transport ∈ {http, sse}. stdio запрещён
-- (запускал бы произвольную команду внутри backend-процесса = RCE в мультитенанте) —
-- зафиксировано CHECK-констрейнтом.
--
-- headers: JSONB map[string]string; значения могут нести ${secret:NAME}, которые
-- резолвятся через SecretResolver на момент подключения (как в sandbox-пути), поэтому
-- сами секреты в этой таблице не хранятся.
-- require_confirmation: гейтить ли каждый вызов инструмента через park-флоу подтверждения.
CREATE TABLE IF NOT EXISTS assistant_mcp_servers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                 VARCHAR(255) NOT NULL,
    transport            VARCHAR(16) NOT NULL DEFAULT 'http',
    url                  VARCHAR(1024) NOT NULL,
    headers              JSONB NOT NULL DEFAULT '{}',
    require_confirmation BOOLEAN NOT NULL DEFAULT TRUE,
    is_enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    CONSTRAINT uq_assistant_mcp_project_name UNIQUE (project_id, name),
    CONSTRAINT ck_assistant_mcp_transport CHECK (transport IN ('http', 'sse'))
);

CREATE INDEX IF NOT EXISTS idx_assistant_mcp_project ON assistant_mcp_servers (project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS assistant_mcp_servers;

-- +goose StatementEnd
