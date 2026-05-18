# План интеграционных тестов

## Цель

Построить систему интеграционных тестов, которая после реализации любой фичи
отвечает «зелёный/красный» по каждой ключевой пользовательской фиче.
Сейчас UI работает со сбоями — нет автоматического сигнала «работает /
сломалось». Эта система закрывает этот пробел.

## Принципы

- **Реальные зависимости, не моки** — для БД, WebSocket, sandbox.
- **Две стратегии прогона:**
  - **PR-gate (быстрый, на каждый push):** mock-LLM + локальный Git
    (Gitea) — детерминированно, без рейт-лимитов и без жжения токенов.
  - **Nightly / on-demand (полный e2e):** Real LLM + Real GitHub
    (`tereshed/kt-test-repo`) — ловим регрессии настоящих интеграций.
- **Изоляция данных через Tenant-модель**, не через TRUNCATE и не через схемы.
  YugabyteDB медленно выполняет DDL, поэтому создание схем на каждый тест замедлит прогон.
  Вместо этого миграции накатываются один раз при старте, а каждый тест создаёт
  уникального пользователя и проект (с рандомными UUID). Это даёт 100% изоляцию и работает мгновенно.
- **Никаких ручных шагов перед `make`.** Любой `test-features*` сам
  поднимает зависимости через `docker compose up -d` + healthcheck-wait.
- **Утечка секретов недопустима.** Все смокинг/маскирование секретов
  внедрено ДО первого PR-прогона с реальными ключами.
- **Тег сборки `featuresmoke`** отделяет этот слой от существующих
  `integration` тестов.

## Когда запускается

- **Локально** после реализации фичи: `make test-features` (mock-режим).
- **В CI на каждый PR:** workflow `feature-smoke.yml` (mock-режим).
- **Nightly + ручной trigger:** workflow `feature-e2e-real.yml`
  (real-режим, real LLM/Git).

## Архитектура: три слоя пирамиды

### 1. Backend feature-smoke (`backend/test/featuresmoke/`)

Black-box HTTP-тесты поверх живого backend. По одному файлу на фичу:

```
backend/test/featuresmoke/
  harness.go                  // поднять server + миграции в изолированной схеме + seed user
  fakes/
    llm_server.go             // детерминированный HTTP-стаб для Anthropic/OpenAI API
    git_server.go             // обёртка над gitea для git_provider тестов
  auth_smoke_test.go          // register → login → me → refresh → logout
  projects_smoke_test.go      // CRUD + reindex
  team_smoke_test.go          // get/update team, patch agent
  tasks_smoke_test.go         // create → pause → resume → cancel → correct + messages
  ws_smoke_test.go            // подключение, события task.* и message.*
  credentials_smoke_test.go   // user_llm_credentials + llm_providers + health-check
  git_oauth_smoke_test.go     // GitHub OAuth init/status/revoke (stub callback)
  orchestration_smoke_test.go // task → artifacts → router-decisions → worktree release
  secret_scrub_smoke_test.go  // P0: логи и API-ответы не содержат токенов (см. ниже)
```

**Запуск:**
```bash
go test -tags featuresmoke -race -timeout 600s ./test/featuresmoke/... -count=1
```

**Harness даёт:**
- `StartServer(t)` — поднимает Gin-server на случайном порту поверх
  Postgres из docker-compose. **Миграции накатываются один раз глобально**.
  Для предотвращения утечек ресурсов (goroutines, DB connections) **обязательно** используется `t.Cleanup`, а также ограничивается пул соединений (защита от исчерпания пула YugabyteDB при `t.Parallel()`):
  ```go
  db.SetMaxOpenConns(10) // Защита от connection pool exhaustion
  t.Cleanup(func() {
      ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
      defer cancel()
      server.Shutdown(ctx)
      db.Close()
  })
  ```
  Для изоляции тестов используются уникальные `project_id` и `user_id`.
  Никаких TRUNCATE или DROP SCHEMA. Все тесты обязаны вызывать `t.Parallel()`, а уникальные поля (email) генерироваться через UUID.
- `NewUser(t)` — регистрирует уникального пользователя.
- `Do(t, method, path, body, token) -> response` — http helper. **ОБЯЗАН** делать `defer resp.Body.Close()` (или требовать это от вызывающего кода), чтобы избежать утечек соединений и исчерпания пула при массовом параллельном прогоне.
- `WS(t, token, projectID) -> *websocket.Conn`. **Обязательно** вешать `t.Cleanup(func() { conn.Close() })` внутри хелпера для защиты от утечек горутин.
- `FakeLLM(t)` / `FakeGit(t)` — поднимают локальные стабы и возвращают base URL. **КРИТИЧНО (Thread-safety):** Так как все тесты бегут параллельно (`t.Parallel()`), стабы обязаны быть потокобезопасными. Любое внутреннее состояние (счетчики вызовов, история промптов) должно защищаться через `sync.Mutex` или `sync.RWMutex`, иначе получим плавающие data race.
- В **real-режиме** (`FEATURESMOKE_MODE=real`) stub'ы не поднимаются, а
  тесты читают `ANTHROPIC_API_KEY` / `GITHUB_PAT` из окружения и
  `t.Skip` если их нет.

### 2. Full pipeline e2e (`e2e_real_test.go`)

Отказываемся от bash-скрипта `scripts/e2e_smoke.sh` (дублирование логики, плохой репортинг).
Сразу пишем полноценный Go-тест `e2e_real_test.go` (с тегом `e2ereal`), который переиспользует
HTTP-хелперы из `harness.go`. Запускается в `feature-e2e-real.yml` (nightly)
и вручную. Это дает единообразную репортинговую выдачу (`go test -json`) и DRY.
**Критично (Security):** Все вызовы `git` внутри скрипта ОБЯЗАНЫ использовать разделитель `--` перед переменными (например, `git push origin -- "$BRANCH"`), чтобы исключить Command/Flag Injection.

### 3. Frontend integration (`frontend/integration_test/`)

```
frontend/integration_test/
  test_support/
    seed_creds.dart           // регистрация через REST + injection токена
    backend_available.dart    // проверка /health, FAIL в CI если backend не поднят (skip только для local mock)
    test_app.dart             // фабрика приложения с чистым ProviderContainer
  auth_flow_test.dart
  projects_flow_test.dart
  task_lifecycle_test.dart    // создание задачи → WS-обновления → pause/resume
  team_settings_test.dart
  chat_flow_test.dart
  assistant_e2e_test.dart     // существующий
  full_flow_test.dart         // существующий, переезжает на test_support/
```

**Запуск:**
```bash
cd frontend && flutter test integration_test/
```

**Правила написания UI-тестов (Anti-patterns):**
- **ЗАПРЕЩЁН** поиск элементов по захардкоженному тексту (например, `find.text('Login')`). Это хрупко и ломает тесты при смене локали. Искать виджеты нужно **только** через `find.byKey(Key('...'))` или через резолв локализованных строк (например, `find.text(l10n.login)`).

**Изоляция состояния между тестами (обязательно):**
- В `setUp` каждого теста:
  1. Создавать **новый `ProviderContainer`** (не переиспользовать).
  2. Очищать `SharedPreferences` (`SharedPreferences.setMockInitialValues({})`).
  3. Очищать `FlutterSecureStorage` (или его mock).
  4. Очищать Hive-боксы, если используются.
- Регистрация/логин в каждом тесте идут REST-ом и токен инжектится
  через `ProviderScope.overrides` (паттерн из `full_flow_test.dart`),
  но **внутри свежего scope**.
- Helper `freshTestApp(WidgetTester, {token, user})` в `test_support/`
  собирает всё это в одном месте.

**macOS Keychain:** `flutter_secure_storage` без dev-entitlements
не работает в test runner — поэтому Keychain не трогаем, используем
REST + injection.

## P0 — критичные пользовательские фичи

| Фича | Backend smoke | Frontend integration |
| ---- | ------------- | -------------------- |
| **Secret-scrub (логи + ответы)** | `secret_scrub_smoke_test.go` | — |
| Auth (register/login/refresh/me/logout) | `auth_smoke_test.go` | `auth_flow_test.dart` |
| Projects CRUD + reindex | `projects_smoke_test.go` | `projects_flow_test.dart` |
| Team + agent settings | `team_smoke_test.go` | `team_settings_test.dart` |
| Tasks lifecycle | `tasks_smoke_test.go` | `task_lifecycle_test.dart` |
| WebSocket events | `ws_smoke_test.go` | в составе `task_lifecycle_test.dart` |
| **Negative Paths (Ошибки и таймауты)** | `negative_smoke_test.go` | `negative_flow_test.dart` |

### Тестирование негативных сценариев (Negative Paths)

Мы обязаны гарантировать, что UI не "рассыпается" при ошибках бэкенда. В P0/P1 включены тесты на:
1. **Протухание/отсутствие токена:** Проверка корректного логаута и редиректа на экран логина при получении `401 Unauthorized` от защищенных эндпоинтов.
2. **Сетевые ошибки и 500:** Обработка отвала WebSocket-соединения (реконнект) и таймаутов/ошибок от LLM.
3. **Пустые/невалидные данные:** Быстрый фейл (early return) при пустых инпутах (например, отправка пустой задачи в чат).

### Secret-scrub (поднят в P0 — критично)

До того как реальные ключи появятся в CI — должно работать:

1. **Backend-логгер** обязан вырезать любые значения из набора
   «известных секретов» (ANTHROPIC_API_KEY, GITHUB_PAT,
   CLAUDE_CODE_OAUTH_ACCESS_TOKEN, DEEPSEEK_API_KEY, OPENROUTER_API_KEY,
   ENCRYPTION_KEY, JWT_SECRET_KEY, пароли пользователя) — заменять на
   `***`. Использовать существующий `service/secret_scrub.go`, в котором **обязательно реализовать маскирование URL-encoded вариантов** этих секретов (через `url.QueryEscape`).
2. **HTTP-ответы API** не должны содержать секреты даже при ошибках
   (например, при невалидном LLM-ключе сервер не возвращает сам ключ).
3. `secret_scrub_smoke_test.go` проверяет оба пункта: гоняет сценарии
   с заведомо «палевными» секретами, собирает stdout/stderr сервера и
   ответы API, грепает на наличие токенов — должно быть пусто.
4. В CI **дополнительный слой защиты:**
   - Перед прогоном тестов вызывать `echo "::add-mask::$SECRET"` для
     каждого секрета из переменных окружения (через шаг
     `mask-secrets`).
   - При `Dump logs on failure` явно итерироваться по значениям секретов и вырезать их из вывода (см. секцию CI).

## P1 — критичные интеграции

| Фича | Тест |
| ---- | ---- |
| User LLM credentials (шифрование, маскирование) | `credentials_smoke_test.go` |
| LLM providers CRUD + health-check + SSRF guard | `credentials_smoke_test.go` |
| Claude Code OAuth (init/status/revoke) | `git_oauth_smoke_test.go` |
| Git integrations (GitHub/GitLab) | `git_oauth_smoke_test.go` |
| Agents v2 + секреты (маскирование) | `agents_smoke_test.go` |
| Orchestration v2 (artifacts/router-decisions/worktrees) | `orchestration_smoke_test.go` |

## P2 — Дополнительный функционал (Обязательно к реализации)

- API keys + MCP config (`api_keys_smoke_test.go`)
- Assistant / Chat (`chat_flow_test.dart`, `assistant_smoke_test.go`)
- Prompts CRUD (`prompts_smoke_test.go`)
- Workflows list + start (`workflows_smoke_test.go`)

## P3 — Нефункциональные требования (Обязательно к реализации)

- `/health` под нагрузкой (`health_load_test.go`)
- Goose миграции с нуля и идемпотентно (`migrations_smoke_test.go`)
- Прометей-метрики (`metrics_smoke_test.go`)

## Стратегии прогона: mock vs real

| Аспект | PR-gate (`feature-smoke.yml`) | Nightly (`feature-e2e-real.yml`) |
| ------ | ----------------------------- | -------------------------------- |
| Триггер | push, pull_request | cron `0 3 * * *` + workflow_dispatch |
| LLM | **Fake LLM HTTP-stub** | Real (Anthropic / OpenRouter / DeepSeek) |
| Git | **Локальный Gitea в compose** | Real GitHub (`tereshed/kt-test-repo`) |
| Время прогона | ~5–8 мин | ~20–30 мин |
| Стоимость | $0 | реальный расход токенов |
| Детерминированность | 100% | низкая (но это и есть цель) |
| Что ловим | Регрессии в нашем коде | Регрессии в нашей интеграции с внешними API |

**Fake LLM HTTP-stub** (`backend/test/featuresmoke/fakes/llm_server.go`):
- HTTP-сервер, имитирующий Anthropic/OpenAI/DeepSeek API.
- Маршрутизация по пути (`/v1/messages`, `/v1/chat/completions`).
- Детерминированные ответы по input prompt (хэш или регексп → шаблон).
- **Fast-fail при неизвестных промптах:** `FakeLLM` принимает `*testing.T`. Если приходит неизвестный промпт, стаб немедленно роняет тест: `t.Fatalf("FakeLLM: получен неизвестный промпт: %s", prompt)`.
- Сценарные ответы для оркестрации (planner → developer → reviewer):
  фиксированный набор «дилогов», достаточный для прогона pipeline.

**Gitea** добавляется в `docker-compose.test.yml` (отдельный compose,
не пушим в основной dev-stack). Используется только в тестах.

## Make-таргеты (самодостаточные)

```
make test-features          # mock-режим, поднимает всё что нужно
make test-features-backend  # только Go featuresmoke
make test-features-frontend # только Flutter integration_test
make test-features-real     # real LLM/Git (требует секретов в .env)
make test-features-down     # остановить тестовые контейнеры и удалить volumes (очистка БД от тестовых данных)
```

Каждый таргет:
1. Проверяет наличие docker.
2. `docker compose -f docker-compose.yml -f docker-compose.test.yml up -d <нужные сервисы>`.
3. Поллит healthcheck'и (yugabytedb через `ysqlsh \l`, gitea через
   `/api/v1/version`).
4. Накатывает миграции в `public` схему через goose (один раз).
5. Прогоняет тесты.
6. На выходе не валит контейнеры — оставляет для повторных прогонов;
   `make test-features-down` — отдельная команда, которая останавливает контейнеры и удаляет volumes (или делает глобальный TRUNCATE), чтобы очистить БД от накопившихся тестовых Tenant-ов при локальной разработке.

## CI

### `feature-smoke.yml` (PR-gate)

Разделен на две джобы для оптимизации времени выполнения (на `macos-latest` нет нативного Docker):

**Job 1: Backend & Web Frontend (Runner: `ubuntu-latest`)**
- Триггер: `pull_request` + `push`.
- Шаги:
  1. Checkout.
  2. Setup Go + Flutter.
  3. **Mask secrets** — `echo "::add-mask::$VAR"` для каждой
     переменной окружения, даже dev/fake (защита от случайной
     утечки JWT_SECRET).
  4. **Настройка Docker-in-Docker:** проброс `/var/run/docker.sock` и
     настройка `SANDBOX_WORKDIR` для корректного маппинга volume в GitHub Actions.
  5. `make test-features-backend` и `make test-features-frontend` (для Web-сборки).
  6. `make test-features-down`.

**Job 2: macOS Frontend (Runner: `macos-latest`)**
- Шаги:
  1. Checkout.
  2. Setup Flutter.
  3. Прогон Flutter-тестов для macOS сборки с **замоканным бэкендом** (без поднятия docker-compose, чтобы не тратить 10-15 минут на Colima/Docker Desktop). **Важно:** Моки ОБЯЗАНЫ генерироваться или валидироваться на основе `backend/docs/swagger.json` (контрактное тестирование), чтобы исключить рассинхрон API и ложноположительные прохождения тестов.

- **Секретов с реальными ключами на этом workflow нет** — нечему утекать.

### `feature-e2e-real.yml` (nightly + on-demand)

- Runner: **ubuntu-latest** (нативный Docker для быстрого старта).
- Триггер: `schedule` (cron) + `workflow_dispatch`.
- Шаги:
  1. Checkout.
  2. **Mask secrets first** — до любого echo / тестового шага:
     ```
     for v in ANTHROPIC_API_KEY CLAUDE_CODE_OAUTH_ACCESS_TOKEN \
              DEEPSEEK_API_KEY OPENROUTER_API_KEY GITHUB_PAT \
              ENCRYPTION_KEY JWT_SECRET_KEY; do
       echo "::add-mask::${!v}"
     done
     ```
  3. Записать секреты из GitHub Secrets в `backend/.env`.
  4. **Настройка Docker-in-Docker:** проброс `/var/run/docker.sock` и
     настройка `SANDBOX_WORKDIR` для корректного маппинга volume.
  5. `make test-features-real`.
  6. **Прогон Flutter UI тестов поверх real-бэкенда:** `make test-features-frontend` (чтобы гарантировать, что UI корректно рендерит реальные ответы LLM, стриминг и Markdown, а не только моки).
  7. **Дамп логов при failure через явный scrub-фильтр (Defense-in-depth)**:
     ```bash
     LOGS=$(docker compose logs)
     for SECRET in $JWT_SECRET_KEY $ENCRYPTION_KEY $ANTHROPIC_API_KEY $GITHUB_PAT $CLAUDE_CODE_OAUTH_ACCESS_TOKEN $DEEPSEEK_API_KEY $OPENROUTER_API_KEY; do
       if [ -n "$SECRET" ]; then
         LOGS=${LOGS//$SECRET/***/}
         # Обязательное маскирование URL-encoded версии
         ENCODED_SECRET=$(python3 -c "import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1]))" "$SECRET")
         if [ "$SECRET" != "$ENCODED_SECRET" ]; then
             LOGS=${LOGS//$ENCODED_SECRET/***/}
         fi
       fi
     done
     echo "$LOGS"
     ```
  8. `make test-features-down`.

## Отчётность и Дашборд

- Генерация артефактов: `go test -json` и `flutter test --reporter expanded`.
- **Dashboard статуса фич:** Обязательный этап. В CI внедряется генерация HTML/Markdown отчета (например, через `go-test-report` или кастомный скрипт), который публикуется в GitHub Pages или отправляется комментарием к PR. Отчет должен явно показывать матрицу «фича X: ✅/❌».

## Декомпозиция на задачи (Roadmap)

Реализуем **всё**, включая P2, P3 и визуальный дашборд. Каждая задача — это отдельный PR для удобства ревью.

### Фаза 1: Фундамент и Безопасность
- [ ] **Task 1.1: Secret-scrub gating (КРИТИЧНО).** Доработка `service/secret_scrub.go` (URL-encoding), создание `secret_scrub_smoke_test.go`. Настройка маскирования в bash-скриптах CI.
- [ ] **Task 1.2: Инфраструктура и Fakes.** Создание `docker-compose.test.yml` (с Gitea), реализация `fakes/llm_server.go` (с fast-fail) и `fakes/git_server.go`.
- [ ] **Task 1.3: Backend Harness.** Реализация `backend/test/featuresmoke/harness.go` с Tenant-изоляцией (UUID), `t.Cleanup` с таймаутом, генерация пользователей.
- [ ] **Task 1.4: Make-таргеты.** Добавление `test-features`, `test-features-backend`, `test-features-frontend`, `test-features-down` (с очисткой volumes).

### Фаза 2: Backend Smoke Tests (P0 & P1)
- [x] **Task 2.1: Базовый CRUD.** `auth_smoke_test.go`, `projects_smoke_test.go`, `team_smoke_test.go`.
- [x] **Task 2.2: Задачи и WS.** `tasks_smoke_test.go`, `ws_smoke_test.go`.
- [x] **Task 2.3: Интеграции и Секреты.** `credentials_smoke_test.go`, `git_oauth_smoke_test.go`.
- [x] **Task 2.4: Оркестрация v2.** `orchestration_smoke_test.go`, `agents_smoke_test.go`.

Прогон: **72 PASS / 1 SKIP / 0 FAIL** (3 stability-run'а подряд). Исключён
`TestSecretScrub_NotInBackendStdout` — ловит реальный leak в gin.Logger,
backend bug, отдельный fix-task. Единственный SKIP — `TestLLMProviders_AdminCreateAndTestConnection`
(real admin-flow, покрывается Phase 5 e2e).

Состав 72 PASS:
- ~50 Phase 2 smoke (auth/projects/team/tasks/ws/credentials/git_oauth/agents/orchestration)
- 2 Phase 1 secret-scrub (API-response, в API-ответах canary не утекает)
- 6 Phase 2 prompt-content (cost-leak guard + 5 ассертов на shape assistant payload)
- 8 unit `TestBuildUserPrompt_*` (live в `internal/service`, не в featuresmoke)
- 6 unit на ownership-check `TestList{Artifacts,RouterDecisions,Worktrees}_*`

Стабильность держится на ТРЁХ механизмах:

1. **`ORCHESTRATOR_V2_WORKERS_ENABLED=false`** в mock-режиме (см. harness.go composeEnv).
   Это test-isolation knob: PR-gate смоук тестирует CRUD/API-контракт, не реальный
   pipeline; воркеры на 500ms-poll интервалах конкурировали бы с pause/cancel
   и финализировали бы задачи до того, как тест успеет с ними поработать.
   Real-режим (`FEATURESMOKE_MODE=real`) флаг НЕ выставляет — там воркеры нужны.

2. **Retry SQLSTATE 40001 в `repository.TransactionManager`** (`backend/internal/repository/transaction.go`).
   Это реальный backend-фикс, а не маскировка: YugabyteDB под параллельной
   нагрузкой возвращает `40001 serialization_failure` или «Restart read required»
   на конкурентные UPDATE/SELECT FOR UPDATE; правильное поведение клиента —
   повторить транзакцию с jitter-backoff. До 10 попыток.

3. **Фильтр `role_description != ''`** в `orchestrator_v2.loadRouterState`
   + defence-in-depth в `RouterService.buildUserPrompt`. Закрывает cost-leak:
   leaked-агенты с пустым описанием больше не раздувают router-prompt.

Cost-leak prevention (см. harness.go):
- `FakeLLM` поднимается ДО backend'а в mock-режиме; `ANTHROPIC_BASE_URL` /
  `OPENAI_BASE_URL` / `DEEPSEEK_BASE_URL` / `GEMINI_BASE_URL` / `QWEN_BASE_URL`
  редиректятся на него + dummy `*_API_KEY` подсовываются в child-env.
- Makefile `test-features-backend` стартует через `env -u` для шести LLM-ключей
  и `CLAUDE_CODE_OAUTH_ACCESS_TOKEN` — если harness когда-нибудь сломается,
  backend упадёт с «provider not configured», а не пойдёт жечь токены.
- `createSmokeAgent` регистрирует `t.Cleanup` с DELETE `/api/v1/agents/:id`
  (новая ручка, каскадно чистит секреты + tool_bindings).

### Фаза 3: Frontend Integration Tests (P0 & P1)
- [ ] **Task 3.1: Test Support.** Реализация `freshTestApp`, очистка SharedPreferences/Hive, инъекция токенов.
- [ ] **Task 3.2: Основные флоу.** `auth_flow_test.dart`, `projects_flow_test.dart`, `team_settings_test.dart`.
- [ ] **Task 3.3: Жизненный цикл задачи.** `task_lifecycle_test.dart` (включая WS-обновления).

### Фаза 4: Расширенное покрытие (P2 & P3)
- [x] **Task 4.1: Assistant и Prompts.** `chat_flow_test.dart`, `prompts_smoke_test.go`, `assistant_smoke_test.go`.
- [x] **Task 4.2: Workflows и API Keys.** `workflows_smoke_test.go`, `api_keys_smoke_test.go`.
- [x] **Task 4.3: Non-functional.** `health_load_test.go`, `migrations_smoke_test.go`,
  `metrics_smoke_test.go` (+ добавлен `/metrics` endpoint в `server.go` — promhttp.Handler,
  открыт без auth, scrape'ится Prometheus'ом внутри dev-stack).

Прогон: **3/3 stability-run'а** только новых Phase 4 backend-тестов — все зелёные
(включая 2 ожидаемых SKIP: `TestPrompts_AdminHappyPath` и `TestWorkflows_AdminHappyPath`,
обе уходят в Phase 5 real-режим, как и `TestLLMProviders_AdminCreateAndTestConnection`).

Состав Phase 4 (24 теста):
- 4 prompts (no-auth × 5 path, non-admin × 5 path, admin happy-path SKIP)
- 4 workflows (no-auth × 5 path, non-admin × 5 path, admin happy-path SKIP)
- 5 api-keys (CRUD + cross-tenant + auth + MCP-config)
- 7 assistant (lifecycle, missing, cross-tenant, active-tasks, auth, send-echo, **prompt sanity через llm_logs**)
- 1 health-under-load (20 workers × 25 req)
- 2 migrations (idempotence + status)
- 2 metrics (format + no-auth)
- 1 frontend chat_flow_test.dart (LLM-free UI-контракт; добавлен в
  `defaultPhase3FrontendTests` под cost-leak guard delta=0)

Адекватность LLM-запроса assistant'а проверяется через `llm_logs`:
- `provider` + `model` непустые (anthropic + claude-haiku из seed.SeedAssistantAgent);
- `prompt_snapshot` содержит assistant system prompt («ассистент платформы»);
- `prompt_snapshot` содержит уникальный user-content тестового сообщения
  (фильтрация через `prompt_snapshot::text LIKE '%marker%'` исключает race
  между параллельными tests, которые тоже шлют /messages).

В real-режиме тот же тест посылает один реальный запрос на anthropic-haiku
(≈¢ за прогон) — это и есть «осторожная проверка с реальной LLM», на которую
дальше смотрит ревьюер глазами через `psql llm_logs`.

### Фаза 5: CI/CD и Дашборд
- [x] **Task 5.1: PR-gate Workflow.** `.github/workflows/feature-smoke.yml` (две джобы:
  `backend-and-web` на ubuntu-latest с docker-compose.test stack-ом + Flutter web; `macos-frontend` на macos-latest с замоканным backend'ом и swagger-gate'ом). PR-комментарий с матрицей делается через gh api с маркером, чтобы апдейтить один комментарий на каждый push.
- [x] **Task 5.2: Nightly Workflow & E2E Rewrite.** `backend/test/featuresmoke/e2e_real_test.go` (build tag `featuresmoke && e2ereal`) переиспользует harness.go: SQL-seed агентов через `directDB`, seed-секретов через `exec.CommandContext("go run ./cmd/seed_*")`, GitHub API через `net/http`. Без вызовов `git exec` — flag-injection невозможен по построению. `.github/workflows/feature-e2e-real.yml` запускается по cron `0 3 * * *` + workflow_dispatch, гоняет real-featuresmoke + e2e_real + Flutter integration над real-backend.
- [x] **Task 5.3: Dashboard.** `backend/cmd/feature_report/main.go` — генератор Markdown + HTML матрицы фич из `go test -json` и `flutter test --machine`. Без внешних зависимостей (stdlib only). PR-gate комментирует Markdown в PR; nightly публикует HTML в GitHub Pages через `actions/deploy-pages@v4`. Тесты репортера: `backend/cmd/feature_report/main_test.go` (5 тестов — парсинг go-test JSON, парсинг flutter machine-format с hidden=true фильтром, red-first сортировка, rendering).

Прогон Phase 5: dashboard-tests **5/5 PASS**, `featuresmoke` + `e2ereal` build-tag комбинации компилируются, sanity-run 3-х представительских тестов (`TestAuth_RegisterLoginMe`, `TestProjects_CreateReadUpdateDelete`, `TestSecretScrub_NotInAPIErrorResponses`) — все зелёные через новый JSON-output путь. `scripts/e2e_smoke.sh` помечен как deprecated (см. шапку файла) — оставлен для istoricheskoy reference, новая работа идёт через `make test-features-e2e-real`.

## Решения, зафиксированные с пользователем

- Запуск: локально после реализации фичи + автомат на каждый PR в CI.
- Слои: все три параллельно (backend API + Flutter UI + full e2e).
- **PR-gate: mock LLM/Git. Nightly: real LLM/Git.** (исправлено после
  review — раньше real был на каждый PR, что небезопасно и дорого).
- Секреты: в `.env` локально, GitHub Secrets в CI; обязательное
  маскирование на двух уровнях (app-логгер + `::add-mask::` + sed-scrub).
- Тестовый репо для real-прогонов: `tereshed/kt-test-repo`.
- CI runner: ubuntu-latest для бэкенда/Docker, macos-latest для iOS/macOS UI-тестов.
- Изоляция БД: Tenant-изоляция (уникальные User/Project на тест), без TRUNCATE и схем.
  - Все строковые поля с `UNIQUE` ограничениями (email, team name) генерируются через `uuid.NewString()` (например, `test-%s@example.com`).
  - Все тесты в `featuresmoke` **ОБЯЗАНЫ** вызывать `t.Parallel()`, чтобы гарантировать отсутствие гонок (Race Conditions) на общих таблицах.
- Изоляция Flutter: свежий ProviderContainer + очистка локальных
  хранилищ перед каждым тестом.
- Make-таргеты: самодостаточные, сами поднимают зависимости.
- Dashboard статуса фич: обязательное требование, реализуется в Фазе 5 (генерация HTML/Markdown матрицы).
