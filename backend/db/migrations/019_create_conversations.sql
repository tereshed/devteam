-- +goose Up
-- +goose StatementBegin

CREATE TABLE conversations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(500) NOT NULL DEFAULT '',
    status      VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_conversations_status CHECK (status IN ('active', 'completed', 'archived'))
);

CREATE INDEX idx_conversations_project_id ON conversations(project_id);
CREATE INDEX idx_conversations_user_id ON conversations(user_id);
CREATE INDEX idx_conversations_project_status ON conversations(project_id, status);

CREATE TABLE conversation_messages (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id   UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role              VARCHAR(50) NOT NULL,
    content           TEXT NOT NULL,
    linked_task_ids   UUID[] NOT NULL DEFAULT '{}',
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_conv_messages_role CHECK (role IN ('user', 'assistant', 'system'))
);

CREATE INDEX idx_conv_messages_conversation_id ON conversation_messages(conversation_id);
CREATE INDEX idx_conv_messages_conv_created ON conversation_messages(conversation_id, created_at);
CREATE INDEX idx_conv_messages_role ON conversation_messages(role);
CREATE INDEX idx_conv_messages_linked_tasks ON conversation_messages USING GIN (linked_task_ids);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_conv_messages_linked_tasks;
DROP INDEX IF EXISTS idx_conv_messages_role;
DROP INDEX IF EXISTS idx_conv_messages_conv_created;
DROP INDEX IF EXISTS idx_conv_messages_conversation_id;
DROP TABLE IF EXISTS conversation_messages;

DROP INDEX IF EXISTS idx_conversations_project_status;
DROP INDEX IF EXISTS idx_conversations_user_id;
DROP INDEX IF EXISTS idx_conversations_project_id;
DROP TABLE IF EXISTS conversations;

-- +goose StatementEnd
