# План: Встроенный LLM-ассистент в правой панели приложения

## Context

DevTeam — оркестратор AI-агентов для разработки. Сейчас в приложении есть **per-project ChatScreen** (`/projects/:id/chat`), который запускает orchestration v2 (Router → Planner → Developer → Reviewer и т.д.). Это **рабочий чат для конкретного проекта**, где сообщения превращаются в задачи.

Нужно добавить **другую сущность** — **глобального ассистента**, доступного из любого экрана приложения через **правую боковую панель**. Этот агент:
- помогает пользователю **управлять самим приложением** (создать проект, посмотреть статусы, переключиться, поменять настройки);
- **понимает структуру системы** (проекты, задачи, агенты, очереди);
- умеет **сам действовать** через MCP-инструменты (полная автономия, для destructive — подтверждение);
- имеет вкладку **«Текущие задачи»** — live-список всех `task.state=active` по всем проектам пользователя, чтобы видеть «что прямо сейчас крутится».

Это **не замена** ChatScreen'а проекта и не extension `conversations` — это новый use case (assistant scope=user, без project_id), поэтому отдельная сущность `assistant_sessions`. Готово на 80% инфраструктурно: LLM factory, WebSocket Hub, MCP registry, agent registry уже есть.

---

## Архитектура высокого уровня

```
Flutter AppShell (Web/Desktop)
  ├─ [Left] NavigationRail (как сейчас)
  ├─ [Center] Главный контент (Routes)
  └─ [Right] AssistantSidebar (НОВОЕ, collapsible)
        ├─ Tab: Chat  → AssistantChatPanel
        └─ Tab: Tasks → AssistantTasksPanel (live across projects)
                              ▲
                              │ WebSocket events
                              │
Go Backend
  ├─ handler/assistant_handler.go    (REST)
  ├─ service/assistant_service.go    (agent loop)
  ├─ repository/assistant_session_repository.go
  ├─ mcp/tools_assistant.go          (new MCP-инструменты ассистента)
  └─ ws/hub.go                       (SendToUser — уже есть)
        ▲
        │
LLM (через существующий llm/factory.go)
Agent registry (role='assistant' в БД)
MCP tool catalog (reuse существующих project/task/conversation/agent-инструментов)
```

Тула-петля исполняется **на бэкенде**: backend → LLM → tool_call → MCP execute → LLM → ... → финальный assistant message. Фронт только рендерит события из WebSocket.

---

## Backend (Go)

### 1. Миграция БД — Goose

`backend/db/migrations/0XX_create_assistant_sessions.sql`

```sql
-- assistant_sessions: глобальный ассистент пользователя (без project_id)
CREATE TABLE assistant_sessions (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title VARCHAR(255),
  status VARCHAR(32) NOT NULL DEFAULT 'active', -- active|archived
  busy BOOLEAN NOT NULL DEFAULT FALSE,           -- сериализация agent loop (см. §3.1)
  busy_since TIMESTAMPTZ,                        -- для stale-detection и таймаутов
  pending_tool_call_id VARCHAR(64),              -- хвост при ожидании confirm
  metadata JSONB,
  last_message_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_assistant_sessions_user ON assistant_sessions(user_id, last_message_at DESC);

-- assistant_messages: сообщения ассистент-сессии
CREATE TABLE assistant_messages (
  id UUID PRIMARY KEY,
  session_id UUID NOT NULL REFERENCES assistant_sessions(id) ON DELETE CASCADE,
  role VARCHAR(16) NOT NULL,        -- user|assistant|tool|system
  content TEXT,                     -- final text (для user/assistant)
  tool_call_id VARCHAR(64),         -- для role=tool
  tool_name VARCHAR(128),
  tool_arguments JSONB,             -- payload tool_call (assistant) или request (tool)
  tool_result JSONB,                -- результат MCP-вызова (tool)
  client_message_id VARCHAR(64),    -- идемпотентность user-сообщений
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_assistant_messages_client_id
  ON assistant_messages(session_id, client_message_id)
  WHERE client_message_id IS NOT NULL;
-- Пагинация истории всегда идёт ORDER BY created_at DESC; в YugabyteDB
-- явный DESC в индексе избавляет от backward scan по распределённым tablet'ам.
CREATE INDEX idx_assistant_messages_session
  ON assistant_messages(session_id, created_at DESC);

-- Атомарность confirm: один tool_call_id может быть «закрыт» только один раз.
-- Используется в UPDATE ... WHERE tool_call_id=? AND tool_result IS NULL (см. §4.1).
CREATE UNIQUE INDEX idx_assistant_messages_tool_call
  ON assistant_messages(tool_call_id)
  WHERE tool_call_id IS NOT NULL;
```

Команда: `make migrate-create name=create_assistant_sessions`, потом руками вписать SQL.

### 2. Model + Repository

- `backend/internal/models/assistant_session.go` — GORM-модели `AssistantSession`, `AssistantMessage`.
- `backend/internal/repository/assistant_session_repository.go` — интерфейс + impl:
  - `CreateSession`, `GetSession`, `ListSessionsByUser`, `UpdateSessionTitle`, `ArchiveSession`
  - `AppendMessage`, `ListMessages(sessionID, limit, beforeCreatedAt, beforeID)` — **курсорная** пагинация (см. §4.1, п. «нестабильный порядок»)
  - `FindMessageByClientID(sessionID, clientID)` — идемпотентность
  - `AcquireBusy(sessionID, userID)` / `ReleaseBusy(sessionID)` — CAS-захват из §3.1
  - `ConfirmAndClosePending(sessionID, userID, toolCallID, resultJSON []byte) error` — атомарный confirm-UPDATE из §4.1 (вся SQL-механика только здесь, service её не видит)

Параллель: `repository/conversation_repository.go` (`backend/internal/repository/`).

**Контракт сортировки**: `ListMessages` всегда `ORDER BY created_at DESC, id DESC`. Курсор — пара `(before_created_at, before_id)`, фильтр через row-comparison `WHERE (created_at, id) < (?, ?)`. Без вторичного ключа порядок нестабилен при пачке сообщений из одной транзакции (pgx/Yugabyte ставят им идентичный `created_at`). Индекс `idx_assistant_messages_session` уже под это заточен (см. §1).

**Слоевой инвариант** (`docs/rules/backend.md` §2.1): SQL/GORM-транзакции/`tx.Raw`/`tx.Exec` живут **только** в repository. Service оперирует методами repo, не имеет поля `db *gorm.DB`, не открывает транзакции. Любой `tx.Raw` в service — блокер на ревью.

### 3. Service — agent loop

`backend/internal/service/assistant_service.go` — **сердце фичи**. Сам сервис тонкий: владеет сессией, репозиторием, авторизацией и делегирует исполнение tool-loop в общий движок (см. §3.2).

Интерфейс:
```go
type AssistantService interface {
    CreateSession(ctx, userID) (*Session, error)
    ListSessions(ctx, userID) ([]Session, error)
    GetHistory(ctx, sessionID, userID, limit, beforeID) ([]Message, error)
    SendMessage(ctx, sessionID, userID, content, clientMsgID) error // 202 Accepted; loop в горутине
    ConfirmToolCall(ctx, sessionID, userID, toolCallID, approved bool) error // resume после confirm
    ListActiveTasks(ctx, userID) ([]TaskSummary, error) // для Tasks-tab
}
```

#### 3.1. Сериализация сессии (race conditions)

**Инвариант: в любой момент времени по одной `assistant_sessions.id` идёт ≤ 1 активная агент-петля.** Это закрывает дабл-клик/гонку двух конкурирующих горутин (interleaved messages в истории, два параллельных LLM-запроса, неконсистентный pending_tool_call_id).

Реализация — **busy-флаг с атомарным CAS** в одной транзакции с проверкой авторизации (паттерн как в `orchestrator_v2` lock):

```go
// Захват: SendMessage / ConfirmToolCall перед стартом горутины.
res := tx.Exec(`
    UPDATE assistant_sessions
       SET busy = TRUE, busy_since = NOW(), updated_at = NOW()
     WHERE id = ? AND user_id = ? AND status = 'active' AND busy = FALSE`,
    sessionID, userID)
if res.RowsAffected == 0 {
    // Либо сессия чужая/архивированная — 404,
    // либо уже занята другой горутиной — 409 Conflict.
    return ErrSessionBusy // → handler возвращает HTTP 409
}
```

Горутина в `defer` обязана **всегда** снимать флаг (`busy = FALSE`, `busy_since = NULL`), включая panic-recover и пути выхода через destructive-confirm (где петля «паркуется» — см. ниже). Альтернативно при destructive-confirm флаг **остаётся `TRUE`** до прихода `ConfirmToolCall`, чтобы между confirm-запросом и резюмом не вклинилось новое `SendMessage` — это и есть желаемое поведение.

**Stale-recovery:** фоновая cron-задача (1× в минуту) сбрасывает busy у сессий, где `busy_since < NOW() - INTERVAL '10 minutes'` и нет `pending_tool_call_id` (то есть процесс упал во время LLM-вызова, а не на confirm). Это аналог восстановления locked задач из `orchestrator_v2`.

**Защита от split-brain (зависшая, но не упавшая горутина).** Сценарий: горутина не упала, а провисла на HTTP к Anthropic/OpenAI 15 минут. Cron сбросит busy через 10 мин, юзер шлёт новое сообщение, запускается вторая горутина, потом первая «отвисает» — обе пишут в БД, сериализация нарушена.

Защита **обязательна на двух уровнях**:

1. **Hard timeout на весь цикл.** Горутина `runAgentLoop` всегда стартует с `ctx, cancel := context.WithTimeout(parent, 5*time.Minute)` и передаёт этот `ctx` дальше в `agentloop.Executor.Run`. Через `defer cancel()`.
2. **Per-call timeout у LLM-клиента.** Каждый `LLMClient.Generate*` оборачивается дополнительно в `context.WithTimeout(ctx, 90*time.Second)`. У провайдеров в `llm/factory.go` HTTP-клиент уже имеет dial/read deadlines — но network stack без application-level ctx не гарантирует прерывание после ответа TCP (slow stream → infinite read).

Инвариант: **`loop_timeout (5 мин) < stale_threshold (10 мин)`** с запасом ≥ 2×. Это значение `const` в `internal/service/assistant_service.go`, а не magic number в трёх местах. Stale-recovery в cron'е читает то же значение через DI/конфиг — если меняем timeout, обновляется и порог.

При превышении timeout'а:
- Executor возвращает `Status: Failed, Cause: ctx.Err()`.
- Defer снимает busy → сессия снова доступна.
- В историю пишется `assistant.message` "запрос к модели не завершился вовремя, попробуйте ещё раз" (без сырых деталей в UI).
- В лог — структурированная запись с `session_id` (без content).

Дополнительно: `agentloop.Executor` на каждой итерации цикла проверяет `ctx.Err()` **до** очередного LLM-вызова — гарантия, что cancellation не «пролетает» между шагами.

**Очередь?** Не вводим. UX-контракт: один в моменте. Фронт дизейблит input, пока сессия busy (статус приходит через `assistant.session_updated`).

HTTP-семантика:
- `POST /messages` при `busy=TRUE` → **409 Conflict** + body `{error: "session_busy", pending_tool_call_id: "..."|null}`.
- `POST /confirm` при `busy=TRUE` и совпадающем `pending_tool_call_id` → принимается (это **единственный** способ продвинуть busy-сессию).
- `POST /confirm` при `busy=FALSE` или mismatch tool_call_id → **409**.

#### 3.2. Общий движок tool-loop (DRY с Router)

Цикл «собрать историю → вызвать LLM с tools → обработать tool_use / final_text → итерация» **не дублируется** между Router и Assistant (см. `docs/rules/review.md` п.2 «копипаст методов запрещён»). Вынесем в:

```
backend/internal/llm/agentloop/
  executor.go        # Executor.Run(ctx, RunRequest) (Result, error)
  types.go           # RunRequest, Result, Tool, ToolHandler, Hooks
  history.go         # сборка messages в нативный формат провайдера
```

Контракт:
```go
type RunRequest struct {
    Client          LLMClient            // anthropic/openai-compat
    SystemPrompt    string
    History         []Message            // полная истории сессии
    Tools           []Tool               // descriptor + handler + requiresConfirmation
    AuthContext     AuthContext          // см. §3.3 (userID, scope)
    MaxIterations   int
    Hooks           Hooks                // OnAssistantMessage, OnToolCall, OnToolResult, OnConfirmRequired, OnError
}

type Hooks struct {
    // Возврат ContinueOrPark из OnConfirmRequired позволяет вызывающему
    // (Assistant) сохранить pending_tool_call_id и завершить горутину
    // до прихода ConfirmToolCall. Router его не использует — для него все
    // инструменты non-destructive, OnConfirmRequired = nil.
    OnAssistantMessage func(ctx, msg AssistantMsg) error
    OnToolCall         func(ctx, call ToolCall) error
    OnToolResult       func(ctx, res ToolResult) error
    OnConfirmRequired  func(ctx, call ToolCall) (ConfirmDecision, error) // {Park}|{Approve}|{Deny}
    OnFinalText        func(ctx, text string) error
}

type Result struct {
    Status      Status  // Completed | Parked | LimitExceeded | Failed
    Iterations  int
    ParkedCall  *ToolCall
}
```

- **Router** использует Executor с пустым `Tools` (он сам — спец-LLM, который возвращает JSON), либо переходит на native tool-calling — это отдельный refactor, отражён в плане как follow-up (см. «Migrating Router to shared loop» в Verification → next steps), без блокировки текущей задачи.
- **Assistant** передаёт реальные MCP-tools и хуки, которые пишут события через `ws.Hub`, сохраняют сообщения через `AssistantRepo`, и в `OnConfirmRequired` возвращают `Park` (записывая `pending_tool_call_id` в сессию и завершая петлю).
- Возобновление после confirm: `ConfirmToolCall` строит свой `RunRequest`, передаёт уже-известный `ToolResult` (или synthetic `denied`) первой строкой истории, продолжает с новой итерации.

Лимит итераций: `MaxIterations = 12`. Превышение → `Status: LimitExceeded`, Assistant пишет `assistant.message` "превышен лимит шагов".

#### 3.3. Authorization (AuthZ) — обязательный контракт

Глобальный ассистент исполняется от имени `userID`, но MCP-инструменты исторически вызывались внутри уже-авторизованного оркестратора. Поэтому:

1. **Все вызовы MCP-инструментов из Assistant идут через wrapper `mcp.AuthorizedExecutor(userID).Execute(ctx, name, args)`**, который:
   - кладёт `userID` в `ctx` через типизированный ключ `authctx.UserIDKey`;
   - до вызова handler'а проверяет белый список инструментов, доступных глобальному ассистенту (см. §5);
   - после вызова валидирует, что возвращённые ресурсы фильтрованы по `user_id` (sentinel-проверка в dev/test режимах).

2. **Каждый MCP-инструмент**, попадающий в каталог ассистента, обязан:
   - **Read (list/get)** — фильтровать выдачу `WHERE user_id = ctx.UserID()` (или через team membership, где применимо). Если инструмент исторически не фильтрует, добавляем фильтр в этом же спринте, иначе он не попадает в каталог.
   - **Mutation (create/update/delete/cancel)** — внутри handler'а валидировать ownership через repo: `repo.GetByID(id)` → проверить `userID == ctx.UserID()` → `ErrForbidden` иначе. Никакого «trust the caller».
   - Возвращать `403`/`ErrForbidden` единым форматом, чтобы движок мог записать tool_result `{status:"forbidden"}` и передать LLM на следующей итерации.

3. **Аудит вызовов**: каждое исполнение MCP-инструмента из Assistant пишется в `assistant_messages` (role=tool, tool_arguments, tool_result со `status`). Это даёт полную traceability «что агент сделал от моего имени».

4. **Чек-лист на каждый инструмент в §5** (см. ниже) — отдельная подзадача: пройтись по `project_list/get/...`, `task_*`, `conversation_*`, `agent_*` и явно зафиксировать, чем именно фильтруется доступ. То, что не проходит, либо чинится, либо исключается из каталога.

#### 3.4. Защита от переполнения контекста (tool_result truncation)

MCP-инструменты вроде `task_list`, `project_list`, `artifact_get` могут вернуть JSON на сотни КБ или мегабайты. Если бездумно сложить это в `history` для следующего LLM-вызова — упрёмся в `context_length_exceeded` (фатальная ошибка провайдера) либо сожжём токены/деньги/латентность. Защита **обязательна на двух уровнях**:

1. **На стороне `AuthorizedExecutor` (выход из MCP-вызова, см. §3.3).** Размер сериализованного `tool_result` ограничивается `MaxToolResultBytes = 16 KiB` (значение в конфиге). При превышении:
   - Сохраняем **полный** результат в `assistant_messages.tool_result` (для traceability и UI — фронт может развернуть карточку и увидеть всё).
   - В LLM-историю отдаём **усечённую версию** с маркером:
     ```json
     {
       "status": "ok",
       "truncated": true,
       "preview": "<первые 12 KiB>",
       "hint": "результат урезан; используй пагинацию (limit/offset) или фильтры"
     }
     ```
   - Это значит, что в `history.go` (§3.2) сборка истории идёт **не из БД as-is**, а через хелпер `truncateToolResultForHistory(rawJSON []byte, max int)` — он же отвечает за маркировку и подсказку модели.

2. **Принудительная пагинация в самих инструментах** (там, где это имеет смысл — `*_list`):
   - У каждого list-tool обязателен параметр `limit` (default 20, max 100) и `cursor`/`offset`.
   - Возвращается `{items, next_cursor, total_estimate}` — у LLM есть механизм добрать следующую страницу через повторный вызов.
   - Это закрывает кейс «дай мне все задачи 200 проектов одним вызовом».

3. **Лимит на суммарный context window** в `agentloop.Executor`:
   - Перед каждым LLM-вызовом считаем оценочный размер истории (`approxTokens = bytes/4`).
   - Если `approxTokens > 0.8 * modelContextWindow` — сжимаем **самые старые** tool_result-сообщения до коротких summary (`tool_name + status + size`), сохраняя последние N полностью.
   - Sliding-window стратегия: систем-промпт + последние K user/assistant сообщений всегда полные; промежуточные tool_call/tool_result агрегируются.

Лимиты (`MaxToolResultBytes`, `MaxHistoryTokens`, `HistoryTailKeep`) хранятся как поля `agentloop.Config`, передаются через DI. Magic numbers в коде запрещены.

**Прочая безопасность (по правилам из docs/rules):**
- Никогда не логировать сырой LLM-output / содержимое сообщений — `logging.NewHandler` с redaction.
- Секреты провайдера читаем через `SecretsResolver`, как `llm/factory.go:61-100`.
- DI: AssistantService получает `LLMFactory`, `agentloop.Executor`, `AuthorizedMCPExecutor`, `Hub`, `AssistantRepo`, `AgentRepo`, `TaskRepo`, `Logger` через конструктор. Никаких globals, никакого `slog.Default()`.

### 4. HTTP Handler

`backend/internal/handler/assistant_handler.go`:

```
POST   /api/v1/assistant/sessions                       создать сессию
GET    /api/v1/assistant/sessions                       список (user)
GET    /api/v1/assistant/sessions/:id                   детали
DELETE /api/v1/assistant/sessions/:id                   archive
GET    /api/v1/assistant/sessions/:id/messages          история (limit, before_id)
POST   /api/v1/assistant/sessions/:id/messages         отправить сообщение {content, client_message_id}
POST   /api/v1/assistant/sessions/:id/confirm          {tool_call_id, approved, client_request_id}
GET    /api/v1/assistant/active-tasks                  in-progress задачи всех проектов user (для Tasks-tab)
```

Аутентификация — через существующий middleware (как у `conversation_handler.go:26-70`). Подключение в `backend/cmd/server/main.go` (group `/api/v1`).

Swagger-аннотации обязательны → потом `make swagger`.

#### 4.1. Идемпотентность и edge-cases

| Сценарий | Поведение |
|---|---|
| `POST /messages` с тем же `client_message_id` дважды (network retry / дабл-клик) | Уникальный индекс `idx_assistant_messages_client_id` → INSERT падает → handler определяет дубликат и возвращает `202 Accepted` с тем же `session_id` (no-op). |
| `POST /messages`, пока сессия busy | `409 Conflict` `{error:"session_busy", pending_tool_call_id?}`. Фронт дизейблит input до `assistant.session_updated busy=false`. |
| `POST /confirm` дважды подряд (дабл-клик по Approve/Deny) | Атомарный UPDATE: `UPDATE assistant_messages SET tool_result=?, updated_at=NOW() WHERE tool_call_id=? AND tool_result IS NULL`. Если `RowsAffected==0` → confirm уже обработан → `409 Conflict` `{error:"already_confirmed"}`. Никакого второго запуска tool'а. |
| `POST /confirm` без активного `pending_tool_call_id` (или mismatch) | `409 Conflict` `{error:"no_pending_confirmation"}`. |
| `POST /confirm` на архивированную/удалённую сессию | `404 Not Found`. |
| Сессия `status='archived'` пока юзер «думал» над confirm-диалогом | `ConfirmToolCall` проверяет `status='active' AND busy=TRUE AND pending_tool_call_id=?` одним SELECT … FOR UPDATE. Mismatch → `409`/`404` без выполнения tool'а. |
| Горутина крашится во время LLM-вызова | Stale-recovery (§3.1) через 10 минут сбросит busy. До этого `POST /messages` → 409, фронт показывает «сессия залипла, ретрай через X». |
| `client_request_id` на confirm | Тот же приём, что и для messages: уникальный индекс по `(tool_call_id, client_request_id)` в payload — но проще держать единственный источник правды через `tool_result IS NULL` атомарный UPDATE (см. выше). `client_request_id` идёт только в лог для трассировки. |

**Раскладка по слоям.** Service координирует и принимает бизнес-решения; всю работу с БД (транзакции, `tx.Raw`, `tx.Exec`, локи, проверки `RowsAffected`) делает repository. Service `assistantService` **не имеет** поля `db *gorm.DB`.

Service (`internal/service/assistant_service.go`):
```go
func (s *assistantService) ConfirmToolCall(
    ctx context.Context, sessionID, userID, toolCallID string, approved bool,
) error {
    // 1) Fast-fail на пустых аргументах — не идём в БД, не блокируем строку FOR UPDATE
    //    зря (docs/rules/review.md §3 "что будет, если на вход придет пустая строка?").
    if sessionID == "" || userID == "" || toolCallID == "" {
        return ErrInvalidInput
    }

    // 2) Сборка tool_result — бизнес-решение (approve → исполняем MCP, deny → synthetic
    //    payload). Сериализация в JSONB происходит ЗДЕСЬ, чтобы repo принимал готовый
    //    []byte. Передавать map[string]any в tx.Exec("UPDATE ... SET tool_result=?", m)
    //    нельзя — database/sql драйвер не сериализует Go-структуры в jsonb автоматически
    //    (упадёт с unsupported type). Магия GORM работает только в `.Updates(&Model{...})`.
    resultJSON, err := s.buildResultJSON(ctx, userID, toolCallID, approved)
    if err != nil {
        return err
    }

    // 3) Repo атомарно: проверяет session.owner+busy+pending, закрывает tool-row,
    //    обнуляет pending_tool_call_id. busy=TRUE остаётся — runAgentLoopResume снимет в defer.
    if err := s.repo.ConfirmAndClosePending(ctx, sessionID, userID, toolCallID, resultJSON); err != nil {
        return err // ErrNoPendingConfirmation / ErrAlreadyConfirmed / ErrSessionNotFound
    }

    // 4) Горутина — СТРОГО после успешного возврата из repo (то есть после COMMIT).
    //    Запуск `go ...` внутри Transaction(func(tx){...}) — антипаттерн: горутина
    //    могла прочитать БД до COMMIT и увидеть старое pending_tool_call_id.
    //    context.Background() — намеренно: HTTP-ctx не должен отменять long-running
    //    агент-петлю; свой timeout живёт внутри runAgentLoopResume (см. §3.1).
    go s.runAgentLoopResume(context.Background(), sessionID, userID)
    return nil
}

// buildResultJSON — для approved=true вызывает AuthorizedExecutor (MCP) и
// маршалит результат; для approved=false возвращает json.Marshal(deny payload).
// Возврат — всегда []byte (готов к INSERT в jsonb-колонку).
```

Repository (`internal/repository/assistant_session_repository.go`):
```go
func (r *assistantSessionRepo) ConfirmAndClosePending(
    ctx context.Context, sessionID, userID, toolCallID string, resultJSON []byte,
) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // GORM Raw().Scan() НЕ возвращает ErrRecordNotFound на пустой результат —
        // это nil error + RowsAffected==0. Проверяем оба поля явно, иначе нулевая
        // sess «пройдёт» дальше как валидная.
        var sess AssistantSession
        res := tx.Raw(`
            SELECT * FROM assistant_sessions
             WHERE id = ? AND user_id = ? AND status = 'active'
               AND busy = TRUE AND pending_tool_call_id = ?
             FOR UPDATE`, sessionID, userID, toolCallID).Scan(&sess)
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNoPendingConfirmation
        }

        // Передаём ГОТОВЫЙ JSONB-байт-слайс. ::jsonb-каст явный, чтобы планировщик
        // не ругался на text→jsonb implicit cast.
        upd := tx.Exec(`
            UPDATE assistant_messages
               SET tool_result = ?::jsonb, updated_at = NOW()
             WHERE tool_call_id = ? AND role = 'tool' AND tool_result IS NULL`,
            resultJSON, toolCallID)
        if upd.Error != nil {
            return upd.Error
        }
        if upd.RowsAffected == 0 {
            // Параллельный confirm уже закрыл tool-row → возвращаем 409 наверх.
            return ErrAlreadyConfirmed
        }

        return tx.Exec(`
            UPDATE assistant_sessions
               SET pending_tool_call_id = NULL, updated_at = NOW()
             WHERE id = ?`, sessionID).Error
    })
}
```

Те же правила (`Error != nil` → возврат; `RowsAffected == 0` → доменная ошибка) применяются ко всем `Raw().Scan()` / `Exec()` в репозитории — это инвариант проекта, его стоит закрыть линтером в follow-up'е.

### 5. MCP-инструменты для ассистента

**Re-use существующих** (из `backend/internal/mcp/`):
- Project: `project_list`, `project_get`, `project_create`, `project_update`, `project_delete` *(destructive)*
- Task: `task_list`, `task_get`, `task_cancel_v2` *(destructive)*, `artifact_list`, `artifact_get`, `router_decision_list`
- Conversation: `conversation_list`, `conversation_get`, `conversation_create`, `conversation_send_message`, `conversation_history`
- Agent: `agent_list`, `agent_get`, `agent_create`, `agent_update`, `agent_set_secret`, `agent_delete_secret` *(destructive)*

**Новые** в `backend/internal/mcp/tools_assistant.go`:
- `app_navigate(route)` — публикует WS-событие `assistant.navigate` для go_router фронта. Tool возвращает `{status:"sent"}`.
- `assistant_active_tasks_count()` — short query, чтобы LLM мог быстро ответить «сколько задач в работе».
- (опц.) `whoami()` — id/email текущего юзера, проекты, ёмкость планов.

Регистрация — в `backend/internal/mcp/server.go:49-80`.

Каждому tool помечаем флаг `requiresConfirmation bool` в дескрипторе. Список destructive ведётся в `service/assistant_service.go` (короткий switch по имени).

#### 5.1. AuthZ-чек-лист (обязательное подзадание перед включением в каталог)

Реализация связана с §3.3. Для **каждого** инструмента, выставляемого глобальному ассистенту, явно фиксируется в коде (`AuthorizedExecutor.Tools()` декларация) и в этом плане:

| Tool | Тип | Фильтр read / проверка mutation | Confirm? |
|---|---|---|---|
| `project_list` | read | WHERE projects.user_id = ctx.UserID() (или через team_members) | — |
| `project_get` | read | repo.GetByID → assert owner; иначе ErrForbidden | — |
| `project_create` | mutation | user_id = ctx.UserID() при INSERT; никаких free-form user_id из args | yes |
| `project_update` | mutation | repo.GetByID → assert owner | yes |
| `project_delete` | mutation | repo.GetByID → assert owner | **yes (destructive)** |
| `task_list` | read | JOIN projects WHERE projects.user_id = ctx.UserID() | — |
| `task_get` | read | то же + assert | — |
| `task_cancel_v2` | mutation | то же + assert | **yes (destructive)** |
| `artifact_list/get`, `router_decision_list`, `worktree_list` | read | JOIN projects по ownership | — |
| `conversation_list/get/history` | read | filter by user_id (или project ownership) | — |
| `conversation_create` | mutation | user_id = ctx.UserID() | yes |
| `conversation_send_message` | mutation | assert session.user_id == ctx.UserID() **и project ownership** | yes |
| `agent_list/get` | read | глобальный реестр (read доступен всем) — допустимо для assistant | — |
| ~~`agent_create/update/set_secret`~~ | mutation | **исключены из каталога** (см. ниже) | — |
| ~~`agent_delete*`~~ | mutation | **исключены из каталога** (см. ниже) | — |
| `app_navigate` | side-effect (WS to user) | tightly scoped: только route string, без id чужих ресурсов | — |
| `whoami` / `assistant_active_tasks_count` | read | по `ctx.UserID()` | — |

**Решение по AuthZ-аудиту (стадия 5):** `models.Agent` — это глобальный реестр шаблонов (`team_id` опционален, прямого `user_id` нет), а `AgentService.Create/Update/Delete/SetSecret/DeleteSecret` не гейтят мутации ни по user, ни по role. Executor же hard-coded подаёт `RoleUser` (защита от admin-обхода). Поэтому agent-мутации не могут пройти AuthZ-чек-лист «как есть» — они **исключены из каталога ассистента**. Управление шаблонами агентов и их секретами выполняется через существующие REST/UI (admin scope). Read-инструменты (`agent_list`, `agent_get`) остаются — глобальный реестр интенциально доступен для чтения. Восстановление полного CRUD требует отдельной задачи по введению per-agent ownership в `AgentService`.

### 6. Bootstrap agent registry

В `backend/internal/seed/` (или новая `seed_assistant.go`) seed-функция: при старте, если в `agents` нет записи `role='assistant'` — INSERT с дефолтным system prompt:

> «Ты — ассистент платформы DevTeam. Помогаешь пользователю управлять проектами, задачами и агентами. Прежде чем менять состояние — кратко объясни намерение. Используй инструменты для чтения и действий. Отвечай по-русски, кратко, без воды.»

Prompt можно потом править через существующий UI редактирования агентов.

### 7. WebSocket-события

Hub (`backend/internal/ws/hub.go:10-42`) уже умеет `SendToUser`. Используем:

```
assistant.session_updated   { session_id, title, last_message_at }
assistant.message           { session_id, message_id, role, content, created_at }
assistant.tool_call         { session_id, message_id, tool_name, arguments }
assistant.tool_result       { session_id, message_id, tool_name, result, status }
assistant.confirm_request   { session_id, tool_call_id, tool_name, arguments, summary }
assistant.navigate          { route }
assistant.task_update       { project_id, task_id, state, title, updated_at } // для Tasks-tab
```

`assistant.task_update` — переиспользуем существующий task-event broadcast (его уже эмитит TaskService при смене state). Просто **расширяем фильтр** на стороне фронта: ChatController смотрит project_id, AssistantTasksController подписан на все user-events. На бэке если SendToUser для task-events не идёт — добавить fan-out в `service/task_service.go` (после уже существующего SendToProject).

---

## Frontend (Flutter)

### 1. Новая фича `lib/features/assistant/`

Структура (по конвенции feature-first):
```
features/assistant/
  data/
    assistant_repository.dart            # Dio, реализует REST
    assistant_providers.dart             # @Riverpod(keepAlive: true)
  domain/
    assistant_session_model.dart         # @freezed abstract class
    assistant_message_model.dart         # роль + content + tool_call поля
    assistant_active_task_model.dart
  presentation/
    controllers/
      assistant_chat_controller.dart     # AsyncValue<ChatState>, посылает messages, слушает WS
      assistant_tasks_controller.dart    # AsyncValue<List<ActiveTask>>, WS task_update
      assistant_sidebar_controller.dart  # collapsed/expanded, current tab
    widgets/
      assistant_sidebar.dart             # главный контейнер: header + TabBar + body
      assistant_chat_panel.dart          # лист + input
      assistant_message_bubble.dart      # markdown, как ChatMessage из features/chat/
      assistant_tool_call_card.dart      # «Агент вызвал project_list(...)»
      assistant_confirm_dialog.dart      # подтверждение destructive операции
      assistant_tasks_panel.dart         # список ActiveTask с тапом → go_router push
      assistant_session_picker.dart      # dropdown в header для смены сессии
```

Все модели — `@freezed abstract class`, generated через `make frontend-codegen`.

### 2. Интеграция в `AppShell`

Файл: `frontend/lib/core/widgets/app_shell.dart:26`.

Добавить третью колонку (right rail) — `AssistantSidebar`:
- **Desktop (>1200dp)**: всегда виден, ширина 360–400dp, collapsible через кнопку в AppBar (хранит state в `assistantSidebarControllerProvider`). По умолчанию open.
- **Tablet (600–1200)**: collapsed; раскрывается поверх контента (Drawer endDrawer).
- **Mobile (<600)**: только endDrawer / bottom sheet.

Кнопка-toggle: `IconButton(Icons.assistant)` в AppBar справа.

### 3. Repository (Dio)

`assistant_repository.dart` использует `dioClientProvider` из `lib/core/api/dio_providers.dart`. Методы повторяют REST из п.4 backend. Параллель — `features/chat/data/conversation_repository.dart`.

### 4. WebSocket подписки

`lib/core/api/websocket_service.dart:26` уже keep-alive singleton. В контроллерах:
- `AssistantChatController` слушает события с `type` начинающимся на `assistant.`, фильтрует по `session_id`.
- `AssistantTasksController` слушает `assistant.task_update` для текущего пользователя (без project-фильтра).

### 5. Поведение для destructive операций

Когда приходит `assistant.confirm_request`:
- В чат вставляется `AssistantConfirmDialog` inline-карточкой (Approve / Deny + сводка args).
- При нажатии — `POST /assistant/sessions/:id/confirm` с `approved=true/false`.
- Backend возобновляет loop.

### 6. `app_navigate` event

`AssistantChatController` подписан на `assistant.navigate` → вызывает `GoRouter.of(context).go(route)`. Полезно: «открой проект X» → агент возвращает текст + navigate.

### 7. Локализация

Добавить в `lib/l10n/app_en.arb` и `app_ru.arb`:
- `assistantSidebarTitle`, `assistantTabChat`, `assistantTabTasks`
- `assistantEmptyChat`, `assistantInputHint`, `assistantSend`
- `assistantConfirmTitle`, `assistantConfirmApprove`, `assistantConfirmDeny`
- `assistantNoActiveTasks`, `assistantActiveTaskInProgress`
- `assistantToggleTooltip`
- `assistantSessionBusy`, `assistantSessionStale`, `assistantToolCallTitle`, `assistantToolResultStatusOk`, `assistantToolResultStatusForbidden`

**Способ обращения к локализациям — строго `requireAppLocalizations`** (по `docs/rules/frontend.md` §2.3). `AppLocalizations.of(context)!` и любые расширения `context.l10n` **запрещены** (трудно отлаживать null-фолбэки):

```dart
@override
Widget build(BuildContext context) {
  final l10n = requireAppLocalizations(context, where: 'AssistantSidebar');
  return Text(l10n.assistantSidebarTitle);
}
```

Параметр `where:` обязателен в каждом виджете фичи — указывает имя класса виджета для трассировки в логах при ошибке.

Команда генерации — **одна**: `make frontend-codegen`. Цель уже включает `flutter gen-l10n` последним шагом (см. `docs/rules/deploy.md`), вручную второй раз вызывать не нужно. Порядок внутри make-цели правильный.

### 8. Стили / UX-детали

- Сообщения agent vs user — разные bubble цвета (как в `features/chat/`).
- Tool-call card сворачивается, по умолчанию свернут с одной строкой `🔧 project_list({...})`, раскрытие показывает args + result JSON.
- Tasks panel: каждая карточка → project_name · task_title · phase · since · «Открыть» (go_router к `/projects/:id/tasks/:taskId`).
- Анимация панели (open/close) через `AnimatedSize` + `AnimatedSwitcher`.

---

## Критичные файлы / ссылки

**Backend существующие — переиспользовать:**
- LLM factory: `backend/internal/llm/factory.go:61-100`
- MCP server / register: `backend/internal/mcp/server.go:49-80`
- WS Hub: `backend/internal/ws/hub.go:10-42`
- Conversation pattern: `backend/internal/service/conversation_service.go:32-65`
- Router pattern (LLM tool-loop reference): `backend/internal/service/router_service.go:19-96`
- Task repo для active-tasks: `backend/internal/models/task.go:79-117`
- Logging redaction: `backend/internal/logging/`

**Backend новые:**
- `backend/db/migrations/0XX_create_assistant_sessions.sql`
- `backend/internal/models/assistant_session.go`
- `backend/internal/repository/assistant_session_repository.go`
- `backend/internal/llm/agentloop/{executor,types,history}.go` — общий tool-calling движок (§3.2)
- `backend/internal/mcp/authorized_executor.go` — wrapper с пробросом `userID` в `ctx` и enforce-каталогом (§3.3)
- `backend/internal/service/assistant_service.go`
- `backend/internal/handler/assistant_handler.go`
- `backend/internal/mcp/tools_assistant.go`
- Регистрация: `backend/cmd/server/main.go` (DI wiring + route group + stale-session cron, §3.1)

**Frontend существующие — переиспользовать:**
- AppShell: `frontend/lib/core/widgets/app_shell.dart:26`
- ChatScreen для паттернов: `frontend/lib/features/chat/presentation/screens/chat_screen.dart:117`
- ChatMessage widget (markdown): `frontend/lib/features/chat/presentation/widgets/`
- WebSocketService: `frontend/lib/core/api/websocket_service.dart:26`
- Dio providers: `frontend/lib/core/api/dio_providers.dart`
- Task list patterns: `frontend/lib/features/tasks/presentation/screens/tasks_list_screen.dart:50`

**Frontend новые:**
- Вся feature-папка `frontend/lib/features/assistant/`
- Расширение `frontend/lib/core/widgets/app_shell.dart`
- Локализация: `frontend/lib/l10n/app_en.arb`, `app_ru.arb`

---

## Порядок реализации (рекомендуемые этапы)

1. **Backend skeleton**: миграция (с busy/pending_tool_call_id) → models → repository → handler-заглушки (без agent loop). Сериализация (busy CAS) уже в репозитории. Проверка REST + 409 через curl.
2. **agentloop движок** (`internal/llm/agentloop`): чистый Executor с тестами на mock LLM (final_text / tool_use / confirm-park / limit / **ctx timeout** / **history truncation**). Принимает `Config{LoopTimeout, PerLLMCallTimeout, MaxToolResultBytes, MaxHistoryTokens, HistoryTailKeep}`. Без интеграции в реальные сервисы.
3. **AuthorizedExecutor** для MCP (`internal/mcp/authorized_executor.go`) + AuthZ-чек-лист (§5.1) + **truncation tool_result** (§3.4 п.1): добавить фильтры/assert'ы там, где их нет; **добавить `limit`/`cursor` ко всем list-инструментам, попадающим в каталог** (§3.4 п.2). Что не чинится в этот спринт — исключить из каталога.
4. **AssistantService без MCP**: простой LLM round-trip через agentloop с пустым каталогом. WS-события `assistant.message`. Сериализация busy с defer-снятием. Stale-recovery cron.
5. **Tool-calling**: подключить AuthorizedExecutor к agentloop в AssistantService. Bootstrap agent registry (`role='assistant'`).
6. **Destructive confirm flow**: `assistant.confirm_request` + `POST /confirm` + атомарный UPDATE (§4.1) + `runAgentLoopResume`.
7. **Active tasks endpoint** + WS broadcast `assistant.task_update` (на бэке fan-out из TaskService к user).
8. **Frontend skeleton**: модели + repository + providers. Заглушка панели.
9. **AssistantSidebar в AppShell** (toggle, ширина, breakpoints).
10. **Chat panel**: render message bubbles, tool-call cards, input (с дизейблом по busy), WS subscription, `requireAppLocalizations`.
11. **Tasks panel**: список ActiveTask, WS-обновления, навигация.
12. **Confirm dialog inline-карточка** + `assistant.navigate` handler.
13. **Локализация (`.arb`) + `make frontend-codegen`** (одной командой).
14. **Тесты + polish**.

**Follow-up (не блокирует этот спринт):** мигрировать Router (`router_service.go`) на тот же `internal/llm/agentloop` — задача `22.x-router-on-agentloop.md`. Сейчас Router остаётся как есть; цель — устранить дублирование. План §3.2 спроектирован так, чтобы Executor покрывал и его use case (native tool-calling вместо JSON-парсинга).

---

## Verification

**Backend:**
- `make test-all` — unit-тесты репозитория + service (mocked LLM с заранее заданным tool_use → проверяем сохранение сообщений и WS-эмиссию).
- Integration-тест: `httptest` сервер + реальная БД (testcontainers / docker-compose test), full happy-path `create session → send message → mocked LLM returns project_list tool_call → executed → returns final text`.
- **Concurrency-тесты**: два параллельных `POST /messages` на одну сессию → один 202, второй 409 + busy в БД. Параллельные `POST /confirm` с одним `tool_call_id` → один OK, второй 409 (`already_confirmed`), tool выполнен ровно 1 раз (счётчик в моке).
- **Timeout-тест**: mock LLM-клиент, который спит дольше `LoopTimeout` → ctx отменяется, в БД пишется failure-сообщение, busy снят, новое сообщение проходит. Стейл-рекавери cron не нужен (timeout сам убирает busy).
- **Split-brain regression**: эмулируем «зависшую» горутину (заблокированный mock provider) + ускоренный `staleThreshold` < `loopTimeout` в тесте → проверяем, что cron **не** трогает сессию, пока сама горутина не вышла по timeout'у. Это подтверждает инвариант §3.1.
- **Truncation-тест**: tool возвращает 200 KiB JSON → `assistant_messages.tool_result` содержит полный payload, но в `history.go` идёт truncated preview с `truncated:true`; LLM-вызов получает ≤ `MaxToolResultBytes`.
- **GORM Scan edge-cases**: unit на репозиторий — `ConfirmAndClosePending` при отсутствии pending row возвращает `ErrNoPendingConfirmation` (а не nil + zero-value sess).
- **Service не имеет `*gorm.DB`**: статическая проверка (grep/линтер) что `assistantService` struct и его методы не содержат `db *gorm.DB` / `tx.Raw` / `tx.Exec` / `WithContext(...).Transaction(`. Любая SQL-механика — только в repo (`docs/rules/backend.md` §2.1).
- **Post-commit goroutine**: тест на гонку — мок repo, у которого `ConfirmAndClosePending` спит 50ms перед возвратом, имитируя длинный COMMIT. Параллельно сразу после возврата сервиса проверяем через тот же mock, что `runAgentLoopResume` НЕ стартовал до возврата `ConfirmAndClosePending` (счётчик/канал). Это запрещает регрессию «`go ...` внутри Transaction()».
- **JSONB-сериализация**: unit на `buildResultJSON` — возвращает валидный `[]byte`, который `tx.Exec("... ?::jsonb ...", b)` принимает без ошибки. Negative: попытка передать `map[string]any` напрямую в `tx.Exec` падает (фиксируем как regression-test, чтобы будущий «оптимизатор» не уронил рантайм).
- **Fast-fail валидация**: unit на `ConfirmToolCall("", "", "", true)` и каждую отдельную пустую строку → `ErrInvalidInput`, repo НЕ вызывается (mock с `t.Fatal` в любом методе).
- **Pagination stability**: integration на `ListMessages` — INSERT'им пачку из 5 сообщений одной транзакцией (одинаковый `created_at`), читаем подряд два запроса с тем же курсором → порядок ID идентичен. Курсор `(before_created_at, before_id)` корректно отдаёт следующую страницу без пропусков и дублей при идентичных timestamp'ах.
- `make swagger` — пере-сгенерить аннотации.
- Прогон правил `make rules`, если что-то поменяется в `docs/rules/`.
- Ручная проверка через MCP CLI: `conversation_send_message` для assistant-сессии не нужен, но проверить, что новые `tools_assistant` зарегистрированы.

**Frontend:**
- `make frontend-codegen` (включает `flutter gen-l10n` внутри — двойной вызов не нужен).
- `make frontend-analyze` — без ошибок.
- Widget-тесты: `AssistantSidebar` рендерит обе вкладки; `AssistantConfirmDialog` отдаёт callback; `AssistantToolCallCard` сворачивается.
- Smoke: запустить `docker compose up` (backend) + `flutter run -d chrome` → залогиниться → открыть правую панель → отправить «Покажи мои проекты» → проверить, что агент вызвал `project_list` (карточка tool-call видна) → ответил списком. Затем «Создай проект Foo» → confirm-диалог → approve → проект появился в Dashboard.
- Tasks-tab: запустить любую orchestration v2 задачу в существующем проекте → панель Tasks показывает её live, статус обновляется.
- Negative: попросить «удали проект X» → confirm → deny → агент сообщает «отменено».

**Контракты на тип-чек:**
- Все freezed-модели — `abstract class` (по правилу из CLAUDE.md).
- Все импорты — абсолютные `package:frontend/...`.
- Все строки UI — через `requireAppLocalizations(context, where: 'WidgetName')`. `AppLocalizations.of(context)!` и `context.l10n` запрещены (`docs/rules/frontend.md` §2.3).
- Backend: никаких `slog.Default()` в новых файлах; всё через DI.
- Backend: tool-loop **не дублируется** между Router и Assistant — оба используют `internal/llm/agentloop` (или Assistant первым, Router как follow-up).
- Backend: каждый MCP-инструмент в каталоге ассистента имеет явный AuthZ-фильтр (см. §5.1); инструменты без него в каталог не попадают.
- Backend: `busy`-флаг сессии — единственная точка сериализации; никаких ad-hoc мьютексов в памяти процесса (не переживёт рестарт).
- Backend: инвариант `loop_timeout < stale_threshold / 2` зафиксирован константой; cron stale-recovery читает тот же конфиг (§3.1).
- Backend: каждый `tx.Raw(...).Scan(&x)` проверяет **и** `.Error`, **и** `.RowsAffected == 0` — GORM не отдаёт `ErrRecordNotFound` для `Scan` (§4.1).
- Backend: `tool_result` в LLM-историю всегда идёт через `truncateToolResultForHistory` (§3.4); сырой content разрешён только в БД и в WS-эмиссии на фронт.
- Backend: все list-инструменты MCP в каталоге ассистента обязаны принимать `limit`/`cursor` (§3.4 п.2). Без пагинации — не в каталоге.
- Backend: SQL/`tx.Raw`/`tx.Exec`/`*gorm.DB` живут **только** в repository (`docs/rules/backend.md` §2.1). Service не имеет поля `db`, не открывает транзакции; всё через методы repo (см. §2, §4.1).
- Backend: `go runAgentLoopResume(...)` стартует **после** возврата из repo (после COMMIT). Запуск горутины внутри `Transaction(func(tx){...})` — антипаттерн (читает out-of-tx состояние).
- Backend: JSONB-поля (`tool_result`, `tool_arguments`, `metadata`) передаются в `tx.Exec` только как `[]byte` после `json.Marshal` с явным `?::jsonb` каст в SQL. Прямая передача `map`/`struct` в `tx.Exec` запрещена (database/sql драйвер не сериализует).
- Backend: каждый публичный метод service делает fast-fail валидацию аргументов (`""` / nil / отрицательные значения) до похода в БД (`docs/rules/review.md` §3).
- Backend: `metadata JSONB` в `assistant_sessions` — только UI-настройки и контекст сессии. OAuth-токены, API-ключи, MCP-credentials туда писать запрещено — секреты лежат в `agent_secrets` с AES-256-GCM (Sprint 17).
- Backend: `ListMessages` всегда `ORDER BY created_at DESC, id DESC`, курсорная пагинация — пара `(before_created_at, before_id)` через row-comparison `WHERE (created_at, id) < (?, ?)`. Без вторичного ключа сортировки порядок при идентичных `created_at` нестабилен (см. §1 / §2 контракт).
