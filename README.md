# Wibe Flutter + Gin Template

Шаблон full-stack приложения: **Flutter** (Web/Android/iOS) + **Gin** (Go) + **YugabyteDB** + **Weaviate**.

Включает систему аутентификации, RBAC-авторизацию, движок воркфлоу с LLM-интеграцией, вебхуки и планировщик задач.

---

## Стек технологий

| Слой | Технологии |
|------|-----------|
| **Backend** | Go 1.24, Gin, GORM, Goose, JWT (HS256), Swagger, MCP Server |
| **Frontend** | Flutter 3.x, Riverpod 2.0, GoRouter, Dio, Freezed |
| **БД** | YugabyteDB (PostgreSQL-совместимая распределённая SQL) |
| **Векторная БД** | Weaviate + sentence-transformers |
| **LLM** | OpenAI, Anthropic, Gemini, Deepseek, Qwen (через OpenRouter) |
| **Инфраструктура** | Docker, Docker Compose, Makefile |

---

## Возможности

### Backend (Go + Gin)

- **Clean Architecture** — слоистая структура: `handler` → `service` → `repository`
- **JWT-аутентификация** — Access-токены (15 мин) + Refresh-токены (7 дней, хранятся в БД)
- **RBAC-авторизация** — роли `guest`, `user`, `admin` с middleware-защитой
- **Движок воркфлоу** — пошаговое выполнение цепочек (LLM-вызовы, API-запросы, условия, циклы)
- **Мульти-LLM интеграция** — OpenAI, Anthropic (Claude), Gemini, Deepseek, Qwen
- **Система промптов** — CRUD для шаблонов, загрузка из YAML-файлов при старте
- **Вебхуки** — публичные эндпоинты, HMAC-верификация, IP-вайтлист, JSONPath
- **Планировщик** — cron-задачи для запуска воркфлоу и синхронизации каталога моделей
- **Каталог моделей** — синхронизация с OpenRouter, расчёт стоимости запросов
- **Логирование LLM** — запись запросов/ответов, токены, стоимость, трейс
- **MCP-сервер** — Model Context Protocol для подключения LLM-клиентов (Cursor, Claude Desktop, VS Code Copilot)
- **API-ключи** — долгосрочные ключи доступа (`wibe_*`) для внешних интеграций и MCP
- **Swagger** — автогенерация документации из аннотаций
- **Миграции** — Goose (12 миграций), поддержка UP/DOWN

### Frontend (Flutter + Riverpod)

- **Feature-First архитектура** — модули: `auth`, `landing`, `admin/prompts`, `admin/workflows`
- **Riverpod 2.0** — state management с кодогенерацией (`riverpod_generator`)
- **Адаптивная вёрстка** — Mobile (< 600dp), Tablet (600–1200dp), Desktop (> 1200dp)
- **Material 3** — светлая и тёмная тема, системный режим
- **Мультиязычность** — русский и английский (ARB-файлы, `flutter gen-l10n`)
- **Безопасное хранение токенов** — `flutter_secure_storage`
- **Маршрутизация** — `go_router` с deep linking, route guards, URL-навигация
- **Freezed-модели** — неизменяемые data-классы с JSON-сериализацией
- **UI Kit** — `CustomButton`, `LoadingIndicator`, `AppErrorWidget`, адаптивные контейнеры

### Инфраструктура

- **Docker Compose** — 4 сервиса: YugabyteDB, Weaviate, Transformers, Backend
- **Multi-stage Dockerfile** — сборка Go-бинарника, генерация Swagger, установка Goose
- **Makefile** — единый интерфейс для всех операций (30+ команд)
- **Healthcheck** — проверки готовности всех сервисов

---

## API-эндпоинты

### Аутентификация (`/api/v1/auth`)

| Метод | Путь | Доступ | Описание |
|-------|------|--------|----------|
| POST | `/auth/register` | Публичный | Регистрация |
| POST | `/auth/login` | Публичный | Вход (возвращает JWT) |
| POST | `/auth/refresh` | Публичный | Обновление токенов |
| GET | `/auth/me` | Авторизованный | Данные текущего пользователя |
| POST | `/auth/logout` | Авторизованный | Выход (отзыв refresh-токенов) |

### LLM (`/api/v1/llm`) — только Admin

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/llm/chat` | Чат с LLM-провайдером |
| GET | `/llm/logs` | Список логов LLM-запросов |

### Промпты (`/api/v1/prompts`) — только Admin

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/prompts` | Создать шаблон промпта |
| GET | `/prompts` | Список промптов |
| GET | `/prompts/:id` | Получить промпт |
| PUT | `/prompts/:id` | Обновить промпт |
| DELETE | `/prompts/:id` | Удалить промпт |

### Воркфлоу (`/api/v1/workflows`) — только Admin

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/workflows` | Список воркфлоу |
| POST | `/workflows/:name/start` | Запуск воркфлоу |

### Выполнения (`/api/v1/executions`) — только Admin

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/executions` | Список выполнений (пагинация) |
| GET | `/executions/:id` | Статус выполнения |
| GET | `/executions/:id/steps` | Шаги выполнения |

### Вебхуки (`/api/v1/webhooks`) — только Admin

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/webhooks` | Создать вебхук |
| GET | `/webhooks` | Список вебхуков |
| GET | `/webhooks/:id` | Детали вебхука |
| PUT | `/webhooks/:id` | Обновить вебхук |
| DELETE | `/webhooks/:id` | Удалить вебхук |
| GET | `/webhooks/:id/logs` | Логи вебхука |

### Публичные вебхуки (`/api/v1/hooks`) — без авторизации

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/hooks/:name` | Триггер вебхука |
| GET | `/hooks/:name` | Триггер вебхука (GET) |

### Служебные

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | Проверка здоровья |
| GET | `/swagger/*any` | Swagger UI |

---

## MCP-сервер (Model Context Protocol)

Приложение поддерживает **MCP** — открытый протокол для подключения LLM-клиентов к внешним инструментам. Это позволяет использовать возможности бэкенда напрямую из Cursor, Claude Desktop, VS Code Copilot и других LLM-клиентов.

### Доступные MCP-инструменты

| Инструмент | Описание |
|------------|----------|
| `llm_generate` | Генерация текста через LLM-провайдеры (OpenAI, Anthropic, Gemini, Deepseek, Qwen) |
| `workflow_list` | Список активных воркфлоу |
| `workflow_start` | Запуск воркфлоу по имени |
| `workflow_status` | Статус выполнения воркфлоу |
| `workflow_steps` | Шаги выполнения (с пагинацией) |
| `prompt_list` | Список активных промптов |
| `prompt_get` | Получение промпта по ID или имени |

### Аутентификация

MCP-сервер использует **API-ключи** (формат `wibe_*`). Ключ передаётся через:
- Заголовок `X-API-Key: wibe_...`
- Заголовок `Authorization: Bearer wibe_...`

API-ключи создаются через REST API (`POST /api/v1/api-keys`).

### Подключение к LLM-клиенту

Пример конфигурации для **Cursor** / **Claude Desktop**:

```json
{
  "mcpServers": {
    "wibe": {
      "url": "http://localhost:8081/mcp",
      "headers": {
        "X-API-Key": "wibe_ваш_ключ"
      }
    }
  }
}
```

### Транспорт

- **Протокол:** HTTP Streamable (SSE)
- **Порт:** `8081` (отдельный от основного API)
- **Эндпоинт:** `/mcp`
- **Health check:** `GET /health` на порту `8081`

### Включение

MCP-сервер выключен по умолчанию. Для включения задайте переменные:

```bash
MCP_ENABLED=true
MCP_PORT=8081
MCP_PUBLIC_URL=https://your-domain.com:8081  # обязательно для production
```

---

## Структура проекта

```
/
├── backend/
│   ├── cmd/api/main.go              # Точка входа, DI, миграции
│   ├── internal/
│   │   ├── config/                   # Конфигурация (env)
│   │   ├── server/                   # Gin: роуты, middleware
│   │   ├── handler/                  # HTTP-обработчики
│   │   ├── service/                  # Бизнес-логика
│   │   ├── repository/              # Работа с БД (GORM)
│   │   ├── models/                   # Доменные модели
│   │   ├── middleware/              # Auth, Admin middleware
│   │   └── mcp/                     # MCP-сервер (tools, auth, result)
│   ├── pkg/
│   │   ├── jwt/                      # JWT-менеджмент
│   │   ├── llm/                      # LLM-провайдеры (фабрика)
│   │   ├── apierror/                 # Обработка ошибок
│   │   ├── password/                 # Хеширование паролей
│   │   ├── promptsloader/           # Загрузка промптов из YAML
│   │   └── workflowloader/          # Загрузка воркфлоу из YAML
│   ├── db/migrations/               # 12 SQL-миграций (Goose)
│   ├── prompts/                      # YAML-шаблоны промптов
│   ├── agents/                       # YAML-определения агентов
│   ├── workflows/                    # YAML-определения воркфлоу
│   └── schedules/                    # YAML-расписания
│
├── frontend/
│   └── lib/
│       ├── main.dart                 # Entrypoint, ProviderScope
│       ├── core/
│       │   ├── api/                  # Dio-клиент с токен-интерсептором
│       │   ├── routing/              # GoRouter, route guards
│       │   ├── storage/              # Secure token storage
│       │   ├── theme/                # Material 3 тема
│       │   ├── utils/                # Responsive-утилиты
│       │   └── widgets/              # Адаптивные layout-виджеты
│       ├── features/
│       │   ├── auth/                 # Логин, регистрация, профиль
│       │   ├── landing/              # Лендинг
│       │   └── admin/
│       │       ├── prompts/          # Управление промптами
│       │       └── workflows/        # Воркфлоу и выполнения
│       ├── shared/widgets/           # UI Kit (кнопки, ошибки, загрузка)
│       └── l10n/                     # Локализация (ru, en)
│
└── deployment/
    └── docker-compose.yaml           # YugabyteDB, Weaviate, Transformers, Backend
```

---

## Модели базы данных

| Таблица | Назначение |
|---------|-----------|
| `users` | Пользователи (UUID, email, password hash, role) |
| `refresh_tokens` | Refresh-токены (SHA256 hash, expiry, revocation) |
| `prompts` | Шаблоны промптов (JSON schema, active flag) |
| `agents` | AI-агенты (привязка к промптам, конфиг модели — JSONB) |
| `workflows` | Определения воркфлоу (конфигурация — JSONB) |
| `executions` | Состояние выполнения воркфлоу (статус, шаги, контекст) |
| `execution_steps` | Пошаговая история (токены, длительность) |
| `scheduled_workflows` | Cron-расписания для воркфлоу |
| `llm_logs` | Логи LLM-запросов (токены, стоимость, трейс) |
| `llm_models` | Каталог моделей (OpenRouter, цены) |
| `webhook_triggers` | Конфигурация вебхуков (секрет, IP whitelist) |
| `webhook_logs` | История вызовов вебхуков |

---

## Быстрый старт

### Требования

- Docker и Docker Compose
- Go 1.24+
- Flutter SDK 3.x
- Make

### Запуск

```bash
# 1. Запуск инфраструктуры
make build && make up

# 2. Подождите ~30 сек пока YugabyteDB инициализируется

# 3. Применение миграций
make migrate-up

# 4. Frontend (первый запуск)
make frontend-setup
make frontend-run-web
```

### Точки доступа

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
# === Инфраструктура ===
make build                    # Сборка Docker-образов
make up / down / logs         # Управление контейнерами

# === Миграции ===
make migrate-up / down        # Применить / откатить миграции
make migrate-status           # Статус миграций
make migrate-create           # Создать новую миграцию

# === Backend тесты ===
make test                     # Все тесты
make test-unit                # Unit-тесты
make test-integration         # Интеграционные тесты

# === Frontend ===
make frontend-setup           # Первоначальная настройка
make frontend-codegen         # Кодогенерация (Riverpod, Freezed, l10n)
make frontend-run-web         # Запуск в Chrome
make frontend-run-android     # Запуск на Android
make frontend-run-ios         # Запуск на iOS
make frontend-test            # Тесты
make frontend-analyze         # Статический анализ
make frontend-build-web       # Сборка Web (release)

# === Документация ===
make swagger                  # Генерация Swagger

make help                     # Все команды
```

---

## Конфигурация

Основные переменные окружения (см. `backend/env.example`):

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `SERVER_PORT` | `8080` | Порт API |
| `DB_HOST` | `yugabytedb` | Хост БД |
| `DB_PORT` | `5433` | Порт YugabyteDB (YSQL) |
| `DB_USER` / `DB_PASSWORD` | `yugabyte` | Учётные данные БД |
| `JWT_SECRET_KEY` | — | Секрет для JWT (мин. 32 символа) |
| `JWT_ACCESS_TOKEN_EXPIRY` | `15m` | Время жизни access-токена |
| `JWT_REFRESH_TOKEN_EXPIRY` | `168h` | Время жизни refresh-токена (7 дней) |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | `admin@example.com` | Начальный администратор |
| `OPENAI_API_KEY` | — | Ключ OpenAI |
| `ANTHROPIC_API_KEY` | — | Ключ Anthropic |
| `GEMINI_API_KEY` | — | Ключ Gemini |
| `OPENROUTER_API_KEY` | — | Ключ OpenRouter (каталог моделей) |
| `MCP_ENABLED` | `false` | Включить MCP-сервер |
| `MCP_PORT` | `8081` | Порт MCP-сервера |
| `MCP_PUBLIC_URL` | — | Публичный URL MCP (обязателен в production) |
| `MCP_MAX_PROMPT_RUNES` | `100000` | Макс. длина промпта (в рунах) |
| `MCP_MAX_TOKENS_LIMIT` | `32768` | Макс. значение max_tokens |
| `MCP_MAX_INPUT_RUNES` | `50000` | Макс. длина input для воркфлоу |

---

## Экраны Frontend

| Роут | Экран | Доступ |
|------|-------|--------|
| `/` | Landing | Публичный |
| `/login` | Логин | Публичный |
| `/register` | Регистрация | Публичный |
| `/dashboard` | Дашборд | Авторизованный |
| `/profile` | Профиль | Авторизованный |
| `/admin/prompts` | Список промптов | Admin |
| `/admin/prompts/:id` | Редактирование промпта | Admin |
| `/admin/workflows` | Список воркфлоу | Admin |
| `/admin/executions` | Список выполнений | Admin |
| `/admin/executions/:id` | Детали выполнения | Admin |

---

## Архитектурные принципы

**Backend:**
- Clean Architecture с Dependency Injection (сборка в `main.go`)
- Явная обработка ошибок (`if err != nil`), без `panic`
- Контекст передаётся через все слои (`c.Request.Context()`)
- Интерфейсы для тестируемости (моки через `testify/mock`)

**Frontend:**
- Feature-First модульная структура
- Riverpod для DI и state management
- Freezed для иммутабельных моделей
- Абсолютные импорты (`package:frontend/...`)
- Весь текст — через локализацию (без хардкода строк)

---

## Подключение к БД

```bash
docker exec -it wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h localhost -U yugabyte
```

---

## Правила разработки

Подробные правила для AI-ассистента и разработчиков находятся в `.cursor/rules/`:
- `main.mdc` — общие правила
- `backend.mdc` — Go/Gin, Clean Architecture, миграции, JWT, тесты
- `frontend.mdc` — Flutter, Riverpod, адаптивность, i18n, тесты
- `deploy.mdc` — Docker, Makefile, окружение
- `mcp.mdc` — MCP-сервер, инструменты, аутентификация, тесты
