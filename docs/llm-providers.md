# LLM-провайдеры

DevTeam Sprint 15 поддерживает несколько способов подачи запросов к LLM:

1. **Прямой клиент** к Anthropic / OpenAI / Gemini / DeepSeek / Qwen — `LLMService` (для разговорной части).
2. **Claude Code в sandbox** (`code_backend = claude-code`) — отдельный CLI внутри Docker-контейнера, ему нужен API-ключ или OAuth-подписка.
3. **Claude Code через free-claude-proxy** (`code_backend = claude-code-via-proxy`) — sidecar, выставляющий Anthropic-совместимый API поверх OpenRouter/DeepSeek/Moonshot/Ollama/Zhipu.

Этот документ — как поднять каждый провайдер и какие переменные окружения / поля в `llm_providers` задавать.

---

## Общее

Все секреты хранятся в БД в зашифрованном виде (AES-256-GCM, `backend/pkg/crypto`). Включается шифрование переменной окружения:

```env
ENCRYPTION_KEY=<64 hex-символа = 32 байта>
```

Без `ENCRYPTION_KEY` бэкенд работает в NoopEncryptor-режиме — credentials хранятся как plaintext (только для локальной разработки, **не для prod**).

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

## 11. free-claude-proxy (sidecar)

Sidecar `Alishahryar1/free-claude-code`, выставляющий **Anthropic-совместимый** API (`/v1/messages`) поверх любого из OpenAI-совместимых провайдеров выше. Используется sandbox-агентами с `code_backend = claude-code-via-proxy`:

- Sandbox получает `ANTHROPIC_BASE_URL=http://free-claude-proxy:8787` + `ANTHROPIC_AUTH_TOKEN=<service-token>` — Claude Code CLI идёт на прокси вместо Anthropic.
- Конфиг прокси (`config.yaml`) генерируется бэкендом из `llm_providers` сервисом `FreeClaudeProxyConfigBuilder` (Sprint 15.17). См. [config.example.yaml](../deployment/free-claude-proxy/config.example.yaml).
- **Старт:**
  ```bash
  docker compose --profile free-claude-proxy up -d free-claude-proxy
  ```
- **Переменные:**
  ```env
  FREE_CLAUDE_PROXY_URL=http://free-claude-proxy:8787
  FREE_CLAUDE_PROXY_SERVICE_TOKEN=<random hex>
  FREE_CLAUDE_PROXY_CONFIG_PATH=/etc/free-claude-proxy/config.yaml
  FREE_CLAUDE_PROXY_ENABLED=true
  ```
- При `FREE_CLAUDE_PROXY_ENABLED=true` оркестратор делает fail-fast `GET /healthz` при старте (Sprint 15.19).

---

## Приоритет аутентификации в sandbox

`service.SandboxAuthEnvResolver` (Sprint 15.18) собирает env для контейнера так:

1. `agent.code_backend == claude-code-via-proxy` → `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`.
2. У владельца проекта есть подписка Claude Code → `CLAUDE_CODE_OAUTH_TOKEN`.
3. Иначе → `ANTHROPIC_API_KEY` из `cfg.LLM.Anthropic.APIKey`.

Sandbox entrypoint (`deployment/sandbox/claude/entrypoint.sh`) считает аутентификацию валидной, если задан **любой** из трёх env.
