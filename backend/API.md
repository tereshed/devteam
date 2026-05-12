# DevTeam Backend — API Reference

Базовый URL: `http://localhost:8080` (см. `SERVER_HOST`/`SERVER_PORT`).
Все защищённые ручки требуют заголовок `Authorization: Bearer <access_token>`.

Полная Swagger-документация: `http://localhost:8080/swagger/index.html`.
Этот файл — краткая выжимка ключевых endpoint'ов; для исчерпывающей правды
читайте Swagger (он генерируется из аннотаций `make swagger`).

---

## Соглашения

* Время — ISO 8601 UTC: `2026-05-12T17:42:08.377007Z`.
* UUID — строка вида `550e8400-e29b-41d4-a716-446655440000`.
* Тело запросов и ответов — JSON (`Content-Type: application/json`).
* Стандартный формат ошибок:
  ```json
  { "error": "external_service_error", "message": "human-readable detail" }
  ```

---

## Health

### `GET /health`

Без авторизации. Возвращает `200` если сервер жив и подключён к БД.

```json
{ "status": "healthy", "timestamp": 1778614201 }
```

---

## Аутентификация

### `POST /api/v1/auth/register`

```json
{ "email": "user@example.com", "password": "Password123!" }
```

→ `201`
```json
{
  "access_token": "...", "refresh_token": "...",
  "token_type": "Bearer", "expires_in": 900
}
```

`409` — email уже занят. `400` — невалидное тело.

### `POST /api/v1/auth/login`

```json
{ "email": "user@example.com", "password": "Password123!" }
```

→ `200` тот же формат, что у `/register`. `401` — неверные креды.

### `POST /api/v1/auth/refresh`

```json
{ "refresh_token": "..." }
```

→ `200` новые токены. `401` — refresh невалидный/отозван.

### `POST /api/v1/auth/logout`

`Authorization: Bearer <access>` → `200`. Отзывает все refresh у пользователя.

### `GET /api/v1/auth/me`

→ `200`
```json
{ "id": "uuid", "email": "...", "role": "user", "email_verified": false }
```

---

## Projects (Sprint 2)

### `POST /api/v1/projects`

Создаёт проект и **автоматически команду** (`teams`).

```json
{
  "name": "kt-test",
  "description": "...",
  "git_provider": "github",
  "git_url": "https://github.com/owner/repo",
  "git_credential_id": "uuid-or-null",
  "git_default_branch": "main"
}
```

`git_provider`: `github` / `gitlab` / `bitbucket` / `local`.
Для `local` `git_url` опционален и `git_credential_id` запрещён.

→ `201` — модель проекта (id, name, git_provider, status, …).
`502 external_service_error` — `git_provider` не смог валидировать доступ.

### `GET /api/v1/projects?limit=&offset=`

Список проектов авторизованного пользователя (admin видит все).

### `GET /api/v1/projects/{id}`

### `PUT /api/v1/projects/{id}`

Частичное обновление. Поля `clear_*` / `remove_git_credential` — флаги сброса.

### `DELETE /api/v1/projects/{id}`

### `POST /api/v1/projects/{id}/reindex`

Перезапуск векторной индексации проекта (Sprint 9).

---

## Team (Sprint 2)

### `GET /api/v1/projects/{id}/team`

Команда + 5 агентов проекта (orchestrator/planner/developer/reviewer/tester).

### `PUT /api/v1/projects/{id}/team`

Обновление команды (имя).

### `PATCH /api/v1/projects/{id}/team/agents/{agentId}`

Частичное обновление агента: model, prompt, code_backend, is_active, tool_bindings.

---

## Tasks (Sprint 3)

### `POST /api/v1/projects/{id}/tasks`

Создаёт задачу и **сразу запускает `OrchestratorService.ProcessTask` в фоне**
(см. `task_handler.go:144`).

```json
{
  "title": "Add HELLO file",
  "description": "Create HELLO.md ...",
  "assigned_agent_id": "uuid",
  "branch_name": "feature/hello",
  "priority": "medium",
  "context": "{}"
}
```

`assigned_agent_id` — обычно orchestrator-агент проекта.
`branch_name` — обязательно для агентов с `code_backend != null` (sandbox),
иначе fail на этапе валидации `ExecutionInput`.

### `GET /api/v1/projects/{id}/tasks?status=&priority=&assigned_agent_id=&order_by=&limit=&offset=`

Список задач проекта с фильтрами.

### `GET /api/v1/tasks/{id}`

### `PUT /api/v1/tasks/{id}`

### `DELETE /api/v1/tasks/{id}`

### `POST /api/v1/tasks/{id}/pause`

State-machine `* → paused`. Запущенный sandbox получает graceful-stop по таймауту.

### `POST /api/v1/tasks/{id}/cancel`

`* → cancelled`. Контейнер sandbox принудительно убивается.

### `POST /api/v1/tasks/{id}/resume`

`paused → pending` + новый запуск `ProcessTask`.

### `POST /api/v1/tasks/{id}/correct`

```json
{ "text": "user correction message" }
```

Прерывает текущий шаг агента и перезапускает с новым контекстом.
`review`/`testing` → `in_progress`; иначе только обновляет контекст.

### `GET /api/v1/tasks/{id}/messages?limit=&offset=`

История сообщений задачи (orchestrator/planner/developer/reviewer/tester +
пользовательские корректировки).

### `POST /api/v1/tasks/{id}/messages`

Добавить пользовательское сообщение (другой вариант для `correct`).

---

## Conversations (Sprint 8, чат с оркестратором)

### `POST /api/v1/projects/{id}/conversations`

### `GET /api/v1/projects/{id}/conversations`

### `GET /api/v1/conversations/{id}`

### `POST /api/v1/conversations/{id}/messages`

Триггерит создание задачи и запуск pipeline.

### `DELETE /api/v1/conversations/{id}`

---

## Prompts (admin)

`GET/POST/PUT/DELETE /api/v1/prompts[/{id}]` — управление промптами агентов.

---

## API Keys

`GET/POST/DELETE /api/v1/api-keys[/{id}]` — ключи доступа для интеграций (Sprint 13).

---

## Global LLM credentials (Sprint 13.5)

`PATCH /api/v1/user/llm-credentials` — установить ключи провайдеров на уровне пользователя
(шифруются `ENCRYPTION_KEY` AES-256-GCM).

---

## WebSocket (Sprint 7)

### `GET /api/v1/projects/{id}/ws`

Upgrade → WebSocket. Аутентификация — query-параметр `?token=<access>` или
заголовок `Authorization: Bearer <access>` (см. `ws.Hub.Connect`).

События в канал (one-of `type`):

| `type` | Когда | Payload |
|---|---|---|
| `task_status` | Task.Transition | task id, from, to, agent |
| `task_message` | TaskMessage create | task id, sender_type, content |
| `agent_log` | Sandbox StreamLogs | task id, line, stderr |
| `conversation_message` | ConversationMessageCreated | conversation id, content |
| `error` | Любая ошибка вне основного потока | message |

`WS_*` в env управляет PING/PONG и лимитом коннектов.

---

## MCP-сервер (опционально)

Если `MCP_ENABLED=true`, поднимается отдельный MCP-сервер на `MCP_PORT` (default 8081),
доступный для Cursor / Claude Desktop / VS Code. Реализует инструменты для управления
проектами/задачами/командами/промптами (см. `backend/internal/mcp/tools_*.go`).

Endpoint: `http://localhost:<MCP_PORT>/mcp`.

---

## Sprint 14 — тестовые сценарии

Покрытие end-to-end делится на три слоя; запускайте при включённом `docker compose up`:

| Слой | Команда | Тест |
|---|---|---|
| Backend integration (моки executor'ов) | `make test-integration` | `backend/internal/service/orchestrator_integration_test.go` — 7 сценариев pipeline (14.1) |
| Real sandbox (Docker, fake-claude) | `make test-integration` | `backend/internal/sandbox/sandbox_real_test.go` — push в local bare repo, isolation (14.4), cancel (14.5), 5 parallel (14.3) |
| Full-stack smoke (реальный Anthropic + GitHub PAT) | `GITHUB_PAT=ghp_xxx make e2e-smoke` | `scripts/e2e_smoke.sh` — register → создание проекта → ожидание `completed` → проверка PR на GitHub (14.7) |
| Frontend full UI flow | `make frontend-test-integration` | `frontend/integration_test/full_flow_test.dart` (14.2) |
