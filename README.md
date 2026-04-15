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
| 5.10 | Конфигурация: `SandboxConfig` в `config.go` | `backend/internal/config/config.go` | ⬜ | [детали](docs/tasks/5.10-sandbox-config.md) |
| 5.11 | docker-compose: монтирование `/var/run/docker.sock` | `deployment/docker-compose.yaml` | ⬜ | |
| 5.12 | Makefile: `sandbox-build` (сборка sandbox-образов) | `Makefile` | ⬜ | |
| 5.13 | Unit-тесты: DockerSandboxRunner (с мок Docker Client) | `backend/internal/sandbox/docker_runner_test.go` | ⬜ | |
| 5.14 | Интеграционный тест: запуск реального контейнера с простой задачей | `backend/internal/sandbox/integration_test.go` | ⬜ | |

**Зависимости:** Sprint 1

---

### Sprint 6 — Orchestrator Agent (Backend)

**Цель:** Базовый оркестратор — принимает запрос от пользователя, создаёт задачи, управляет pipeline.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 6.1 | Интерфейс `AgentExecutor` (запуск агента с задачей) | `backend/internal/agent/executor.go` | ⬜ |
| 6.2 | Реализация: `LLMAgentExecutor` (вызов LLM с промптом + tools) | `backend/internal/agent/llm_executor.go` | ⬜ |
| 6.3 | Реализация: `SandboxAgentExecutor` (запуск sandbox-контейнера для Developer) | `backend/internal/agent/sandbox_executor.go` | ⬜ |
| 6.4 | Orchestrator: `OrchestratorService` — основной цикл управления | `backend/internal/service/orchestrator_service.go` | ⬜ |
| 6.5 | Pipeline: линейный поток `Plan → Develop → Review → Test` | В `OrchestratorService` | ⬜ |
| 6.6 | Обработка результатов: `completed` → следующий шаг, `changes_requested` → назад к Developer | В `OrchestratorService` | ⬜ |
| 6.7 | Обработка пользовательских команд: `pause`, `cancel`, `resume`, `correct` | В `OrchestratorService` | ⬜ |
| 6.8 | Промпты агентов: Orchestrator, Planner, Developer, Reviewer, Tester | `backend/prompts/orchestrator.yaml`, `planner.yaml`, `developer.yaml`, `reviewer.yaml`, `tester.yaml` | ⬜ |
| 6.9 | Агенты по умолчанию (YAML-конфиг) | `backend/agents/orchestrator.yaml`, `planner.yaml`, `developer.yaml`, `reviewer.yaml`, `tester.yaml` | ⬜ |
| 6.10 | Unit-тесты: OrchestratorService (полный pipeline, ретраи, отмена) | `backend/internal/service/orchestrator_service_test.go` | ⬜ |

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
| 7.1 | WebSocket Hub: менеджер подключений (по project_id) | `backend/internal/ws/hub.go` | ⬜ |
| 7.2 | WebSocket Handler: upgrade HTTP → WS, аутентификация через JWT | `backend/internal/ws/handler.go` | ⬜ |
| 7.3 | Типы сообщений: `task_status`, `task_message`, `agent_log`, `error` | `backend/internal/ws/types.go` | ⬜ |
| 7.4 | Event Bus: внутренний pub/sub для уведомлений между сервисами | `backend/internal/ws/eventbus.go` | ⬜ |
| 7.5 | Интеграция: TaskService → EventBus при изменении статуса | `backend/internal/service/task_service.go` | ⬜ |
| 7.6 | Интеграция: SandboxRunner → EventBus для стриминга логов | `backend/internal/sandbox/docker_runner.go` | ⬜ |
| 7.7 | Роут: `GET /api/v1/projects/:id/ws` (WebSocket) | `backend/internal/server/server.go` | ⬜ |
| 7.8 | Unit-тесты: WebSocket Hub | `backend/internal/ws/hub_test.go` | ⬜ |

**Зависимости:** Sprint 3, Sprint 6

---

### Sprint 8 — Conversation API (Backend)

**Цель:** API для чатов пользователя с оркестратором.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 8.1 | Repository: `ConversationRepository` | `backend/internal/repository/conversation_repository.go` | ⬜ |
| 8.2 | Repository: `ConversationMessageRepository` | `backend/internal/repository/conversation_message_repository.go` | ⬜ |
| 8.3 | Service: `ConversationService` (создание, отправка сообщения → Orchestrator, получение истории) | `backend/internal/service/conversation_service.go` | ⬜ |
| 8.4 | Handler: `ConversationHandler` | `backend/internal/handler/conversation_handler.go` | ⬜ |
| 8.5 | Связка: `ConversationService` → `OrchestratorService` (новое сообщение запускает обработку) | Интеграция | ⬜ |
| 8.6 | Swagger-аннотации | В handler | ⬜ |
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
| 9.1 | Коллекция Weaviate per project: `DevTeam_Project_{id}` | Конфигурация в `vectordb` | ⬜ |
| 9.2 | Индексатор кода: чанкинг файлов (по файлам для малых, по функциям для больших) | `backend/internal/indexer/code_indexer.go` | ⬜ |
| 9.3 | Индексатор задач: описание + результат + сообщения | `backend/internal/indexer/task_indexer.go` | ⬜ |
| 9.4 | Индексатор чатов: сообщения пользователя и ассистента | `backend/internal/indexer/conversation_indexer.go` | ⬜ |
| 9.5 | Service: `IndexerService` (полная индексация проекта, инкрементальное обновление) | `backend/internal/service/indexer_service.go` | ⬜ |
| 9.6 | Хук: индексация кода при создании проекта (после clone) | В `ProjectService` | ⬜ |
| 9.7 | Хук: индексация задачи при создании/обновлении | В `TaskService` | ⬜ |
| 9.8 | Хук: индексация сообщения при создании | В `ConversationService` | ⬜ |
| 9.9 | API: `POST /api/v1/projects/:id/reindex` (полная переиндексация) | Handler + Service | ⬜ |
| 9.10 | Контекстный поиск: `SearchContext(projectID, query)` → релевантные чанки | В `IndexerService` | ⬜ |
| 9.11 | Интеграция с Orchestrator: перед запуском агента — vector search для контекста | В `OrchestratorService` | ⬜ |
| 9.12 | Unit-тесты | `backend/internal/service/indexer_service_test.go` | ⬜ |

**Зависимости:** Sprint 2, Sprint 3, Sprint 8

---

### Sprint 10 — Frontend: Проекты и навигация (Flutter)

**Цель:** Базовый UI — список проектов, создание проекта, навигация.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 10.1 | Модели (Freezed): `ProjectModel`, `TeamModel`, `AgentModel` | `frontend/lib/features/projects/domain/` | ⬜ |
| 10.2 | Repository: `ProjectRepository` (Dio → backend API) | `frontend/lib/features/projects/data/project_repository.dart` | ⬜ |
| 10.3 | Providers: `projectListProvider`, `projectProvider` | `frontend/lib/features/projects/data/project_providers.dart` | ⬜ |
| 10.4 | Экран: Список проектов (карточки, статусы, поиск) | `frontend/lib/features/projects/presentation/screens/projects_list_screen.dart` | ⬜ |
| 10.5 | Экран: Создание проекта (форма: имя, описание, git URL, провайдер) | `frontend/lib/features/projects/presentation/screens/create_project_screen.dart` | ⬜ |
| 10.6 | Экран: Дашборд проекта (hub → чат, задачи, команда, настройки) | `frontend/lib/features/projects/presentation/screens/project_dashboard_screen.dart` | ⬜ |
| 10.7 | Обновить GoRouter: новые роуты `/projects`, `/projects/:id`, `/projects/:id/*` | `frontend/lib/core/routing/app_router.dart` | ⬜ |
| 10.8 | Локализация: новые строки (ru, en) | `frontend/lib/l10n/app_ru.arb`, `app_en.arb` | ⬜ |
| 10.9 | Widget-тесты: ProjectCard, CreateProjectForm | `frontend/test/features/projects/` | ⬜ |

**Зависимости:** Sprint 2

---

### Sprint 11 — Frontend: Чат (Flutter)

**Цель:** Основной интерфейс — чат с оркестратором, live-обновления.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 11.1 | Модели (Freezed): `ConversationModel`, `ConversationMessageModel` | `frontend/lib/features/chat/domain/` | ⬜ |
| 11.2 | WebSocket Service: подключение, реконнект, парсинг событий | `frontend/lib/core/api/websocket_service.dart` | ⬜ |
| 11.3 | Repository: `ConversationRepository` | `frontend/lib/features/chat/data/conversation_repository.dart` | ⬜ |
| 11.4 | Controller: `ChatController` (AsyncNotifier — загрузка, отправка, стрим) | `frontend/lib/features/chat/presentation/controllers/chat_controller.dart` | ⬜ |
| 11.5 | Экран: Чат (список сообщений, поле ввода, отправка) | `frontend/lib/features/chat/presentation/screens/chat_screen.dart` | ⬜ |
| 11.6 | Виджет: `ChatMessage` (user/assistant/system, markdown, код, стримящийся текст) | `frontend/lib/features/chat/presentation/widgets/chat_message.dart` | ⬜ |
| 11.7 | Виджет: `TaskStatusCard` (встроенная карточка статуса задачи в чате) | `frontend/lib/features/chat/presentation/widgets/task_status_card.dart` | ⬜ |
| 11.8 | Виджет: `ChatInput` (текстовое поле + кнопки: отправить, стоп, attach) | `frontend/lib/features/chat/presentation/widgets/chat_input.dart` | ⬜ |
| 11.9 | Реалтайм: подписка на WebSocket → обновление UI при новых сообщениях/статусах | В `ChatController` | ⬜ |
| 11.10 | Локализация | `.arb` файлы | ⬜ |
| 11.11 | Widget-тесты | `frontend/test/features/chat/` | ⬜ |

**Зависимости:** Sprint 7, Sprint 8, Sprint 10

---

### Sprint 12 — Frontend: Задачи (Flutter)

**Цель:** UI для просмотра задач, их статусов, деталей и логов агентов.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 12.1 | Модели (Freezed): `TaskModel`, `TaskMessageModel` | `frontend/lib/features/tasks/domain/` | ⬜ |
| 12.2 | Repository: `TaskRepository` | `frontend/lib/features/tasks/data/task_repository.dart` | ⬜ |
| 12.3 | Controller: `TaskListController`, `TaskDetailController` | `frontend/lib/features/tasks/presentation/controllers/` | ⬜ |
| 12.4 | Экран: Список задач (Kanban-доска по статусам или таблица) | `frontend/lib/features/tasks/presentation/screens/tasks_list_screen.dart` | ⬜ |
| 12.5 | Экран: Детали задачи (статус, описание, лог агентов, diff, результат) | `frontend/lib/features/tasks/presentation/screens/task_detail_screen.dart` | ⬜ |
| 12.6 | Виджет: `TaskCard` (статус, приоритет, агент, время) | `frontend/lib/features/tasks/presentation/widgets/task_card.dart` | ⬜ |
| 12.7 | Виджет: `DiffViewer` (отображение git diff с подсветкой) | `frontend/lib/shared/widgets/diff_viewer.dart` | ⬜ |
| 12.8 | Действия: кнопки Pause/Cancel/Resume на задаче | В `TaskDetailScreen` | ⬜ |
| 12.9 | Реалтайм: обновление статусов задач через WebSocket | В controllers | ⬜ |
| 12.10 | Widget-тесты | `frontend/test/features/tasks/` | ⬜ |

**Зависимости:** Sprint 3, Sprint 10, Sprint 11

---

### Sprint 13 — Frontend: Настройки команды и проекта (Flutter)

**Цель:** UI для управления агентами команды и настройками проекта.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 13.1 | Экран: Команда проекта (список агентов, их роли, модели, статус) | `frontend/lib/features/team/presentation/screens/team_screen.dart` | ⬜ |
| 13.2 | Виджет: `AgentCard` (роль, модель, code_backend, on/off) | `frontend/lib/features/team/presentation/widgets/agent_card.dart` | ⬜ |
| 13.3 | Диалог: редактирование агента (модель, промпт, tools) | `frontend/lib/features/team/presentation/widgets/agent_edit_dialog.dart` | ⬜ |
| 13.4 | Экран: Настройки проекта (git credentials, tech stack, vector index) | `frontend/lib/features/projects/presentation/screens/project_settings_screen.dart` | ⬜ |
| 13.5 | Экран: Глобальные настройки (API keys для LLM-провайдеров) | `frontend/lib/features/settings/presentation/screens/settings_screen.dart` | ⬜ |
| 13.6 | Локализация | `.arb` файлы | ⬜ |
| 13.7 | Widget-тесты | `frontend/test/features/team/` | ⬜ |

**Зависимости:** Sprint 2, Sprint 10

---

### Sprint 14 — E2E интеграция и тестирование

**Цель:** Полный сквозной тест: от сообщения в чате до кода в репозитории.

| # | Задача | Файлы | Статус |
|---|--------|-------|--------|
| 14.1 | E2E тест (backend): создать проект → отправить запрос → Orchestrator создаёт задачи → Developer выполняет → Reviewer одобряет → Completed | `backend/internal/service/orchestrator_integration_test.go` | ⬜ |
| 14.2 | E2E тест (frontend): интеграционный тест полного flow в UI | `frontend/integration_test/full_flow_test.dart` | ⬜ |
| 14.3 | Нагрузочное тестирование: 5 параллельных sandbox-контейнеров | Скрипт | ⬜ |
| 14.4 | Тест безопасности: sandbox не может выйти за пределы `/workspace` | Тест | ⬜ |
| 14.5 | Тест отмены: пользователь нажимает Cancel → контейнер убивается | Тест | ⬜ |
| 14.6 | Документация: обновить README, API.md, env.example | Корень | ⬜ |

**Зависимости:** Все предыдущие спринты

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
```

**Параллельные потоки:**
- **Поток A (Backend Core):** 1 → 2 → 4 → 9
- **Поток B (Backend Tasks):** 1 → 3 → 6 → 7 → 8
- **Поток C (Sandbox):** 5 (параллельно, объединяется в Sprint 6)
- **Поток D (Frontend):** 10 → 11 → 12 → 13
- **Финал:** 14

---

## Текущий стек и инфраструктура

| Слой | Технологии |
|------|-----------|
| **Backend** | Go 1.24, Gin, GORM, Goose, JWT (HS256), Swagger, MCP Server |
| **Frontend** | Flutter 3.x, Riverpod 2.0, GoRouter, Dio, Freezed |
| **БД** | YugabyteDB (PostgreSQL-совместимая, порт 5433) |
| **Векторная БД** | Weaviate + sentence-transformers |
| **LLM** | OpenAI, Anthropic, Gemini, Deepseek, Qwen |
| **Sandbox** | Docker containers (Claude Code CLI, Aider) |
| **Инфраструктура** | Docker, Docker Compose, Makefile |

---

## Быстрый старт

```bash
# 1. Запуск инфраструктуры
make build && make up

# 2. Подождать ~30 сек (инициализация YugabyteDB)

# 3. Миграции
make migrate-up

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

---

## Основные команды

```bash
make build / up / down / logs        # Инфраструктура
make migrate-up / down / status      # Миграции
make test / test-unit / test-integration  # Backend тесты
make swagger                         # Генерация Swagger
make sandbox-build                   # Сборка sandbox-образов
make frontend-setup                  # Первоначальная настройка Flutter
make frontend-codegen                # Кодогенерация (Riverpod, Freezed, l10n)
make frontend-run-web                # Запуск Flutter в Chrome
make frontend-test                   # Flutter тесты
make help                            # Все команды
```

---

## Правила разработки

Детальные правила в `.cursor/rules/`:
- `main.mdc` — концепция DevTeam, domain model, архитектура агентов
- `backend.mdc` — Go/Gin, Clean Architecture, миграции, JWT, тесты
- `frontend.mdc` — Flutter, Riverpod, адаптивность, i18n, тесты
- `deploy.mdc` — Docker, Makefile, окружение
