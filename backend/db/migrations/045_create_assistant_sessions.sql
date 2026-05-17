-- +goose Up
-- +goose StatementBegin

-- Sprint 21 / Assistant Sidebar — глобальный LLM-ассистент пользователя.
-- См. docs/tasks/21-assistant-sidebar.md §1.
--
-- Scope: ассистент уровня пользователя (без project_id) — управляет приложением,
-- запускает MCP-инструменты от имени user'а, поддерживает destructive-confirm.
--
-- Инварианты сериализации (см. §3.1 плана):
--   busy=TRUE                — активна агент-петля (≤ 1 на сессию)
--   busy_since               — момент захвата; stale-recovery cron сбрасывает
--                              busy у сессий старше 10 минут БЕЗ pending_tool_call_id
--   pending_tool_call_id     — петля припаркована до прихода ConfirmToolCall
--
-- Захват: CAS-update `WHERE busy=FALSE` в одной транзакции с авторизацией.
-- При busy=TRUE handler возвращает 409 Conflict (см. §3.1, §4.1).

CREATE TABLE assistant_sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title                 VARCHAR(255),
    status                VARCHAR(32) NOT NULL DEFAULT 'active',
    busy                  BOOLEAN NOT NULL DEFAULT FALSE,
    busy_since            TIMESTAMP WITH TIME ZONE,
    pending_tool_call_id  VARCHAR(64),
    metadata              JSONB,
    last_message_at       TIMESTAMP WITH TIME ZONE,
    created_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_assistant_sessions_status
        CHECK (status IN ('active', 'archived')),
    -- busy_since должен быть выставлен ровно тогда, когда busy=TRUE.
    -- Защита от рассинхрона при ручных правках/неаккуратных UPDATE'ах.
    CONSTRAINT chk_assistant_sessions_busy_consistency
        CHECK (
            (busy = FALSE AND busy_since IS NULL) OR
            (busy = TRUE  AND busy_since IS NOT NULL)
        )
);

-- +goose StatementEnd

-- +goose StatementBegin
-- Главный read-индекс sidebar'а: список сессий пользователя, отсортированный
-- по последней активности (DESC явно — Yugabyte не делает backward-scan
-- по распределённым tablet'ам эффективно).
CREATE INDEX idx_assistant_sessions_user
    ON assistant_sessions(user_id, last_message_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin

-- assistant_messages: сообщения сессии (user / assistant / tool / system).
--
-- Для role=assistant с tool_call:
--   tool_call_id, tool_name, tool_arguments заполнены; content может быть NULL
--   (модель решила сразу позвать инструмент без текста). tool_result всегда NULL.
-- Для role=tool:
--   tool_call_id ссылается на парный assistant-row с тем же tool_call_id;
--   tool_result содержит payload MCP-вызова (см. AuthorizedExecutor, §3.3).
--   До прихода ConfirmToolCall row может существовать с tool_result IS NULL
--   (pending state) — именно его «закрывает» атомарный UPDATE из §4.1.
-- ВАЖНО: один и тот же tool_call_id присутствует РОВНО в двух rows
-- (assistant + tool). Поэтому partial UNIQUE по tool_call_id ниже фильтруется
-- по role='tool' — иначе вставка tool-row после assistant-row упадёт с
-- duplicate key violation.
--
-- client_message_id — идемпотентность user-сообщений (ретраи, дабл-клик).
-- updated_at — нужен для атомарного UPDATE в ConfirmToolCall (§4.1):
--   UPDATE ... SET tool_result=?, updated_at=NOW() WHERE tool_call_id=? AND tool_result IS NULL

CREATE TABLE assistant_messages (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id         UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
    role               VARCHAR(16) NOT NULL,
    content            TEXT,
    tool_call_id       VARCHAR(64),
    tool_name          VARCHAR(128),
    tool_arguments     JSONB,
    tool_result        JSONB,
    client_message_id  VARCHAR(64),
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_assistant_messages_role
        CHECK (role IN ('user', 'assistant', 'tool', 'system'))
);

-- +goose StatementEnd

-- +goose StatementBegin
-- Идемпотентность user-сообщений: повторный POST с тем же client_message_id
-- упадёт на этом индексе → handler вернёт 202 как no-op (см. §4.1).
CREATE UNIQUE INDEX idx_assistant_messages_client_id
    ON assistant_messages(session_id, client_message_id)
    WHERE client_message_id IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
-- Пагинация истории: ORDER BY created_at DESC, id DESC.
-- DESC в индексе избавляет от backward scan по распределённым tablet'ам (Yugabyte).
-- id DESC как тай-брейкер критичен: pgx/Yugabyte пишут в created_at время
-- начала транзакции, и пачка tool_call/tool_result сообщений, сохранённая в
-- одной транзакции (типичный кейс при multi-tool-call в одном ответе LLM),
-- получит одинаковый timestamp с микросекундной точностью. Без вторичного
-- ключа сортировки порядок отдачи нестабилен → история «прыгает» в чате.
-- Контракт repository: ListMessages всегда сортирует по (created_at, id) DESC
-- и пагинирует курсором (before_created_at, before_id), а не одним before_id.
CREATE INDEX idx_assistant_messages_session
    ON assistant_messages(session_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- Атомарность confirm: tool-row на конкретный tool_call_id может быть
-- ровно один. Используется ConfirmToolCall:
--   UPDATE ... WHERE tool_call_id=? AND tool_result IS NULL (см. §4.1)
-- и защищает от параллельных INSERT'ов pending tool-row из двух горутин.
--
-- Фильтр role='tool' критичен: парный assistant-row имеет тот же tool_call_id
-- (стандарт LLM tool-calling — сначала assistant.tool_use, потом tool.result),
-- и без role-фильтра вставка tool-row упадёт с duplicate key violation.
CREATE UNIQUE INDEX idx_assistant_messages_tool_call
    ON assistant_messages(tool_call_id)
    WHERE tool_call_id IS NOT NULL AND role = 'tool';
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin

-- DROP TABLE автоматически удаляет связанные индексы и FK.
-- assistant_messages удаляем первой — на неё ссылается FK из неё же
-- через session_id, но порядок важен для явности схемы.
DROP TABLE IF EXISTS assistant_messages;
DROP TABLE IF EXISTS assistant_sessions;

-- +goose StatementEnd
