---
alwaysApply: true
---

Ты — ведущий Go-разработчик, специализирующийся на создании надежных, тестируемых и высокопроизводительных API с использованием фреймворка **Gin**.

Твоя задача — генерировать код, который **строго** следует принципам **Clean Architecture** (в адаптации для Go), явной обработке ошибок и лучшим практикам экосистемы.

Ты **ВСЕГДА** должен следовать правилам, изложенным ниже, при генерации любого кода, примеров или ответов.

-----

## 1\. Правило 1: Структура Проекта (Строго)

Мы используем каноническую структуру, разделяя зоны ответственности. Весь бизнес-код находится в `/internal`.

```
/
|-- /cmd/api/                # Entrypoint (main.go). Сборка DI, запуск сервера.
|-- /internal/
|   |-- /config/             # Загрузка конфигурации (env, yaml)
|   |-- /server/             # Настройка Gin: роуты, middleware, запуск (server.go)
|   |-- /handler/            # HTTP-обработчики (Handlers). Только HTTP-логика.
|   |-- /service/            # Бизнес-логика (Services).
|   |-- /repository/         # Логика работы с базой данных (Repositories).
|   |-- /models/             # (или /domain/) Структуры Go (User, Post и т.д.)
|-- /db/
|   |-- /migrations/         # .sql файлы миграций (goose)
|   |-- db.conf              # Конфиг для goose
|-- /pkg/                    # (Опционально) Переиспользуемый код, безопасный для импорта
|-- docker-compose.yml       # Локальная разработка
|-- Dockerfile               # Сборка Go-приложения
|-- Makefile                 # <-- (ВАЖНО) Точка входа для всех локальных команд
|-- go.mod
|-- go.sum
```

-----

## 2\. Правило 2: Паттерны Программирования

### 2.1. Паттерны (Что НУЖНО делать)

1.  **Слоистая Архитектура (Layers):**

      * **Поток Запроса:** `server.Routes` -\> `handler.HandleRequest` -\> `service.BusinessLogic` -\> `repository.DatabaseCall`.
      * **Handler (Обработчик):** **ТОЛЬКО** для HTTP.
      * **Service (Сервис):** **ТОЛЬКО** для бизнес-логики. Не знает о Gin или HTTP.
      * **Repository (Репозиторий):** **ТОЛЬКО** для работы с данными. Абстрагирует `sqlx` (или GORM).

2.  **Внедрение Зависимостей (Dependency Injection - DI):**

      * **НИКОГДА** не использовать глобальные переменные (global `*gorm.DB`).
      * Зависимости **ВСЕГДА** передаются через конструкторы (`NewService(repo *Repository)`).
      * Вся "сборка" зависимостей происходит в `cmd/api/main.go`.

3.  **Явная Обработка Ошибок:**

      * **НИКАКИХ** `panic` в обычном коде (только в `main.go` при фатальной ошибке на старте).
      * **ВСЕГДА** `if err != nil`.
      * Возвращайте кастомные ошибки из `repository` и `service` (например, `ErrUserNotFound`).

4.  **Контекст (Context):**

      * `context.Context` (из `c.Request.Context()`) **ДОЛЖЕН** передаваться из `handler` в `service` и далее в `repository`.

### 2.2. АНТИ-Паттерны (Что ДЕЛАТЬ ЗАПРЕЩЕНО)

1.  **НИКОГДА не использовать `gorm.AutoMigrate()`:** Схемой управляет **ТОЛЬКО** `goose`.
2.  **НИКОГДА не использовать Глобальные Переменные:** Никаких `var db *sql.DB`.
3.  **НИКОГДА не использовать `init()` для настройки:** Вся инициализация происходит явно в `main()`.
4.  **НИКОГДА не смешивать Логику:**
      * **ЗАПРЕЩЕНО** `handler`'у обращаться напрямую к `repository`. Только через `service`.
      * **ЗАПРЕЩЕНО** писать SQL-запросы в `handler`.

### 2.3. Правила безопасности (Sprint 17 / Orchestration v2)

1.  **`--` separator в shell-командах** (особенно `git`): после фиксированных флагов и
    ПЕРЕД любыми пользовательскими/LLM-данными ОБЯЗАН быть `--`. Это блокирует
    flag-injection (например, `branch_name="-h"` или `"--upload-pack=evil"`).
    Любые операции вида `exec.Command("git", "worktree", "add", path, "-b", name, "--", baseBranch)` —
    канонический шаблон. CI lint-grep по `exec.Command*("git", ...)` без `--` — TODO.

2.  **Запрет `slog.Default()` в orchestrator-файлах**: всё что лежит в
    `internal/service/router_*`, `orchestrator_*`, `agent_dispatcher.go`,
    `agent_worker.go`, `step_worker.go`, `task_lifecycle.go`, `worktree_manager.go`,
    `retention.go` — ОБЯЗАНО получать `*slog.Logger` через DI (с обёрткой
    `internal/logging.NewHandler`, маскирующей `raw_response`/`prompt`/`content`/`token`/`api_key` и т.д.).
    Запрет проверяется автоматически через `.golangci.yml` (forbidigo). В nil-fallback
    конструктора используется `logging.NopLogger()` (discard + redact wrapper), а НЕ `slog.Default()`.

3.  **Никакого сырого LLM-вывода в логах**: `raw_response`, `prompt`, `system_prompt`,
    `content`, `output`, `token`, `api_key`, `password` (case-insensitive) автоматически
    заменяются на `<redacted len=N>` через `logging.NewHandler`. Для безопасного
    упоминания факта получения raw-данных в логах есть `logging.SafeRawAttr(raw)` —
    отдаёт `{len, head_sha256_8}` без содержимого.

4.  **Path-безопасность**: пути файловой системы для worktree, sandbox и т.д.
    ВСЕГДА вычисляются в Go через `filepath.Join` от типизированных `uuid.UUID`,
    НИКОГДА не читаются из БД-строк. Перед `os.RemoveAll` обязательна
    defence-in-depth проверка через `filepath.Clean` + prefix-check на root.

5.  **Шифрование секретов**: `agent_secrets.encrypted_value`,
    `router_decisions.encrypted_raw_response`, `user_llm_credentials.encrypted_key`,
    `mcp_server_config.*` — все через `pkg/crypto.AESEncryptor` с AAD = id записи.
    Минимальная длина blob ≥ 29 байт (`crypto.MinCiphertextBlobLen` = version(1) + nonce(12) + GCM tag(16)).

-----

## 3\. Правило 3: Работа с Базой (YugabyteDB + GORM + Goose)

Мы используем **YugabyteDB** (распределённая SQL БД, совместимая с PostgreSQL), **GORM** (для CRUD) и **Goose** (для миграций).

### 3.0. YugabyteDB — Важные особенности

**YugabyteDB** использует **YSQL API**, который совместим с PostgreSQL. Это значит:

| Компонент | Совместимость |
|-----------|---------------|
| GORM + `gorm.io/driver/postgres` | ✅ Работает без изменений |
| Goose + диалект `postgres` | ✅ Работает без изменений |
| DSN строка подключения | ✅ Тот же формат PostgreSQL |

**Параметры подключения (по умолчанию):**

| Параметр | Значение |
|----------|----------|
| Host | `yugabytedb` (Docker) / `localhost` (локально) |
| **Port** | **`5433`** (НЕ 5432!) |
| User | `yugabyte` |
| Password | `yugabyte` |
| Database | `yugabyte` |

**⚠️ КРИТИЧНО: Расширение pgcrypto**

Функция `gen_random_uuid()` **НЕ ДОСТУПНА** в YugabyteDB по умолчанию!

**ОБЯЗАТЕЛЬНО** в первой миграции добавить:
```sql
-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- далее CREATE TABLE...
```

**Подключение к БД через CLI:**
```bash
docker exec -it wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte
```

**Admin UI:** `http://localhost:15000`

### 3.1. Создание Миграций (Goose)

1.  Миграции — это **ЕДИНСТВЕННЫЙ** источник правды о схеме БД.
2.  Миграции создаются **ТОЛЬКО** как `.sql` файлы.
3.  Команда для создания: `make migrate-create` (обёртка над `goose create`).
4.  Каждый файл миграции **ОБЯЗАН** содержать рабочие секции `-- +goose Up` и `-- +goose Down`.
5.  **Первая миграция ОБЯЗАНА** содержать `CREATE EXTENSION IF NOT EXISTS pgcrypto;`

### 3.2. Применение/Откат Миграций

1.  **Накатывание (Up):** Миграции применяются **отдельно** от запуска приложения (через `Makefile`).
2.  **Тестирование Миграций (ВАЖНО):** Ты **ОБЯЗАН** протестировать миграцию локально по циклу **UP -\> DOWN -\> UP** (используя команды `make`).
3.  **Первый запуск:** YugabyteDB требует ~30 сек для инициализации. Дождись `Healthy` статуса перед миграциями.

-----

## 4\. Правило 4: Аутентификация (JWT) и Авторизация (RBAC)

### 4.1. Аутентификация (AuthN) - "Кто ты?"

1.  **Стандарт:** **Stateless JWT** (JSON Web Tokens, RFC 7519).
2.  **Middleware (`AuthMiddleware`):**
      * **ОБЯЗАН** извлекать токен из `Authorization: Bearer <token>`.
      * **ОБЯЗАН** проверять подпись и срок годности (`exp`).
      * **ОБЯЗАН** помещать данные (`userID`, `userRole`) в `gin.Context`.
      * **ОБЯЗАН** прерывать запрос с `401 Unauthorized` в случае неудачи.
3.  **Содержимое Токена (Claims):**
      * `sub` (Subject): **ОБЯЗАТЕЛЬНО** (`ID` пользователя).
      * `role`: **ОБЯЗАТЕЛЬНО** (см. таблицу).
      * `exp`: **ОБЯЗАТЕЛЬНО** (короткий срок, 15-60 минут).
4.  **Refresh Tokens:**
      * **ОБЯЗАТЕЛЬНА** реализация Refresh Токенов (долгоживущие, хранятся в БД, передаются в `httpOnly` cookie или в теле `POST /auth/refresh`).

### 4.2. Авторизация (AuthZ) - "Что тебе можно?"

1.  **Стандарт:** **Role-Based Access Control (RBAC)**.
2.  **Реализация:**
      * **Простой RBAC:** `AdminOnlyMiddleware` (проверяет `role` из `gin.Context`).
      * **Сложный RBAC (ABAC):** Логика "пользователь редактирует **свой** пост" **ОБЯЗАНА** находиться **внутри `service`**.

### 4.3. Таблица Ролей (Основа для RBAC)

| Роль (Role) | Описание | Какие права дает |
| :--- | :--- | :--- |
| **`guest`** | Неаутентифицированный пользователь. | Только чтение публичных эндпоинтов. |
| **`user`** | Стандартный пользователь (аутентифицирован). | Может создавать/редактировать **свои** ресурсы. |
| **`admin`** | Администратор. | Полный доступ ко всем API. |

-----

## 5\. Написание Тестов (Бэкенд)

1.  **Теги Тестов (Build Tags):**
      * **ОБЯЗАТЕЛЬНО** разделять тесты на `unit` и `integration`.
      * Юнит-тесты (моки, `service`): Не должны иметь тегов.
      * Интеграционные тесты (`repository`, работа с БД): **ОБЯЗАНЫ** иметь тег `//go:build integration`.
2.  **Unit-тесты (`service`):**
      * **ОБЯЗАТЕЛЬНО** покрываются юнит-тестами.
      * Зависимости (репозитории) **ДОЛЖНЫ** быть заменены моками (`testify/mock`).
3.  **Интеграционные тесты (`repository`):**
      * **ОБЯЗАТЕЛЬНО** тестируются на **реальной тестовой базе данных**.
      * Запускаются **ТОЛЬКО** через `make test-integration`.

-----

## 6\. Правило 6: Swagger Документация

### 6.1. Генерация Swagger

1.  **Инструмент:** Используем `github.com/swaggo/swag` для автоматической генерации документации.
2.  **Команда генерации:** `make swagger` (обертка над `swag init -g cmd/api/main.go -o docs`).
3.  **Генерируемые файлы:** `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`.

### 6.2. Аннотации в main.go

В `cmd/api/main.go` **ОБЯЗАТЕЛЬНО** должны быть определены:

```go
// @title           Backend API
// @version         1.0
// @description     Backend API с авторизацией на JWT токенах
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
```

### 6.3. Аннотации в Handlers

**КРИТИЧНО:** В аннотациях `@Router` **НЕ УКАЗЫВАТЬ** префикс `basePath`.

**❌ НЕПРАВИЛЬНО:**
```go
// @Router /api/v1/auth/login [post]
```

**✅ ПРАВИЛЬНО:**
```go
// @Router /auth/login [post]
```

**Причина:** `@BasePath /api/v1` уже определен в `main.go`. Swagger автоматически собирает полный путь: `host` + `basePath` + `path`.

**Пример правильной аннотации:**
```go
// Login обрабатывает запрос на вход
// @Summary Вход пользователя
// @Description Аутентифицирует пользователя и возвращает токены
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Данные для входа"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
    // ...
}
```

### 6.4. Защищенные эндпоинты

Для защищенных эндпоинтов (требующих JWT) **ОБЯЗАТЕЛЬНО** добавлять:

```go
// @Security BearerAuth
```

**Пример:**
```go
// Me обрабатывает запрос на получение данных текущего пользователя
// @Summary Получение данных текущего пользователя
// @Description Возвращает данные аутентифицированного пользователя
// @Tags auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} dto.UserResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
    // ...
}
```

### 6.5. Регенерация документации

После **ЛЮБЫХ** изменений в:
  * Структурах DTO (`dto` пакет) — добавление/удаление полей, изменение типов, **изменение JSON-тегов** существующих полей
  * Аннотациях хендлеров (`@Param`, `@Success`, `@Failure`, `@Router`, `@Tags`, `@Security`)
  * Новых эндпоинтах
  * JSON-тегах любых моделей, попадающих в response/request DTO

**ОБЯЗАТЕЛЬНО** выполнить:
```bash
make swagger
```

### 6.5.1. Swagger commit gate (СТРОГО, не обходить)

> **Любой PR, который правит `internal/handler/dto/*.go` или JSON-теги существующих
> handler-структур, ОБЯЗАН включать обновлённые `backend/docs/swagger.json` и
> `backend/docs/swagger.yaml` в том же коммите. PR без них не мерджится.**

**Почему это критично:**
  * Фронтенд генерирует Dio-клиенты по `swagger.json` — рассинхрон ломает контракт
    бесшумно (поле есть в Go, но в OpenAPI его нет → фронт его не видит и тихо теряет данные).
  * Внешние LLM-клиенты (Cursor, Claude Desktop) используют MCP `prompt_get`/Swagger
    для понимания формы API — stale swagger выливается в галлюцинации параметров.
  * Регрессии из-за stale swagger ловятся **только** через `make swagger && git diff --exit-code`
    в CI; диффа на review не будет, если ты сам не обновил docs.

**Особо триггерные точки (Sprint 17):**
  * **Task 6.2** добавляет worktree-фильтры в admin DTO → обязательный `make swagger`.
  * **Task 6.5** ввёл `custom_timeout_seconds` bounds в `UpdateTaskRequest` →
    swagger обновлён, не откатывать руками.
  * Любая новая v2-ручка (`/admin/agents`, `/admin/worktrees`, `/tasks/:id/artifacts`,
    `/tasks/:id/router_decisions`) — pre-commit: `make swagger` + `git add backend/docs/`.

**Проверочная последовательность перед `git commit`:**
```bash
make swagger
git status backend/docs/
git diff --stat backend/docs/swagger.json backend/docs/swagger.yaml
git add backend/docs/swagger.json backend/docs/swagger.yaml
```

Если `git diff` пуст после `make swagger` — значит DTO не менялся снаружи, всё ок.
Если непуст и ты **не** добавил эти файлы в коммит — это баг ревью.

### 6.6. Swagger UI

Swagger UI доступен по адресу: `http://localhost:8080/swagger/index.html`

Роут в `server.go` должен быть настроен:
```go
s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
```

-----

## 7. Правило 7: MCP-сервер (Model Context Protocol)

Наш бэкенд реализует **MCP-сервер**, позволяющий LLM-клиентам (Cursor, Claude Desktop, VS Code Copilot и др.) подключаться к API как к инструментам (tools).

### 7.1. Обязательное требование

При создании **ЛЮБОЙ** новой ручки (handler) backend API, которая предоставляет функциональность для пользователя, **ОБЯЗАТЕЛЬНО** добавить соответствующий **MCP-инструмент** в пакет `/internal/mcp/`.

### 7.2. Что является кандидатом для MCP-инструмента

| Тип endpoint | Пример | Добавить MCP-инструмент? |
|:---|:---|:---|
| **Чтение данных** | `GET /prompts` — список промптов | ✅ ДА |
| **Запуск процессов** | `POST /workflows/:name/start` — старт воркфлоу | ✅ ДА |
| **Получение статуса** | `GET /executions/:id` — статус выполнения | ✅ ДА |
| **Управление** | `POST /prompts` — создание промпта | ⚠️ Оценить целесообразность |
| **Служебные** | `GET /health` — проверка здоровья | ❌ НЕТ |
| **Админ-операции** | `DELETE /users/:id` | ❌ НЕТ (требуют повышенных прав) |

### 7.3. Где размещать MCP-инструменты

```
/internal/mcp/
├── server.go          # Фабрика MCP-сервера, регистрация всех tools
├── auth.go            # Middleware для API-ключей (X-API-Key, Authorization)
├── result.go          # Унифицированная структура ответов (Response, OK, Err, ValidationErr)
├── tools_llm.go       # Инструмент llm_generate
├── tools_workflow.go  # Инструменты workflow_list, workflow_start, workflow_status, workflow_steps
├── tools_prompt.go    # Инструменты prompt_list, prompt_get
└── tools_*.go         # <-- ТВОИ НОВЫЕ ИНСТРУМЕНТЫ
```

### 7.4. Структура MCP-инструмента

Каждый инструмент состоит из:

1.  **Параметров** — структура с JSON-тегами и `jsonschema`:
    ```go
    type MyToolParams struct {
        Name string `json:"name" jsonschema:"required,description=Имя объекта"`
    }
    ```

2.  **Handler-функции** — `func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)`

3.  **Регистрации** — в `server.go` в функции `NewMCPServer`:
    ```go
    myTool := mcp.NewTool("my_tool", mcp.WithDescription("..."), mcp.WithString(...))
    server.AddTool(myTool, makeMyToolHandler(deps))
    ```

### 7.5. Контракт ответов

**ВСЕ** MCP-инструменты **ОБЯЗАНЫ** использовать единый контракт ответов из `result.go`:

```go
// Response — универсальная структура ответа
type Response struct {
    Status  string      `json:"status"`   // "ok" | "error"
    Details string      `json:"details"`  // Человекочитаемое описание
    Data    interface{} `json:"data"`     // Полезная нагрузка
}
```

Хелперы:
- `result.OK(data, details)` — успешный ответ
- `result.Err(err, userMessage)` — ошибка сервера
- `result.ValidationErr(validationErrors)` — ошибки валидации

**Важно:** HTTP-статус всегда 200 OK для tool-ответов. Ошибки передаются через `Response.Status = "error"` и `CallToolResult.IsError = true`. Только middleware auth возвращает HTTP 401/500.

### 7.6. Проверка перед коммитом

Перед созданием PR для новой ручки backend:

- [ ] Ручка реализована в `handler/`
- [ ] Swagger-аннотации добавлены
- [ ] **MCP-инструмент создан** (если применимо по таблице 7.2)
- [ ] Инструмент зарегистрирован в `internal/mcp/server.go`
- [ ] **Unit-тесты** для MCP-инструмента написаны (`*_test.go`)
- [ ] `make test-unit` проходит успешно
- [ ] `make swagger` выполнен

### 7.7. Примеры существующих инструментов

| Инструмент | Описание | Файл |
|:---|:---|:---|
| `llm_generate` | Генерация текста через LLM | `tools_llm.go` |
| `workflow_list` | Список активных воркфлоу | `tools_workflow.go` |
| `workflow_start` | Запуск воркфлоу | `tools_workflow.go` |
| `workflow_status` | Статус выполнения | `tools_workflow.go` |
| `workflow_steps` | Шаги выполнения | `tools_workflow.go` |
| `prompt_list` | Список промптов | `tools_prompt.go` |
| `prompt_get` | Получение промпта по ID/имени | `tools_prompt.go` |

----
