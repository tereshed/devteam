# Dashboard Redesign — GCP Console Style

Ветка: `ui_refactoring`. Задача — переделать `/dashboard` и общий навигационный shell приложения в стиле Google Cloud Console и добавить разделы «Интеграции» (LLM + Git).

---

## 1. Цель

Дать пользователю единую панель управления, где он за несколько кликов:

1. Видит сводное состояние своего тенанта (проектов, агентов, подключений).
2. Управляет **проектами** (CRUD, привязка к git-репо).
3. Управляет **агентами** (CRUD, привязка к LLM и sandbox-настройкам).
4. Подключает **LLM-провайдеров**, включая Claude Code по OAuth-подписке (без ввода API-ключа).
5. Подключает **GitHub / GitLab** для авто-создания PR и доступа к репозиториям.

## 2. UX-референс — GCP Cloud Console

Берём ключевые паттерны:

- **Узкая левая колонка** (NavigationRail) с иконками + label, сворачивается в desktop-плотном режиме.
- **Группировка пунктов меню** по логическим секциям с разделителями (например, _«Главная»_ → _«Ресурсы»_ → _«Интеграции»_ → _«Администрирование»_).
- **AppBar сверху** с breadcrumb (`Главная / Интеграции / LLM`), поиском (заглушка пока), аватаром/меню пользователя.
- **Content area** — карточки или таблицы с консистентными отступами 16/24 px, status chips, чёткой типографикой Material 3.
- **Empty-states** с понятным CTA («Подключить первого провайдера»).
- **Тёмная и светлая тема** — оба варианта; пока используем существующий `core/theme/app_theme.dart`.

Не копируем:
- Облачные графики/биллинг — у нас другой домен.
- Кастомные дроп-дауны Google — Material 3 достаточно.

## 3. Информационная архитектура и маршруты

| Группа         | Пункт меню               | Маршрут                              | Статус               |
|----------------|--------------------------|--------------------------------------|----------------------|
| Главная        | Обзор                    | `/dashboard`                         | редизайн             |
| Ресурсы        | Проекты                  | `/projects`                          | переиспользуем       |
| Ресурсы        | Агенты                   | `/admin/agents-v2`                   | переиспользуем       |
| Ресурсы        | Воркtrees                | `/admin/worktrees`                   | переиспользуем       |
| Интеграции     | LLM-провайдеры           | `/integrations/llm` (новый)          | этап 2               |
| Интеграции     | Git-провайдеры           | `/integrations/git` (новый)          | этап 3               |
| Администрирование | Промпты               | `/admin/prompts`                     | переиспользуем       |
| Администрирование | Воркфлоу              | `/admin/workflows`                   | переиспользуем       |
| Администрирование | Запуски               | `/admin/executions`                  | переиспользуем       |
| Настройки      | Профиль                  | `/profile`                           | переиспользуем       |
| Настройки      | API-ключи (для MCP)      | `/profile/api-keys`                  | переиспользуем       |
| Настройки      | Глобальные               | `/settings`                          | переиспользуем       |

Пункты «Интеграции» — единственные **новые маршруты** в этом редизайне.

## 4. Wireframes (ASCII)

### 4.1. Shell (общий)

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ⌂ DevTeam     Главная / Интеграции / LLM-провайдеры        🔍   👤 ▾    │  ← AppBar
├────────┬─────────────────────────────────────────────────────────────────┤
│ ┌────┐ │                                                                 │
│ │ ⌂  │ │   <Content>                                                     │
│ │Обз │ │                                                                 │
│ ├────┤ │                                                                 │
│ │📁  │ │                                                                 │
│ │Прк │ │                                                                 │
│ │🤖  │ │                                                                 │
│ │Агн │ │                                                                 │
│ │📦  │ │                                                                 │
│ │Wtr │ │                                                                 │
│ ├────┤ │                                                                 │
│ │🔌  │ │                                                                 │
│ │LLM │ │                                                                 │
│ │🔗  │ │                                                                 │
│ │Git │ │                                                                 │
│ ├────┤ │                                                                 │
│ │⚙️  │ │                                                                 │
│ │Адм │ │                                                                 │
│ │👤  │ │                                                                 │
│ │Пр  │ │                                                                 │
│ └────┘ │                                                                 │
└────────┴─────────────────────────────────────────────────────────────────┘
```

Adaptive:
- **Desktop ≥ 1200dp** — расширенный NavigationRail с лейблами.
- **Tablet 600–1200dp** — свёрнутый rail (только иконки) + tooltip.
- **Mobile < 600dp** — Drawer по burger-кнопке в AppBar.

### 4.2. Dashboard `/dashboard`

```
┌────────────────────────────────────────────────────────────────┐
│  Главная > Обзор                                                │
│                                                                 │
│  Добро пожаловать, k.tereshin@icloud.com                       │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────┐│
│  │ 📁 Проекты  │  │ 🤖 Агенты   │  │ 🔌 LLM      │  │ 🔗 Git ││
│  │             │  │             │  │             │  │        ││
│  │   3 актив.  │  │  7 активн.  │  │  2 подкл.   │  │ 0 подк ││
│  │   1 архив   │  │  1 ошибка   │  │  Claude OK  │  │ Нет    ││
│  │             │  │             │  │             │  │        ││
│  │ → Управлять │  │ → Управлять │  │ → Управлять │  │ →Управ ││
│  └─────────────┘  └─────────────┘  └─────────────┘  └────────┘│
│                                                                 │
│  Последние задачи                                              │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ #42  Add login form         project-A   running          │ │
│  │ #41  Fix migration 031      backend     completed        │ │
│  │ #40  Refactor auth module   frontend    failed           │ │
│  │                                              [Все задачи →]│ │
│  └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

### 4.3. Интеграции / LLM `/integrations/llm`

```
Главная > Интеграции > LLM-провайдеры

┌──────────────────────────────────────────────────────────────┐
│ Подключённые сервисы                          [+ Подключить] │
├──────────────────────────────────────────────────────────────┤
│ ┌──────────┐ Claude Code (subscription)                       │
│ │  CC OAuth│ Status: ✅ подключён · истекает через 12 дней    │
│ │          │ [Обновить] [Отключить]                           │
│ └──────────┘                                                  │
│ ┌──────────┐ Anthropic API                                    │
│ │   key    │ Status: ✅ ключ задан · последняя проверка 5м    │
│ │          │ [Тест] [Обновить ключ] [Удалить]                 │
│ └──────────┘                                                  │
│                                                               │
│ Доступные провайдеры                                          │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│ │ OpenAI   │ │OpenRouter│ │ DeepSeek │ │  Zhipu   │          │
│ │ [Подкл.] │ │ [Подкл.] │ │ [Подкл.] │ │ [Подкл.] │          │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
└──────────────────────────────────────────────────────────────┘
```

Подключение API-key провайдера → диалог с полями ключа и опциональной `base_url`. Сохраняется через `POST /api/v1/me/llm-credentials` (тенант-уровень) и/или `POST /api/v1/llm-providers` (админский CRUD).

Подключение Claude Code → диалог с инструкцией + кнопка «Начать», которая дергает `POST /api/v1/claude-code/auth/init`, открывает `authorize_url` в системном браузере (через `url_launcher`), фронт ждёт callback (poll `/status`).

### 4.4. Интеграции / Git `/integrations/git`

```
Главная > Интеграции > Git-провайдеры

┌──────────────────────────────────────────────────────────────┐
│  GitHub                                                       │
│  Status: ❌ не подключён                                      │
│  Соединение даёт право: чтение репозиториев, push в PR-ветки  │
│  [Подключить GitHub]                                          │
├──────────────────────────────────────────────────────────────┤
│  GitLab                                                       │
│  Status: ❌ не подключён                                      │
│  Self-hosted GitLab: укажи host                               │
│  [Подключить GitLab]                                          │
└──────────────────────────────────────────────────────────────┘
```

## 4a. Обязательные сквозные требования

Применяются на каждом этапе. Если этап нарушит хоть один пункт — PR ревьюим до фикса.

### 4a.1. Безопасность

- **Шифрование секретов в БД.** Любой токен/ключ/credential, который пишется в Postgres/Yugabyte, проходит через `pkg/crypto.AESEncryptor` (AES-GCM) — это закреплено в `docs/rules/main.md` §1 (Orchestration v2). Plain-text запрещён.
  - Этап 2: `me/llm-credentials` — поле API-ключа шифруется тем же `AESEncryptor`, который уже используется для других секретов. Если репозиторий ещё пишет plain-text — это **блокирующий баг**, чиним до фронтового экрана.
  - Этап 3: `git_integration_credentials.access_token` / `refresh_token` / `byo_client_secret` — `BYTEA`, шифруются `AESEncryptor`. `byo_client_id` остаётся **`VARCHAR(255)` plain** — по спеке OAuth 2.0 это публичная константа, она и так светится в URL'е браузера; шифровать её — лишний CPU и помехи при отладке.
  - **AAD = id записи** (UUID первичный ключ). Так предписано `docs/rules/main.md` §2.3 п.5 — единая конвенция по всему проекту. Это защищает от атаки «вставить блоб от одного юзера в строку другого юзера»: AAD не совпадёт, GCM-тэг не пройдёт. Использовать `user_id|provider` как AAD — отступление от стандарта, делать **не нужно**.
  - Ключ — `INTEGRATION_TOKEN_ENC_KEY` (32 байта, base64) из env.
- **Валидация GitLab host + защита от DNS Rebinding (SSRF).** Один валидатор недостаточен — между «проверили хост» и «сделали HTTP» атакующий DNS-сервер может переключить ответ. Делаем **два** уровня защиты:
  1. **`validateGitProviderHost(raw string) (canonical string, allowedIPs []net.IP, err error)`** — синтаксис + первый DNS-резолв:
     - схема обязательна и только `https://` (для прода) или `http://` (только если `ENV != production`);
     - убираем trailing slash, нормализуем регистр;
     - запрещаем `userinfo` в URL (`https://attacker@example.com`);
     - резолвим DNS и блокируем приватные диапазоны (`127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16` link-local, `::1`, `fc00::/7`) — кроме `ENV != production`, где разрешаем `localhost` для локальной разработки;
     - возвращает canonical host **и** список разрешённых IP, прошедших проверку.
  2. **`safeGitHTTPClient(allowedIPs []net.IP) *http.Client`** — кастомный `http.Transport` с переопределённым `DialContext`:
     - При каждом dial — **не делаем повторный DNS-резолв**, а просто проверяем, что `host:port` из URL ведёт на один из `allowedIPs` (тех самых, что прошли validate).
     - Если резолвер вдруг вернул другой IP (DNS rebinding attempt) — `DialContext` возвращает ошибку, никакого TCP-соединения не открываем.
     - TLS-handshake продолжает идти по `host`-имени (для SNI и проверки сертификата) — это важно, мы не подменяем имя на IP.
  3. На каждый outbound HTTP-запрос к GitLab берём **свежевалидированные** `allowedIPs` (не доверяем БД-кэшу за дни) — то есть фактически перед каждым запросом делаем `validateGitProviderHost(savedHost)`, и используем `safeGitHTTPClient`. Дороже на DNS, но безопасно.
- **Отзыв токенов на стороне провайдера при Revoke.** Когда юзер жмёт «Отключить» в UI, мы **сначала** делаем запрос отзыва в GitHub/GitLab API, **потом** удаляем строку из БД. Иначе токен остаётся живым у провайдера — это дыра безопасности:
  - GitHub: `DELETE /applications/{client_id}/grant` с HTTP Basic (`client_id:client_secret`) — отзывает всю выданную авторизацию.
  - GitLab: `POST /oauth/revoke` с `token=<access_token>&client_id=...&client_secret=...`.
  - Если revoke-запрос к провайдеру **упал по сети** — мы пишем в логи (с redact) и **всё равно удаляем** локальную строку, но в UI показываем notice «токен удалён локально, но провайдер мог не подтвердить отзыв — отзовите его вручную в настройках GitHub/GitLab». Без UI-молчания.
- **Маскирование секретов в логах OAuth.** Любая ошибка OAuth-flow логируется через `internal/logging/redact` хэндлер (`docs/rules/backend.md` §2.3, существующий `backend/internal/logging/redact.go`). Сырые URL-ы (содержат `code`, `state`), bodies провайдеров (могут содержать `access_token`, `refresh_token`, `client_secret`), error-описания провайдеров — обязательно проходят через redact / `SafeRawAttr`. Никогда не делаем `slog.Info("oauth callback", "url", req.URL.String())` или `slog.Error(err.Error())` без проверки, что error-message не вкладывает в себя body провайдера.

### 4a.2. Соответствие правилам репозитория

- **Локализация (frontend).** Все новые виджеты получают строки через хелпер:
  ```dart
  final l10n = requireAppLocalizations(context, where: 'IntegrationProviderCard');
  ```
  (см. `frontend/lib/core/l10n/require.dart`). Запрещено `AppLocalizations.of(context)!` и хардкод строк (`docs/rules/frontend.md` §2.3, §5.2). Все новые лейблы попадают в `app_ru.arb` + `app_en.arb`.
- **Порядок кодогенерации.** Для всех этапов: после правок Freezed/Riverpod-аннотаций и `.arb`-файлов запускаем
  ```bash
  make frontend-codegen
  ```
  который под капотом делает `dart run build_runner build --delete-conflicting-outputs` и **затем** `flutter gen-l10n` — порядок важен (`docs/rules/frontend.md` §2.3). Ручной запуск `flutter gen-l10n` перед `build_runner` ломает билд.
- **Без gorm.AutoMigrate (backend).** GORM-модели (`backend/internal/models/*.go`) не содержат вызовов `AutoMigrate` ни в `init`, ни в DI-сборке. Схемой управляет только Goose (`make migrate-up`). Это явно прописано в `docs/rules/backend.md`.

### 4a.3. DRY — единый виджет интеграционной карточки

Вместо трёх специализированных карточек (`connected_provider_card`, `available_provider_card`, `git_provider_card`) создаём **один** переиспользуемый компонент:

```
frontend/lib/shared/widgets/integration_provider_card.dart
```

API виджета — параметрический:

```dart
class IntegrationProviderCard extends StatelessWidget {
  final Widget logo;                 // иконка/svg слева
  final String title;
  final String? subtitle;
  final IntegrationStatus status;    // connected | disconnected | error | pending
  final String? statusDetail;        // "истекает через 12 дней" / "Ошибка: ..."
  final List<IntegrationAction> actions; // [Тест, Обновить, Отключить]
  final VoidCallback? onTap;
}
```

`IntegrationStatus` — отдельный enum в `shared/widgets/integration_status.dart`, маппится на цвета через `Theme.of(context).colorScheme` (никаких `Colors.green/red` в виджете).

Специфичные «обёртки» — это **не отдельные виджеты**, а функции-конструкторы в feature-папках, которые собирают параметры:
- `llm/presentation/widgets/llm_provider_cards.dart` — `Widget claudeCodeCard(...)`, `Widget anthropicCard(...)`.
- `git/presentation/widgets/git_provider_cards.dart` — `Widget githubCard(...)`, `Widget gitlabCard(...)`.

### 4a.4. Realtime через EventBus → WebSocket (не поллинг)

Поллинг `/auth/status` каждые 2 секунды — нарушает `docs/rules/main.md` §7.3 (realtime через WS/SSE). Меняем подход:

1. **Бэкенд при OAuth-callback'е публикует доменное событие** в существующий `events.EventBus` (`backend/internal/domain/events/eventbus.go`). Тип события — `IntegrationConnectionChanged{UserID, Provider, Status, ConnectedAt, ExpiresAt}`. Регистрация — внутри `service.ClaudeCodeAuthService.HandleCallback` и `service.GitIntegrationService.HandleCallback`.
2. **HubBridge** (`backend/internal/ws/hubbridge.go`) уже транслирует события в WS-каналы — добавляем туда `case IntegrationConnectionChanged → publish to user-channel`.
3. **Frontend** подписывается на свой user-channel через существующий `websocket_service` (`frontend/lib/core/api/websocket_service.dart`) и обрабатывает событие в Riverpod-нотифаере экрана интеграций. Никакого таймера.
4. **Fallback:** при открытии экрана делаем **один** `GET /status` для инициализации стейта (не таймер, не поллинг). Если WS отвалился — UI показывает баннер «соединение потеряно, обновите страницу», как уже делается в других местах.

Bonus: единый канал интеграций даёт нам бесплатно обновление UI при смене статуса в другом устройстве/вкладке.

### 4a.5. Обработка ошибок OAuth (cancel / access_denied / network)

Каждый OAuth-flow (Claude Code, GitHub, GitLab) обязан корректно обработать на бэке и на фронте следующие случаи. Это часть acceptance-criteria каждого этапа.

| Случай                                         | Бэкенд                                                              | UI-стейт                                    |
|------------------------------------------------|---------------------------------------------------------------------|---------------------------------------------|
| Юзер нажал «Cancel» в OAuth-окне               | callback приходит с `?error=access_denied` → ранний return, событие `IntegrationConnectionChanged{Status: cancelled, Reason}` | «Доступ отклонён. Попробуйте снова.» + кнопка повтора |
| Провайдер вернул `error=*` (server_error и т.д.) | логируем, событие со `Status: failed, Reason`                       | «Не удалось подключить: <reason>»            |
| Истёк `state` (CSRF) / неверный `state`        | 400 + `error_code: invalid_state`, событие со `Status: failed`      | «Сессия подключения устарела. Начните заново.» |
| Сетевая ошибка при exchange code → token       | 502 + `error_code: provider_unreachable`, событие со `Status: failed`| «Провайдер недоступен. Повторите позже.»     |
| Юзер закрыл окно без callback'а                | таймаут стороны фронта (20 минут после init) — фронт сбрасывает локальный pending-state | После таймаута возвращаемся в `disconnected` |

Никаких бесконечных лоадеров. Каждое исключительное состояние имеет explicit UI с понятным текстом (через `requireAppLocalizations`) и кнопкой «Попробовать снова». Pending-state не должен висеть дольше 20 минут.

## 5. Этапы реализации

### Этап 1 — Frontend shell + dashboard hub  (1 PR, ~½ дня)

**Цель:** перевести приложение на новый shell, увидеть GCP-стилизованный layout. LLM/Git — заглушки.

**Файлы создать:**
- `frontend/lib/core/widgets/app_shell.dart` — главный layout: NavigationRail/Drawer + AppBar + breadcrumb + content slot. Принимает `child` от GoRouter ShellRoute.
- `frontend/lib/core/widgets/app_shell_destinations.dart` — список разделов меню (icon, label, route, группа).
- `frontend/lib/core/widgets/breadcrumb.dart` — компонент breadcrumb из route hierarchy.
- `frontend/lib/features/dashboard/presentation/screens/dashboard_screen.dart` — новый hub с 4 stat-карточками + блок «Последние задачи».
- `frontend/lib/features/dashboard/presentation/widgets/stat_card.dart` — переиспользуемая карточка раздела (icon, title, value, CTA).
- `frontend/lib/features/integrations/presentation/screens/llm_integrations_screen.dart` — **заглушка** «Скоро».
- `frontend/lib/features/integrations/presentation/screens/git_integrations_screen.dart` — **заглушка** «Скоро».

**Файлы изменить:**
- `frontend/lib/core/routing/app_router.dart` — обернуть авторизованную часть в `ShellRoute` с `AppShell`. Добавить маршруты `/integrations/llm` и `/integrations/git`.
- `frontend/lib/l10n/app_ru.arb` + `app_en.arb` — лейблы меню, заголовки секций, CTA.

**Скрин current dashboard** — удалить старый или преобразовать в редирект. Решим в процессе.

**Тесты:** widget-тест на `AppShell` (раскрытие/свёртывание rail по breakpoint), navigation тест на наличие 4 stat-карточек.

### Этап 2 — Экран LLM Integrations  (1 PR, ~1 день)

**Цель:** реальный экран подключения LLM, включая Claude Code OAuth.

**Бэкенд:** ничего нового — используем существующее:
- `GET /api/v1/llm-providers` (admin)
- `POST /api/v1/llm-providers` (admin create)
- `POST /api/v1/llm-providers/test-connection`
- `POST /api/v1/llm-providers/:id/health-check`
- `GET/POST /api/v1/me/llm-credentials`
- Полный набор `/api/v1/claude-code/auth/*`

**Бэкенд — проверка инвариантов перед стартом:**
- Убедиться, что `me/llm-credentials` шифрует поле API-ключа через `pkg/crypto.AESEncryptor` (см. §4a.1). Если нет — сначала миграция + правка репозитория, **затем** UI.
- Убедиться, что для всех LLM-провайдеров есть события в `EventBus` для смены статуса (нужны для §4a.4). Если нет — добавить (отдельный мини-PR перед фронтом).

**Файлы создать (frontend):**
- `frontend/lib/features/integrations/llm/data/llm_integrations_repository.dart` — Dio-обёртка под все эндпоинты выше.
- `frontend/lib/features/integrations/llm/data/llm_integrations_providers.dart` — Riverpod провайдеры (repository, async lists, **stream из WS** для статусов — см. §4a.4).
- `frontend/lib/features/integrations/llm/domain/llm_provider_model.dart` — Freezed `abstract class LlmProviderModel`.
- `frontend/lib/features/integrations/llm/domain/claude_code_status_model.dart` — Freezed модель статуса.
- `frontend/lib/features/integrations/llm/presentation/screens/llm_integrations_screen.dart` — основной экран (замена заглушки этапа 1).
- `frontend/lib/features/integrations/llm/presentation/widgets/llm_provider_cards.dart` — функции-конструкторы (`claudeCodeCard`, `anthropicCard`, ...), все возвращают **общий** `IntegrationProviderCard` из `shared/widgets/` (§4a.3).
- `frontend/lib/features/integrations/llm/presentation/widgets/connect_api_key_dialog.dart`
- `frontend/lib/features/integrations/llm/presentation/widgets/connect_claude_code_dialog.dart` — содержит OAuth-flow: init, открыть browser, **ждать события из WS** (не polling), показывать pending/cancelled/failed-стейты согласно §4a.5.

**Особенности Claude Code OAuth flow на desktop:**
- `url_launcher` уже в pubspec? Если нет — добавить.
- После `auth/init` бэк возвращает `authorize_url`. Открываем во внешнем браузере.
- Локальный callback бэка обрабатывает code, публикует доменное событие → фронт получает через WS (§4a.4). Один `GET /status` при открытии диалога — для инициализации, дальше слушаем WS.
- Pending-state с таймаутом 20 минут (§4a.5). По таймауту откатываемся в `disconnected`.

**Тесты:**
- Unit на repository (моки Dio responses).
- Widget на экран (loading / connected / empty states).

### Этап 3 — Git Integrations  (1 PR backend + 1 PR frontend, ~2–3 дня)

**Цель:** подключение GitHub и GitLab по OAuth с зашифрованным хранением токенов.

**Бэкенд:**

Миграция `043_create_git_integration_credentials.sql` (управляется только Goose, см. §4a.2):
```sql
CREATE TABLE git_integration_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(16) NOT NULL,  -- 'github' | 'gitlab'
    host            VARCHAR(255),          -- canonical, после validateGitProviderHost (§4a.1)
    access_token    BYTEA NOT NULL,        -- AES-GCM через pkg/crypto.AESEncryptor (§4a.1)
    refresh_token   BYTEA,
    token_expires_at TIMESTAMPTZ,
    scopes          TEXT[],
    external_user_id VARCHAR(255),
    external_username VARCHAR(255),
    -- BYO Application для self-hosted GitLab (см. oauth-setup-guide.md §5).
    -- Заполняются ТОЛЬКО для provider='gitlab' AND host IS NOT NULL.
    -- Для shared-приложений (github.com / gitlab.com) — NULL.
    --
    -- client_id по спецификации OAuth 2.0 — публичная константа (она светится в URL
    -- браузера на странице authorize), её НЕ шифруем.
    -- client_secret — шифруется AESEncryptor с AAD = id записи (см. §4a.1).
    byo_client_id     VARCHAR(255),
    byo_client_secret BYTEA,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_user_provider_host UNIQUE (user_id, provider, host),
    CONSTRAINT chk_provider CHECK (provider IN ('github', 'gitlab')),
    -- self-hosted GitLab требует BYO-кредов; shared-провайдеры не должны их иметь.
    CONSTRAINT chk_byo_required CHECK (
        (provider = 'gitlab' AND host IS NOT NULL AND byo_client_id IS NOT NULL AND byo_client_secret IS NOT NULL)
        OR
        (host IS NULL AND byo_client_id IS NULL AND byo_client_secret IS NULL)
    )
);
CREATE INDEX idx_git_creds_user ON git_integration_credentials(user_id);
```

Файлы:
- `backend/internal/models/git_integration_credential.go` — GORM-модель **без** `AutoMigrate` и без тегов миграций (§4a.2).
- `backend/internal/repository/git_integration_repository.go` — шифрование `access_token`/`refresh_token` через `pkg/crypto.AESEncryptor` на входе, расшифровка на выходе. AAD включает `user_id` + `provider` (доп. защита от подмены).
- `backend/internal/service/git_integration_service.go` — OAuth init/callback, **публикация `IntegrationConnectionChanged` в EventBus** на каждом изменении (§4a.4), обработка всех error-case'ов из §4a.5 (early return + событие со `Status: cancelled/failed`).
- `backend/internal/service/git_provider_host_validator.go` — функция `validateGitProviderHost` (§4a.1) + unit-тесты на private-IP блок-листы, schema, userinfo, trailing slash.
- `backend/internal/handler/git_integration_handler.go`
- `backend/internal/handler/dto/git_integration_dto.go`
- Регистрация в `backend/internal/server/server.go`:
  ```
  /api/v1/integrations/github/auth/{init,callback,status,revoke}
  /api/v1/integrations/gitlab/auth/{init,callback,status,revoke}
  ```
- MCP-инструмент (по правилам CLAUDE.md): `backend/internal/mcp/git_integrations_tool.go` (read-only — `list_git_integrations`).
- Swagger-аннотации + `make swagger`.
- Тесты: unit на service + integration на handlers.

ENV-новые:
```
GITHUB_OAUTH_CLIENT_ID=
GITHUB_OAUTH_CLIENT_SECRET=
GITLAB_OAUTH_CLIENT_ID=
GITLAB_OAUTH_CLIENT_SECRET=
INTEGRATION_TOKEN_ENC_KEY=  # base64, 32 bytes — для AES-GCM
```

**Фронтенд** (после бэкенда):
- `frontend/lib/features/integrations/git/data/git_integrations_repository.dart`
- `frontend/lib/features/integrations/git/data/git_integrations_providers.dart` — Riverpod-нотифаер, который **подписан на WS-stream** (§4a.4), а не на таймер.
- `frontend/lib/features/integrations/git/domain/git_integration_model.dart` — Freezed `abstract class GitIntegrationModel`.
- `frontend/lib/features/integrations/git/presentation/screens/git_integrations_screen.dart` (замена заглушки этапа 1).
- `frontend/lib/features/integrations/git/presentation/widgets/git_provider_cards.dart` — функции-конструкторы `githubCard`, `gitlabCard`, оборачивающие общий `IntegrationProviderCard` (§4a.3).
- `frontend/lib/features/integrations/git/presentation/widgets/connect_gitlab_host_dialog.dart` — поле host, **клиент-сайд валидация** (легковесная: схема + базовый формат) перед отправкой; источник истины — серверный `validateGitProviderHost` (§4a.1).
- Error-states из §4a.5 — отдельные UI-стейты в экране (`cancelled`, `failed`, `provider_unreachable`, `invalid_state`), у каждого свой ARB-ключ.

## 6. Открытые вопросы

1. ~~OAuth-приложения GitHub/GitLab — кто их регистрирует?~~ **Закрыто.** Модель: Shared OAuth App для github.com + gitlab.com (мы регистрируем `DevTeam (dev)` под localhost и `DevTeam` под `polymaths.work`). Для self-hosted GitLab — BYO (юзер регистрирует Application у себя, вводит `client_id`/`client_secret` в наш UI). Подробности и пошагово: [oauth-setup-guide.md](oauth-setup-guide.md).
2. **Storage уровня:** хранение OAuth-токенов — на уровне `user_id` или `tenant_id` (если есть multi-user в одном проекте)?
3. **Claude Code OAuth detail:** в `claude-code/auth/init` сейчас возвращается URL для какого flow — PKCE / device code / authorization_code? Это влияет на UX подтверждения (нужно ли вводить code обратно в форму).
4. **Удалить ли старый dashboard или оставить через флаг?** Если есть зависимости в скриншотах/тестах — оставить временно через feature-flag.
5. **Состояние шифрования `me/llm-credentials` сегодня** — поле API-ключа уже AES-GCM или ещё plain-text? Нужно подтвердить чтением кода репозитория `user_llm_credential_repository.go` до старта этапа 2 (см. §4a.1).
6. **Тон ARB-формулировок** — согласовать с тобой строки CTA («Подключить», «Отключить»), error-сообщения из §4a.5. ~40 ключей (RU+EN).

## 7. Acceptance Criteria (для каждого этапа)

**Сквозные (проверяем на каждом этапе):**
- Все новые виджеты используют `requireAppLocalizations(context, where: '<Component>')` — нет ни одного `AppLocalizations.of(context)!` (§4a.2).
- Нет хардкода строк в UI — `flutter analyze` + grep по новым файлам.
- `make frontend-codegen` запускается без ошибок и в правильном порядке (§4a.2).
- `make frontend-l10n-check` зелёный (паритет RU/EN ARB).

**Этап 1:**
- `/dashboard` рендерится с новым shell, sidebar показывает 9 пунктов.
- 4 stat-карточки кликабельны и ведут на соответствующие маршруты.
- Адаптивный layout: на 1200dp+ rail развернут, на 600–1200 свёрнут, на <600 — burger.
- `flutter analyze` чисто, `flutter test` зелёный.
- `IntegrationProviderCard` создан в `shared/widgets/` (§4a.3), используется в stat-карточках или в stub-экранах LLM/Git.

**Этап 2:**
- Можно подключить Anthropic-ключ через UI; в БД ключ хранится как **AES-GCM blob** (визуально проверить через `ysqlsh`: поле не должно быть похоже на JWT/sk-...). §4a.1.
- Можно пройти Claude Code OAuth до конца на macOS desktop (открывается браузер, статус-чип обновляется **через WS-событие**, не по таймеру). §4a.4.
- Все error-states из §4a.5 проверяемы: ручной cancel в OAuth-окне → UI «Доступ отклонён»; искусственный 502 от провайдера → «Провайдер недоступен»; устаревший state → «Сессия устарела».
- Empty-state корректен при нулевых подключениях.
- `make test-all` (бэк) и `flutter test` (фронт) зелёные.

**Этап 3:**
- Можно подключить GitHub через UI; токен сохранён зашифрованно — поле `access_token` в БД это валидный AES-GCM blob, не plain text (визуально проверить через `ysqlsh`: формат отличается от `ghp_...`). `byo_client_id` (для self-hosted GitLab) — plain VARCHAR. AAD у шифрования — `id` записи. §4a.1.
- Можно подключить self-hosted GitLab с указанием host; невалидные host'ы (private IP, http без явного dev-env, userinfo) отклоняются с понятной ошибкой и unit-тестами на validator. §4a.1.
- **DNS-rebinding защита проверена тестом.** Мокированный резолвер с rebind-сценарием — outbound HTTP падает в `DialContext`, не уходит на 127.0.0.1. §4a.1.
- **Отзыв токена у провайдера.** При нажатии «Отключить» — сначала HTTP-вызов revoke к GitHub/GitLab, потом DELETE строки. При сетевой ошибке revoke — локально удаляем, в WS-событии флаг `remote_revoke_failed: true`, UI показывает notice. §4a.1.
- **Маскирование в логах.** Тест: эмуляция ответа провайдера с `access_token=...` / `client_secret=...` в body — в захваченных логах эти значения не присутствуют. §4a.1.
- WS-событие `IntegrationConnectionChanged` приходит фронту при подключении/отключении из другой вкладки (§4a.4).
- **Resync при reconnect WS.** Тест: обрыв соединения + reconnect → фронт делает повторный `GET /status` и обновляет state, нет залипших `pending`. §4a.4.
- Все error-states из §4a.5 покрыты UI + handler-тестами.
- `make test-integration` зелёный, MCP-инструмент `list_git_integrations` работает, `make swagger` обновлён.

## 8. Что не делаем сейчас

- Биллинг / лимиты использования.
- Multi-tenant role management (роли беру существующие admin/user).
- Глубокая интеграция с GitHub Actions / GitLab CI — только OAuth + базовые операции через `pkg/gitprovider`.
- Перевод существующих экранов проектов/агентов в новый дизайн глубже, чем оборачивание в shell.
