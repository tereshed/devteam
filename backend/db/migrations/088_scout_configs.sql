-- +goose Up
-- +goose StatementBegin

-- scout_configs — per-project конфиг агента-разведчика (scout). Одна строка на
-- проект. Разведчик — это headless sandbox-прогон Claude Code CLI на ПОДПИСКЕ
-- (а не на metered API ассистента): он диспатчится проектным ассистентом, читает
-- весь репозиторий (primary + sibling-репо проекта) на диске и собирает досье
-- контекста для формулирования задачи. Конфиг задаёт включён ли скаут, его
-- редактируемый промпт, бэкенд (claude-code/hermes/antigravity) и какую
-- подписку использовать.
CREATE TABLE IF NOT EXISTS scout_configs (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID         NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    -- created_by — владелец конфига; от его имени резолвятся подписка/ключи
    -- (как enhancer_configs.created_by, scheduled_tasks.created_by).
    created_by      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_enabled      BOOLEAN      NOT NULL DEFAULT false,
    -- prompt — редактируемый промпт разведчика; пусто → рантайм использует
    -- встроенный дефолтный промпт (как projects.assistant_prompt: NULL/'' = дефолт).
    prompt          TEXT         NOT NULL DEFAULT '',
    -- code_backend — CLI внутри sandbox-образа (models.CodeBackend): claude-code
    -- (дефолт, OAuth-подписка), hermes, antigravity.
    code_backend    VARCHAR(32)  NOT NULL DEFAULT 'claude-code',
    -- subscription_id — какая подключённая Claude-подписка
    -- (claude_code_subscriptions) используется для прогона. NULL → дефолтная
    -- подписка владельца. Сейчас подписка одна на пользователя (UNIQUE user_id),
    -- поле держим селектор-ready под мульти-аккаунт (по образцу git multi-account).
    subscription_id UUID         REFERENCES claude_code_subscriptions(id) ON DELETE SET NULL,
    -- timeout_seconds — жёсткий потолок прогона разведчика в sandbox.
    timeout_seconds INTEGER      NOT NULL DEFAULT 600,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_scout_timeout CHECK (timeout_seconds BETWEEN 60 AND 3600)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS scout_configs;

-- +goose StatementEnd
