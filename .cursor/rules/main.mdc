---
alwaysApply: true
---

# DevTeam — AI Agent Orchestrator для разработки приложений

Ты — ведущий архитектор и разработчик платформы **DevTeam**. Это приложение-оркестратор AI-агентов, которое автоматизирует полный цикл разработки ПО: от идеи до работающего кода.

Твой стек: **Go (Gin)**, **Flutter**, **YugabyteDB**, **Weaviate**, **Claude Code CLI / Aider**.

-----

## 1. Концепция продукта

**DevTeam** — это платформа, где пользователь описывает идею в чате, а команда AI-агентов реализует её: планирует, пишет код, ревьюит, тестирует и деплоит.

**Ключевые принципы:**
- Пользователь общается через **чат** — единая точка входа
- Агенты взаимодействуют между собой через **задачи (Tasks)**
- Весь контекст (код, задачи, чаты) индексируется в **векторной БД** для быстрого доступа
- Пользователь может **остановить** выполнение и **скорректировать** направление в любой момент

-----

## 2. Основные сущности (Domain Model)

### 2.1. Project (Проект) — центральная сущность

Проект — это привязка к конкретному репозиторию + команда агентов + весь контекст.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | Уникальный идентификатор |
| `name` | string | Название проекта |
| `description` | text | Описание проекта и его целей |
| `git_provider` | enum | `github` / `gitlab` / `bitbucket` / `local` |
| `git_url` | string | URL репозитория (для remote) |
| `git_default_branch` | string | Основная ветка (main/master) |
| `git_credentials_id` | UUID | Ссылка на зашифрованные credentials |
| `vector_collection` | string | Имя коллекции в Weaviate для этого проекта |
| `tech_stack` | jsonb | Языки, фреймворки, инструменты проекта |
| `status` | enum | `active` / `paused` / `archived` |
| `settings` | jsonb | Настройки проекта (code style, conventions) |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

**Сценарии создания проекта:**
1. **Из существующего репозитория** — импорт из GitHub/GitLab/Bitbucket, индексация кода в векторку
2. **Новый локальный** — создание структуры, инициализация git, опциональный push в remote
3. **Новый в системе** — создание репозитория через API GitHub/GitLab/Bitbucket

### 2.2. Team (Команда)

Команда — набор агентов, назначенных на проект. Пока поддерживаем один тип: **команда разработки**.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `name` | string | Название команды |
| `project_id` | UUID | FK на Project |
| `type` | enum | `development` (пока единственный) |
| `created_at` | timestamp | |

### 2.3. Agent (AI-агент)

Агент — это AI-сущность с определённой ролью, моделью и набором инструментов.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `name` | string | Имя агента |
| `role` | enum (`chk_agents_role`) | `router` / `planner` / `decomposer` / `developer` / `reviewer` / `tester` / `merger` / `devops` / `orchestrator` / `worker` / `supervisor`. Канон Orchestration v2 — первые 7; `orchestrator/worker/supervisor` остаются для совместимости со старыми сидами. Расширение enum'а — через goose-миграцию `chk_agents_role`. |
| `team_id` | UUID | FK на Team (nullable) |
| `model` | string | LLM модель (`claude-sonnet-4-20250514`, `gpt-4o`, `gemini-2.5-pro`) |
| `prompt_id` | UUID | FK на Prompt (системный промпт) |
| `skills` | jsonb | Теги навыков `["golang", "flutter", "testing"]` |
| `code_backend` | enum | `claude-code` / `aider` / `custom` / NULL (для роли developer) |
| `settings` | jsonb | Доп. настройки (temperature, max_tokens и т.д.) |
| `is_active` | bool | |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

**Связи через binding-таблицы:**
- **Инструменты** — `agent_tool_bindings` (M:N с `tool_definitions`)
- **MCP-серверы** — `agent_mcp_bindings` (M:N с `mcp_server_configs`)

### 2.3.1. ToolDefinition (Реестр инструментов)

Централизованный реестр всех доступных инструментов. Встроенные загружаются из YAML при старте.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `name` | string | UNIQUE идентификатор (`create_task`, `vector_search`) |
| `description` | text | Описание для LLM (передаётся в function calling) |
| `category` | string | Группировка: `task_management`, `code`, `search`, `review` |
| `parameters_schema` | jsonb | JSON Schema параметров |
| `is_builtin` | bool | true = встроенный (из YAML), false = пользовательский |
| `is_active` | bool | |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

### 2.3.2. AgentToolBinding (Привязка инструментов к агенту)

| Поле | Тип | Описание |
|:---|:---|:---|
| `agent_id` | UUID | FK на Agent — composite PK |
| `tool_definition_id` | UUID | FK на ToolDefinition — composite PK |
| `config` | jsonb | Переопределение настроек для этого агента |
| `created_at` | timestamp | |

### 2.3.3. MCPServerConfig (Конфигурация MCP-серверов)

Конфигурации MCP-серверов, привязанные к проекту. Credentials хранятся зашифрованными (AES-256-GCM).

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `project_id` | UUID | FK на Project |
| `name` | string | Имя для UI ("GitHub MCP", "PostgreSQL MCP") |
| `url` | string | URL MCP-сервера |
| `auth_type` | enum | `none` / `api_key` / `oauth` / `bearer` |
| `encrypted_credentials` | bytea | Зашифрованные auth данные |
| `settings` | jsonb | Доп. настройки (headers, timeout) |
| `is_active` | bool | |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

UNIQUE constraint: `(project_id, name)`

### 2.3.4. AgentMCPBinding (Привязка MCP-серверов к агенту)

| Поле | Тип | Описание |
|:---|:---|:---|
| `agent_id` | UUID | FK на Agent — composite PK |
| `mcp_server_config_id` | UUID | FK на MCPServerConfig — composite PK |
| `settings` | jsonb | Переопределение настроек для этого агента |
| `created_at` | timestamp | |

### 2.4. Task (Задача) — единица работы между агентами

Задачи — основной механизм взаимодействия агентов. Оркестратор создаёт задачи, назначает их агентам, контролирует статус.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `project_id` | UUID | FK на Project |
| `parent_task_id` | UUID | FK на Task (для подзадач) |
| `title` | string | Краткое название |
| `description` | text | Подробное описание |
| `status` | enum | см. ниже |
| `priority` | enum | `critical` / `high` / `medium` / `low` |
| `assigned_agent_id` | UUID | Какому агенту назначена |
| `created_by_type` | enum | `user` / `agent` |
| `created_by_id` | UUID | ID пользователя или агента-создателя |
| `context` | jsonb | Релевантный контекст из векторки |
| `result` | text | Результат выполнения |
| `artifacts` | jsonb | Артефакты (diff, test results, PR URL) |
| `branch_name` | string | Git-ветка для этой задачи |
| `error_message` | text | Описание ошибки (если failed) |
| `started_at` | timestamp | |
| `completed_at` | timestamp | |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

**Состояния задачи (`tasks.state`, Orchestration v2):**

С Sprint 5 (Orchestration v2) пайплайн больше не зашит в Go state-machine —
маршрутизацией управляет LLM-агент `router` (см. §2.3 / §3.1). Поэтому на уровне
`tasks` хранятся **только 5 высокоуровневых терминальных/жизненных состояний**,
а реальный "этап" восстанавливается по `task_events` / `router_decisions` /
`task_artifacts`.

```
              ┌─────────────► done
              │
   active ────┼─────────────► failed
              │
              ├─────────────► cancelled       (отменена пользователем)
              │
              └─────────────► needs_human     (требует ручного решения)
```

| Состояние | Описание |
|:---|:---|
| `active` | Задача жива: router-цикл планирует/делает/ревьюит/тестит, есть step'ы или agent_jobs in-flight |
| `done` | Терминальное успешное состояние (reviewer approve + merger завершён) |
| `failed` | Терминальная ошибка (исчерпан `router_retry_budget`, не-восстановимая ошибка агента или sandbox) |
| `cancelled` | Пользователь отменил (через `task_cancel_v2`) |
| `needs_human` | Router не смог принять решение (галлюцинации, исчерпан retry-бюджет, конфликт артефактов) — требуется вмешательство оператора |

**Прогресс по фазам** (что раньше было pending/planning/in_progress/review/testing)
в v2 **не** хранится колонкой `tasks.status`. Чтобы понять, "где сейчас задача",
нужно смотреть:
  * последний `router_decisions.chosen_agents` — кому ушёл следующий шаг,
  * `task_artifacts` (по `kind`: `plan`, `code_diff`, `review`, `test_report`, …) —
    что уже произведено,
  * `task_events` — pending step_req / agent_job.

> **Важно (миграция):** Legacy `TaskStatus` (10 значений pipeline:
> `pending|planning|in_progress|review|changes_requested|testing|completed|
> failed|cancelled|paused`) **удалён в Sprint 5**. Если встречаете его в коде,
> промптах агентов или фронтовых моделях — это баг, требующий миграции
> на `tasks.state`. См. `docs/orchestration-v2-plan.md`.

### 2.5. TaskMessage (Сообщения в контексте задачи)

Лог коммуникации между агентами по конкретной задаче.

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `task_id` | UUID | FK на Task |
| `sender_type` | enum | `user` / `agent` |
| `sender_id` | UUID | ID отправителя |
| `content` | text | Содержимое сообщения |
| `message_type` | enum | `instruction` / `result` / `question` / `feedback` / `error` |
| `metadata` | jsonb | Доп. данные (tokens_used, model, duration_ms) |
| `created_at` | timestamp | |

### 2.6. Conversation (Чат с пользователем)

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `project_id` | UUID | FK на Project |
| `user_id` | UUID | FK на User |
| `title` | string | Автогенерируемое название |
| `status` | enum | `active` / `completed` / `archived` |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

### 2.7. ConversationMessage (Сообщения чата)

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `conversation_id` | UUID | FK на Conversation |
| `role` | enum | `user` / `assistant` / `system` |
| `content` | text | Текст сообщения |
| `linked_task_ids` | uuid[] | Задачи, созданные из этого сообщения |
| `metadata` | jsonb | |
| `created_at` | timestamp | |

### 2.8. GitCredential (Учётные данные Git)

| Поле | Тип | Описание |
|:---|:---|:---|
| `id` | UUID | |
| `user_id` | UUID | FK на User |
| `provider` | enum | `github` / `gitlab` / `bitbucket` |
| `auth_type` | enum | `token` / `ssh_key` / `oauth` |
| `encrypted_value` | bytea | Зашифрованные credentials |
| `label` | string | Пользовательская метка |
| `created_at` | timestamp | |

-----

## 3. Система AI-агентов

### 3.1. Роли агентов (канон Orchestration v2)

| Роль | Назначение | Входные данные | Выходные данные |
|:---|:---|:---|:---|
| **Router** *(v2, ядро)* | LLM-диспатчер. После каждого шага решает, кому передать управление дальше (один или несколько агентов в `chosen_agents`). **Заменяет hardcoded state-machine оркестратора** — flow=данные. | Текущее состояние задачи: `task_artifacts` (metadata), DAG подзадач, in-flight `agent_jobs`, последние `router_decisions`. | Запись в `router_decisions` с `chosen_agents[]` + `outcome` + `reason`, ставит `step_req`/`agent_job` события в `task_events`. |
| **Planner** | Анализирует задачу высокого уровня, формирует план шагов | Описание задачи + контекст из векторки | Артефакт `kind=plan` (список шагов + зависимости) |
| **Decomposer** *(v2)* | Разбивает план на параллелизуемые подзадачи (узлы DAG), назначает им `depends_on` | Артефакт `plan` | Артефакты `kind=subtask` с DAG-связями |
| **Developer** | Пишет код в worktree-изоляции, коммитит | Подзадача + контекст + кодовая база (worktree) | Артефакт `kind=code_diff` + commit hash + branch name |
| **Reviewer** *(gate)* | **Единственная инстанция, выносящая вердикт** по любому артефакту (plan / code_diff / test_report). `approve` → router двигает дальше; `request_changes` → router возвращает к автору. | Артефакт + контекст + код проекта | Артефакт `kind=review` с `outcome ∈ {approve, request_changes}` + комментарии |
| **Tester** | Пишет и запускает тесты в sandbox | Code_diff + задача + тест-план | Артефакт `kind=test_report` (pass/fail + логи) |
| **Merger** *(v2)* | Сливает approved-ветки подзадач в основную ветку задачи (или открывает PR в upstream) | Approved `code_diff` артефакты + branch names | Финальный commit/merge, артефакт `kind=merge_result` |
| **DevOps** *(будущее)* | CI/CD, деплой, инфраструктура | Код + конфиг | Деплой статус, URL |

> **Про legacy `orchestrator`-роль.** В v2 функция "решить, кому передать следующий
> шаг" вынесена из Go-кода `orchestrator_v2.go` в LLM-агента `router`. Сам
> `Orchestrator.Step()` в Go остаётся как **транспортный** слой (lock per-task,
> tx, enqueue events) — но он **не решает**, какой агент пойдёт следующим. Если
> в старых сидах/промптах вы видите role=`orchestrator` — это либо унаследованная
> запись (совместимость enum'а), либо место, которое нужно мигрировать на `router`.

### 3.2. Поток выполнения (Agent Pipeline v2)

В Orchestration v2 пайплайн **не линейный, а циклический**. В центре —
LLM-агент **Router**, который после каждого шага анализирует артефакты
(`task_artifacts`, DAG подзадач, in-flight `agent_jobs`) и решает, кому
передать управление дальше. Никаких hardcoded `if phase == ...` в Go —
переходы между фазами кодируются в промпте router'а и реестре `agents`.

```text
Пользователь (чат)
       │
       ▼
┌──────────────┐
│    Router    │ ◄──────────────────────────────────┐
└──────┬───────┘   (новый шаг после каждого         │
       │            артефакта / approve / reject)   │
       │ chosen_agents[]                            │
       ▼                                            │
┌──────────────────────────────┐                    │
│  Planner / Decomposer /      │                    │
│  Developer / Tester / Merger │ ──(создают         │
│  (исполнители-продьюсеры)    │    артефакты)──►  │
└──────────────┬───────────────┘                    │
               │ kind=plan / code_diff /            │
               │ test_report / merge_result         │
               ▼                                    │
        ┌──────────────┐                            │
        │   Reviewer   │ ─── approve ───────────────┤
        │    (gate)    │                            │
        └──────┬───────┘                            │
               │                                    │
               └── request_changes ─────────────────┘
                  (router возвращает к автору)

   терминал: tasks.state ∈ {done, failed, cancelled, needs_human}
```

**Как читать схему:**
  * **Router** — единственная точка маршрутизации, вызывается на каждом шаге
    `Orchestrator.Step()`. Может выбрать **несколько** агентов сразу
    (`chosen_agents[]`) — например, параллельно запустить нескольких Developer'ов
    по узлам DAG.
  * **Продьюсеры** (Planner/Decomposer/Developer/Tester/Merger) производят
    артефакты в `task_artifacts` и не решают, что будет дальше — это работа
    router'а на следующем шаге.
  * **Reviewer — единственный gate.** Любой артефакт-продьюсер проходит через
    Reviewer; результат (`approve` / `request_changes`) router использует,
    чтобы продвинуться вперёд или вернуться к автору.
  * **Цикл завершается**, когда router выставляет терминальное состояние
    `tasks.state` (`done` / `failed` / `needs_human`) или пользователь отменяет
    задачу (`cancelled`, через `task_cancel_v2`).

### 3.3. Характеристики агента

Каждый агент определяется через:

1. **Модель (Model)** — какая LLM используется (claude-sonnet-4-20250514, gpt-4o, gemini-2.5-pro, deepseek-v3 и т.д.)
2. **Промпт (Prompt)** — системный промпт, определяющий поведение и экспертизу
3. **Инструменты (Tools)** — связь через `agent_tool_bindings` → `tool_definitions`:
   - Реестр инструментов (`tool_definitions`) с JSON Schema параметров
   - Встроенные загружаются из YAML при старте, пользовательские — через API
   - Категории: `task_management`, `code`, `search`, `review`, `git`, `notify`
   - Каждый агент получает свой набор через binding-таблицу
4. **Скиллы (Skills)** — теги навыков (JSONB в agents):
   - `golang`, `flutter`, `react`, `python` — знание стека
   - `testing`, `security_review`, `performance` — области экспертизы
   - `docker`, `kubernetes`, `ci_cd` — DevOps навыки
5. **MCP-серверы** — связь через `agent_mcp_bindings` → `mcp_server_configs`:
   - Конфигурации привязаны к проекту (project_id)
   - Credentials зашифрованы (AES-256-GCM)
   - Поддержка auth: `none`, `api_key`, `oauth`, `bearer`
   - Каждый агент получает свой набор MCP через binding-таблицу

### 3.4. Механизм управления пользователем

Пользователь **ВСЕГДА** имеет приоритет над агентами:

- **Пауза** — остановить выполнение текущей задачи (статус → `paused`)
- **Коррекция** — отправить сообщение агенту с уточнением / изменением направления
- **Отмена** — полностью отменить задачу (статус → `cancelled`)
- **Переназначение** — сменить агента или модель на задаче
- **Ручное вмешательство** — пользователь может сам внести изменения в код, агент продолжит с учётом этих изменений

-----

## 4. Инструменты для написания кода (Code Backends)

### 4.1. Принцип: Полная изоляция в Docker-контейнерах

**Каждая задача выполняется в отдельном Docker-контейнере.** Это критически важно:
- Задачи не конфликтуют друг с другом (файловая система, зависимости, порты)
- Можно параллельно выполнять десятки задач
- Контейнер можно убить в любой момент (пользователь нажал "Стоп")
- Безопасность: агент не имеет доступа к хост-системе и другим проектам
- Воспроизводимость: одинаковое окружение для каждой задачи

### 4.2. Архитектура Sandbox Runner

Go-бэкенд управляет контейнерами через **Docker API** (не CLI):

```go
type SandboxRunner interface {
    // Создать и запустить изолированный контейнер для задачи
    RunTask(ctx context.Context, opts SandboxOptions) (*SandboxInstance, error)
    // Получить статус и логи контейнера
    GetStatus(ctx context.Context, sandboxID string) (*SandboxStatus, error)
    // Стримить логи в реальном времени (для WebSocket → фронтенд)
    StreamLogs(ctx context.Context, sandboxID string) (<-chan LogEntry, error)
    // Остановить контейнер (пользователь отменил задачу)
    Stop(ctx context.Context, sandboxID string) error
    // Удалить контейнер и очистить ресурсы
    Cleanup(ctx context.Context, sandboxID string) error
}

type SandboxOptions struct {
    TaskID        string
    ProjectID     string
    Backend       CodeBackendType       // claude-code | aider | custom
    Image         string                // Docker-образ (devteam/sandbox-claude, devteam/sandbox-aider)
    RepoURL       string                // URL репозитория для клонирования
    Branch        string                // Ветка для работы
    Instruction   string                // Задание для агента
    Context       string                // Контекст из векторки
    EnvVars       map[string]string     // API ключи, настройки
    Timeout       time.Duration         // Максимальное время выполнения
    ResourceLimit ResourceLimit         // CPU, Memory лимиты
}

type SandboxInstance struct {
    ID            string                // Docker container ID
    TaskID        string
    Status        SandboxStatusType     // creating | running | completed | failed | stopped
    CreatedAt     time.Time
}

type SandboxStatus struct {
    ID            string
    Status        SandboxStatusType
    ExitCode      int
    Logs          string                // Последние N строк
    Result        *CodeResult           // Заполняется после завершения
    RunningFor    time.Duration
}

type CodeResult struct {
    Success       bool
    FilesChanged  []string
    Diff          string                // Unified diff всех изменений
    CommitHash    string
    Output        string                // Вывод агента
    TokensUsed    int
    DurationMs    int
    BranchName    string                // Имя ветки с изменениями
}

type ResourceLimit struct {
    CPUs          float64               // Количество CPU (например, 2.0)
    MemoryMB      int                   // Лимит RAM в MB (например, 4096)
    DiskMB        int                   // Лимит диска в MB
}
```

### 4.3. Docker-образы для Sandbox

Предсобранные образы для каждого Code Backend:

**`devteam/sandbox-claude`** — основной образ:
```dockerfile
FROM node:20-slim

RUN apt-get update && apt-get install -y git curl && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code

# Entrypoint-скрипт: клонирует репо, создаёт ветку, запускает claude, собирает результат
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
```

**`devteam/sandbox-aider`** — альтернативный образ:
```dockerfile
FROM python:3.12-slim

RUN apt-get update && apt-get install -y git && rm -rf /var/lib/apt/lists/*
RUN pip install aider-chat

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
```

**Entrypoint-скрипт** (общая логика):
```bash
#!/bin/bash
set -e

# 1. Клонировать репозиторий
git clone --depth=50 "$REPO_URL" /workspace/repo
cd /workspace/repo

# 2. Создать рабочую ветку
git checkout -b "$BRANCH_NAME"

# 3. Записать контекст в файл (для передачи агенту)
echo "$TASK_CONTEXT" > /tmp/context.md

# 4. Запустить Code Backend
if [ "$BACKEND" = "claude-code" ]; then
    claude -p "$TASK_INSTRUCTION" \
        --output-format json \
        --max-turns "${MAX_TURNS:-30}" \
        > /workspace/result.json 2>/workspace/agent.log
elif [ "$BACKEND" = "aider" ]; then
    aider --message "$TASK_INSTRUCTION" \
        --model "${MODEL:-claude-sonnet-4-20250514}" \
        --yes-always --no-stream --auto-commits \
        > /workspace/agent.log 2>&1
fi

# 5. Собрать результат (diff, changed files, commit hash)
git diff origin/main --stat > /workspace/changes.txt
git diff origin/main > /workspace/full.diff
git log origin/main..HEAD --oneline > /workspace/commits.txt

# 6. Push (если настроено)
if [ "$AUTO_PUSH" = "true" ]; then
    git push origin "$BRANCH_NAME"
fi

echo '{"status": "completed"}' > /workspace/status.json
```

### 4.4. Жизненный цикл контейнера

```
Go Backend (Orchestrator)
    │
    │  1. docker.ContainerCreate(image, env, volumes, limits)
    │  2. docker.ContainerStart(containerID)
    ▼
┌──────────────────────────────────────┐
│  Docker Container (sandbox)          │
│                                      │
│  /workspace/                         │
│  ├── repo/          ← git clone      │
│  │   └── (код проекта)               │
│  ├── result.json    ← результат      │
│  ├── full.diff      ← изменения      │
│  ├── agent.log      ← логи агента    │
│  └── status.json    ← финальный статус│
│                                      │
│  Процесс:                            │
│  clone → branch → agent work → diff  │
└──────────────────────────────────────┘
    │
    │  3. docker.ContainerLogs() → стрим в WebSocket
    │  4. docker.ContainerWait() → забрать результат
    │  5. docker.CopyFromContainer(/workspace/result.json, /workspace/full.diff)
    │  6. docker.ContainerRemove() → очистка
    ▼
Go Backend → сохранить результат в БД → уведомить фронтенд
```

### 4.5. Параллельное выполнение и очередь

```
                    ┌─────────────┐
                    │ Task Queue  │ (Redis / in-memory)
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │Container1│ │Container2│ │Container3│
        │ Task #12 │ │ Task #13 │ │ Task #14 │
        │ claude   │ │ aider    │ │ claude   │
        └──────────┘ └──────────┘ └──────────┘
```

- **Пул воркеров** — ограничивает количество одновременных контейнеров (по ресурсам хоста)
- **Очередь задач** — задачи ожидают свободный слот
- **Приоритеты** — critical/high задачи выполняются первыми
- **Resource Limits** — каждый контейнер ограничен по CPU/RAM/Disk

### 4.6. Взаимодействие бэкенда с Docker

Go-бэкенд использует **Docker SDK для Go** (`github.com/docker/docker/client`):

```go
import (
    "github.com/docker/docker/client"
    "github.com/docker/docker/api/types/container"
)
```

**Требования к хосту:**
- Docker Engine доступен через `/var/run/docker.sock` (монтируется в контейнер бэкенда)
- Предсобранные образы `devteam/sandbox-*` доступны локально или в registry
- Достаточно ресурсов для параллельных контейнеров

**docker-compose (бэкенд):**
```yaml
app:
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock  # Доступ к Docker API
  environment:
    - SANDBOX_MAX_CONCURRENT=5                    # Макс. параллельных контейнеров
    - SANDBOX_DEFAULT_TIMEOUT=30m                 # Таймаут на задачу
    - SANDBOX_MEMORY_LIMIT=4096                   # MB на контейнер
    - SANDBOX_CPU_LIMIT=2.0                       # CPU на контейнер
```

### 4.7. Реализации Code Backend

| Backend | Docker-образ | Описание | Когда использовать |
|:---|:---|:---|:---|
| **`claude-code`** | `devteam/sandbox-claude` | Claude Code CLI в контейнере | Основной — сложные многофайловые задачи |
| **`aider`** | `devteam/sandbox-aider` | Aider CLI в контейнере | Мультимодельность, альтернативные LLM |
| **`custom`** | `devteam/sandbox-base` | Базовый образ + LLM tool calling | Простые задачи, кастомные инструменты |

### 4.8. Безопасность контейнеров

- **Нет привилегированного режима** — контейнеры запускаются как обычные
- **Read-only root FS** — только `/workspace` доступен на запись
- **Нет доступа к сети хоста** — отдельная Docker network, доступ только к API LLM-провайдеров
- **Секреты** — API ключи передаются через environment variables, не хранятся в образе
- **Таймаут** — жёсткий лимит времени, контейнер убивается при превышении
- **Resource limits** — CPU, memory, disk ограничены per-container

-----

## 5. Интеграция с Git-системами

### 5.1. Поддерживаемые провайдеры

| Провайдер | API | Возможности |
|:---|:---|:---|
| **GitHub** | REST + GraphQL | Repos, Issues, PRs, Actions, Webhooks |
| **GitLab** | REST v4 | Projects, Issues, MRs, CI/CD, Webhooks |
| **Bitbucket** | REST 2.0 | Repos, Issues, PRs, Pipelines, Webhooks |
| **Local** | git CLI | Локальные операции без remote |

### 5.2. Абстракция Git Provider

```go
type GitProvider interface {
    CloneRepo(ctx context.Context, opts CloneOptions) error
    CreateBranch(ctx context.Context, repo, branch, base string) error
    CreatePR(ctx context.Context, opts PROptions) (*PullRequest, error)
    GetPRStatus(ctx context.Context, repo string, prID int) (*PRStatus, error)
    ListIssues(ctx context.Context, repo string, filters IssueFilters) ([]Issue, error)
    CreateIssue(ctx context.Context, opts IssueOptions) (*Issue, error)
    AddWebhook(ctx context.Context, repo, url string, events []string) error
}
```

### 5.3. Сценарии работы с репозиторием

1. **Импорт существующего** — пользователь указывает URL, система клонирует, индексирует в векторку
2. **Создание нового локально** — scaffold проекта по шаблону, инициализация git
3. **Создание в remote** — через API создать репозиторий в GitHub/GitLab/Bitbucket, затем push

-----

## 6. Векторная индексация (Weaviate)

### 6.1. Что индексируется

| Тип контента | ContentType | Описание | Триггер обновления |
|:---|:---|:---|:---|
| **Код** | `code_file` | Файлы проекта (chunked) | Push в репо / коммит агента |
| **Задачи** | `task` | Описание + результат задач | Создание / обновление задачи |
| **Сообщения задач** | `task_message` | Лог коммуникации агентов | Новое сообщение |
| **Чаты** | `conversation` | Сообщения пользователя | Новое сообщение |
| **PR/MR** | `pull_request` | Описание, diff, комментарии | Создание / обновление PR |
| **Документация** | `documentation` | README, docs/, wiki | Обновление файлов |

### 6.2. Стратегия индексации кода

Код разбивается на чанки с сохранением структуры:
- **По файлам** — каждый файл как документ (для малых файлов)
- **По функциям/классам** — AST-парсинг для больших файлов
- **Metadata**: `file_path`, `language`, `last_modified`, `project_id`

### 6.3. Использование контекста агентами

Перед выполнением задачи агент:
1. Делает `vector_search` по описанию задачи → получает релевантный код
2. Добавляет контекст из связанных задач и чатов
3. Формирует полный контекст для LLM

### 6.4. Актуализация

- **Код** — webhook на push или polling с `git diff`
- **Задачи/чаты** — синхронно при создании/обновлении (event-driven)
- **Полная переиндексация** — ручная команда или по расписанию

-----

## 7. Chat UI (Фронтенд)

### 7.1. Основной интерфейс — чат

Пользователь взаимодействует с системой через чат:

1. **Описание идеи** — "Создай REST API для управления пользователями с JWT авторизацией"
2. **Оркестратор** отвечает планом действий и создаёт задачи
3. **Live-обновления** — в чат приходят статусы задач:
   - "📋 Планировщик создал 4 подзадачи..."
   - "⚙️ Разработчик начал задачу: Модель User..."
   - "✅ Ревьюер одобрил код"
   - "🧪 Тесты пройдены: 12/12"
4. **Пользователь может вмешаться** — "Стоп, используй UUID вместо auto-increment для ID"

### 7.2. Структура экранов

```
/projects                    — список проектов
/projects/:id                — дашборд проекта (чат + задачи)
/projects/:id/chat           — основной чат
/projects/:id/chat/:convId   — конкретный разговор
/projects/:id/tasks          — список задач (Kanban / Table)
/projects/:id/tasks/:taskId  — детали задачи + лог агентов
/projects/:id/team           — настройка команды агентов
/projects/:id/settings       — настройки проекта (git, credentials)
/settings                    — глобальные настройки (API keys, модели)
```

### 7.3. Реалтайм обновления

- **WebSocket** для live-стриминга статусов задач и сообщений агентов
- **Server-Sent Events** как fallback
- Пользователь видит прогресс в реальном времени

-----

## 8. Общие правила для работы с репозиторием

* **ВАЖНО:** Запрещено создавать итоговые файлы .md с результатами. Можно писать только сжатую информацию в README.md
* **`README.md` (roadmap) и `docs/tasks/N.M-*.md`:** формулировка задачи в roadmap и заголовок H1 в файле задачи с тем же номером **должны совпадать**. Меняя любой из двух источников, синхронизируйте второй (канон смысла — заголовок в `docs/tasks/`).

-----

## 9. Ссылки на детальные правила

| Файл | Описание |
|:---|:---|
| `.cursor/rules/backend.mdc` | Go + Gin, Clean Architecture, YugabyteDB, JWT, Swagger, MCP |
| `.cursor/rules/frontend.mdc` | Flutter, Riverpod, i18n, Feature-First архитектура |
| `.cursor/rules/deploy.mdc` | Docker, docker-compose, Makefile |

-----

## 10. Приоритеты реализации (Roadmap)

### Phase 1 — MVP (Фундамент)
- [ ] Сущности: Project, Team, Agent, Task, TaskMessage, Conversation
- [ ] Миграции БД
- [ ] CRUD API для проектов и агентов
- [ ] Интеграция с GitHub (clone, branch, PR)
- [ ] Sandbox Runner: Docker-контейнеры для изолированного выполнения задач
- [ ] Docker-образ `devteam/sandbox-claude` (Claude Code CLI)
- [ ] Базовый Orchestrator (линейный pipeline: plan → develop → review → test)
- [ ] Chat UI с WebSocket
- [ ] Векторная индексация кода проекта
- [ ] Пользовательское управление (пауза, отмена задачи через остановку контейнера)

### Phase 2 — Улучшения
- [ ] GitLab и Bitbucket интеграции
- [ ] Docker-образ `devteam/sandbox-aider` (Aider, мультимодельный)
- [ ] Очередь задач + пул воркеров (параллельные контейнеры)
- [ ] Продвинутый Orchestrator (параллельные задачи, ретраи, коррекция)
- [ ] Индексация задач и чатов в векторку
- [ ] Настраиваемые промпты и скиллы агентов
- [ ] Мониторинг ресурсов контейнеров

### Phase 3 — Расширение
- [ ] DevOps агент (CI/CD, деплой)
- [ ] Шаблоны проектов (scaffold)
- [ ] Мультипользовательский доступ
- [ ] Метрики и аналитика агентов (стоимость, время, качество)
- [ ] Marketplace скиллов и промптов
- [ ] Auto-scaling контейнеров (Kubernetes / Docker Swarm)

-----

## Orchestration v2 (Sprint 17 — LLM-driven, flow-as-data)

С 2026-05 архитектура оркестрации задач **полностью переписана** на LLM-driven модель.
Подробный план: [`docs/orchestration-v2-plan.md`](../orchestration-v2-plan.md).

**Ключевые инварианты (НЕ нарушать в новом коде):**

1. **Flow — это данные, не код.** Никакого hardcoded state-machine в Go.
   Никаких `switch task.Phase`/`if role == "developer"` поверх роутинга.
   Маршрутизация между фазами решается LLM-агентом `router` по реестру `agents`
   из БД + текущим артефактам. Добавление новой роли = `INSERT INTO agents`,
   правки промптов = `UPDATE agents.system_prompt` (или через UI Agents Management).
   Если нужна "ветка" логики — это либо новый промпт, либо новый агент в БД,
   но **не** новая ветка в Go-коде оркестратора.

2. **Артефакты идут через reviewer.** Developer/Planner/Tester складывают
   результат в `task_artifacts`, и **только** Reviewer-агент решает, годен ли
   артефакт к следующему шагу (`approve` → router двигает дальше; `request_changes` →
   router возвращает к автору). Никакого автоматического "developer закончил → сразу
   merge/PR". Промежуточные артефакты без review — баг.

3. **`--` separator во всех git-вызовах.** После фиксированных флагов и **перед**
   любыми пользовательскими / LLM-данными (branch name, base ref, путь worktree,
   автор коммита) **обязан** быть `--`. Это блокирует flag-injection вида
   `branch="-h"` или `"--upload-pack=evil"`. См. `docs/rules/backend.md` §2.3 п.1.

4. **Event-driven Step.** `Orchestrator.Step` — атомарный шаг, открывается через
   `task_events` очередь (`step_req`/`agent_job`). `SELECT FOR UPDATE NOWAIT`
   per-task сериализует step'ы; agent_jobs параллельны по подзадачам через
   git-worktree-изоляцию.

5. **Secrets — только через `agent_secrets` (AES-GCM).** API-ключи, токены,
   приватные ключи для агентов **запрещено** класть в `sandbox_settings`,
   `agents.settings`, env-vars docker-compose или любые plaintext JSONB-поля.
   Используется таблица `agent_secrets` (зашифровано через `pkg/crypto.AESEncryptor`
   с AAD = id записи) + MCP-инструменты `agent_set_secret`/`agent_delete_secret`.
   То же касается `router_decisions.encrypted_raw_response`,
   `user_llm_credentials.encrypted_key`, `mcp_server_config.*`.

6. **Безопасность по дефолту:** см. `docs/rules/backend.md` §2.3 — redact-обёртка
   logger'а (никакого `slog.Default()` в orchestrator-файлах), `filepath.Clean` +
   prefix-check перед `os.RemoveAll`, типизированные `uuid.UUID` вместо строк из БД
   при построении путей.

**5 высокоуровневых состояний задачи** (`tasks.state`):
`active` | `done` | `failed` | `cancelled` | `needs_human`.
Legacy `TaskStatus` (10 значений pipeline) удалён в Sprint 5.

**MCP-инструменты v2:** `agent_list/get/create/update/set_secret/delete_secret`,
`artifact_list/get`, `router_decision_list`, `worktree_list`, `task_cancel_v2`.

**Фронтенд v2-фичи:** см. `docs/rules/frontend.md` §5 "Orchestration v2 фичи"
(Agents Management, Worktrees debug, Task Detail v2 sections).
