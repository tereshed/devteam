-- +goose Up
-- +goose StatementBegin

-- Per-project промпт ассистента: копия промпта ассистента владельца на момент
-- создания проекта (наследование role → user → project, каждая копия дальше
-- редактируется независимо). NULL = legacy-проект без снапшота → рантайм
-- (assistant_loop) падает обратно на user-промпт ассистента, как раньше.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS assistant_prompt TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN IF EXISTS assistant_prompt;

-- +goose StatementEnd
