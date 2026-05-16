# LLM-провайдеры

DevTeam Sprint 15 поддерживает несколько способов подачи запросов к LLM:

1. **Прямой клиент** к Anthropic / OpenAI / Gemini / DeepSeek / Qwen — `LLMService` (для разговорной части).
2. **Claude Code в sandbox** (`code_backend = claude-code`) — отдельный CLI внутри Docker-контейнера. Auth выбирается per-agent через `provider_kind`:
   - `anthropic` → классический `ANTHROPIC_API_KEY` (per-user из `user_llm_credentials`).
   - `anthropic_oauth` → OAuth-токен подписки Claude Code (per-user из `claude_code_subscriptions`).
   - `deepseek` / `zhipu` / `openrouter` → native Anthropic-совместимый endpoint провайдера + per-user ключ (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`).

**Sprint 15.e2e (2026-05-13):** sidecar-прокси `free-claude-code` удалён. Был однотенантным (одни общие ключи на инстанс) и не подходил под мультиюзера. Не-Anthropic провайдеры теперь подключаются через свой native Anthropic endpoint напрямую. См. блок «Не-Anthropic в sandbox» ниже.

Этот документ — как поднять каждый провайдер и какие переменные окружения / поля задавать.

---

## Общее

Все секреты хранятся в БД в зашифрованном виде (AES-256-GCM, `backend/pkg/crypto`). Включается шифрование переменной окружения:

```env
ENCRYPTION_KEY=<64 hex-символа = 32 байта>
```

Без `ENCRYPTION_KEY` бэкенд работает в NoopEncryptor-режиме — credentials хранятся как plaintext (только для локальной разработки, **не для prod**).

### Аудит шифрования (UI Refactoring — Этап 0.1, verified)

Проверено 2026-05-16 в рамках [dashboard-redesign §4a.1](tasks/ui_refactoring/dashboard-redesign-plan.md#4a1-безопасность). Инвариант «секреты в БД только зашифрованными» соблюдён для всех каналов хранения пользовательских LLM-ключей:

| Таблица | Колонка с секретом | Шифрование | Где |
|---|---|---|---|
| `user_llm_credentials` (миграция 022) | `encrypted_key BYTEA NOT NULL` | AES-256-GCM, AAD = `row.ID.String()` | [user_llm_credential_service.go](../backend/internal/service/user_llm_credential_service.go) (`setProviderKey`, `tryEncryptUpdate`, `GetMasked`, `GetPlaintext`) |
| `llm_providers` (миграция 023) | `credentials_encrypted BYTEA` | AES-256-GCM, AAD = `provider.ID` | [llm_provider_service.go](../backend/internal/service/llm_provider_service.go) |
| `claude_code_subscriptions` (миграция 024) | OAuth-токены | AES-256-GCM | OAuth refresher worker |
| `agent_secrets` (миграция 032) | `value_encrypted BYTEA` | AES-256-GCM | [agent_secret_repository.go](../backend/internal/repository/agent_secret_repository.go) |
| `git_credentials` | `token_encrypted BYTEA` | AES-256-GCM, AAD = `id.String()` | [git_credential_repository.go](../backend/internal/repository/git_credential_repository.go) |

Plain-text путь отсутствует: в API-слое `service.Encryptor` подменяется на `NoopEncryptor` только если `ENCRYPTION_KEY` не задан ([cmd/api/main.go](../backend/cmd/api/main.go) — `len(cfg.Encryption.Key) == 32`); в этом режиме `NoopEncryptor.Decrypt` отказывается читать blob с маркером `0x01` (см. `ErrNoopDecryptBlobRequiresKey`), исключая «гибрид» зашифрованного и plain-text содержимого в одной таблице. Backfill-миграция для `user_llm_credentials` не нужна: схема 022 создана уже с `BYTEA NOT NULL`, а production сразу пишет AES-GCM blob.

Визуальная проверка (`ysqlsh -c "SELECT provider, length(encrypted_key) FROM user_llm_credentials LIMIT 5;"`) — длина blob = `1 (версия) + 12 (nonce) + len(plain) + 16 (GCM tag)`, явно не похоже на `sk-...`.

CRUD по провайдерам — таблица `llm_providers` (миграция 023). Поля:

| Поле | Назначение |
|------|------------|
| `name` | Уникальное имя в UI |
| `kind` | См. таблицу ниже |
| `base_url` | Endpoint провайдера (или пусто — будет использован дефолт) |
| `auth_type` | `api_key`, `oauth`, `bearer`, `none` |
| `credentials_encrypted` | AES-blob с ключом/токеном |
| `default_model` | Модель по умолчанию |
| `enabled` | Видимость в UI / используется в фабрике клиентов |

---

## 1. Anthropic (API-ключ)

- **kind:** `anthropic`
- **base_url:** `https://api.anthropic.com`
- **Где взять ключ:** [console.anthropic.com](https://console.anthropic.com/) → Settings → API Keys.
- **Модели:** `claude-3-5-sonnet-20240620`, `claude-3-haiku-20240307`, `claude-3-opus-20240229`.

## 2. Anthropic OAuth (подписка Claude Code)

- **kind:** `anthropic_oauth`
- Не использует API-ключ; access/refresh-токены хранятся в таблице `claude_code_subscriptions` (миграция 024) после прохождения device-flow.
- **Включение в бэкенде:**
  ```env
  CLAUDE_CODE_OAUTH_CLIENT_ID=<from anthropic>
  CLAUDE_CODE_OAUTH_DEVICE_URL=https://console.anthropic.com/v1/oauth/device
  CLAUDE_CODE_OAUTH_TOKEN_URL=https://console.anthropic.com/v1/oauth/token
  CLAUDE_CODE_OAUTH_SCOPES=org:create_api_key user:profile user:inference
  ```
- **Endpoints:** `POST /api/v1/claude-code/auth/init`, `POST /callback`, `GET /status`, `DELETE`.
- **Refresher:** фоновый воркер раз в минуту обновляет токены за 10 минут до истечения.

## 3. OpenAI

- **kind:** `openai`
- **base_url:** `https://api.openai.com/v1`
- **Где взять ключ:** [platform.openai.com](https://platform.openai.com/api-keys).
- **Модели:** `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`.

## 4. Gemini

- **kind:** `gemini`
- **base_url:** `https://generativelanguage.googleapis.com`
- **Где взять ключ:** [aistudio.google.com](https://aistudio.google.com/app/apikey).
- **Модели:** `gemini-1.5-pro`, `gemini-1.5-flash`.

## 5. DeepSeek

- **kind:** `deepseek`
- **base_url:** `https://api.deepseek.com/v1`
- **Где взять ключ:** [platform.deepseek.com](https://platform.deepseek.com).
- **Модели:** `deepseek-chat`, `deepseek-coder`.

## 6. Qwen (DashScope)

- **kind:** `qwen`
- **base_url:** `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **Где взять ключ:** [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com).
- **Модели:** `qwen-turbo`, `qwen-plus`, `qwen-max`.

## 7. OpenRouter

- **kind:** `openrouter`
- **base_url:** `https://openrouter.ai/api/v1`
- **Где взять ключ:** [openrouter.ai/keys](https://openrouter.ai/keys).
- **Модели:** `openrouter/auto` (автовыбор), `anthropic/claude-3.5-sonnet`, `openai/gpt-4o`, любой из каталога OpenRouter.
- **Заголовки:** `HTTP-Referer` и `X-Title` — рекомендуются (атрибуция).

## 8. Moonshot AI (Kimi)

- **kind:** `moonshot`
- **base_url:** `https://api.moonshot.cn/v1`
- **Где взять ключ:** [platform.moonshot.cn](https://platform.moonshot.cn/console/api-keys).
- **Модели:** `moonshot-v1-8k`, `moonshot-v1-32k`, `moonshot-v1-128k`.

## 9. Ollama (локальный)

- **kind:** `ollama`
- **base_url:** `http://<host>:11434/v1` (внутри docker-compose: имя сервиса).
- **auth_type:** `none` (Ollama игнорирует API-ключ).
- **Модели:** локально установленные (`ollama pull llama3` и т.п.).

## 10. Zhipu AI (GLM)

- **kind:** `zhipu`
- **base_url:** `https://open.bigmodel.cn/api/paas/v4`
- **Где взять ключ:** [open.bigmodel.cn](https://open.bigmodel.cn).
- **Модели:** `glm-4-plus`, `glm-4-flash`.

## 11. Не-Anthropic провайдеры в sandbox (native endpoint)

**Sprint 15.e2e:** заменили sidecar-прокси на прямой выход на native Anthropic-совместимый endpoint провайдера. Никаких отдельных сервисов; per-agent `provider_kind` и per-user ключ в `user_llm_credentials`.

Маппинг kind → endpoint (зашит в [models/workflow.go `AnthropicBaseURL()`](../backend/internal/models/workflow.go)):

| `provider_kind` | `ANTHROPIC_BASE_URL` в sandbox | Источник ключа |
|---|---|---|
| `deepseek` | `https://api.deepseek.com/anthropic` | `user_llm_credentials.deepseek` |
| `zhipu` | `https://open.bigmodel.cn/api/anthropic` | `user_llm_credentials.zhipu` |
| `openrouter` | `https://openrouter.ai/api/v1` | `user_llm_credentials.openrouter` |
| `anthropic` | _не выставляется_ (CLI идёт на api.anthropic.com) | `user_llm_credentials.anthropic` |
| `anthropic_oauth` | _не выставляется_ | `claude_code_subscriptions` |

Ключ юзера расшифровывается резолвером, кладётся в `ANTHROPIC_AUTH_TOKEN` (для non-anthropic kind) или `ANTHROPIC_API_KEY` (для kind=anthropic) и попадает в env контейнера.

### Как поменять provider_kind у агента

`provider_kind` — поле строки `agents`, выставляется через **team PATCH-эндпоинт**, а не через `/agents/:id/settings` (тот занимается только code_backend / MCP / Skills / permissions; колонка `agents.llm_provider_id` удалена миграцией 029).

| Способ | Что использовать |
|---|---|
| Frontend UI | Диалог «Редактировать агента» (`agent_edit_dialog.dart`) → выпадающий список «LLM провайдер» |
| REST API | `PATCH /api/v1/projects/:id/team/agents/:agentId` с телом `{"provider_kind": "deepseek"}` (значение / `null` для сброса / поле опущено для no-op) |
| MCP | пока не покрыто отдельным инструментом; используйте тот же REST через ваш HTTP-клиент |
| Прямой SQL (для e2e/seed) | `UPDATE agents SET provider_kind = 'deepseek' WHERE id = '...'` |

Ключ юзера под выбранный kind вносится отдельно: вкладка «Глобальные настройки → LLM-ключи» (`/me/llm-credentials`) — она пишет в `user_llm_credentials`. Для `anthropic_oauth` — вкладка «Claude Code», OAuth device-flow → `claude_code_subscriptions`.

---

## Приоритет аутентификации в sandbox

`service.SandboxAuthEnvResolver` (Sprint 15.18 + Sprint 15.e2e rewrite) собирает env по `agent.provider_kind`:

1. `kind=anthropic_oauth` → `claude_code_subscriptions(owner)` → `CLAUDE_CODE_OAUTH_TOKEN`.
2. `kind=anthropic` → `user_llm_credentials(owner, anthropic)` → `ANTHROPIC_API_KEY`.
3. `kind=deepseek` / `zhipu` / `openrouter` → `user_llm_credentials(owner, <kind>)` → `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`.
4. `provider_kind` не задан (legacy) → fallback на OAuth-подписку владельца → static `ANTHROPIC_API_KEY` из `cfg.LLM.Anthropic.APIKey`.

Sandbox entrypoint (`deployment/sandbox/claude/entrypoint.sh`) считает аутентификацию валидной, если задан **любой** из трёх env (`CLAUDE_CODE_OAUTH_TOKEN`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY`).

**Замечание Sprint 15.e2e:** флаг Claude Code `--bare` блокирует auth через `CLAUDE_CODE_OAUTH_TOKEN` env (CLI отвечает `Not logged in`), поэтому в entrypoint его не передаём. Hooks/CLAUDE.md auto-discovery внутри `/workspace/repo` (свежий клон) не страшны.
