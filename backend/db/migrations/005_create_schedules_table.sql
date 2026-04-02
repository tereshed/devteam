-- +goose Up
-- +goose StatementBegin
CREATE TABLE scheduled_workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE, -- Уникальное имя расписания
    workflow_name VARCHAR(255) NOT NULL, -- Какое workflow запускать
    cron_expression VARCHAR(50) NOT NULL, -- Cron формат: "0 8 * * *"
    input_template TEXT, -- Входные данные (можно шаблон)
    is_active BOOLEAN DEFAULT true,
    last_run_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS scheduled_workflows;
-- +goose StatementEnd

