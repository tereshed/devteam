-- +goose Up
-- +goose StatementBegin
CREATE TABLE llm_models (
    id VARCHAR(255) PRIMARY KEY, -- ID от OpenRouter (например, "openai/gpt-4o")
    name VARCHAR(255) NOT NULL,
    description TEXT,
    context_length INT DEFAULT 0,
    architecture JSONB DEFAULT '{}',
    
    -- Pricing (храним как decimal для точности, значения могут быть очень малыми)
    pricing_prompt NUMERIC(20, 10) DEFAULT 0,
    pricing_completion NUMERIC(20, 10) DEFAULT 0,
    pricing_request NUMERIC(20, 10) DEFAULT 0,
    pricing_image NUMERIC(20, 10) DEFAULT 0,
    
    is_active BOOLEAN DEFAULT true,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_llm_models_is_active ON llm_models(is_active);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS llm_models;
-- +goose StatementEnd

