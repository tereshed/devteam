-- +goose Up
-- +goose StatementBegin

-- Таблица webhook триггеров
CREATE TABLE webhook_triggers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    workflow_name VARCHAR(255) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Настройки обработки входных данных
    input_json_path TEXT,
    input_template TEXT,
    
    -- Настройки безопасности
    allowed_ips TEXT,
    require_secret BOOLEAN DEFAULT TRUE,
    
    -- Статистика
    trigger_count BIGINT DEFAULT 0,
    last_triggered TIMESTAMP,
    
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Таблица логов webhook
CREATE TABLE webhook_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhook_triggers(id) ON DELETE CASCADE,
    execution_id UUID REFERENCES executions(id) ON DELETE SET NULL,
    
    -- Информация о запросе
    source_ip VARCHAR(45) NOT NULL,
    method VARCHAR(10) NOT NULL,
    headers TEXT,
    body TEXT,
    
    -- Результат
    success BOOLEAN DEFAULT FALSE,
    error_message TEXT,
    response_code INT DEFAULT 0,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Индексы
CREATE INDEX idx_webhook_triggers_workflow ON webhook_triggers(workflow_name);
CREATE INDEX idx_webhook_triggers_active ON webhook_triggers(is_active);
CREATE INDEX idx_webhook_logs_webhook_id ON webhook_logs(webhook_id);
CREATE INDEX idx_webhook_logs_created_at ON webhook_logs(created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS webhook_logs;
DROP TABLE IF EXISTS webhook_triggers;

-- +goose StatementEnd

