# UI Refactoring — Task Breakdown

Декомпозиция [dashboard-redesign-plan.md](dashboard-redesign-plan.md) в последовательность атомарных задач. Каждая задача — отдельный PR (или мини-PR), размером ½–1 рабочий день.

## Обозначения

- 🟢 — можно начинать параллельно
- 🔒 — есть блокирующие зависимости (см. поле «Deps»)
- 🛡 — критично для безопасности (правила §4a.1)
- ⚡ — требует WS-инфраструктуры (§4a.4)
- 🌐 — затрагивает l10n (RU+EN ARB)

## Условия готовности (сквозно для каждой задачи)

Берутся из [§4a Обязательные сквозные требования](dashboard-redesign-plan.md#4a-обязательные-сквозные-требования) и применяются ко всему, что задача добавляет:

- ✅ Frontend-строки только через `requireAppLocalizations(context, where: '...')` ([§4a.2](dashboard-redesign-plan.md#4a2-соответствие-правилам-репозитория), `docs/rules/frontend.md` §2.3, §5.2).
- ✅ `make frontend-codegen` отрабатывает чисто (build_runner → gen-l10n, именно в таком порядке).
- ✅ `make frontend-l10n-check` зелёный (ru/en паритет, плейсхолдеры).
- ✅ `flutter analyze` чисто на изменённых файлах.
- ✅ Backend-модели без `gorm.AutoMigrate` ([§4a.2](dashboard-redesign-plan.md#4a2-соответствие-правилам-репозитория), `docs/rules/backend.md`).
- ✅ Никакого хардкода секретов или строк в коде.

---

## Этап 0 — Подготовка / блокирующие проверки

Цель — устранить «мины» до начала фронтовых работ. Обе задачи маленькие, ½ дня каждая.

### 0.1 🛡🔒 Аудит и фикс шифрования LLM-credentials

- **Deps:** —
- **Refs:**
  - [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность) — обязательное AES-GCM для всех секретов.
  - [Plan §6.5](dashboard-redesign-plan.md#6-открытые-вопросы) — открытый вопрос: проверить текущее состояние.
  - `docs/rules/main.md` §1 (Orchestration v2 — шифрование секретов).
- **Что сделать:**
  1. Прочитать `backend/internal/repository/user_llm_credential_repository.go` (или эквивалент).
  2. Если API-ключи хранятся plain-text — создать миграцию `0XX_encrypt_user_llm_credentials.sql`, добавить шифрование через `pkg/crypto.AESEncryptor` в repository, миграционный backfill для существующих записей.
  3. Если уже зашифровано — задокументировать в `docs/llm-providers.md`, что инвариант соблюдён, и закрыть задачу как verified.
- **AC:**
  - Поле API-key в БД — AES-GCM blob (визуально через `ysqlsh`: не похоже на `sk-...`).
  - Существующие записи мигрированы без потерь (если потребовался backfill).
  - `make test-integration` зелёный.
- **Блокирует:** все таски Этапа 2.

### 0.2 ⚡🔒 Подготовка WS-канала для `IntegrationConnectionChanged`

- **Deps:** —
- **Refs:**
  - [Plan §4a.4](dashboard-redesign-plan.md#4a4-realtime-через-eventbus-→-websocket-не-поллинг)
  - `backend/internal/domain/events/eventbus.go`
  - `backend/internal/ws/hubbridge.go`
  - `frontend/lib/core/api/websocket_events.dart`
- **Что сделать:**
  1. Добавить тип `IntegrationConnectionChanged` в `backend/internal/domain/events/` (поля: `UserID`, `Provider`, `Status`, `Reason?`, `ConnectedAt?`, `ExpiresAt?`).
  2. В `HubBridge` — кейс на этот тип события, маршрутизация в user-channel.
  3. Зеркальная Freezed-модель события на фронте в `core/api/websocket_events.dart` + декодер.
  4. Unit-тесты на маршалинг события и доставку через `HubBridge`.
- **AC:**
  - Backend-тест: `EventBus.Publish(IntegrationConnectionChanged{...})` доставляется до `Hub.SendToUser`.
  - Frontend-тест: десериализация события возвращает корректную модель.
  - `make test-integration` + `flutter test` зелёные.
- **Блокирует:** 2.2, 2.7, 3.8, 3.10.

---

## Этап 1 — Shell + Dashboard hub

Цель — увидеть новый layout. Никакой бизнес-функциональности, только каркас.

### 1.1 🟢 Универсальный `IntegrationProviderCard`

- **Deps:** —
- **Refs:** [Plan §4a.3](dashboard-redesign-plan.md#4a3-dry--единый-виджет-интеграционной-карточки), [Plan §4.3](dashboard-redesign-plan.md#43-интеграции--llm-integrationsllm), `docs/rules/frontend.md` §1.2 (декомпозиция виджетов), §2.1 (UI Kit в `shared/widgets`).
- **Файлы:**
  - `frontend/lib/shared/widgets/integration_provider_card.dart`
  - `frontend/lib/shared/widgets/integration_status.dart` (enum + цветовая мапа через `ColorScheme`)
  - `frontend/lib/shared/widgets/integration_action.dart` (Freezed-модель кнопки действия)
  - `frontend/lib/shared/widgets/integration_provider_card_test.dart`
- **AC:**
  - Виджет принимает все 4 состояния (`connected | disconnected | error | pending`) и корректно рендерит chip + actions.
  - Golden-тесты или базовые widget-тесты на каждое состояние.
  - Используется через `requireAppLocalizations` для всех своих внутренних строк (например, дефолтных лейблов).

### 1.2 🟢 `AppShell` + Destinations + Breadcrumb

- **Deps:** —
- **Refs:** [Plan §3](dashboard-redesign-plan.md#3-информационная-архитектура-и-маршруты), [Plan §4.1](dashboard-redesign-plan.md#41-shell-общий), `docs/rules/frontend.md` §2.2 (адаптивная вёрстка, breakpoints).
- **Файлы:**
  - `frontend/lib/core/widgets/app_shell.dart` — `Scaffold` + `NavigationRail`/`Drawer` + `AppBar`.
  - `frontend/lib/core/widgets/app_shell_destinations.dart` — статический список разделов (группа, icon, label-key, route).
  - `frontend/lib/core/widgets/breadcrumb.dart` — вычисление цепочки из `GoRouterState.matchedLocation`.
  - Widget-тесты на shell: раскрытие/свёртывание rail на breakpoints 600/1200.
- **AC:**
  - Desktop (≥1200) — rail развёрнут с лейблами.
  - Tablet (600–1200) — rail свёрнут (только иконки + tooltip).
  - Mobile (<600) — Drawer по burger.
  - Breadcrumb отражает иерархию маршрута.

### 1.3 🔒 Подключить `ShellRoute` в `app_router.dart` + новые маршруты-заглушки

- **Deps:** 1.2
- **Refs:** [Plan §3](dashboard-redesign-plan.md#3-информационная-архитектура-и-маршруты), существующий `frontend/lib/core/routing/app_router.dart`.
- **Файлы:**
  - `frontend/lib/core/routing/app_router.dart` — обернуть авторизованные маршруты в `ShellRoute(builder: AppShell)`.
  - `frontend/lib/features/integrations/presentation/screens/llm_integrations_screen.dart` — **заглушка** «Скоро» с `IntegrationProviderCard.disabled(...)` примерами.
  - `frontend/lib/features/integrations/presentation/screens/git_integrations_screen.dart` — **заглушка** с двумя карточками GitHub/GitLab в `disconnected` статусе.
- **AC:**
  - Все существующие маршруты (`/projects`, `/admin/*`, `/profile`, `/settings`) работают внутри shell.
  - `/integrations/llm` и `/integrations/git` доступны и показывают «Скоро» / disconnected карточки.
  - Sidebar подсвечивает активный пункт.

### 1.4 🔒🌐 Новый `/dashboard` — hub

- **Deps:** 1.1, 1.3
- **Refs:** [Plan §4.2](dashboard-redesign-plan.md#42-dashboard-dashboard).
- **Файлы:**
  - `frontend/lib/features/dashboard/presentation/screens/dashboard_screen.dart`
  - `frontend/lib/features/dashboard/presentation/widgets/stat_card.dart` — лёгкая обёртка/композиция вокруг `IntegrationProviderCard` или отдельный виджет если семантика расходится.
  - `frontend/lib/features/dashboard/presentation/providers/dashboard_summary_provider.dart` — Riverpod: счётчики проектов/агентов/LLM/git (агрегирует существующие репозитории).
  - Widget-тесты на dashboard: 4 карточки, переходы по тапу.
- **AC:**
  - Все 4 карточки кликабельны и ведут на соответствующие маршруты.
  - Блок «Последние задачи» показывает 5 последних (через существующий tasks-repository).
  - Empty-state, если данных нет.

### 1.5 🔒🌐 Локализация (RU + EN) — Этап 1

- **Deps:** 1.1–1.4 (накапливающаяся; можно делать на каждой подзадаче по чуть-чуть)
- **Refs:** [Plan §4a.2](dashboard-redesign-plan.md#4a2-соответствие-правилам-репозитория), `docs/rules/frontend.md` §2.3.
- **Файлы:**
  - `frontend/lib/l10n/app_ru.arb`, `app_en.arb` — все новые ключи навигации, breadcrumb, dashboard.
- **AC:**
  - `make frontend-l10n-check` зелёный (полный паритет, плейсхолдеры match).
  - Нет ни одного `Text('строка')` в новых файлах — только через `l10n.<key>`.

### 1.6 🔒 Acceptance Этапа 1

- **Deps:** 1.1–1.5
- **AC:**
  - Все [Acceptance Criteria Этапа 1](dashboard-redesign-plan.md#7-acceptance-criteria-для-каждого-этапа).
  - PR можно мержить в `ui_refactoring`.

---

## Этап 2 — LLM Integrations

Подключение LLM-провайдеров и Claude Code через OAuth.

### 2.1 🛡🔒 Бэкенд: гарантировать шифрование `me/llm-credentials`

- **Deps:** 0.1
- Если задача 0.1 закрыла этот вопрос — этот пункт превращается в no-op verify (проверка ещё раз). Если 0.1 только сделала аудит и нашла plain-text — миграционный фикс делается **тут**, перед UI.
- **Refs:** [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность).

### 2.2 ⚡🔒 Бэкенд: события `IntegrationConnectionChanged` для Claude Code OAuth

- **Deps:** 0.2
- **Refs:** [Plan §4a.4](dashboard-redesign-plan.md#4a4-realtime-через-eventbus-→-websocket-не-поллинг), [§4a.5](dashboard-redesign-plan.md#4a5-обработка-ошибок-oauth-cancel--access_denied--network).
- **Файлы:**
  - `backend/internal/service/claude_code_auth_service.go` — в `HandleCallback`:
    - на success → `EventBus.Publish(IntegrationConnectionChanged{Status: connected, ...})`
    - на `?error=access_denied` → `Publish(... Status: cancelled, Reason: "user_cancelled")`
    - на любую другую ошибку → `Publish(... Status: failed, Reason: <error>)`
  - Юнит-тесты на каждый из 4 кейсов из таблицы §4a.5.
- **AC:**
  - 4 теста зелёные.
  - Поллинг (если есть) удалён из `claude_code_auth_handler.go`.

### 2.3 🛡🔒 Бэкенд: error-cases в Claude Code callback handler

- **Deps:** 2.2
- **Refs:** [Plan §4a.5](dashboard-redesign-plan.md#4a5-обработка-ошибок-oauth-cancel--access_denied--network), [Plan §4a.1 — Маскирование секретов](dashboard-redesign-plan.md#4a1-безопасность).
- **Что сделать:**
  - Все 4 строки таблицы §4a.5 как явные ветки `Callback` хэндлера с понятными `error_code` в ответе.
  - **Маскирование в логах.** Любой `slog.*` внутри callback-флоу — через `internal/logging/redact`-хэндлер. Запрещено: `slog.Error(err.Error())`, `slog.Info("...", "url", req.URL.String())`, `slog.Info("...", "body", string(body))`. Использовать `SafeRawAttr` для тела ответа провайдера. error-сообщения от провайдера сначала прогонять через redact (если внутри обнаружится `access_token=...` / `client_secret=...` / `code=...`).
- **AC:**
  - Integration-тесты по каждой ветке.
  - Отдельный тест: эмулируем ответ провайдера с `access_token=xxx` в body — в захваченных логах строка `xxx` не присутствует ни в каком виде (через grep по log-output).

### 2.4 🟢 Фронт: модели + repository для LLM Integrations

- **Deps:** —
- **Refs:** [Plan §5 Этап 2](dashboard-redesign-plan.md#этап-2--экран-llm-integrations-1-pr-1-день).
- **Файлы:**
  - `frontend/lib/features/integrations/llm/domain/llm_provider_model.dart` (Freezed, `abstract class`)
  - `frontend/lib/features/integrations/llm/domain/claude_code_status_model.dart`
  - `frontend/lib/features/integrations/llm/data/llm_integrations_repository.dart` (Dio: `/llm-providers/*`, `/me/llm-credentials`, `/claude-code/auth/*`)
- **AC:** unit-тесты на repository (моки Dio).

### 2.5 🔒⚡ Фронт: Riverpod-провайдеры с подпиской на WS + resync при reconnect

- **Deps:** 0.2, 2.4
- **Refs:** [Plan §4a.4](dashboard-redesign-plan.md#4a4-realtime-через-eventbus-→-websocket-не-поллинг).
- **Файлы:**
  - `frontend/lib/features/integrations/llm/data/llm_integrations_providers.dart`.
- **Поведение:**
  1. При первом открытии экрана — `GET /status` для инициализации стейта.
  2. Дальше слушаем `IntegrationConnectionChanged` через существующий `websocket_service`.
  3. **При событии `onReconnect` от WS-клиента — повторный `GET /status`** для re-sync. Закрывает гэп: если бэк послал событие в момент обрыва WS, фронт его потерял; пере-fetch инициализации стейта гарантирует, что мы видим текущее состояние, а не залипший «pending».
- **AC:**
  - Провайдер ловит fake `IntegrationConnectionChanged` event из тестового WS, состояние мутирует.
  - Отдельный тест: эмулируем `disconnect → reconnect` WS-канала — `GET /status` вызывается повторно.

### 2.6 🔒🌐 Фронт: экран `llm_integrations_screen`

- **Deps:** 1.1, 1.3, 2.5
- **Файлы:**
  - `frontend/lib/features/integrations/llm/presentation/screens/llm_integrations_screen.dart` — секция «Подключённые», секция «Доступные», CTA.
  - `frontend/lib/features/integrations/llm/presentation/widgets/llm_provider_cards.dart` — функции-фабрики (`claudeCodeCard`, `anthropicCard`, `openAiCard`, `openRouterCard`, `deepseekCard`, `zhipuCard`) — возвращают сконфигурированный `IntegrationProviderCard` (§4a.3).
- **AC:** widget-тесты на 3 состояния экрана (loading, connected, empty).

### 2.7 🔒⚡🌐 Фронт: диалоги подключения

- **Deps:** 2.5, 2.6
- **Refs:** [Plan §4a.5](dashboard-redesign-plan.md#4a5-обработка-ошибок-oauth-cancel--access_denied--network).
- **Файлы:**
  - `frontend/lib/features/integrations/llm/presentation/widgets/connect_api_key_dialog.dart` — для Anthropic/OpenAI/etc. (поля: ключ + опц. base_url).
  - `frontend/lib/features/integrations/llm/presentation/widgets/connect_claude_code_dialog.dart` — Claude Code OAuth: init → `url_launcher` → WS-ожидание → success/cancelled/failed.
  - Если `url_launcher` нет в `pubspec.yaml` — добавить и запустить `make frontend-codegen`.
- **AC:**
  - На macOS desktop OAuth доходит до конца, UI обновляется без таймера.
  - Ручной cancel в OAuth-окне → UI «Доступ отклонён» (тест-сценарий).
  - Pending-state имеет таймаут 20 мин и откатывается в `disconnected`.

### 2.8 🔒🌐 Acceptance Этапа 2

- **Deps:** 2.1–2.7
- **AC:** все [Acceptance Этапа 2](dashboard-redesign-plan.md#7-acceptance-criteria-для-каждого-этапа), в том числе визуальная проверка AES-GCM в БД через `ysqlsh`.

---

## Этап 3a — Git Integrations: Backend

### 3.1 🛡🟢 Валидатор `validateGitProviderHost` + DNS-rebinding-safe HTTP-клиент

- **Deps:** —
- **Refs:** [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность).
- **Файлы:**
  - `backend/internal/service/git_provider_host_validator.go` — функция `validateGitProviderHost(raw string) (canonical string, allowedIPs []net.IP, err error)`.
  - `backend/internal/service/git_provider_safe_http.go` — `safeGitHTTPClient(allowedIPs []net.IP) *http.Client` с кастомным `http.Transport.DialContext`, который при каждом dial проверяет адрес из `host:port` против списка `allowedIPs` и **не делает повторный DNS-резолв** (защита от DNS Rebinding / TOCTOU). TLS-handshake по host'у (для SNI и certificate verification) — не подменяем имя на IP.
  - `*_test.go` для обоих.
- **AC:**
  - 100% покрытие веток валидатора: схема, trailing slash, userinfo, private IPs (v4 + v6), link-local, `localhost` вне prod допустим, в prod-режиме все приватные диапазоны и localhost отклоняются.
  - **DNS-rebinding тест:** мокаем резолвер, который при первом запросе отдаёт `8.8.8.8`, при втором — `127.0.0.1`. `validateGitProviderHost` отдаёт `[8.8.8.8]` как allowed; `safeGitHTTPClient.DialContext` при попытке резолва `127.0.0.1` (либо при попытке коннекта на «не-allowed» IP) возвращает ошибку, TCP не открывает.
  - Smoke-тест: оба компонента используются вместе → реальный outbound HTTP к публичному URL проходит; к URL с private-IP резолвом — нет.

### 3.2 🛡🔒 Миграция и модель `git_integration_credentials`

- **Deps:** 3.1
- **Refs:** [Plan §5 Этап 3 / SQL](dashboard-redesign-plan.md#этап-3--git-integrations-1-pr-backend--1-pr-frontend-23-дня), [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность), [Plan §4a.2](dashboard-redesign-plan.md#4a2-соответствие-правилам-репозитория).
- **Файлы:**
  - `backend/db/migrations/043_create_git_integration_credentials.sql` (раздельные `-- +goose StatementBegin/End` блоки для DDL, см. урок миграции 031).
  - `backend/internal/models/git_integration_credential.go` (без AutoMigrate).
- **AC:** `make migrate-up` чисто проходит на свежей БД, `make migrate-down` корректно откатывает.

### 3.3 🛡🔒 Repository с AES-GCM (AAD = id записи)

- **Deps:** 3.2
- **Refs:** [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность), `docs/rules/main.md` §2.3 п.5.
- **Файлы:**
  - `backend/internal/repository/git_integration_repository.go` — шифрование `access_token`, `refresh_token`, `byo_client_secret` через `pkg/crypto.AESEncryptor`. **AAD = id записи** (UUID PK, не `user_id|provider`) — это конвенция проекта по `docs/rules/main.md` §2.3 п.5. Защищает от cross-row substitution (вставить чужой блоб в свою строку — AAD не сойдётся).
  - `byo_client_id` — plain `VARCHAR(255)`, не шифруется (публичная константа по спеке OAuth 2.0).
  - Integration-тесты на round-trip шифрования.
- **AC:**
  - В БД `access_token`, `refresh_token`, `byo_client_secret` — binary blob, не похож на `ghp_...` / `gloas-...`.
  - `byo_client_id` — читаемый VARCHAR.
  - Тест cross-row substitution: подмена `access_token` блобом из другой строки → расшифровка падает с GCM-tag-mismatch.

### 3.4 🛡🔒⚡ `GitIntegrationService` для GitHub

- **Deps:** 3.3, 0.2
- **Refs:** [Plan §4a.1 — Маскирование секретов, Отзыв токенов](dashboard-redesign-plan.md#4a1-безопасность), [§4a.5](dashboard-redesign-plan.md#4a5-обработка-ошибок-oauth-cancel--access_denied--network).
- **Файлы:**
  - `backend/internal/service/git_integration_service.go` — `InitGitHub(ctx, userID)`, `HandleGitHubCallback(ctx, code, state)`, `Revoke(ctx, userID)`, `Status(ctx, userID)`.
  - Публикация `IntegrationConnectionChanged` на любое изменение (success/cancel/error), таблица §4a.5.
  - **`Revoke` обязан вызвать GitHub API на отзыв токена ДО удаления локальной строки.** Эндпоинт: `DELETE https://api.github.com/applications/{client_id}/grant` с HTTP Basic (`client_id:client_secret`) и body `{"access_token": "..."}`. Если запрос упал по сети — логируем (с redact) и **всё равно** удаляем локальную строку, но в WS-событии отдаём флаг `remote_revoke_failed: true`, фронт показывает соответствующее уведомление.
  - **Маскирование в логах.** Все `slog.*` внутри сервиса — через `internal/logging/redact` хэндлер. URL ответа провайдера, response body — через `SafeRawAttr`. Никогда: `slog.Info("github callback", "url", req.URL.String())` (содержит `code=...`), `slog.Error(err)` если `err` создан из тела GitHub-ответа (может содержать `access_token`).
- **AC:**
  - Unit-тесты на 4 OAuth-сценария.
  - Тест: `Revoke` сначала делает HTTP к GitHub revoke-эндпоинту, потом удаляет запись (проверка порядка через mock).
  - Тест: при сетевой ошибке revoke-вызова к GitHub — локальная строка всё равно удаляется, но WS-событие содержит `remote_revoke_failed: true`.
  - Тест маскирования: эмулируем ответ GitHub с `access_token=...` в body, проверяем что captured log не содержит `access_token` значения.

### 3.5 🛡🔒⚡ `GitIntegrationService` для GitLab.com (Shared)

- **Deps:** 3.3, 0.2
- **Refs:** [oauth-setup-guide.md §3-4](oauth-setup-guide.md), [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность).
- **Файлы:** тот же сервис + `gitlab_oauth_client.go`. Использует `GITLAB_OAUTH_CLIENT_ID/SECRET` из env (без host).
- **`Revoke` к GitLab.com:** `POST https://gitlab.com/oauth/revoke` с form-body `token=<access_token>&client_id=...&client_secret=...`. Тот же fail-soft, что и в 3.4 (`remote_revoke_failed: true` при ошибке сети).
- **Маскирование в логах:** аналогично 3.4 — через `internal/logging/redact`, response bodies через `SafeRawAttr`.
- **AC:**
  - Unit-тесты на 4 OAuth-сценария.
  - Тест порядка вызовов в `Revoke` (HTTP до DB).
  - Тест маскирования логов.

### 3.5b 🛡🔒⚡ `GitIntegrationService` для self-hosted GitLab (BYO)

- **Deps:** 3.1, 3.5
- **Refs:** [oauth-setup-guide.md §5](oauth-setup-guide.md), [Plan §4a.1 — DNS Rebinding, AAD, Маскирование, Revoke](dashboard-redesign-plan.md#4a1-безопасность).
- **Что сделать:**
  - Расширить `Init` дополнительными параметрами `host`, `byo_client_id` (plain), `byo_client_secret` (шифруется AESEncryptor с `AAD = id` записи — после INSERT возвращаем id, шифруем secret, делаем UPDATE; либо генерируем UUID до INSERT и сразу шифруем).
  - **Перед каждым outbound HTTP к BYO GitLab-инстансу:** вызвать `validateGitProviderHost(savedHost)`, получить свежий `allowedIPs`, выполнить запрос через `safeGitHTTPClient(allowedIPs)` (см. 3.1). Это закрывает DNS rebinding: между «сохранили host» и «делаем запрос» резолв мог измениться.
  - `Revoke` к BYO GitLab: `POST https://<host>/oauth/revoke` с BYO-кредами + access_token, тот же fail-soft.
  - Маскирование логов — особенно важно: `byo_client_secret` может оказаться в error-message провайдера; всё через redact.
- **AC:**
  - Unit-тесты: подключение к private-IP host отклоняется; невалидный URL (userinfo, http в проде) отклоняется.
  - **DNS-rebinding integration-тест:** мокаем DNS-резолвер, первый вызов отдаёт публичный IP (проходит validate), второй — `127.0.0.1` (попытка rebind); фактический HTTP запрос должен упасть в `DialContext`, а не выполниться на 127.0.0.1.
  - В БД `byo_client_secret`, `access_token`, `refresh_token` — AES-GCM blob; `byo_client_id` — plain VARCHAR (читается напрямую через `ysqlsh`).
  - Тест порядка revoke→delete и маскирования логов.

### 3.6 🔒 Handlers + DTOs + Routes + Swagger

- **Deps:** 3.4, 3.5
- **Refs:** `docs/rules/backend.md` (Clean Architecture, Swagger).
- **Файлы:**
  - `backend/internal/handler/git_integration_handler.go`
  - `backend/internal/handler/dto/git_integration_dto.go`
  - Регистрация маршрутов `/integrations/{github,gitlab}/auth/{init,callback,status,revoke}` в `server.go`.
  - `make swagger` обновляет `docs/`.
- **AC:** Swagger JSON содержит новые эндпоинты.

### 3.7 🔒 MCP-инструмент `list_git_integrations`

- **Deps:** 3.4
- **Refs:** `CLAUDE.md` (правило про обязательный MCP-инструмент для новых публичных ручек).
- **Файлы:** `backend/internal/mcp/git_integrations_tool.go` (read-only `list_git_integrations(user_id)`).
- **AC:** инструмент зарегистрирован и проходит smoke-тест.

### 3.8 🔒 Backend Acceptance Этапа 3a

- **Deps:** 3.1–3.7
- **AC:** `make test-integration` зелёный, Swagger обновлён, MCP работает.

---

## Этап 3b — Git Integrations: Frontend

### 3.9 🔒🟢 Модели + repository

- **Deps:** 3.6 (стабильное API)
- **Файлы:**
  - `frontend/lib/features/integrations/git/domain/git_integration_model.dart`
  - `frontend/lib/features/integrations/git/data/git_integrations_repository.dart`
- **AC:** unit-тесты на repository.

### 3.10 🔒⚡ Riverpod-провайдеры с WS + resync при reconnect

- **Deps:** 0.2, 3.9
- **Refs:** [Plan §4a.4](dashboard-redesign-plan.md#4a4-realtime-через-eventbus-→-websocket-не-поллинг).
- **Файлы:** `frontend/lib/features/integrations/git/data/git_integrations_providers.dart`.
- **Поведение** (зеркало 2.5):
  1. `GET /status` при первом открытии.
  2. Слушаем `IntegrationConnectionChanged` через `websocket_service`.
  3. **На `onReconnect` WS — повторный `GET /status`** для re-sync, чтобы не потерять события, отправленные во время обрыва.
- **AC:**
  - Провайдер реагирует на `IntegrationConnectionChanged{Provider: github|gitlab}`.
  - Тест: `disconnect → reconnect` WS-канала вызывает повторный `GET /status`.
  - Тест: если backend во время обрыва прислал состояние `connected`, после reconnect фронт переходит из `pending` в `connected` (через resync).

### 3.11 🔒🌐 Экран `git_integrations_screen`

- **Deps:** 1.1, 1.3, 3.10
- **Файлы:**
  - `frontend/lib/features/integrations/git/presentation/screens/git_integrations_screen.dart`
  - `frontend/lib/features/integrations/git/presentation/widgets/git_provider_cards.dart` (фабрики `githubCard`, `gitlabCard` → общий `IntegrationProviderCard`).
- **AC:** широкоформатный/мобильный layout оба корректные.

### 3.12 🔒🌐 Диалог GitLab self-hosted (BYO)

- **Deps:** 3.5b, 3.11
- **Refs:** [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность), [oauth-setup-guide.md §5](oauth-setup-guide.md).
- **Файлы:** `frontend/lib/features/integrations/git/presentation/widgets/connect_gitlab_host_dialog.dart`.
- **Поля диалога:** `host`, `client_id`, `client_secret` + раскрывающийся блок «Как зарегистрировать Application в моём GitLab» (текст из oauth-setup-guide.md §5, через `l10n`).
- **AC:**
  - Клиент-сайд lightweight валидация (схема https/http, формат host) — до отправки на сервер.
  - Серверная ошибка от `validateGitProviderHost` (private-IP, userinfo, etc.) отрисовывается понятным error-баннером.
  - Подключение успешно для валидного self-hosted GitLab (smoke-тест на моке).

### 3.13 🔒🌐 Acceptance Этапа 3b + общий wrap

- **Deps:** 3.9–3.12
- **AC:**
  - Все [Acceptance Этапа 3](dashboard-redesign-plan.md#7-acceptance-criteria-для-каждого-этапа).
  - WS-событие при подключении из другой вкладки обновляет UI первой вкладки (manual smoke).
  - `make test-all` (бэк) + `flutter test` (фронт) зелёные.
  - Все error-states из §4a.5 проверены вручную + покрыты тестами.

---

## Граф зависимостей (текст)

```
0.1 ─┐
     ├─→ 2.1 ─→ 2.8
0.2 ─┼─→ 2.2 ─→ 2.3 ─→ 2.8
     ├─→ 2.5 ─→ 2.6 ─→ 2.7 ─→ 2.8
     ├─→ 3.4 ─┐
     └─→ 3.5 ─┤
              ├─→ 3.6 ─→ 3.7 ─→ 3.8
3.1 ─→ 3.2 ─→ 3.3 ─┘     ↓
                          3.9 ─→ 3.10 ─→ 3.11 ─→ 3.12 ─→ 3.13
1.1 ─┐
1.2 ─┼─→ 1.3 ─→ 1.4 ─→ 1.5 ─→ 1.6
1.1 ─┘                ↓
                      (нужен также для 2.6 и 3.11)
2.4 ─→ 2.5 (см. выше)
```

## Параллелизация (что можно делать одновременно)

- **День 1 (старт):** 0.1, 0.2, 1.1, 1.2, 2.4, 3.1 — все «🟢» можно брать в работу одновременно (разные части кодовой базы).
- **После 0.2:** разблокируются все ⚡-задачи (2.2, 2.5, 3.4, 3.5, 3.10).
- **После 0.1:** разблокируется 2.1, дальше — весь Этап 2.
- **После 1.1+1.2:** разблокируется 1.3 → 1.4 → 1.5 → 1.6 (последовательно).

## Оценка трудоёмкости (грубо)

| Этап       | Задач | ~Дни (1 разраб)            |
|------------|-------|----------------------------|
| Этап 0     | 2     | 0.5–1                      |
| Этап 1     | 6     | 1–1.5                      |
| Этап 2     | 8     | 2–3                        |
| Этап 3a    | 8     | 2–2.5                      |
| Этап 3b    | 5     | 1–1.5                      |
| **Итого**  | **29**| **6.5–9.5 рабочих дней**   |
