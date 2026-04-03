-- +goose Up
-- +goose StatementBegin

CREATE TABLE task_messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id       UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    sender_type   VARCHAR(50) NOT NULL,
    sender_id     UUID NOT NULL,
    content       TEXT NOT NULL,
    message_type  VARCHAR(50) NOT NULL,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_task_messages_sender_type CHECK (sender_type IN ('user', 'agent')),
    CONSTRAINT chk_task_messages_message_type CHECK (message_type IN (
        'instruction', 'result', 'question', 'feedback', 'error'
    ))
);

CREATE INDEX idx_task_messages_task_id ON task_messages(task_id);
CREATE INDEX idx_task_messages_sender ON task_messages(sender_type, sender_id);
CREATE INDEX idx_task_messages_message_type ON task_messages(message_type);
CREATE INDEX idx_task_messages_task_created ON task_messages(task_id, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_task_messages_task_created;
DROP INDEX IF EXISTS idx_task_messages_message_type;
DROP INDEX IF EXISTS idx_task_messages_sender;
DROP INDEX IF EXISTS idx_task_messages_task_id;
DROP TABLE IF EXISTS task_messages;

-- +goose StatementEnd
