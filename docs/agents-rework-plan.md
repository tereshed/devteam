# План: Переработка системы агентов

## Контекст

Текущая система агентов опирается на YAML-конфиги (`backend/agents/*.yaml`), которые загружаются при старте через `pkg/agentsloader/` и используются как шаблоны для параметров агентов в оркестраторе. Это внутренний механизм, не предназначенный для пользователя. Пользователь должен конфигурировать агентов через UI.

### Текущее состояние

| Что | Где | Проблема |
|-----|-----|----------|
| YAML-конфиги агентов | `backend/agents/*.yaml` | Внутренние настройки, недоступны пользователю |
| Загрузчик YAML | `backend/pkg/agentsloader/` | Дублирует данные с БД |
| Контекст-билдер | `orchestrator_context_builder.go` | Берёт model/temperature из YAML, а не из агента в БД |
| Seed assistant | `internal/seed/assistant.go` | Глобальный синглтон, не per-user |
| Seed system agents | Миграция 038 | Глобальные агенты без привязки к пользователю |

### Целевое состояние

- **YAML-конфиги удалены** — единственный источник конфигурации агентов: БД + UI.
- **Автосоздание агентов**: assistant при регистрации, orchestrator + router при создании проекта.
- **UI настройки агентов**: роль, тип, LLM, MCP-инструменты, скиллы.

---

## Фаза 1: Схема и миграции

### 1.1 Добавить `user_id` в таблицу agents

Сейчас агент привязан к `team_id` (проект) или NULL (системный). Для per-user assistant нужен `user_id`.

**Два класса агентов по мультипликации:**

| Класс | Роли | Правило |
|-------|------|---------|
| **Синглтон** | `assistant` (per-user), `orchestrator`, `router` (per-team) | Один на scope. Уникальность через partial unique index. |
| **Мульти-инстанс** | `developer`, `reviewer`, `tester`, `planner`, `decomposer`, `merger`, `worker`, `supervisor`, `devops` | Может быть несколько в одной команде. Уникальность — по имени (`uq_agents_team_name`). |

**Новая миграция:**
```sql
ALTER TABLE agents ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- Агент принадлежит ОДНОМУ из: user, team, или никому (системный).
ALTER TABLE agents ADD CONSTRAINT chk_agents_ownership
  CHECK (
    (user_id IS NOT NULL AND team_id IS NULL) OR   -- user-level (assistant)
    (user_id IS NULL AND team_id IS NOT NULL) OR    -- team-level (orchestrator, router, developers...)
    (user_id IS NULL AND team_id IS NULL)           -- system-level (legacy seed)
  );

-- Синглтон-роли: один assistant на пользователя.
CREATE UNIQUE INDEX idx_agents_user_singleton
  ON agents (user_id, role)
  WHERE user_id IS NOT NULL AND role IN ('assistant');

-- Синглтон-роли: один orchestrator/router на команду.
-- Мульти-инстанс роли (developer, reviewer, ...) НЕ попадают в этот индекс.
CREATE UNIQUE INDEX idx_agents_team_singleton
  ON agents (team_id, role)
  WHERE team_id IS NOT NULL AND role IN ('orchestrator', 'router');

-- Уникальность имени внутри команды уже есть (uq_agents_team_name).
-- Уникальность имени для user-level агентов:
CREATE UNIQUE INDEX idx_agents_user_name
  ON agents (user_id, name)
  WHERE user_id IS NOT NULL;

-- B-Tree индекс для GET /me/agents (фильтрация по user_id).
CREATE INDEX idx_agents_user_id ON agents (user_id) WHERE user_id IS NOT NULL;
```

> **Пояснение:** Мульти-инстанс агенты (developer, reviewer и т.д.) могут существовать в нескольких экземплярах в одной команде — например, `developer-backend`, `developer-frontend`, `reviewer-security`. Их уникальность гарантируется по имени (`uq_agents_team_name`), а не по роли.

### 1.2 Обновить CHECK ролей

Убедиться, что `chk_agents_role` содержит все нужные роли: `assistant`, `orchestrator`, `router`.

### 1.3 Ослабить CHECK `chk_agents_kind_requirements`

Текущий CHECK запрещает `model IS NULL` для LLM-агентов:
```sql
-- Текущий (миграция 031):
(execution_kind='llm' AND model IS NOT NULL AND code_backend IS NULL)
OR (execution_kind='sandbox' AND code_backend IS NOT NULL AND model IS NULL)
```

Это блокирует создание "ненастроенных" агентов. Нужно ослабить — разрешить LLM-агентам `model IS NULL` (состояние "не сконфигурирован"):

```sql
ALTER TABLE agents DROP CONSTRAINT chk_agents_kind_requirements;
ALTER TABLE agents ADD CONSTRAINT chk_agents_kind_requirements CHECK (
  (execution_kind = 'llm' AND code_backend IS NULL)
  OR (execution_kind = 'sandbox' AND code_backend IS NOT NULL AND model IS NULL)
);
```

Валидация полноты настроек (model + provider_kind заполнены) — **ответственность сервисного слоя** при попытке запуска агента, а не CHECK в БД. Это позволяет:
- Создавать агентов с пустыми настройками при регистрации/создании проекта.
- Запрещать запуск ненастроенного агента на уровне `AgentExecutor` / `orchestrator_context_builder`.

> **Data-миграция YAML → БД не нужна.** Системные агенты из миграции 038 уже имеют `model` и `temperature` в БД. Контекст-билдер переключается на чтение из БД (шаг 1.4), YAML становится не нужен.

### 1.4 Таблица `agent_role_prompts` — реестр дефолтных промптов

Отдельная таблица для стартовых промптов по ролям. Редактируется в админке. При автосоздании агента — промпт копируется из этой таблицы в `agent.system_prompt`.

**Миграция:**
```sql
CREATE TABLE agent_role_prompts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role        VARCHAR(50) NOT NULL UNIQUE,  -- AgentRole: assistant, orchestrator, router, ...
    content     TEXT NOT NULL,
    description TEXT,                          -- пояснение для админки ("Системный промпт ассистента")
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  UUID REFERENCES users(id)      -- кто последний раз редактировал
);

CREATE INDEX idx_agent_role_prompts_role ON agent_role_prompts (role);
```

**Seed при старте** — Go-константы как дефолты, ON CONFLICT DO NOTHING (уважаем правки админа):

```go
func SeedRolePrompts(ctx context.Context, db *gorm.DB) error {
    defaults := []models.AgentRolePrompt{
        {
            Role:        string(models.AgentRoleAssistant),
            Content:     assistantDefaultPrompt,
            Description: "Системный промпт ассистента пользователя",
        },
        {
            Role:        string(models.AgentRoleOrchestrator),
            Content:     orchestratorDefaultPrompt,
            Description: "Системный промпт оркестратора проекта",
        },
        {
            Role:        string(models.AgentRoleRouter),
            Content:     routerDefaultPrompt,
            Description: "Системный промпт роутера задач",
        },
        {
            Role:        string(models.AgentRoleDeveloper),
            Content:     developerDefaultPrompt,
            Description: "Системный промпт агента-разработчика",
        },
        {
            Role:        string(models.AgentRoleReviewer),
            Content:     reviewerDefaultPrompt,
            Description: "Системный промпт агента-ревьюера",
        },
        // ... другие роли по мере необходимости
    }
    for _, p := range defaults {
        res := db.Clauses(clause.OnConflict{
            Columns:   []clause.Column{{Name: "role"}},
            DoNothing: true,
        }).Create(&p)
        if res.Error != nil {
            // Логируем ошибки, отличные от конфликта уникальности.
            // ON CONFLICT DO NOTHING безопасен при конкурентном старте
            // нескольких инстансов (YugabyteDB гарантирует корректность).
            logger.Error("seed role prompt failed",
                slog.String("role", p.Role), slog.Any("error", res.Error))
        }
    }
    return nil
}
```

**Жизненный цикл промпта:**

```
Go-константа (код)  ──seed ON CONFLICT DO NOTHING──▶  agent_role_prompts (БД)
                                                            │
                                                     Админ редактирует
                                                     через UI / API
                                                            │
                    CreateDefaultAssistant() ◀── читает ────┘
                            │
                    agent.system_prompt = prompt.content
                            │
                    Пользователь может переопределить
                    через PUT /me/agents/:id
```

**API для админки:**

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/api/v1/admin/agent-role-prompts` | Список всех дефолтных промптов |
| `GET` | `/api/v1/admin/agent-role-prompts/:role` | Промпт по роли |
| `PUT` | `/api/v1/admin/agent-role-prompts/:role` | Обновить промпт (RBAC: admin) |

> Удаление не поддерживается — промпты seed'ятся заново при рестарте. Сброс к дефолту = удалить запись в БД, перезапустить backend.

### 1.5 Убрать зависимость от YAML в контекст-билдере

Контекст-билдер (`orchestrator_context_builder.go`) должен брать model, temperature, prompt из агента в БД, а не из YAML-кэша.

---

## Фаза 2: Автосоздание агентов

### 2.1 Фабричные методы в `agent_service.go`

Вся логика создания дефолтных агентов инкапсулирована в `AgentService`. `auth_service` и `project_service` не знают деталей конфигурации агентов — они вызывают сервис.

**Файл:** `internal/service/agent_service.go`

```go
// CreateDefaultAssistant создаёт per-user assistant, готового к работе
// после настройки LLM-провайдера. В отличие от project-агентов,
// assistant сразу получает системный промпт (из agent_role_prompts) и scoped MCP-доступ.
func (s *agentService) CreateDefaultAssistant(ctx context.Context, userID uuid.UUID) error {
    // Читаем дефолтный промпт для роли assistant из таблицы agent_role_prompts.
    prompt, err := s.rolePromptRepo.GetByRole(ctx, string(models.AgentRoleAssistant))
    if err != nil {
        return fmt.Errorf("default prompt for assistant: %w", err)
    }

    agent := &models.Agent{
        Name:               "assistant",
        Role:               models.AgentRoleAssistant,
        ExecutionKind:      models.AgentExecutionKindLLM,
        UserID:             &userID,
        IsActive:           true,
        InternalMCPEnabled: true,
        SystemPrompt:       &prompt.Content,
        // provider_kind, model, temperature — NULL.
        // Пользователь настроит через UI. До этого LLM-вызовы невозможны,
        // но системный промпт и MCP-доступ готовы.
    }
    if err := s.agentRepo.Create(ctx, agent); err != nil {
        return err
    }

    // Scoped API key для MCP: ограничивает blast radius —
    // даже при prompt injection ключ имеет scope "mcp" и привязан к user.
    return s.provisionMCPKey(ctx, agent.ID, userID)
}
```

#### 2.1.1 Scoped API key для MCP

При создании assistant генерируется **per-user MCP-ключ** с ограниченным scope:

```go
func (s *agentService) provisionMCPKey(ctx context.Context, agentID, userID uuid.UUID) error {
    // 1. Генерация ключа: wibe_ + 32 байта crypto/rand → hex.
    rawKey := generateAPIKey()  // "wibe_a1b2c3..."

    // 2. Сохраняем hash в api_keys с scope = "mcp".
    apiKey := &models.ApiKey{
        UserID:    userID,
        KeyHash:   sha256Hex(rawKey),
        KeyPrefix: rawKey[:16],
        Scopes:    `"mcp"`,
        ExpiresAt: nil, // бессрочный, отзывается при удалении агента
    }
    if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
        return err
    }

    // 3. Сохраняем raw key зашифрованным в agent_secrets.
    return s.agentSecretRepo.Create(ctx, &models.AgentSecret{
        AgentID:  agentID,
        KeyName:  "DEVTEAM_MCP_TOKEN",
        // encrypted via pkg/crypto.AESEncryptor (AAD = secret.ID)
    })
}
```

**Что это даёт:**

| Свойство | Значение |
|----------|---------|
| Scope | `"mcp"` — ключ работает **только** с MCP-эндпоинтами |
| Ownership | Привязан к `user_id` — видит только ресурсы пользователя |
| Хранение | Raw key зашифрован в `agent_secrets` (AES-256-GCM) |
| Отзыв | CASCADE: удаление агента → удаление secret → ключ бесполезен |
| Blast radius | Prompt injection → максимум MCP-операции от имени пользователя, не admin |

#### 2.1.3 Enforce scopes в MCP middleware

Текущий `NewAuthMiddleware` (mcp/auth.go) валидирует ключ, но **не проверяет scopes**. Нужно добавить:

```go
// В NewAuthMiddleware, после валидации ключа:
if apiKey.Scopes != `"*"` {
    // Проверяем что scope ключа разрешает MCP.
    if !scopeAllows(apiKey.Scopes, "mcp") {
        c.JSON(403, gin.H{"error": "key scope does not permit MCP access"})
        return
    }
}
```

#### 2.1.4 Default project agents (без MCP bootstrap)

```go
// CreateDefaultProjectAgents создаёт orchestrator + router для команды проекта.
// Каждый получает системный промпт из agent_role_prompts.
// LLM-настройки пустые — пользователь конфигурирует через UI.
func (s *agentService) CreateDefaultProjectAgents(ctx context.Context, teamID uuid.UUID) error {
    roles := []struct {
        name string
        role models.AgentRole
    }{
        {"orchestrator", models.AgentRoleOrchestrator},
        {"router", models.AgentRoleRouter},
    }
    for _, r := range roles {
        prompt, err := s.rolePromptRepo.GetByRole(ctx, string(r.role))
        if err != nil {
            return fmt.Errorf("default prompt for %s: %w", r.role, err)
        }
        agent := &models.Agent{
            Name:          r.name,
            Role:          r.role,
            ExecutionKind: models.AgentExecutionKindLLM,
            TeamID:        &teamID,
            IsActive:      true,
            SystemPrompt:  &prompt.Content,
        }
        if err := s.agentRepo.Create(ctx, agent); err != nil {
            return err
        }
    }
    return nil
}
```

### 2.2 При регистрации → assistant

**Файл:** `internal/service/auth_service.go` → метод `Register()`

```go
err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
    if err := s.userRepo.Create(txCtx, user); err != nil {
        return err
    }
    return s.agentService.CreateDefaultAssistant(txCtx, user.ID)
})
```

> **Состояние после регистрации:** assistant создан с системным промптом и MCP-доступом, но **LLM-настройки пустые** (provider_kind, model = NULL). Пока пользователь не подключит провайдера и не выберет модель, assistant не сможет обрабатывать сообщения. UI показывает онбординг: "Подключите LLM-провайдера, чтобы ассистент заработал."

### 2.3 При создании проекта → orchestrator + router

**Файл:** `internal/service/project_service.go` → метод `Create()`

```go
err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
    if err := s.projectRepo.Create(txCtx, project); err != nil {
        return mapProjectRepoErr(err)
    }
    team := &models.Team{
        Name:      devTeamDefaultName,
        ProjectID: project.ID,
        Type:      models.TeamTypeDevelopment,
    }
    if err := s.teamRepo.Create(txCtx, team); err != nil {
        return err
    }
    return s.agentService.CreateDefaultProjectAgents(txCtx, team.ID)
})
```

### 2.4 Удалить `SeedAssistantAgent`

- Удалить `internal/seed/assistant.go`
- Убрать вызов из `cmd/api/main.go`
- Миграция: удалить глобального assistant-агента (если он не привязан к user)

---

## Фаза 3: Удаление YAML-конфигов

> **Предусловие:** Фаза 1.4 (контекст-билдер читает из БД) уже выполнена.

### 3.1 Удалить файлы

```
backend/agents/*.yaml
backend/agents/agent_schema.json
backend/pkg/agentsloader/   (весь пакет)
```

### 3.2 Рефакторинг зависимостей

| Файл | Что делать |
|------|-----------|
| `cmd/api/main.go` | Убрать `agentsloader.NewCache(...)` и передачу кэша |
| `internal/service/orchestrator_context_builder.go` | Брать все параметры из агента в БД (`agent.Model`, `agent.Temperature` и т.д.) вместо `cfg.ModelConfig` |
| `internal/service/sandbox_launcher.go` (если есть) | Sandbox permissions из агента в БД |
| Тесты, использующие `agentsloader` | Рефакторить на моки/фикстуры агентов из БД |

### 3.3 Промпты

Промпты (`backend/prompts/*.yaml`) **остаются** — они не являются конфигурацией агентов, а содержат шаблоны системных промптов. Агент ссылается на промпт через `prompt_id`.

---

## Фаза 4: Backend API — настройки агентов

### 4.1 Расширить модель Agent

Добавить в модель и миграцию:
```go
// Флаг: подключить внутренний MCP DevTeam (для управления сущностями).
InternalMCPEnabled bool `gorm:"default:false" json:"internal_mcp_enabled"`
```

**Миграция:**
```sql
ALTER TABLE agents ADD COLUMN internal_mcp_enabled BOOLEAN NOT NULL DEFAULT false;
```

> **Безопасность (MCP-инструменты):** Конфигурация внешних MCP-серверов хранится в `code_backend_settings` (JSONB) — только метаданные: имена серверов, эндпоинты, типы транспорта. **Любые креды** (API-ключи, токены) для MCP-серверов хранятся **строго** в таблице `agent_secrets` и шифруются через `pkg/crypto.AESEncryptor` (формат: `[version 1b][nonce 12b][AES-256-GCM sealed]`). JSONB-поля **никогда** не содержат секретов.

### 4.2 Эндпоинты для user-level агентов

**Файл:** `internal/handler/agent_v2_handler.go`

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/api/v1/me/agents` | Список агентов текущего пользователя |
| `GET` | `/api/v1/me/agents/:id` | Детали агента пользователя |
| `PUT` | `/api/v1/me/agents/:id` | Обновить настройки агента |

Проектные агенты (orchestrator, router) продолжают работать через существующие эндпоинты teams/agents.

#### Авторизация (ABAC) — КРИТИЧНО

Каждый handler для `/me/agents/:id` **обязан** проверять принадлежность:

```go
func (h *AgentV2Handler) GetMyAgent(c *gin.Context) {
    userID := auth.UserIDFromCtx(c)
    agentID := uuid.MustParse(c.Param("id"))

    agent, err := h.agentService.GetByID(c.Request.Context(), agentID)
    if err != nil { ... }

    // ABAC: агент принадлежит текущему пользователю?
    if agent.UserID == nil || *agent.UserID != userID {
        c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
        return
    }
    ...
}
```

Для проектных агентов — аналогичная проверка: пользователь должен иметь доступ к project/team, которому принадлежит агент. Без этой проверки — IDOR-уязвимость.

#### Валидация DTO на входе

В DTO для `PUT /api/v1/me/agents/:id` добавить early-return валидацию:
- Запрещено передавать `team_id` (user-level агент не может быть привязан к команде)
- Запрещено менять `role` и `execution_kind` для автосозданных агентов (require recreate)
- При невалидном состоянии — `400 Bad Request` до обращения к БД

#### Swagger

После добавления ручек обязательно выполнить `make swagger` для обновления спецификации. Без этого фронтенд не сгенерирует Dio-клиенты.

#### MCP-инструменты

Создать MCP-инструмент `agent_update_my` в `internal/mcp/tools_agents_v2.go` для управления user-level агентами через assistant. Аналогично `agent_update`, но с проверкой `agent.UserID == currentUser.ID`.

### 4.3 Валидация провайдера

При сохранении LLM-настроек агента (provider_kind + model):
- Проверить, что у пользователя есть подключённый провайдер данного kind в `user_llm_credentials`
- Если нет — вернуть `422 Unprocessable Entity` с сообщением: "Провайдер {kind} не подключён. Подключите его в настройках."

---

## Фаза 5: Frontend — экраны настройки агентов

### 5.0 Стандарты фронтенда

Все экраны и модели этой фазы **строго** соблюдают правила `docs/rules/frontend.mdc`:

- **i18n:** хардкод строк в UI запрещён. Все тексты через `requireAppLocalizations(context, where: 'AgentConfigScreen')`. Новые ключи добавляются в `.arb`-файлы с последующим `flutter gen-l10n`.
- **Freezed:** все новые модели состояния — `@freezed abstract class` (напр. `abstract class AgentConfigState`). После добавления/изменения — `make frontend-codegen`.
- **Импорты:** только абсолютные (`package:frontend/...`), никаких `../`.
- **Riverpod 3.x:** провайдеры через `flutter_riverpod` ^3 с code generation.

### 5.1 Модели (Freezed)

```dart
@freezed
abstract class AgentConfigModel with _$AgentConfigModel {
  const factory AgentConfigModel({
    required String id,
    required String name,
    required String role,
    required String executionKind,
    String? providerKind,
    String? model,
    double? temperature,
    required bool internalMcpEnabled,
    required bool isActive,
    required List<AgentSkillModel> skills,
  }) = _AgentConfigModel;

  factory AgentConfigModel.fromJson(Map<String, dynamic> json) =>
      _$AgentConfigModelFromJson(json);
}
```

### 5.2 Структура экрана настройки агента

```
AgentConfigScreen
├── AgentRoleSection          — Роль (read-only для авто-созданных)
├── AgentTypeSection          — Тип: API (llm) / С бекендом (sandbox)
├── AgentLLMSettingsSection   — Провайдер, модель, температура
├── AgentMCPToolsSection      — Список MCP, галочка "DevTeam MCP"
└── AgentSkillsSection        — Список подключённых скиллов

Переменные окружения управляются отдельно:
├── ProjectVariablesSection   — в настройках проекта (для project-агентов)
└── UserVariablesSection      — в настройках пользователя (для assistant)
```

### 5.3 Секция "Роль"

- Выпадающий список: `assistant`, `orchestrator`, `router`, `planner`, `developer`, `reviewer`, `tester`, `decomposer`, `merger`
- Для автосозданных агентов (assistant, orchestrator, router) — read-only
- Для вручную созданных — изменяемый

### 5.4 Секция "Тип агента"

| Значение | ExecutionKind | Что показать |
|----------|---------------|-------------|
| API | `llm` | Секцию LLM-настроек |
| С бекендом | `sandbox` | Секцию code_backend + sandbox permissions |

Переключение типа скрывает/показывает соответствующие секции.

### 5.5 Секция "LLM настройки"

- **Провайдер**: dropdown из подключённых провайдеров пользователя (фильтр по `user_llm_credentials`). Если нет ни одного — показать локализованное сообщение со ссылкой на настройки.
- **Модель**: текстовое поле (или dropdown популярных моделей для выбранного провайдера).
- **Температура**: slider 0.0–2.0 с шагом 0.1, дефолт пустой.

### 5.6 Переменные окружения (уровень проекта / пользователя)

Переменные — единственный source of truth для секретов. Живут **на уровне проекта** (для project-агентов) и **на уровне пользователя** (для assistant). Все конфиги (MCP, скиллы, sandbox env) ссылаются на них через плейсхолдеры `${VAR_NAME}`.

#### Почему не per-agent

`GITHUB_TOKEN` один на проект — зачем дублировать в каждом developer'е и reviewer'е? Переменные на уровне проекта:
- Один раз задал — все агенты проекта видят
- Ротация токена — одно обновление, не N
- Нет рассинхрона между агентами

#### Два уровня

| Уровень | Для кого | Таблица | Пример |
|---------|----------|---------|--------|
| **Проект** | orchestrator, router, developer, reviewer... | `project_secrets` (**новая**) | `GITHUB_TOKEN`, `LINEAR_API_KEY` |
| **Пользователь** | assistant (вне проекта) | `user_secrets` (**новая**) | `PERSONAL_API_KEY` |

> `agent_secrets` остаётся для agent-specific секретов (например, `DEVTEAM_MCP_TOKEN` для scoped MCP-доступа assistant). Но пользовательские переменные — на уровне проекта/пользователя.

#### Схема резолвинга

```
Агент в проекте:                      User-level агент (assistant):

┌──────────────────┐                  ┌──────────────────┐
│  project_secrets │                  │  user_secrets    │
│  GITHUB_TOKEN=…  │                  │  SLACK_TOKEN=…   │
│  LINEAR_KEY=…    │                  └────────┬─────────┘
└────────┬─────────┘                           │
         │                                     │
         │  резолвится при BuildArtifacts()     │
         ▼                                     ▼
┌─────────────────────────────────────────────────┐
│  mcp_servers[].env:                             │
│    GITHUB_TOKEN: "${GITHUB_TOKEN}"              │  ← плейсхолдеры
│  skills[].config:                               │
│    token: "${LINEAR_KEY}"                       │
└─────────────────────────────────────────────────┘
```

#### Миграция

```sql
CREATE TABLE project_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key_name        VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_project_secrets_key UNIQUE (project_id, key_name),
    CONSTRAINT chk_project_secrets_key_format CHECK (key_name ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    CONSTRAINT chk_project_secrets_min_len CHECK (octet_length(encrypted_value) >= 29)
);

CREATE TABLE user_secrets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_name        VARCHAR(128) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_user_secrets_key UNIQUE (user_id, key_name),
    CONSTRAINT chk_user_secrets_key_format CHECK (key_name ~ '^[A-Z][A-Z0-9_]{0,127}$'),
    CONSTRAINT chk_user_secrets_min_len CHECK (octet_length(encrypted_value) >= 29)
);
```

#### API

**Project secrets** (доступ: участник проекта):

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/api/v1/projects/:id/secrets` | Список ключей (без значений) |
| `POST` | `/api/v1/projects/:id/secrets` | Создать/обновить переменную |
| `DELETE` | `/api/v1/projects/:id/secrets/:secret_id` | Удалить переменную |

**User secrets** (доступ: только владелец):

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/api/v1/me/secrets` | Список ключей (без значений) |
| `POST` | `/api/v1/me/secrets` | Создать/обновить переменную |
| `DELETE` | `/api/v1/me/secrets/:secret_id` | Удалить переменную |

#### UI

**В настройках проекта** → секция "Переменные":
```
ProjectVariablesSection
├── GITHUB_TOKEN    [••••••••] [✏️] [🗑]
├── LINEAR_API_KEY  [••••••••] [✏️] [🗑]
├── SLACK_WEBHOOK   [••••••••] [✏️] [🗑]
│
├── [+ Добавить переменную]
│       Имя: [___________]  (^[A-Z][A-Z0-9_]{0,127}$)
│       Значение: [___________]  (masked)
│
└── Подсказка: "Доступны всем агентам проекта через ${ИМЯ}"
```

**В настройках пользователя** → секция "Переменные" (для assistant):
```
UserVariablesSection  (аналогичный UI)
└── Подсказка: "Доступны вашему ассистенту через ${ИМЯ}"
```

#### Резолвинг плейсхолдеров

`BuildArtifacts()` получает scope (project или user) и резолвит `${VAR_NAME}`.

**Performance:** секреты загружаются **одним bulk-запросом** на весь проект перед началом резолвинга. Не один запрос на плейсхолдер.

```go
func (s *agentSettingsService) BuildArtifacts(ctx context.Context, agent *models.Agent, project *models.Project) (*BackendArtifacts, error) {
    // Bulk load: один запрос, расшифровка в память.
    var secrets map[string]string
    if agent.UserID != nil {
        secrets, _ = s.userSecretService.GetAllDecrypted(ctx, *agent.UserID)
    } else if project != nil {
        secrets, _ = s.projectSecretService.GetAllDecrypted(ctx, project.ID)
    }
    // ... резолвинг всех конфигов через resolveEnvPlaceholders(raw, secrets)
}
```

```go
func resolveEnvPlaceholders(raw map[string]string, secrets map[string]string) (map[string]string, []string) {
    var missing []string
    result := make(map[string]string, len(raw))
    for k, v := range raw {
        result[k] = placeholderRe.ReplaceAllStringFunc(v, func(ph string) string {
            name := ph[2 : len(ph)-1] // "${FOO}" → "FOO"
            if val, ok := secrets[name]; ok {
                return val
            }
            missing = append(missing, name)
            return ph
        })
    }
    return result, missing
}

var placeholderRe = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)
```

**Неразрешённые плейсхолдеры → ошибка запуска.** Если после резолвинга остались `missing` — `AgentExecutor` **не запускает** агента и возвращает внятную ошибку: `"Missing required environment variables: GITHUB_TOKEN, LINEAR_KEY. Configure them in project settings."`. Агент не должен пытаться авторизоваться строкой `"${GITHUB_TOKEN}"`.

#### Шифрование (DRY)

`project_secrets` и `user_secrets` используют **тот же** `pkg/crypto.AESEncryptor` с AAD = record ID, что и `agent_secrets`. Логика шифрования/расшифровки **не копипастится** в репозитории. Вместо этого — общий сервисный слой:

```go
// internal/service/secret_service.go — общая логика для всех типов секретов.
type SecretService struct {
    encryptor crypto.Encryptor
}

func (s *SecretService) Encrypt(id uuid.UUID, plaintext string) ([]byte, error) {
    return s.encryptor.Encrypt([]byte(plaintext), id[:]) // AAD = record ID
}

func (s *SecretService) Decrypt(id uuid.UUID, ciphertext []byte) (string, error) {
    plain, err := s.encryptor.Decrypt(ciphertext, id[:])
    return string(plain), err
}
```

`ProjectSecretService`, `UserSecretService`, `AgentSecretService` — тонкие обёртки над `SecretService` + свой репозиторий + ABAC-проверки. Бизнес-логика (валидация key_name, шифрование) в service-слое, **не в handler**.

#### Безопасность: редакция секретов в логах

В handler'ах `project_secret_set`, `user_secret_set` и MCP-инструментах `project_secret_set`, `user_secret_set` **запрещено логировать plaintext значения**. При логировании payload значение заменяется на `<redacted>`:

```go
logger.Info("secret set",
    slog.String("key_name", req.KeyName),
    slog.String("value", "<redacted>"),  // NEVER log plaintext
    slog.String("project_id", projectID.String()),
)
```

#### Валидация формата на фронтенде

Regex `^[A-Z][A-Z0-9_]{0,127}$` валидируется **и в БД** (CHECK constraint), **и на фронтенде** (в форме добавления переменной). При невалидном имени — локализованное сообщение об ошибке до отправки запроса на сервер.

#### Data-миграция существующих agent_secrets

Data-миграция `agent_secrets → project_secrets` **не требуется**. Причина: на текущей стадии (pre-production) нет реальных пользовательских секретов в `agent_secrets`, только тестовые данные. `agent_secrets` остаётся для agent-specific секретов (scoped MCP-токен assistant). Пользовательские переменные (`GITHUB_TOKEN` и т.д.) создаются с нуля в `project_secrets`/`user_secrets`.

> Если в будущем потребуется миграция (пост-MVP, production) — написать Go-миграцию через Goose, которая: 1) группирует agent_secrets по team_id → project_id, 2) дедуплицирует одинаковые ключи, 3) переносит в project_secrets.

### 5.7 Секция "MCP инструменты"

#### Текущее состояние

MCP-серверы настраиваются через **raw JSON редактор** в `AgentSandboxSettingsDialog` (таб "MCP"). Данные хранятся в `code_backend_settings.mcp_servers` (JSONB). Агент ссылается на MCP-сервер **по имени** из глобального реестра `mcp_servers_registry`. При сборке артефактов (`BuildArtifacts`) имя резолвится в полную конфигурацию.

#### Целевой UI

Заменяем JSON-редактор на form-based UI. **Env values в конфиге — только плейсхолдеры**, реальные значения берутся из секции "Переменные".

```
AgentMCPToolsSection
├── [✓] DevTeam MCP (internal)     ← toggle → agent.internal_mcp_enabled
│       Управление проектами, задачами, агентами
│
├── Внешние MCP-серверы:
│   ├── [✓] github                 ← toggle is_active
│   │       Transport: stdio, Scope: global
│   │       Env: GITHUB_TOKEN → ${GITHUB_TOKEN}  ← плейсхолдер, не значение
│   │
│   └── [✗] linear                 ← выключен
│           Transport: http, Scope: project
│
└── [+ Добавить MCP-сервер]        ← picker из mcp_servers_registry
        Dropdown: выбор из реестра (name, description)
        Показать требуемые env vars из registry.env_template
        Предложить создать переменные, если их нет в agent_secrets
```

**При добавлении MCP-сервера:**
1. Пользователь выбирает сервер из реестра (например, "github")
2. UI показывает требуемые переменные из `env_template` (например, `GITHUB_TOKEN`)
3. Если переменная уже есть в секции "Переменные" — подставляет `${GITHUB_TOKEN}` автоматически
4. Если переменной нет — предлагает создать (redirect на секцию "Переменные" или inline-форма)
5. В `code_backend_settings.mcp_servers` сохраняется:
```json
{"name": "github", "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}}
```
6. При `BuildArtifacts()` плейсхолдер резолвится в реальное значение из `agent_secrets`

**Механизм хранения:**

| Что | Где | Формат |
|-----|-----|--------|
| "DevTeam MCP включён" | `agents.internal_mcp_enabled` | Boolean |
| Подключённые серверы | `code_backend_settings.mcp_servers[]` | `{name, env: {KEY: "${KEY}"}}` — только плейсхолдеры |
| Реальные значения | `agent_secrets` | AES-256-GCM (секция "Переменные") |
| Каталог серверов | `mcp_servers_registry` | Глобальный реестр, read-only для пользователя |

**API-зависимости:**
- `GET /agents/:id/settings` — текущие настройки (env — плейсхолдеры, не значения)
- `PUT /agents/:id/settings` — обновить `code_backend_settings.mcp_servers`
- `GET /api/v1/admin/mcp-servers` — каталог серверов из реестра

#### 5.6.1 Админка: реестр MCP-серверов

Реестр `mcp_servers_registry` существует в БД, но **нет ни REST API, ни UI** для управления. Репозиторий read-only (List, GetByName). Нужно добавить полный CRUD.

**Backend API** (RBAC: admin):

| Метод | URL | Описание |
|-------|-----|----------|
| `GET` | `/api/v1/admin/mcp-servers` | Список серверов (с фильтром `?scope=global&only_active=true`) |
| `GET` | `/api/v1/admin/mcp-servers/:id` | Детали сервера |
| `POST` | `/api/v1/admin/mcp-servers` | Создать сервер |
| `PUT` | `/api/v1/admin/mcp-servers/:id` | Обновить сервер |
| `DELETE` | `/api/v1/admin/mcp-servers/:id` | Удалить сервер (soft delete → `is_active = false`) |

**Репозиторий** — расширить `MCPServerRegistryRepository`:
- Добавить `Create`, `Update`, `Delete` (soft)
- Валидация: name уникален, transport валиден, env_template — ключи UPPER_SNAKE_CASE

**Frontend** (admin only):

**Навигация:** Глобальные настройки → "MCP серверы"

```
MCPServersRegistryScreen
├── MCPServerTile (name: github, transport: stdio, scope: global, ✓ active)
├── MCPServerTile (name: linear, transport: http, scope: project, ✓ active)
└── [+ Добавить MCP-сервер]
    └── MCPServerEditDialog
        ├── Имя: [___________]
        ├── Описание: [___________]
        ├── Transport: ○ stdio  ○ http  ○ sse
        ├── (stdio) Command: [___________]  Args: [___________]
        ├── (http/sse) URL: [___________]
        ├── Scope: ○ global  ○ project  ○ agent
        ├── Env Template (ключи которые нужны агенту):
        │   ├── GITHUB_TOKEN: [описание]
        │   └── [+ Добавить ключ]
        └── Активен: [✓]
```

> **Важно:** `env_template` задаёт **какие ключи** нужны (имена + описания), а не значения. Значения вводит пользователь при подключении сервера к агенту и хранятся в `agent_secrets`.

### 5.7 Секция "Скиллы"

#### Текущее состояние

Скиллы хранятся двумя способами:
1. **Таблица `agent_skills`** — нормализованное хранение (agent_id, skill_name, skill_source, config_json, is_active).
2. **JSONB `code_backend_settings.skills`** — массив `{name, source, config}` (дублирует для sandbox-артефактов).

Текущий UI — raw JSON редактор в `AgentSandboxSettingsDialog` (таб "Skills").

#### Целевой UI

```
AgentSkillsSection
├── Подключённые скиллы:
│   ├── [✓] pdf (builtin)          ← toggle is_active
│   │       Работа с PDF-файлами
│   │
│   ├── [✓] xlsx (builtin)         ← toggle is_active
│   │       Работа с Excel-файлами
│   │
│   └── [✗] custom_lint (path)     ← выключен
│           Path: /workspace/.skills/lint.md
│
└── [+ Добавить скилл]
        Источник: ○ Builtin  ○ Plugin  ○ Path
        Имя: [___________]
        Конфиг (если path/plugin): [___________]
```

**Механизм хранения:**

| Что | Где |
|-----|-----|
| Скиллы агента | `agent_skills` таблица (source of truth) |
| Копия для sandbox | `code_backend_settings.skills[]` (синхронизируется при `BuildArtifacts`) |

**API-зависимости:**
- MCP-tool `skill_list` — доступные скиллы (с фильтром по agent_id)
- `PUT /agents/:id/settings` — обновить skills в code_backend_settings
- Синхронизация `agent_skills ↔ code_backend_settings.skills` — при сохранении через UI обновляются оба

### 5.8 Админка: дефолтные промпты агентов

Доступна только пользователям с `role = admin`.

**Навигация:** Глобальные настройки → "Промпты агентов"

**Экран:** Список ролей с текущим промптом (preview первые ~100 символов). По клику — редактор:

```
AgentRolePromptsScreen
├── RolePromptTile (role: assistant, preview: "Ты — ассистент платформы...")
├── RolePromptTile (role: orchestrator, preview: "Ты — оркестратор проекта...")
├── RolePromptTile (role: router, preview: "Ты — роутер задач...")
└── ...
    └── RolePromptEditDialog
        ├── Роль (read-only)
        ├── Описание (read-only)
        └── Промпт (multiline TextField, сохранение через PUT)
```

> Промпт влияет только на **вновь создаваемых** агентов. Уже существующие агенты хранят свою копию в `agent.system_prompt` и могут быть изменены через свой экран настройки.

### 5.10 Навигация

- **Настройки пользователя** → "Мой ассистент" → AgentConfigScreen (user-level assistant)
- **Проект** → "Настройки" → "Агенты" → список агентов проекта → AgentConfigScreen (per-agent)
- **Глобальные настройки** (admin) → "Промпты агентов" → AgentRolePromptsScreen
- **Глобальные настройки** (admin) → "MCP серверы" → MCPServersRegistryScreen

---

## Фаза 6: Тесты

### 6.1 Unit-тесты (backend)

| Файл теста | Что покрывает |
|------------|---------------|
| `agent_service_test.go` | `CreateDefaultAssistant` (agent + prompt из `agent_role_prompts` + MCP key), `CreateDefaultProjectAgents` (prompt из таблицы), валидация провайдера (422 если не подключён), запрет изменения role/execution_kind, запуск ненастроенного агента (model=NULL) → ошибка |
| `agent_v2_handler_test.go` | ABAC: доступ к чужому агенту → 403, DTO-валидация: team_id в /me/agents → 400 |
| `role_prompt_handler_test.go` | RBAC: non-admin → 403, admin GET/PUT, обновление промпта |
| `mcp_server_registry_handler_test.go` | CRUD реестра MCP: создание, обновление, soft delete, RBAC admin |
| `auth_service_test.go` | Регистрация создаёт user + assistant + scoped API key в одной транзакции; rollback при ошибке |
| `project_service_test.go` | Создание проекта создаёт team + orchestrator (с промптом) + router (с промптом); rollback при ошибке |
| `mcp_auth_middleware_test.go` | Scope enforcement: ключ с scope "mcp" → доступ к MCP ✓, ключ без scope "mcp" → 403 |

### 6.2 Integration-тесты (backend)

| Файл теста | Что покрывает |
|------------|---------------|
| `agents_me_integration_test.go` | Полный CRUD `/me/agents` с реальной БД: создание при регистрации, получение, обновление, ABAC между двумя пользователями |
| `project_agents_integration_test.go` | Создание проекта → проверка что orchestrator и router существуют в team |

### 6.3 Frontend-тесты

- Widget-тесты для `AgentConfigScreen` (секции рендерятся корректно, toggle скрывает/показывает)
- Provider-тесты для `agentConfigProvider` (загрузка, обновление, обработка ошибок)

---

## Фаза 7: Онбординг

### 7.1 После регистрации

Показать подсказку: локализованное сообщение о необходимости настроить ассистента — подключить LLM-провайдера и выбрать модель.

### 7.2 После создания проекта

Показать подсказку: локализованное сообщение о настройке агентов orchestrator и router для запуска оркестрации.

---

## Порядок реализации

| Шаг | Фаза | Что | Зависимости |
|-----|-------|-----|-------------|
| 1 | 1.1 | Миграция: `user_id` в agents, индексы, CHECK ownership | — |
| 2 | 1.2 | Обновить CHECK ролей | Шаг 1 |
| 3 | 1.3 | Миграция: ослабить `chk_agents_kind_requirements` (model NULL для LLM) | Шаг 1 |
| 4 | 1.4 | Миграция: таблица `agent_role_prompts` | — |
| 5 | 5.6 | Миграция: таблицы `project_secrets`, `user_secrets` | — |
| 6 | — | Go-модели: Agent (UserID), AgentRolePrompt, ProjectSecret, UserSecret + репозитории | Шаги 1–5 |
| 7 | 1.4 | Seed `SeedRolePrompts` в `main.go` (ON CONFLICT DO NOTHING) | Шаги 4, 6 |
| 8 | 1.4 | API `/admin/agent-role-prompts` (GET, PUT) + RBAC admin | Шаги 6, 7 |
| 9 | 5.6 | API `/projects/:id/secrets` + `/me/secrets` (CRUD) | Шаг 6 |
| 10 | 2.1 | Фабричные методы `agent_service.go` + `provisionMCPKey` | Шаги 6, 7 |
| 11 | 2.1.2 | Scope enforcement в `mcp/auth.go` | Шаг 6 |
| 12 | 1.5 | Контекст-билдер: читать model/temperature из БД, а не YAML | Шаг 6 |
| 13 | 5.6 | `BuildArtifacts`: резолвинг `${VAR}` из project_secrets / user_secrets | Шаги 6, 9 |
| 14 | 2.2 | Автосоздание assistant при регистрации | Шаги 6, 10, 11 |
| 15 | 2.3 | Автосоздание orchestrator + router при создании проекта | Шаги 6, 10 |
| 16 | 2.4 | Удалить `SeedAssistantAgent`, обновить `main.go` | Шаг 14 |
| 17 | 3.1–3.2 | Удалить YAML-конфиги и `agentsloader`, рефакторить зависимости | Шаги 10, 12, 15 |
| 18 | 4.1 | Миграция: `internal_mcp_enabled` | Шаг 1 |
| 19 | 4.2 | API `/me/agents` + ABAC + DTO-валидация + Swagger + MCP-инструмент | Шаги 6, 10 |
| 20 | 4.3 | Валидация провайдера при настройке агента | Шаг 19 |
| 21 | 5.7.1 | Backend: CRUD API `/admin/mcp-servers` + расширить MCPServerRegistryRepository | Шаг 6 |
| 22 | 6.1–6.2 | Backend: unit + integration тесты | Шаги 8–21 |
| 23 | 5.0–5.10 | Frontend: экраны настройки агентов + переменные проекта/пользователя + админки | Шаги 8, 9, 19–21 |
| 24 | 6.3 | Frontend-тесты | Шаг 23 |
| 25 | 7 | Онбординг-подсказки | Шаги 23–24 |

---

## Затронутые файлы (ключевые)

### Backend — миграции
- `db/migrations/` — user_id в agents, ослабление CHECK, `agent_role_prompts`, `project_secrets`, `user_secrets`, `internal_mcp_enabled`

### Backend — новые файлы
- `internal/models/agent_role_prompt.go` — модель AgentRolePrompt
- `internal/models/project_secret.go` — модель ProjectSecret
- `internal/models/user_secret.go` — модель UserSecret
- `internal/repository/agent_role_prompt_repository.go` — GetByRole, List, Update
- `internal/repository/project_secret_repository.go` — CRUD (тонкий, шифрование в service)
- `internal/repository/user_secret_repository.go` — CRUD (тонкий, шифрование в service)
- `internal/service/secret_service.go` — **общая** логика шифрования/расшифровки (DRY: один Encryptor для всех типов секретов)
- `internal/handler/agent_role_prompt_handler.go` — admin API дефолтных промптов
- `internal/handler/project_secret_handler.go` — API `/projects/:id/secrets`
- `internal/handler/user_secret_handler.go` — API `/me/secrets`
- `internal/handler/mcp_server_registry_handler.go` — admin CRUD `/admin/mcp-servers`
- `internal/seed/role_prompts.go` — SeedRolePrompts

### Backend — изменённые файлы
- `internal/models/workflow.go` — UserID, InternalMCPEnabled
- `internal/service/agent_service.go` — фабрики, provisionMCPKey, валидация
- `internal/service/agent_settings_service.go` — резолвинг `${VAR}` из project_secrets/user_secrets
- `internal/service/auth_service.go` — CreateDefaultAssistant в транзакции
- `internal/service/project_service.go` — CreateDefaultProjectAgents в транзакции
- `internal/service/orchestrator_context_builder.go` — убрать зависимость от YAML
- `internal/mcp/auth.go` — scope enforcement
- `internal/handler/agent_v2_handler.go` — `/me/agents` + ABAC
- `internal/repository/mcp_server_registry_repository.go` — расширить CRUD
- `internal/mcp/tools_agents_v2.go` — MCP-инструмент `agent_update_my`
- `cmd/api/main.go` — убрать agentsloader, добавить SeedRolePrompts

### Backend — удалить
- `internal/seed/assistant.go`
- `pkg/agentsloader/` (весь пакет)
- `agents/*.yaml`
- `agents/agent_schema.json`

### Backend тесты
- `internal/service/agent_service_test.go` — фабрики и валидация
- `internal/handler/agent_v2_handler_test.go` — ABAC, DTO-валидация
- `internal/handler/project_secret_handler_test.go` — CRUD + ABAC project secrets
- `internal/handler/user_secret_handler_test.go` — CRUD + ABAC user secrets
- `internal/handler/mcp_server_registry_handler_test.go` — CRUD + RBAC admin
- `internal/service/auth_service_test.go` — регистрация + assistant
- `internal/service/project_service_test.go` — создание проекта + agents
- `tests/integration/agents_me_integration_test.go` — /me/agents
- `tests/integration/project_agents_integration_test.go` — project agents
- `tests/integration/project_secrets_integration_test.go` — плейсхолдер-резолвинг

### Frontend
- `lib/features/agents/` — AgentConfigScreen, секции LLM, MCP, Skills (Freezed + Riverpod 3.x)
- `lib/features/projects/presentation/widgets/project_variables_section.dart` — переменные проекта
- `lib/features/settings/presentation/widgets/user_variables_section.dart` — переменные пользователя
- `lib/features/admin/mcp_registry/` — админка реестра MCP-серверов
- `lib/features/admin/role_prompts/` — админка промптов
- `lib/l10n/*.arb` — ключи локализации
- `test/features/agents/` — widget + provider тесты
- `test/features/projects/` — тесты переменных проекта

### MCP-инструменты

Новые файлы в `internal/mcp/`:
- `tools_project_secrets.go` — project_secret_list/set/delete
- `tools_user_secrets.go` — user_secret_list/set/delete
- `tools_role_prompts.go` — role_prompt_list/get

Изменённые:
- `tools_agents_v2.go` — agent_my_list/get/update
- `authorized_executor.go` — обновить `Catalog()` (добавить новые инструменты)

Все новые API должны быть доступны assistant через MCP — чтобы он мог помогать пользователю создавать и настраивать агентов через диалог.

**Новые MCP-инструменты:**

| Инструмент | Файл | Что делает | Scope |
|------------|------|-----------|-------|
| `agent_my_list` | `tools_agents_v2.go` | Список агентов текущего пользователя | user |
| `agent_my_get` | `tools_agents_v2.go` | Детали user-level агента | user |
| `agent_my_update` | `tools_agents_v2.go` | Обновить настройки user-level агента (LLM, MCP, skills) | user |
| `project_secret_list` | `tools_project_secrets.go` (**новый**) | Список переменных проекта (только ключи) | project member |
| `project_secret_set` | `tools_project_secrets.go` | Создать/обновить переменную проекта | project member |
| `project_secret_delete` | `tools_project_secrets.go` | Удалить переменную проекта | project member |
| `user_secret_list` | `tools_user_secrets.go` (**новый**) | Список переменных пользователя (только ключи) | user |
| `user_secret_set` | `tools_user_secrets.go` | Создать/обновить переменную пользователя | user |
| `user_secret_delete` | `tools_user_secrets.go` | Удалить переменную пользователя | user |
| `role_prompt_list` | `tools_role_prompts.go` (**новый**) | Список дефолтных промптов по ролям | any |
| `role_prompt_get` | `tools_role_prompts.go` | Промпт по роли | any |

> **ABAC:** каждый инструмент проверяет `auth.UserID` через `AuthorizedExecutor.injectAuth()`. Scope "user" — только свои ресурсы. Scope "project member" — проверка доступа к проекту. `role_prompt_list/get` — read-only, доступно всем (assistant должен видеть шаблоны).

> **Безопасность:** `project_secret_list` и `user_secret_list` возвращают **только ключи** (key_name, created_at), **никогда значения**. `_set` принимает plaintext → шифрует через `AESEncryptor`. Plaintext **никогда не логируется** — в логах заменяется на `<redacted>`.

> **RBAC промптов:** MCP-инструмента `role_prompt_update` нет намеренно. Редактирование промптов — только через admin REST API (`PUT /admin/agent-role-prompts/:role`). Assistant может читать промпты (list/get), но не менять.

**Обновить `AuthorizedExecutor.Catalog()`:** добавить новые инструменты в каталог, доступный assistant.

### Compliance checklist

- [ ] `make swagger` после добавления ручек
- [ ] `make frontend-codegen` после добавления Freezed-моделей
- [ ] `make frontend-analyze` перед коммитом фронтенда
- [ ] `make test-all` после каждой бекенд-фазы
- [ ] `flutter gen-l10n` после добавления .arb ключей
- [ ] Шифрование через общий `SecretService` → `pkg/crypto.AESEncryptor` (AAD = record ID). Не копипастить в репозитории
- [ ] Plaintext секретов **не логируется** — `<redacted>` в handler'ах и MCP-инструментах `*_secret_set`
- [ ] ABAC-проверки во всех handler'ах и MCP-инструментах
- [ ] Каждая новая ручка → MCP-инструмент в `internal/mcp/`
- [ ] `AuthorizedExecutor.Catalog()` обновлён
- [ ] Неразрешённые плейсхолдеры `${VAR}` → ошибка запуска агента, не silent pass-through
- [ ] `BuildArtifacts()` — bulk load секретов одним запросом, не N+1
- [ ] Валидация `^[A-Z][A-Z0-9_]{0,127}$` — и в БД (CHECK), и на фронтенде (с i18n ошибкой)
- [ ] Нет хардкоженных строк в UI
