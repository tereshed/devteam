-- +goose Up
-- +goose StatementBegin
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL, -- 'worker', 'supervisor'
    prompt_id UUID REFERENCES prompts(id),
    model_config JSONB, -- { "temperature": 0.7, "model": "gpt-4" }
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    configuration JSONB NOT NULL, -- Граф шагов: steps, start_step, etc.
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID REFERENCES workflows(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- 'pending', 'running', 'completed', 'failed', 'cancelled'
    current_step_id VARCHAR(255),
    input_data TEXT, -- Входные данные от пользователя
    context JSONB DEFAULT '{}', -- Общая память исполнения
    step_count INT DEFAULT 0, -- Текущее количество шагов
    max_steps INT DEFAULT 20, -- Лимит шагов (защита от циклов)
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE execution_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID REFERENCES executions(id) ON DELETE CASCADE,
    step_id VARCHAR(255) NOT NULL, -- ID шага из конфигурации workflow
    agent_id UUID REFERENCES agents(id),
    prompt_snapshot TEXT, -- Промпт, который был использован (snapshot)
    input_context TEXT, -- Что подали на вход агенту
    output_content TEXT, -- Что ответил агент
    tokens_used INT DEFAULT 0,
    duration_ms INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы
CREATE INDEX idx_executions_workflow_id ON executions(workflow_id);
CREATE INDEX idx_executions_status ON executions(status);
CREATE INDEX idx_execution_steps_execution_id ON execution_steps(execution_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS execution_steps;
DROP TABLE IF EXISTS executions;
DROP TABLE IF EXISTS workflows;
DROP TABLE IF EXISTS agents;
-- +goose StatementEnd

