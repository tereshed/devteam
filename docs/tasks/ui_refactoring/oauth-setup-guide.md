# OAuth Apps Setup Guide (GitHub + GitLab)

Пошаговая инструкция по регистрации OAuth-приложений для подключения Git-провайдеров в DevTeam.

## Что регистрируем и зачем

Для модели **Shared OAuth App** (см. [dashboard-redesign-plan.md §4a и §6.1](dashboard-redesign-plan.md)) DevTeam использует **наши** зарегистрированные приложения GitHub/GitLab. Пользователь жмёт «Connect» → видит prompt «DevTeam wants access to...» → готово.

Регистрируем **по два приложения на каждого провайдера**:

| Провайдер     | Приложение              | Callback URL                                                              |
|---------------|-------------------------|---------------------------------------------------------------------------|
| GitHub        | `DevTeam (dev)`         | `http://localhost:8080/api/v1/integrations/github/auth/callback`         |
| GitHub        | `DevTeam (prod)`        | `https://polymaths.work/api/v1/integrations/github/auth/callback`        |
| GitLab.com    | `DevTeam (dev)`         | `http://localhost:8080/api/v1/integrations/gitlab/auth/callback`         |
| GitLab.com    | `DevTeam (prod)`        | `https://polymaths.work/api/v1/integrations/gitlab/auth/callback`        |

> **Если backend живёт на поддомене** (например `api.polymaths.work`, а не `polymaths.work`) — поменяй URL соответственно во всех «prod»-полях ниже.

Self-hosted GitLab делается **по другой схеме** — это BYO (Bring Your Own), пользователь регистрирует Application у себя; см. раздел 4.

---

## 1. GitHub OAuth App — Dev

Цель: получить `client_id` + `client_secret` для локальной разработки.

### 1.1. Открыть форму

Открой в браузере: **https://github.com/settings/developers**

Это ведёт в твой персональный аккаунт → Developer Settings → OAuth Apps. Если хочешь, чтобы приложение принадлежало организации (например `polymaths`) — перейди в `https://github.com/organizations/<org>/settings/applications`.

Слева в меню выбери **OAuth Apps**, потом сверху справа кнопку **New OAuth App**.

```
GitHub Settings / Developer Settings
├─ GitHub Apps
├─ OAuth Apps          ← здесь
└─ Personal access tokens
                                              [ New OAuth App ]  ←  жми
```

### 1.2. Заполнить форму

| Поле                          | Значение                                                            |
|-------------------------------|---------------------------------------------------------------------|
| **Application name**          | `DevTeam (dev)`                                                     |
| **Homepage URL**              | `http://localhost:8080`                                             |
| **Application description**   | `DevTeam AI agent orchestrator — local development.` (опционально)  |
| **Authorization callback URL**| `http://localhost:8080/api/v1/integrations/github/auth/callback`    |
| Enable Device Flow            | НЕ отмечаем                                                         |

> **Где настройки доступов (scopes)?**
> В отличие от GitLab (или GitHub Apps), для **GitHub OAuth Apps** права доступа не настраиваются в интерфейсе регистрации. Наше приложение (backend) само запросит нужные права (например, `scope=repo,user`) динамически при формировании ссылки для авторизации пользователя.

Жми **Register application**.

### 1.3. Сохранить креды

После регистрации откроется страница с твоим новым приложением:

```
DevTeam (dev)
─────────────────────────────────────────────
Client ID:      1234567890abcdef1234          ←  скопируй
                              [ Generate a new client secret ]
```

1. Скопируй **Client ID** в буфер — это `GITHUB_OAUTH_CLIENT_ID`.
2. Жми **Generate a new client secret** — появится длинная случайная строка (обычно 40 символов, например `a1b2c3d4e5f6...`). **Скопируй её немедленно** — GitHub показывает secret **только один раз**, после reload вместо неё будут точки.

Положи обе строки в `backend/.env`:

```env
GITHUB_OAUTH_CLIENT_ID=1234567890abcdef1234
GITHUB_OAUTH_CLIENT_SECRET=a1b2c3d4e5f6...
```

### 1.4. (Опционально) Логотип

В разделе **Application logo** загрузи иконку 200×200 — она появится в OAuth-prompt'е у юзера. Можно пропустить, дефолт — серая буква `D`.

---

## 2. GitHub OAuth App — Prod

Создаётся **отдельным** приложением (не делай Dev-приложение «и dev и prod» — там жёстко один callback).

Повторяешь шаги 1.1–1.4 со следующими значениями:

| Поле                          | Значение                                                            |
|-------------------------------|---------------------------------------------------------------------|
| **Application name**          | `DevTeam`                                                           |
| **Homepage URL**              | `https://polymaths.work`                                            |
| **Authorization callback URL**| `https://polymaths.work/api/v1/integrations/github/auth/callback`   |

Креды кладёшь в **прод-окружение** (не в локальный `backend/.env`!) — секретохранилище хостинга (Vault, AWS Secrets Manager, Doppler, etc.). Те же имена переменных:

```env
GITHUB_OAUTH_CLIENT_ID=...
GITHUB_OAUTH_CLIENT_SECRET=...
```

---

## 3. GitLab.com OAuth Application — Dev

GitLab.com называет это не «OAuth App», а **Application**.

### 3.1. Открыть форму

Открой: **https://gitlab.com/-/user_settings/applications**

(Это твой персональный аккаунт. Для организации/группы используй `https://gitlab.com/groups/<group>/-/settings/applications`.)

Сверху увидишь форму **Add new application**.

### 3.2. Заполнить форму

| Поле               | Значение                                                              |
|--------------------|-----------------------------------------------------------------------|
| **Name**           | `DevTeam (dev)`                                                       |
| **Redirect URI**   | `http://localhost:8080/api/v1/integrations/gitlab/auth/callback`     |
| **Confidential**   | ✅ (галка должна стоять — это server-side OAuth flow, у нас есть secret) |
| **Scopes**         | Отметь: `api`, `read_user`, `read_repository`, `write_repository`     |

Про scopes:
- `api` — даёт доступ ко всему API; нужен для большинства операций.
- `read_user` — узнать имя/email подключённого юзера для отображения в UI.
- `read_repository` / `write_repository` — клонировать репо и пушить PR-ветки.

Жми **Save application**.

### 3.3. Сохранить креды

После сохранения GitLab покажет:

```
DevTeam (dev)
─────────────────────────────────────────────
Application ID:   91a2b3c4d5e6...        ←  это client_id
Secret:           gloas-aBcDeF...        ←  скопируй, GitLab покажет только раз
```

Положи в `backend/.env`:

```env
GITLAB_OAUTH_CLIENT_ID=91a2b3c4d5e6...
GITLAB_OAUTH_CLIENT_SECRET=gloas-aBcDeF...
```

---

## 4. GitLab.com OAuth Application — Prod

Повторяешь шаги 3.1–3.3 с теми же скоупами и:

| Поле               | Значение                                                                  |
|--------------------|---------------------------------------------------------------------------|
| **Name**           | `DevTeam`                                                                 |
| **Redirect URI**   | `https://polymaths.work/api/v1/integrations/gitlab/auth/callback`         |

Креды кладёшь в прод-секрет-хранилище.

---

## 5. Self-hosted GitLab (BYO Application) — для пользователей

Если юзер подключает свой **внутренний** GitLab (например `gitlab.acme-corp.internal`), мы **не можем** использовать наше DevTeam-приложение — у acme-corp нет о нём представления. Юзер регистрирует Application **в своём GitLab** и вводит креды в DevTeam UI (поле появится в диалоге «Подключить self-hosted GitLab»).

Эту инструкцию мы покажем юзеру **в самом UI** (раскрывающийся блок «Как зарегистрировать Application в моём GitLab»):

> 1. Открой `https://<your-gitlab-host>/-/user_settings/applications`.
> 2. Жми **Add new application**.
> 3. Заполни:
>    - **Name:** `DevTeam`
>    - **Redirect URI:** `https://polymaths.work/api/v1/integrations/gitlab/auth/callback`
>      (или `http://localhost:8080/...` если ты подключаешься к локально запущенному DevTeam)
>    - **Confidential:** ✅
>    - **Scopes:** `api`, `read_user`, `read_repository`, `write_repository`
> 4. Жми **Save**, скопируй `Application ID` и `Secret`.
> 5. Вставь их в форму «Подключить GitLab (self-hosted)» в DevTeam.

В БД эти креды хранятся **в той же таблице** `git_integration_credentials`, дополнительно к access_token (см. [Plan §4a.1](dashboard-redesign-plan.md#4a1-безопасность)):

- `byo_client_id VARCHAR(255)` — **plain**, не шифруется. По спеке OAuth 2.0 это публичная константа (она и так присутствует в URL `/oauth/authorize?client_id=...` в браузере юзера). Шифровать её — лишний CPU и помехи при отладке.
- `byo_client_secret BYTEA` — шифруется `pkg/crypto.AESEncryptor` с `AAD = id записи` (конвенция проекта по `docs/rules/main.md` §2.3 п.5).

Заполняются только для self-hosted GitLab (`provider='gitlab' AND host IS NOT NULL`). Для shared-приложений (gitlab.com / github.com) остаются NULL.

---

## 6. Итоговый чек-лист `backend/.env`

После шагов 1–4 в `backend/.env` должно быть:

```env
# GitHub OAuth (Shared App, Dev)
GITHUB_OAUTH_CLIENT_ID=xxxxxxxxxxxxxxxxxxxx
GITHUB_OAUTH_CLIENT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# GitLab.com OAuth (Shared App, Dev)
GITLAB_OAUTH_CLIENT_ID=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
GITLAB_OAUTH_CLIENT_SECRET=gloas-xxxxxxxxxxxxxxxxxxxxxxxxxx

# Master ключ для шифрования OAuth-токенов в БД (см. §4a.1)
# Генерация: openssl rand -base64 32
INTEGRATION_TOKEN_ENC_KEY=<32-bytes-base64>
```

Для прод-окружения те же три переменные кладутся в секрет-хранилище хостинга.

---

## 7. Проверка после реализации

Когда задачи 3.1–3.6 (см. [tasks-breakdown.md](tasks-breakdown.md)) будут готовы:

1. `make up` — поднять стек локально с обновлённым `.env`.
2. Открыть macOS-app → войти → раздел «Интеграции / Git» → жми «Подключить GitHub».
3. Должно открыться окно Safari/Chrome с GitHub-prompt'ом «Authorize DevTeam (dev)».
4. Кликнул Authorize → браузер показывает «Готово, вернитесь в приложение».
5. В приложении статус-чип Connect стал зелёным **без ручного обновления** (через WS-событие, §4a.4).
6. В БД (`docker exec wibe_yugabytedb /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -U yugabyte -d yugabyte -c "SELECT provider, length(access_token) FROM git_integration_credentials;"`) — поле `access_token` это **бинарный blob** длиной ~80–120 байт (AES-GCM overhead), а не plain-text `a1b2c3...`.

Если хоть один пункт не сходится — баг, чиним до мерджа.

---

## 8. Что делать, если client_secret протёк

GitHub/GitLab оба поддерживают **revoke + regenerate** секрета без пересоздания всего приложения:

- **GitHub:** OAuth Apps → `DevTeam (...)` → **Generate a new client secret** → старый сразу инвалидируется через ~5 минут. Положи новый в `.env`, перезапусти backend.
- **GitLab:** Applications → `DevTeam (...)` → **Renew secret** → аналогично.

Старые подключения юзеров продолжают работать (у них уже есть access_token, который хранится у нас в БД — он от secret не зависит). Сломается только новый init-флоу до обновления `.env`.

Если протёк **`INTEGRATION_TOKEN_ENC_KEY`** — это catastrophic. Нужна миграция с rotation: расшифровать существующие токены старым ключом, зашифровать новым, обновить `.env`. Эту процедуру оставим под отдельный runbook когда реально потребуется.
