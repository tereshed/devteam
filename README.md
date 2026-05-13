# DevTeam — AI Agent Orchestrator

Платформа-оркестратор AI-агентов для автоматизации полного цикла разработки ПО. Пользователь описывает идею в чате — команда AI-агентов реализует: планирует, пишет код, ревьюит, тестирует.

**Стек:** Go (Gin) · Flutter · YugabyteDB · Weaviate · Docker · Claude Code CLI / Aider

---

## Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│                        Flutter UI (Chat)                        │
└──────────────────────────────┬──────────────────────────────────┘
                               │ WebSocket + REST
┌──────────────────────────────▼──────────────────────────────────┐
│                      Go Backend (Gin)                           │
│                                                                 │
│  ┌─────────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │
│  │ Orchestrator│  │ Planner  │  │ Reviewer  │  │   Tester    │  │
│  │   Agent     │  │  Agent   │  │  Agent    │  │   Agent     │  │
│  └──────┬──────┘  └────┬─────┘  └─────┬────┘  └──────┬──────┘  │
│         │              │              │               │         │
│         └──────────────┴──────┬───────┴───────────────┘         │
│                               │                                 │
│                    ┌──────────▼──────────┐                      │
│                    │   Sandbox Runner    │                      │
│                    │  (Docker API)       │                      │
│                    └──────────┬──────────┘                      │
│                               │                                 │
│  ┌──────────┐  ┌──────────┐  │  ┌──────────┐  ┌─────────────┐  │
│  │YugabyteDB│  │ Weaviate │  │  │ Git      │  │ MCP Server  │  │
│  │  (SQL)   │  │ (Vector) │  │  │ Provider │  │ (port 8081) │  │
│  └──────────┘  └──────────┘  │  └──────────┘  └─────────────┘  │
└──────────────────────────────┼──────────────────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │ Sandbox  │    │ Sandbox  │    │ Sandbox  │
        │ Claude   │    │ Aider    │    │ Claude   │
        │ Code CLI │    │          │    │ Code CLI │
        └──────────┘    └──────────┘    └──────────┘
        Изолированные Docker-контейнеры (1 задача = 1 контейнер)
```

---

## Детальный план реализации

### Sprint 1 — Новые модели данных и миграции (Backend)

**Цель:** Создать схему БД для всех новых сущностей.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 1.1 | Миграция: таблицы `git_credentials` + `projects` (одна миграция, FK связь) | `014_create_projects.sql` | ✅ | [детали](docs/tasks/1.1-migration-projects.md) |
| 1.2 | Миграция: таблица `teams` | `015_create_teams.sql` | ✅ | [детали](docs/tasks/1.2-migration-teams.md) |
| 1.3 | Миграция: обновление таблицы `agents` + таблицы `tool_definitions`, `agent_tool_bindings`, `mcp_server_configs`, `agent_mcp_bindings` | `016_alter_agents.sql` | ✅ | [детали](docs/tasks/1.3-migration-alter-agents.md) |
| 1.4 | Миграция: таблица `tasks` (со всеми статусами, связями, артефактами) | `017_create_tasks.sql` | ⬜ | [детали](docs/tasks/1.4-migration-tasks.md) |
| 1.5 | Миграция: таблица `task_messages` | `018_create_task_messages.sql` | ⬜ | [детали](docs/tasks/1.5-migration-task-messages.md) |
| 1.6 | Миграция: таблица `conversations` + `conversation_messages` | `019_create_conversations.sql` | ⬜ | [детали](docs/tasks/1.6-migration-conversations.md) |
| 1.7 | Go-модели для всех новых сущностей | `models/project.go`, `team.go`, `task.go`, `conversation.go`, `git_credential.go` | ⬜ |
| 1.8 | Тест: UP → DOWN → UP для всех миграций | `make migrate-up && make migrate-down && make migrate-up` | ⬜ |

**Зависимости:** Нет (можно начинать сразу)

---

### Sprint 2 — CRUD API для Project и Team (Backend)

**Цель:** API для управления проектами и командами.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 2.1 | Repository: `ProjectRepository` (CRUD + список с пагинацией) | `backend/internal/repository/project_repository.go` | ⬜ | [детали](docs/tasks/2.1-repository-project.md) |
| 2.2 | Repository: `TeamRepository` | `backend/internal/repository/team_repository.go` | ⬜ | [детали](docs/tasks/2.2-repository-team.md) |
| 2.3 | Repository: `GitCredentialRepository` | `backend/internal/repository/git_credential_repository.go` | ⬜ | [детали](docs/tasks/2.3-repository-git-credential.md) |
| 2.4 | Service: `ProjectService` (создание проекта + автоматическое создание команды + шифрование credentials) | `backend/internal/service/project_service.go` | ⬜ | [детали](docs/tasks/2.4-service-project.md) |
| 2.5 | DTO: request/response структуры для проектов | `backend/internal/handler/dto/project_dto.go` | ⬜ | [детали](docs/tasks/2.5-dto-project.md) |
| 2.6 | Handler: `ProjectHandler` (POST/GET/PUT/DELETE /projects) | `backend/internal/handler/project_handler.go` | ⬜ | [детали](docs/tasks/2.6-handler-project.md) |
| 2.7 | Handler: настройка команды (`GET/PUT /projects/:id/team`) | `backend/internal/handler/team_handler.go` | ⬜ | [детали](docs/tasks/2.7-handler-team.md) |
| 2.8 | Роуты: регистрация в `server.go` | `backend/internal/server/server.go` | ⬜ | [детали](docs/tasks/2.8-routes-server.md) |
| 2.9 | Swagger-аннотации для всех новых эндпоинтов | В каждом handler | ⬜ | [детали](docs/tasks/2.9-swagger-annotations.md) |
| 2.10 | Unit-тесты: ProjectService | `backend/internal/service/project_service_test.go` | ⬜ |
| 2.11 | MCP-инструменты: `project_list`, `project_get`, `project_create` | `backend/internal/mcp/tools_project.go` | ⬜ | [детали](docs/tasks/2.11-mcp-tools-project.md) |

**API эндпоинты:**

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/projects` | Создать проект |
| GET | `/api/v1/projects` | Список проектов |
| GET | `/api/v1/projects/:id` | Получить проект |
| PUT | `/api/v1/projects/:id` | Обновить проект |
| DELETE | `/api/v1/projects/:id` | Удалить проект |
| GET | `/api/v1/projects/:id/team` | Получить команду проекта |
| PUT | `/api/v1/projects/:id/team` | Обновить команду (агенты, роли) |

**Зависимости:** Sprint 1

---

### Sprint 3 — CRUD API для Tasks (Backend)

**Цель:** API для управления задачами и их жизненным циклом.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 3.1 | Repository: `TaskRepository` (CRUD + фильтрация по project/status/agent + пагинация) | `backend/internal/repository/task_repository.go` | ✅ | [детали](docs/tasks/3.1-repository-task.md) |
| 3.2 | Repository: `TaskMessageRepository` | `backend/internal/repository/task_message_repository.go` | ✅ | [детали](docs/tasks/3.2-repository-task-message.md) |
| 3.3 | Service: `TaskService` (создание, смена статуса, назначение агента, валидация переходов) | `backend/internal/service/task_service.go` | ✅ | [детали](docs/tasks/3.3-service-task.md) |
| 3.4 | DTO: request/response для задач | `backend/internal/handler/dto/task_dto.go` | ✅ | [детали](docs/tasks/3.4-dto-task.md) |
| 3.5 | Handler: `TaskHandler` | `backend/internal/handler/task_handler.go` | ✅ | [детали](docs/tasks/3.5-handler-task.md) |
| 3.6 | Валидация: state machine для статусов задач (допустимые переходы) | В `TaskService` | ✅ | [детали](docs/tasks/3.6-state-machine.md) |
| 3.7 | Swagger-аннотации | В handler | ✅ | [детали](docs/tasks/3.7-swagger-annotations-tasks.md) |
| 3.8 | Unit-тесты: TaskService (особенно переходы статусов) | `backend/internal/service/task_service_test.go` | ✅ | [детали](docs/tasks/3.8-unit-tests-task-service.md) |
| 3.9 | MCP-инструменты: `task_list`, `task_get`, `task_create`, `task_update` | `backend/internal/mcp/tools_task.go` | ⬜ | [детали](docs/tasks/3.9-mcp-tools-task.md) |

**API эндпоинты:**

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/projects/:id/tasks` | Создать задачу |
| GET | `/api/v1/projects/:id/tasks` | Список задач (фильтры: status, agent, priority) |
| GET | `/api/v1/tasks/:id` | Получить задачу |
| PUT | `/api/v1/tasks/:id` | Обновить задачу |
| POST | `/api/v1/tasks/:id/pause` | Приостановить задачу |
| POST | `/api/v1/tasks/:id/cancel` | Отменить задачу |
| POST | `/api/v1/tasks/:id/resume` | Возобновить задачу |
| GET | `/api/v1/tasks/:id/messages` | Сообщения задачи |
| POST | `/api/v1/tasks/:id/messages` | Добавить сообщение (пользовательская коррекция) |

**Зависимости:** Sprint 1

---

### Sprint 4 — Git Provider Integration (Backend)

**Цель:** Абстракция для работы с GitHub/GitLab/Bitbucket, клонирование репозиториев.

| # | Задача | Файлы | Статус | Спека |
|---|--------|-------|--------|-------|
| 4.1 | Интерфейс `GitProvider` | `backend/pkg/gitprovider/provider.go` | ⬜ | [детали](docs/tasks/4.1-gitprovider-interface.md) |
| 4.2 | Типы: `CloneOptions`, `PROptions`, `PullRequest` и т.д. | `backend/pkg/gitprovider/types.go` | ⬜ | [детали](docs/tasks/4.2-gitprovider-types.md) |
| 4.3 | Реализация: `GitHubProvider` (REST API v3 + go-github) | `backend/pkg/gitprovider/github.go` | ⬜ | [детали](docs/tasks/4.3-github-provider.md) |
| 4.4 | Реализация: `LocalGitProvider` (git CLI через exec) | `backend/pkg/gitprovider/local.go` | ⬜ | [детали](docs/tasks/4.4-local-provider.md) |
| 4.5 | Фабрика: `NewGitProvider(providerType, credentials)` | `backend/pkg/gitprovider/factory.go` | ⬜ | [детали](docs/tasks/4.5-gitprovider-factory.md) |
| 4.6 | Service: интеграция GitProvider в `ProjectService` (clone при создании проекта) | `backend/internal/service/project_service.go` | ⬜ | [детали](docs/tasks/4.6-project-service-gitprovider.md) |
| 4.7 | Шифрование credentials (AES-256-GCM) | `backend/pkg/crypto/encrypt.go` | ⬜ | [детали](docs/tasks/4.7-encrypt-credentials.md) |
| 4.8 | Unit-тесты: GitHubProvider (с моками HTTP) | `backend/pkg/gitprovider/github_test.go` | ⬜ |  [детали](docs/tasks/4.8-unit-tests-github-provider.md)  |
| 4.9 | Unit-тесты: LocalGitProvider | `local_test.go`, `local_cli_test.go`, `helpers_test.go` (+ `local_integration_test.go`) в `backend/pkg/gitprovider/` | ✅ | [детали](docs/tasks/4.9-unit-tests-local-provider.md) |

**Зависимости:** Sprint 2

---

### Sprint 5 — Sandbox Runner (Backend, Docker)

**Цель:** Запуск задач в изолированных Docker-контейнерах.

| # | Задача | Файлы | Статус | Детали |
|---|--------|-------|--------|--------|
| 5.1 | Dockerfile: `devteam/sandbox-claude` (Node.js + Claude Code CLI + git) | `deployment/sandbox/claude/Dockerfile` | ✅ | [детали](docs/tasks/5.1-dockerfile-sandbox-claude.md) |
| 5.2 | Entrypoint-скрипт sandbox (clone → branch → agent → diff → result) | `deployment/sandbox/claude/entrypoint.sh` | ✅ | [детали](docs/tasks/5.2-entrypoint-sandbox-claude.md) |
| 5.3 | Интерфейс `SandboxRunner` | `backend/internal/sandbox/runner.go` | ✅ | [детали](docs/tasks/5.3-sandbox-runner-interface.md) |
| 5.4 | Типы: `SandboxOptions`, `SandboxStatus`, `CodeResult`, `ResourceLimit` | `backend/internal/sandbox/types.go` | ✅ | [детали](docs/tasks/5.4-sandbox-types.md) |
| 5.5 | Реализация: `DockerSandboxRunner` (Docker SDK для Go) | `backend/internal/sandbox/docker_runner.go` | ✅ | [детали](docs/tasks/5.5-docker-sandbox-runner.md) |
| 5.6 | Стрим логов из контейнера (`docker.ContainerLogs` → channel) | `stream_logs.go`, `stream_line_writer.go` | ✅ | [детали](docs/tasks/5.6-sandbox-stream-logs.md) |
| 5.7 | Сбор результата (`CopyFromContainer` → `status.json` + diff/log) | `collect_artifacts.go`, `status_json.go`, `docker_runner.go` | ✅ | [детали](docs/tasks/5.7-sandbox-collect-results.md) |
| 5.8 | Таймаут и принудительная остановка | `lifecycle_manager.go`, `docker_stopper.go`, `docker_runner.go` | ✅ | [детали](docs/tasks/5.8-sandbox-timeout-and-stop.md) |
| 5.9 | Resource limits (CPU, Memory) при создании контейнера | `docker_runner.go`, `resource_limits*.go`, `options_validate.go` | ✅ | [детали](docs/tasks/5.9-sandbox-resource-limits.md) |
| 5.10 | Конфигурация: `SandboxConfig` в `config.go` | `backend/internal/config/config.go` | ✅ | [детали](docs/tasks/5.10-sandbox-config.md) |
| 5.11 | docker-compose: монтирование `/var/run/docker.sock` | `docker-compose.yml` (корень), `Makefile` | ✅ | [детали](docs/tasks/5.11-docker-compose-docker-sock.md) |
| 5.12 | Makefile: `sandbox-build` (сборка sandbox-образов) | `Makefile` | ✅ | [детали](docs/tasks/5.12-makefile-sandbox-build.md) |
| 5.13 | Unit-тесты: DockerSandboxRunner (с мок Docker Client) | `backend/internal/sandbox/docker_runner_test.go` | ✅ | [детали](docs/tasks/5.13-unit-tests-docker-sandbox-runner.md) |
| 5.14 | Интеграционный тест: запуск реального контейнера с простой задачей | `backend/internal/sandbox/integration_test.go` | ✅ | [детали](docs/tasks/5.14-integration-test-docker-sandbox-runner.md) |

Параметры **`SANDBOX_*`** (лимиты, таймаут по умолчанию, `SANDBOX_MAX_CONCURRENT`) загружаются в `config.Load()`; имена и дефолты — `backend/internal/config/sandbox_config.go`.

**Зависимости:** Sprint 1

---

### Sprint 6 — Orchestrator Agent (Backend)

**Цель:** Базовый оркестратор — принимает запрос от пользователя, создаёт задачи, управляет pipeline.

| # | Задача | Файлы | Статус | Детали |
|---|--------|-------|--------|--------|
| 6.1 | Интерфейс `AgentExecutor` (запуск агента с задачей) | `backend/internal/agent/executor.go` | ✅ | [детали](docs/tasks/6.1-agent-executor-interface.md) |
| 6.2 | Реализация: `LLMAgentExecutor` (вызов LLM с промптом + tools) | `backend/internal/agent/llm_executor.go` | ⬜ | [детали](docs/tasks/6.2-llm-agent-executor.md) |
| 6.3 | Реализация: `SandboxAgentExecutor` (запуск sandbox-контейнера для Developer) | `backend/internal/agent/sandbox_executor.go` | ⬜ | [детали](docs/tasks/6.3-sandbox-agent-executor.md) |
| 6.4 | Orchestrator: `OrchestratorService` — основной цикл управления | `backend/internal/service/orchestrator_service.go` | ⬜ | [детали](docs/tasks/6.4-orchestrator-service.md) |
| 6.5 | Pipeline: линейный поток `Plan → Develop → Review → Test` | `backend/internal/service/orchestrator_service.go` | ⬜ | [детали](docs/tasks/6.5-pipeline-linear-flow.md) |
| 6.6 | Обработка результатов: `completed` → следующий шаг, `changes_requested` → назад к Developer | `backend/internal/service/result_processor.go` | ⬜ | [детали](docs/tasks/6.6-result-processor.md) |
| 6.7 | Обработка пользовательских команд: `pause`, `cancel`, `resume`, `correct` | В `OrchestratorService` | ⬜ | [детали](docs/tasks/6.7-user-commands.md) |
| 6.8 | Промпты агентов: Orchestrator, Planner, Developer, Reviewer, Tester | `backend/prompts/base_prompt.yaml`, `orchestrator.yaml`, `planner.yaml`, `developer.yaml`, `reviewer.yaml`, `tester.yaml`, `prompt_schema.json` | ⬜ | [детали](docs/tasks/6.8-agent-prompts.md) |
| 6.9 | Агенты по умолчанию (YAML-конфиг) | `backend/agents/orchestrator.yaml`, `planner.yaml`, `developer.yaml`, `reviewer.yaml`, `tester.yaml` | ✅ | [детали](docs/tasks/6.9-agents-default-config.md) |
| 6.10 | Unit-тесты: OrchestratorService (полный pipeline, ретраи, отмена) | `backend/internal/service/orchestrator_service_test.go` | ✅ | |

**Pipeline (линейный MVP):**
```
User Message
    → Orchestrator (LLM: анализ запроса)
        → Planner (LLM: декомпозиция задачи)
            → Developer (Sandbox: Claude Code CLI)
                → Reviewer (LLM: ревью diff)
                    → [если changes_requested → Developer]
                    → Tester (Sandbox: запуск тестов)
                        → [если failed → Developer]
                        → Completed
```

**Зависимости:** Sprint 3, Sprint 5

---

### Sprint 7 — WebSocket и Реалтайм (Backend)

**Цель:** Стриминг статусов задач и логов агентов в реальном времени.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 7.1 | WebSocket Hub: менеджер подключений (по project_id) | `backend/internal/ws/hub.go` | ✅ |
| 7.2 | WebSocket Handler: upgrade HTTP → WS, аутентификация через JWT | `backend/internal/ws/handler.go` | ⬜ |
| 7.3 | Типы сообщений: `task_status`, `task_message`, `agent_log`, `error` | `backend/internal/ws/types.go` | ⬜ |
| 7.4 | Event Bus: внутренний pub/sub для уведомлений между сервисами | `backend/internal/ws/eventbus.go` | ⬜ |
| 7.5 | Интеграция: TaskService → EventBus при изменении статуса | `backend/internal/service/task_service.go` | ⬜ |
| 7.6 | Интеграция: SandboxRunner → EventBus для стриминга логов | `backend/internal/sandbox/docker_runner.go` | ⬜ |
| 7.7 | Роут: `GET /api/v1/projects/:id/ws` (WebSocket) | `backend/internal/server/server.go` | ⬜ |
| 7.8 | Unit-тесты: WebSocket Hub | `backend/internal/ws/hub_test.go` | ⬜ |
| 7.9 | HubBridge: трансляция `ConversationMessageCreated` в WebSocket | `backend/internal/ws/hubbridge.go`, `backend/internal/ws/types.go` | ⬜ | [детали](docs/tasks/7.9-hubbridge-conversation-message-ws.md) |

**Зависимости:** Sprint 3, Sprint 6

---

### Sprint 8 — Conversation API (Backend)

**Цель:** API для чатов пользователя с оркестратором.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 8.1 | Repository: `ConversationRepository` | `backend/internal/repository/conversation_repository.go` | ✅ | [детали](docs/tasks/8.1-repository-conversation.md) |
| 8.2 | Repository: `ConversationMessageRepository` | `backend/internal/repository/conversation_message_repository.go` | ✅ | [детали](docs/tasks/8.2-repository-conversation-message.md) |
| 8.3 | Service: `ConversationService` (создание, отправка сообщения → Orchestrator, получение истории) | `backend/internal/service/conversation_service.go` | ⏳ |
| 8.4 | Handler: `ConversationHandler` | `backend/internal/handler/conversation_handler.go` | ⬜ | [детали](docs/tasks/8.4-handler-conversation.md) |
| 8.5 | Связка: `ConversationService` → `OrchestratorService` (новое сообщение запускает обработку) | Интеграция | ⬜ |
| 8.6 | Swagger-аннотации для Conversation API | В handler | [docs/tasks/8.6-swagger-annotations.md](docs/tasks/8.6-swagger-annotations.md) | ⬜ |
| 8.7 | Swagger-аннотации для Task-эндпоинтов (ревизия) | В handler | [docs/tasks/3.7-swagger-annotations-tasks.md](docs/tasks/3.7-swagger-annotations-tasks.md) | ✅ |
| 8.7 | Unit-тесты | `backend/internal/service/conversation_service_test.go` | ⬜ |

**API эндпоинты:**

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/projects/:id/conversations` | Создать разговор |
| GET | `/api/v1/projects/:id/conversations` | Список разговоров |
| GET | `/api/v1/conversations/:id` | Получить разговор с сообщениями |
| POST | `/api/v1/conversations/:id/messages` | Отправить сообщение (триггерит Orchestrator) |
| DELETE | `/api/v1/conversations/:id` | Удалить разговор |

**Зависимости:** Sprint 6, Sprint 7

---

### Sprint 9 — Векторная индексация проекта (Backend)

**Цель:** Индексация кода, задач и чатов проекта в Weaviate для контекстного поиска.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 9.1 | Коллекция Weaviate per project: `DevTeam_Project_{id}` | Конфигурация в `vectordb` | ✅ |
| 9.2 | Индексатор кода: чанкинг файлов (по файлам для малых, по функциям для больших) | `backend/internal/indexer/code_indexer.go` | ✅ |
| 9.3 | Индексатор задач: описание + результат + сообщения | `backend/internal/indexer/task_indexer.go` | ✅ |
| 9.4 | Индексатор чатов: сообщения пользователя и ассистента | `backend/internal/indexer/conversation_indexer.go` | ✅ |
| 9.5 | Service: `IndexerService` (полная индексация проекта, инкрементальное обновление) | `backend/internal/service/indexer_service.go` | ⏳ |
| 9.6 | Хук: индексация кода при создании проекта (после clone) | В `ProjectService` | ✅ |
| 9.7 | Хук: индексация задачи при создании/обновлении | В `TaskService` | ✅ |
| 9.8 | Хук: индексация сообщения при создании | В `ConversationService` | ✅ |
| 9.9 | API: `POST /api/v1/projects/:id/reindex` (полная переиндексация) | Handler + Service | ✅ |
| 9.10 | Контекстный поиск: `SearchContext(projectID, query)` → релевантные чанки | В `CodeIndexer` | ✅ |
| 9.11 | Интеграция с Orchestrator: перед запуском агента — vector search для контекста | В `OrchestratorService` | ✅ |
| 9.12 | Unit-тесты | `backend/internal/service/indexer_service_test.go` | ✅ |

**Зависимости:** Sprint 2, Sprint 3, Sprint 8

---

### Sprint 10 — Frontend: Проекты и навигация (Flutter)

**Цель:** Базовый UI — список проектов, создание проекта, навигация.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 10.1 | Модели (Freezed): `ProjectModel`, `TeamModel`, `AgentModel` | `frontend/lib/features/projects/domain/` | ⬜ | [детали](docs/tasks/10.1-models-freezed.md) |
| 10.2 | Repository: `ProjectRepository` (Dio → backend API) | `frontend/lib/features/projects/data/project_repository.dart` | ⬜ | [детали](docs/tasks/10.2-repository-project.md) |
| 10.3 | Providers: `projectListProvider`, `projectProvider` | `frontend/lib/features/projects/data/project_providers.dart` | ⬜ | [детали](docs/tasks/10.3-providers-project.md) |
| 10.4 | Экран: Список проектов (карточки, статусы, поиск) | `frontend/lib/features/projects/presentation/screens/projects_list_screen.dart` | ⬜ | [детали](docs/tasks/10.4-projects-list-screen.md) |
| 10.5 | Экран: Создание проекта (форма: имя, описание, git URL, провайдер) | `frontend/lib/features/projects/presentation/screens/create_project_screen.dart` | ⬜ | [детали](docs/tasks/10.5-create-project-screen.md) |
| 10.6 | Экран: Дашборд проекта (hub → чат, задачи, команда, настройки) | `frontend/lib/features/projects/presentation/screens/project_dashboard_screen.dart` | ⬜ | [детали](docs/tasks/10.6-project-dashboard-screen.md) |
| 10.7 | Обновить GoRouter: новые роуты `/projects`, `/projects/:id`, `/projects/:id/*` | `frontend/lib/core/routing/app_router.dart` | ⬜ | [детали](docs/tasks/10.7-gorouter-projects-routes.md) |
| 10.8 | Локализация / аудит l10n для Sprint 10 (ru, en) | `frontend/lib/l10n/app_ru.arb`, `app_en.arb` | ✅ | [детали](docs/tasks/10.8-l10n-projects-strings.md) |
| 10.9 | Widget-тесты: ProjectCard, CreateProjectForm и сопутствующие экраны | `frontend/test/features/projects/` | ✅ | [детали](docs/tasks/10.9-projects-widget-tests.md) |

**Зависимости:** Sprint 2

---

### Sprint 11 — Frontend: Чат (Flutter)

**Цель:** Основной интерфейс — чат с оркестратором, live-обновления.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 11.1 | Модели (Freezed): сущности чата + пагинационные обёртки (`ConversationModel`, `ConversationMessageModel`, `ConversationListResponse`, `MessageListResponse`) | `frontend/lib/features/chat/domain/` | ✅ | [детали](docs/tasks/11.1-models-freezed-chat.md) |
| 11.2 | WebSocket Service: подключение, реконнект, парсинг событий | `frontend/lib/core/api/websocket_service.dart`, `frontend/lib/core/api/websocket_events.dart`, `frontend/lib/core/api/websocket.dart` | ✅ | [детали](docs/tasks/11.2-websocket-service.md) |
| 11.3 | Repository: `ConversationRepository` (Dio → backend API) | `frontend/lib/features/chat/data/conversation_repository.dart` | ✅ | [детали](docs/tasks/11.3-repository-conversation.md) |
| 11.4 | Controller: `ChatController` (AsyncNotifier — загрузка, отправка, стрим) | `frontend/lib/features/chat/presentation/controllers/chat_controller.dart` | ✅ | [детали](docs/tasks/11.4-chat-controller.md) |
| 11.5 | Экран: Чат (список сообщений, поле ввода, отправка) | `frontend/lib/features/chat/presentation/screens/chat_screen.dart` | ✅ | [детали](docs/tasks/11.5-chat-screen.md) |
| 11.6 | Виджет: `ChatMessage` (user/assistant/system, markdown, код, стримящийся текст) | `frontend/lib/features/chat/presentation/widgets/chat_message.dart` | ✅ | [детали](docs/tasks/11.6-chat-message-widget.md) |
| 11.7 | Виджет: `TaskStatusCard` (встроенная карточка статуса задачи в чате) | `task_status_card.dart`, `task_status_visuals.dart`, `chat_screen.dart`; live-статусы — задача **11.9** ниже в этом спринте | ✅ | [детали](docs/tasks/11.7-task-status-card-widget.md) |
| 11.8 | Виджет: `ChatInput` (текстовое поле + кнопки: отправить, стоп, attach) | `frontend/lib/features/chat/presentation/widgets/chat_input.dart` | ✅ | [детали](docs/tasks/11.8-chat-input-widget.md) |
| 11.9 | Реалтайм: подписка на WebSocket → обновление UI при новых сообщениях/статусах | В `ChatController` | ✅ | [детали](docs/tasks/11.9-realtime-websocket-subscription.md) |
| 11.10 | Локализация / аудит l10n для Sprint 11 — чат (ru, en) | `frontend/lib/l10n/app_ru.arb`, `app_en.arb`; `frontend/lib/features/chat/` | ✅ | [детали](docs/tasks/11.10-l10n-chat-strings.md) |
| 11.11 | Widget-тесты: ChatScreen, ChatMessage, ChatInput, TaskStatusCard и сопутствующие сценарии | `frontend/test/features/chat/` | ✅ | [детали](docs/tasks/11.11-chat-widget-tests.md) |

**Зависимости:** Sprint 7, Sprint 8, Sprint 10

---

### Sprint 12 — Frontend: Задачи (Flutter)

**Цель:** UI для просмотра задач, их статусов, деталей и логов агентов.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 12.1 | Модели (Freezed): `TaskModel`, `TaskMessageModel` | `frontend/lib/features/tasks/domain/` | ✅ | [детали](docs/tasks/12.1-models-freezed-tasks.md) |
| 12.2 | Repository: `TaskRepository` (Dio → backend API) | `frontend/lib/features/tasks/data/task_repository.dart` | ✅ | [детали](docs/tasks/12.2-repository-task.md) |
| 12.3 | Controller: `TaskListController`, `TaskDetailController` | `frontend/lib/features/tasks/presentation/controllers/` | ✅ | [детали](docs/tasks/12.3-task-list-detail-controllers.md) |
| 12.4 | Экран: Список задач (гибрид: список / Kanban) | `frontend/lib/features/tasks/presentation/screens/tasks_list_screen.dart` | ✅ | [детали](docs/tasks/12.4-tasks-list-screen.md) |
| 12.5 | Экран: Детали задачи (статус, описание, лог агентов, diff, результат) | `frontend/lib/features/tasks/presentation/screens/task_detail_screen.dart` | ✅ | [детали](docs/tasks/12.5-task-detail-screen.md) |
| 12.6 | Виджет: `TaskCard` (статус, приоритет, агент, время) | `frontend/lib/features/tasks/presentation/widgets/task_card.dart` | ✅ | [детали](docs/tasks/12.6-task-card-widget.md) |
| 12.7 | Виджет: `DiffViewer` (отображение git diff с подсветкой) | `frontend/lib/shared/widgets/diff_viewer.dart` | ✅ | [детали](docs/tasks/12.7-diff-viewer-widget.md) |
| 12.8 | Действия: кнопки Pause/Cancel/Resume на задаче | `frontend/lib/features/tasks/presentation/screens/task_detail_screen.dart` | ✅ | [детали](docs/tasks/12.8-task-detail-actions-pause-cancel-resume.md) |
| 12.9 | Реалтайм: обновление статусов задач через WebSocket | В controllers | ✅ | [детали](docs/tasks/12.9-realtime-task-status-websocket.md) |
| 12.10 | Widget-тесты | `frontend/test/features/tasks/` | ✅ | [детали](docs/tasks/12.10-tasks-widget-tests.md) |

**Зависимости:** Sprint 3, Sprint 10, Sprint 11

---

### Sprint 13 — Frontend: Настройки команды и проекта (Flutter)

**Цель:** UI для управления агентами команды и настройками проекта.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 13.1 | Экран: Команда проекта (список агентов, их роли, модели, статус) | `frontend/lib/features/team/presentation/screens/team_screen.dart` | ⬜ | [детали](docs/tasks/13.1-team-screen.md) |
| 13.2 | Виджет: `AgentCard` (роль, модель, code_backend, on/off) | `frontend/lib/features/team/presentation/widgets/agent_card.dart` | ⬜ | [детали](docs/tasks/13.2-agent-card-widget.md) |
| 13.3 | Диалог: редактирование агента (модель, промпт, активность, code_backend) | `frontend/lib/features/team/presentation/widgets/agent_edit_dialog.dart` | ✅ | [детали](docs/tasks/13.3-agent-edit-dialog.md) |
| 13.3.1 | API и UI: секция «Инструменты» агента (реестр tool_definitions + bindings в team/PATCH) | `frontend/lib/features/team/presentation/widgets/agent_edit_dialog.dart`, backend — см. [детали](docs/tasks/13.3.1-agent-tools-section.md) | ✅ | [детали](docs/tasks/13.3.1-agent-tools-section.md) |
| 13.4 | Экран: Настройки проекта (git credentials, tech stack, vector index) | `frontend/lib/features/projects/presentation/screens/project_settings_screen.dart` | ✅ | [детали](docs/tasks/13.4-project-settings-screen.md) |
| 13.5 | Экран: Глобальные настройки (API keys для LLM-провайдеров) | `frontend/lib/features/settings/presentation/screens/global_settings_screen.dart`, `frontend/test/features/settings/presentation/screens/global_settings_screen_test.dart` | ✅ | [детали](docs/tasks/13.5-global-settings-screen.md) |
| 13.6 | Локализация / аудит l10n для Sprint 13 (ru, en) | `frontend/lib/l10n/app_ru.arb`, `app_en.arb` | ✅ | [детали](docs/tasks/13.6-l10n-sprint13-arb.md) |
| 13.7 | Widget-тесты (команда проекта; доп. сценарии для `settings` — только сверх обязательного набора **13.5**, см. [13.5](docs/tasks/13.5-global-settings-screen.md)) | `frontend/test/features/team/`, при расширении — `frontend/test/features/settings/` | ✅ | |

**Зависимости:** Sprint 2, Sprint 10.

**Отдельный PR (не смешивать со скоупом экранов 13.1–13.4):** DRY для Dio — довести `mapDioExceptionForRepository` (`frontend/lib/core/api/dio_repository_error_map.dart`) до **`TaskRepository`**, **`ConversationRepository`**, **`PromptsRepository`** и любых оставшихся копий `_handleDioError` (сейчас на хелпере уже `ProjectRepository` и `TeamRepository`). Канон для репозиториев — **`TeamRepository`**: общий маппинг, **`CancelToken`**, типизированные исключения, **freezed**-модели где применимо. Порядок работы **`PromptsRepository`** с **13.3** — **`docs/tasks/13.3-agent-edit-dialog.md`**, раздел **«PromptsRepository (канон)»**. В том же или следующем PR — **юнит-тесты на сам хелпер** (ветки switch, в первую очередь **`on409: null` → `onOtherHttp` с `statusCode == 409`**), чтобы контракт не зависел только от транзитивного покрытия через `team_repository_test`.

**Решения по дизайну (не менять без отдельной задачи):** `TeamApiException` без поля `apiErrorCode` — намеренное зеркало `ProjectApiException` (стабильный код из JSON пока не протаскиваем). **404 команды** в UI — общий `dataLoadError` + retry, без отдельного экрана как у `ProjectNotFoundException` на дашборде (404 команды при живом проекте — аномалия; отдельный UX не входил в 13.1). В **`AgentCard`** связка `Semantics` + `Chip(label: Text(...))` оставлена для a11y; в 13.3 при добавлении tooltip / `excludeSemantics` не плодить лишние источники озвучивания. **Переключатель `is_active` только в диалоге 13.3** (не на карточке, **13.2**): осознанный trade-off — ~5 шагов (тап → диалог → switch → сохранить → закрыть) ради защиты от случайных мутаций; inline-toggle на **`AgentCard`** — отдельная задача после метрик/обоснования.

---

### Sprint 14 — E2E интеграция и тестирование

**Цель:** Полный сквозной тест: от сообщения в чате до кода в репозитории.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 14.1 | E2E тест (backend): создать проект → отправить запрос → Orchestrator создаёт задачи → Developer выполняет → Reviewer одобряет → Completed | `backend/internal/service/orchestrator_integration_test.go` | ✅ |
| 14.2 | E2E тест (frontend): интеграционный тест полного flow в UI | `frontend/integration_test/full_flow_test.dart` | ✅ |
| 14.3 | Нагрузочное тестирование: 5 параллельных sandbox-контейнеров | `backend/internal/sandbox/sandbox_real_test.go` (`TestSandbox_LoadFiveParallel`) | ✅ |
| 14.4 | Тест безопасности: sandbox не может выйти за пределы `/workspace` | `backend/internal/sandbox/sandbox_real_test.go` (`TestSandbox_Isolation_AgentCannotWriteOutsideWorkspace`) | ✅ |
| 14.5 | Тест отмены: пользователь нажимает Cancel → контейнер убивается | `backend/internal/sandbox/sandbox_real_test.go` (`TestSandbox_Cancel_StopSignalKillsRunningContainer`) | ✅ |
| 14.6 | Документация: обновить README, API.md, env.example | Корень | ✅ |
| 14.7 | Full-stack smoke: реальный pipeline → PR на GitHub | `scripts/e2e_smoke.sh` + `make e2e-smoke` | ✅ |

**Зависимости:** Все предыдущие спринты

---

### Sprint 15 — Авторизация Claude Code, мультипровайдерный LLM и кастомизация агентов

**Цель:** Освободиться от жёсткой завязки на Anthropic API-ключи: разрешить логин Claude Code по подписке (OAuth), подключить альтернативные LLM-провайдеры с per-user креденшелами, и дать пользователю UI для тонкой настройки каждого агента (модель/провайдер, MCP-серверы, Skills, разрешения Claude Code для sandbox).

**Контекст:** в sandbox-контейнере (Sprint 5) Claude Code сейчас запускается с `ANTHROPIC_API_KEY` и спотыкается на интерактивных подтверждениях операций (write/bash/network) — оркестратор зависает в ожидании. Нужно: (а) разрешить аутентификацию подпиской вместо API-ключа, (б) поддержать совместимые провайдеры через native Anthropic endpoints провайдеров (DeepSeek `api.deepseek.com/anthropic`, Zhipu `open.bigmodel.cn/api/anthropic`, OpenRouter `openrouter.ai/api/v1`), (в) пробросить per-agent `settings.json` с `permissions.allow` / `permissions.defaultMode` и `--dangerously-skip-permissions` там, где это безопасно (изолированный контейнер).

> **Sprint 15.e2e revision (2026-05-13):** изначальный план с sidecar-прокси `free-claude-code` оказался однотенантным и не ложился на мультиюзера. Заменён на per-agent `provider_kind` + per-user `user_llm_credentials`; sandbox ходит напрямую на native Anthropic endpoint провайдера. Sidecar и весь связанный код удалены — см. блок 15.C ниже.

#### 15.A — Backend: модель данных и провайдеры

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 15.1 | Миграция: таблица `llm_providers` (id, kind: `anthropic`/`anthropic_oauth`/`openrouter`/`deepseek`/`moonshot`/`ollama`/`zhipu`, base_url, auth_type, credentials_encrypted, default_model, enabled). _Sprint 15.e2e: kind=`free_claude_proxy` удалён._ | `backend/migrations/020_create_llm_providers.sql` | ✅ |
| 15.2 | Миграция: таблица `claude_code_subscriptions` (id, user_id, oauth_access_token, oauth_refresh_token, expires_at, scopes — всё AES-256-GCM) | `backend/migrations/021_create_claude_code_subscriptions.sql` | ✅ |
| 15.3 | Миграция: `ALTER TABLE agents` — добавить `llm_provider_id`, `code_backend_settings JSONB` (модель, MCP-сервера, Skills, claude code permissions), `sandbox_permissions JSONB` (allow/deny/defaultMode) | `backend/migrations/022_alter_agents_provider_and_settings.sql` | ✅ |
| 15.3a | _Sprint 15.e2e:_ Миграция `028_agents_provider_kind_and_drop_proxy.sql`: `agents.provider_kind` (enum `anthropic`/`anthropic_oauth`/`deepseek`/`zhipu`/`openrouter`), `zhipu` в чек `user_llm_credentials`, схлопывание `code_backend` (без `claude-code-via-proxy`) | `backend/db/migrations/028_*.sql` | ✅ |
| 15.4 | Миграция: таблица `mcp_servers_registry` (id, name, transport: `stdio`/`http`/`sse`, command/url, env_template, scope: `global`/`project`/`agent`) — расширение существующей `mcp_server_configs` или новая | `backend/migrations/023_create_mcp_servers_registry.sql` | ✅ |
| 15.5 | Миграция: таблица `agent_skills` (id, agent_id, skill_name, skill_source: `builtin`/`plugin`/`path`, config_json) | `backend/migrations/024_create_agent_skills.sql` | ✅ |
| 15.6 | Go-модели: `LLMProvider`, `ClaudeCodeSubscription`, `MCPServer`, `AgentSkill`, расширение `Agent` | `backend/internal/models/` | ✅ |
| 15.7 | Интерфейс `LLMProviderClient` (`Chat`, `Embed`, `HealthCheck`, `ResolveBaseURL`) — для `LLMAgentExecutor` (6.2) | `backend/internal/llm/provider.go` | ✅ |
| 15.8 | Реализации провайдеров: `OpenRouterClient`, `DeepSeekClient`, `MoonshotClient`, `OllamaClient`, `ZhipuClient` (OpenAI-compatible REST) + `AnthropicClient` (OAuth и API-key режимы) | `backend/internal/llm/openrouter.go`, `deepseek.go`, `moonshot.go`, `ollama.go`, `zhipu.go`, `anthropic.go` | ✅ |
| 15.9 | Фабрика: `NewLLMClient(provider *LLMProvider, secrets Resolver)` — выбор реализации по `kind`, дешифровка кредов | `backend/internal/llm/factory.go` | ✅ |
| 15.10 | Repository + Service: `LLMProviderService` (CRUD, health-check, тест подключения) | `backend/internal/repository/llm_provider_repository.go`, `backend/internal/service/llm_provider_service.go` | ✅ |
| 15.11 | Unit-тесты: провайдеры (с моками HTTP) + фабрика | `backend/internal/llm/*_test.go` | ✅ |

#### 15.B — Авторизация Claude Code по подписке (OAuth)

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 15.12 | OAuth flow: `POST /api/v1/claude-code/auth/init` → возвращает device-code URL; `POST /api/v1/claude-code/auth/callback` → сохраняет токен в `claude_code_subscriptions` | `backend/internal/handler/claude_code_auth_handler.go`, `backend/internal/service/claude_code_auth_service.go` | ✅ |
| 15.13 | Refresh-токен: фоновый воркер обновляет токены до `expires_at - 5m` | `backend/internal/service/claude_code_token_refresher.go` | ✅ |
| 15.14 | Проброс OAuth-токена в sandbox: entrypoint получает `CLAUDE_CODE_OAUTH_TOKEN` env вместо `ANTHROPIC_API_KEY` | `deployment/sandbox/claude/entrypoint.sh`, `backend/internal/sandbox/docker_runner.go` | ✅ |
| 15.15 | Swagger + MCP-инструменты: `claude_code_auth_status`, `claude_code_auth_revoke` | handler + `backend/internal/mcp/tools_claude_code_auth.go` | ✅ |

#### 15.C — Не-Anthropic провайдеры через native endpoint (заменяет sidecar-прокси)

**Sprint 15.e2e (2026-05-13):** sidecar-прокси `free-claude-code` оказался однотенантным (один общий ключ на инстанс) и не подходил под мультиюзера. Заменён на прямое подключение Claude Code CLI к **native Anthropic endpoint** провайдера (DeepSeek, Zhipu, OpenRouter уже отдают совместимый `/v1/messages`); per-user ключ берётся из `user_llm_credentials`. Sidecar полностью выпилен из репо.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| ~~15.16~~ | ~~Sidecar `free-claude-proxy`~~ → **отменено**, заменено на native endpoint в резолвере | `backend/internal/service/sandbox_auth_resolver.go` | ✅ (заменено) |
| ~~15.17~~ | ~~Config builder прокси~~ → не нужен | — | ✅ (удалено) |
| 15.18 | Sandbox entrypoint: резолвер по `agent.provider_kind` выставляет `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` для DeepSeek/Zhipu/OpenRouter, либо `CLAUDE_CODE_OAUTH_TOKEN` для OAuth, либо `ANTHROPIC_API_KEY` для kind=anthropic | `deployment/sandbox/claude/entrypoint.sh`, `backend/internal/sandbox/docker_runner.go`, `backend/internal/service/sandbox_auth_resolver.go` | ✅ |
| ~~15.19~~ | ~~Health-check прокси~~ → не нужен (нет прокси) | — | ✅ (удалено) |
| 15.20 | Документация: how-to для каждого provider_kind (где взять ключ, какие модели) | `docs/llm-providers.md` | ⚠️ требует обновления под per-user model |

#### 15.D — Кастомизация агентов: MCP, Skills, Permissions

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 15.21 | Service: `AgentSettingsService` — генерация `~/.claude/settings.json` для агента в sandbox (permissions.allow/deny/defaultMode, env, hooks) | `backend/internal/service/agent_settings_service.go` | ✅ |
| 15.22 | Sandbox entrypoint: монтирование per-task `settings.json` + `.mcp.json` + `~/.claude/skills/` из `code_backend_settings` агента; запуск `claude` с `--permission-mode acceptEdits` (или `bypassPermissions` для изолированного контейнера — настраивается per-agent) | `deployment/sandbox/claude/entrypoint.sh`, `backend/internal/sandbox/docker_runner.go` | ✅ |
| 15.23 | API: `GET/PUT /api/v1/agents/:id/settings` — JSON-схема для `code_backend_settings` + валидация (allowedTools regex, MCP transport, известные Skills) | `backend/internal/handler/agent_settings_handler.go` | ✅ |
| 15.24 | MCP-инструменты: `agent_settings_get`, `agent_settings_update`, `mcp_server_list`, `skill_list` | `backend/internal/mcp/tools_agent_settings.go` | ✅ |
| 15.25 | Дефолтные пресеты ролей: `developer.yaml`, `reviewer.yaml`, `tester.yaml` — обновить с разумными `permissions.allow` (`Read`, `Edit`, `Write`, `Bash(git diff:*)`, `Bash(go test:*)` и т.д.) и `defaultMode: acceptEdits` | `backend/agents/*.yaml` | ✅ |
| 15.26 | Unit-тесты: `AgentSettingsService` (генерация settings.json), валидация permission-паттернов | `backend/internal/service/agent_settings_service_test.go` | ✅ |
| 15.27 | Интеграционный тест: sandbox с агентом, у которого `bypassPermissions` — задача с `Edit` + `Bash` проходит без зависания на подтверждениях | `backend/internal/sandbox/integration_permissions_test.go` | ✅ |

#### 15.E — Frontend: UI для провайдеров и настройки агентов

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 15.28 | Freezed-модели: `LLMProviderModel`, `ClaudeCodeAuthStatus`, `AgentSettingsModel` (MCP, Skills, permissions) | `frontend/lib/features/settings/domain/`, `frontend/lib/features/team/domain/` | ✅ |
| 15.29 | Repository + providers: `LLMProvidersRepository`, `ClaudeCodeAuthRepository`, `AgentSettingsRepository` | `frontend/lib/features/settings/data/`, `frontend/lib/features/team/data/` | ✅ |
| 15.30 | Экран: Глобальные настройки → вкладка «LLM-провайдеры»: список доступных kinds (OpenRouter/DeepSeek/Zhipu/Anthropic), per-user ключи (раздел уже частично есть как `/me/llm-credentials`). _Sprint 15.e2e:_ переключатель `free-claude-proxy` удалён, опция больше не существует | `frontend/lib/features/settings/presentation/screens/global_settings_screen.dart` (расширение) | ⚠️ требует адаптации под per-user creds |
| 15.31 | Экран: Глобальные настройки → вкладка «Claude Code» с кнопкой «Войти по подписке» (OAuth device flow), статус токена, отзыв | `frontend/lib/features/settings/presentation/widgets/claude_code_auth_section.dart` | ✅ |
| 15.32 | Диалог редактирования агента (расширение 13.3): вкладки «Модель/провайдер», «MCP-серверы», «Skills», «Разрешения Claude Code» (UI для `allow/deny/defaultMode`) | `frontend/lib/features/team/presentation/widgets/agent_edit_dialog.dart` (расширение) | ✅ |
| 15.33 | Локализация (ru, en) для всех новых строк | `frontend/lib/l10n/app_ru.arb`, `app_en.arb` | ✅ |
| 15.34 | Widget-тесты: новые секции global settings + новые вкладки agent edit dialog | `frontend/test/features/settings/`, `frontend/test/features/team/` | ✅ |

#### 15.F — E2E и безопасность

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 15.35 | E2E: mixed-agents pipeline — `developer/tester=anthropic_oauth` + `reviewer=deepseek` (native endpoint) + `orchestrator/planner=global ANTHROPIC_API_KEY`. Один прогон скрипта проверяет все три auth-пути в одной задаче, открывает PR на GitHub, грепает логи на утечки. _Sprint 15.e2e: переписан с per-profile запусков на единую mixed-матрицу._ | `scripts/e2e_smoke.sh` | ✅ |
| 15.36 | E2E: агент с `bypassPermissions` в Docker делает `Edit` + `Bash(git commit)` без интерактивного блокирования | расширение 14.4 / новый тест | ✅ |
| 15.37 | Security-аудит: OAuth-токены и API-ключи провайдеров не логируются (grep по логам в тесте), `settings.json` не уезжает в индекс Weaviate | `backend/internal/...` (тесты + аудит) | ✅ |

**Зависимости:** Sprint 5 (sandbox), Sprint 6 (Orchestrator/LLMAgentExecutor 6.2), Sprint 13 (Frontend настройки и agent edit dialog 13.3).

**Важно (стек / правила):**
- Backend строго следует Clean Architecture (handler → service → repository); никаких глобалов, миграции — только Goose (`make migrate-create`), `gorm.AutoMigrate` запрещён.
- Все новые публичные ручки получают MCP-инструмент в `backend/internal/mcp/` + Swagger-аннотации (`make swagger`).
- Frontend: абсолютные импорты `package:frontend/...`, Freezed `abstract class`, никакого хардкода строк — только `.arb` + `flutter gen-l10n`; `make frontend-codegen` после изменения моделей.
- Все секреты (OAuth-токены, ключи провайдеров) — AES-256-GCM через `backend/pkg/crypto/encrypt.go` (4.7), никогда не в plaintext в БД и не в логах.

---

## Порядок выполнения

```
Sprint 1 (модели + миграции)
    │
    ├── Sprint 2 (Project CRUD)
    │       ├── Sprint 4 (Git Provider)
    │       ├── Sprint 10 (Frontend: проекты)
    │       │       └── Sprint 13 (Frontend: настройки)
    │       └── Sprint 9 (Векторная индексация)
    │
    ├── Sprint 3 (Task CRUD)
    │       └── Sprint 12 (Frontend: задачи)
    │
    ├── Sprint 5 (Sandbox Runner)
    │
    └── Sprint 6 (Orchestrator) ← зависит от 3, 5
            │
            ├── Sprint 7 (WebSocket)
            │       └── Sprint 8 (Conversation API)
            │               └── Sprint 11 (Frontend: чат)
            │
            └── Sprint 14 (E2E тесты) ← зависит от всех
                    │
                    └── Sprint 15 (Auth + LLM providers + кастомизация агентов)
                         ← зависит от 5, 6, 13
```

**Параллельные потоки:**
- **Поток A (Backend Core):** 1 → 2 → 4 → 9
- **Поток B (Backend Tasks):** 1 → 3 → 6 → 7 → 8
- **Поток C (Sandbox):** 5 (параллельно, объединяется в Sprint 6)
- **Поток D (Frontend):** 10 → 11 → 12 → 13
- **Финал:** 14 → 15 (расширение sandbox/UI: OAuth Claude Code, мультипровайдерный LLM, MCP/Skills/permissions per agent)

---

## Текущий стек и инфраструктура

| Слой | Технологии |
|------|-----------|
| **Backend** | Go 1.24, Gin, GORM, Goose, JWT (HS256), Swagger, MCP Server |
| **Frontend** | Flutter 3.x, Riverpod 2.0, GoRouter, Dio, Freezed |
| **БД** | YugabyteDB (PostgreSQL-совместимая, порт 5433) |
| **Векторная БД** | Weaviate + sentence-transformers |
| **LLM** | Anthropic (API-ключ или OAuth-подписка Claude Code), OpenAI, Gemini, DeepSeek, Qwen, OpenRouter, Moonshot AI, Ollama, Zhipu AI. Не-Anthropic провайдеры в sandbox используют native Anthropic-совместимый endpoint (`api.deepseek.com/anthropic`, `open.bigmodel.cn/api/anthropic`, …), per-user ключи в `user_llm_credentials` — Sprint 15.e2e |
| **Sandbox** | Docker containers (Claude Code CLI, Aider) |
| **Инфраструктура** | Docker, Docker Compose, Makefile |

---

## Быстрый старт

```bash
# 0. Конфиг
cp backend/env.example backend/.env
#    Заполните ANTHROPIC_API_KEY и сгенерируйте ENCRYPTION_KEY:
#    `openssl rand -hex 32`

# 1. Запуск инфраструктуры
make build && make up

# 2. Подождать ~30 сек (инициализация YugabyteDB) и применить миграции
make migrate-up

# 3. Собрать sandbox-образ для агентов (developer/reviewer/tester)
make sandbox-build

# 4. Frontend
make frontend-setup
make frontend-run-web
```

| Сервис | URL |
|--------|-----|
| Backend API | `http://localhost:8080` |
| MCP-сервер | `http://localhost:8081/mcp` |
| Swagger UI | `http://localhost:8080/swagger/index.html` |
| YugabyteDB Admin | `http://localhost:15000` |
| Weaviate | `http://localhost:8082` |

### Docker Engine и sandbox

Файл `docker-compose.yml` монтирует **`/var/run/docker.sock`** в сервис **`app`** (режим **rw**, без `:ro`), чтобы `DockerSandboxRunner` мог вызывать Docker API с хоста, на котором выполняется `make up`. Путь на хосте тот же для **Docker Desktop** (macOS / Windows): сокет проксируется в VM, монтирование в compose не отличается.

**Безопасность:** доступ к сокету в dev даёт контейнеру **`app`** по сути полный контроль над Docker хоста. В production так не деплоят; нужны отдельные паттерны (TLS к удалённому демону, Kubernetes и т.п.).

**Linux:** если при обращении к API Docker в логах **`permission denied`** на сокет, задайте GID группы `docker` на хосте — compose подставляет его в **`group_add`** (процесс остаётся без жёсткого `user:`):

```bash
export DOCKER_GID=$(getent group docker | cut -d: -f3)   # один раз в сессии или в `.env` в корне репозитория
make up
```

Если **`DOCKER_HOST`** указывает на удалённый демон (`tcp://...`), клиент Docker в контейнере использует его в приоритете над смонтированным сокетом; для локального сокета не задавайте **`DOCKER_HOST`** в `backend/.env`.

Перед **`make test-integration`** (или другими проверками с реальным sandbox) на хосте должен существовать образ **`devteam/sandbox-claude:local`**. Если Docker пишет, что образ не найден, выполните **`make sandbox-build`** (или явно **`make sandbox-build-claude`**). Подробности — [5.12](docs/tasks/5.12-makefile-sandbox-build.md).

---

## Основные команды

```bash
make build / up / down / logs        # Инфраструктура
make migrate-up / down / status      # Миграции
make test / test-unit / test-integration  # Backend тесты
make validate-agent-prompts          # Проверка YAML промптов пайплайна (6.8) против prompt_schema.json
make swagger                         # Генерация Swagger
make sandbox-build                   # Сборка sandbox-образов
make frontend-setup                  # Первоначальная настройка Flutter
make frontend-codegen                # Кодогенерация (Riverpod, Freezed, l10n)
make frontend-run-web                # Запуск Flutter в Chrome
make frontend-test                   # Flutter тесты
make frontend-test-integration       # Flutter integration_test (полный UI flow, Sprint 14.2)
make e2e-smoke                       # Full-stack smoke на поднятом стеке (Sprint 14.7)
make help                            # Все команды
```

## Тестирование (Sprint 14)

| Уровень | Команда | Покрытие |
|---|---|---|
| Backend unit | `make test-unit` | сервисы, репозитории, pipeline без БД |
| Backend integration | `make test-integration` | + Yugabyte, + Docker sandbox (если есть образ `devteam/sandbox-claude:local`): orchestrator pipeline (14.1), sandbox real-push / isolation / cancel / 5-parallel (14.3-14.5) |
| Backend full-stack smoke | `GITHUB_PAT=ghp_xxx make e2e-smoke` | реальный POST `/tasks` → ожидаем `completed` → проверяем PR на GitHub (14.7) |
| Frontend unit/widget | `make frontend-test` | компоненты, контроллеры с моками |
| Frontend integration | `make frontend-test-integration` | полный UI flow на реальном backend: register → создание проекта → проект виден в списке (14.2) |

Подробности по тестовым артефактам — в `backend/API.md` (раздел «Sprint 14 — тестовые сценарии»).
Для full-stack smoke (`e2e-smoke`) в `backend/.env` должен быть валидный
`ANTHROPIC_API_KEY` и в env shell — `GITHUB_PAT` с правами `Contents: RW`,
`Pull requests: RW` на репозиторий из `--repo` (по умолчанию `tereshed/kt-test-repo`).

---

## Правила разработки

Детальные правила в `.cursor/rules/`:
- `main.mdc` — концепция DevTeam, domain model, архитектура агентов
- `backend.mdc` — Go/Gin, Clean Architecture, миграции, JWT, тесты
- `frontend.mdc` — Flutter, Riverpod, адаптивность, i18n, тесты
- `deploy.mdc` — Docker, Makefile, окружение
