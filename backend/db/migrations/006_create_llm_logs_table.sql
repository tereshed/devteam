-- +goose Up
-- +goose StatementBegin
CREATE TABLE llm_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    prompt_snapshot JSONB, -- Может быть большим, поэтому TEXT. Если нужно JSON, можно JSONB
    response_snapshot JSONB,
    duration_ms INT DEFAULT 0,
    
    -- Trace info
    workflow_execution_id UUID REFERENCES executions(id) ON DELETE SET NULL,
    step_id VARCHAR(255),
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llm_logs_created_at ON llm_logs(created_at);
CREATE INDEX idx_llm_logs_execution_id ON llm_logs(workflow_execution_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS llm_logs;
-- +goose StatementEnd

