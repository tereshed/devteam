-- +goose Up
-- +goose StatementBegin
CREATE TABLE prompts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    template TEXT NOT NULL,
    json_schema JSONB,                 -- Схема для валидации JSON-ответа от LLM (опционально)
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Создаем индекс для быстрого поиска по имени (так как оно уникально и используется для лукапа)
CREATE INDEX idx_prompts_name ON prompts(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS prompts;
-- +goose StatementEnd

