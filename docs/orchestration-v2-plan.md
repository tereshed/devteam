# Orchestration v2 — LLM-driven, flow-as-data

Статус: **MVP реализован, Sprint 17 (Sprints 1-5F) завершён 2026-05-15**
Дата последнего обновления: 2026-05-15
История:
- v1 — MVP-набросок (детерминированные точки вызова Router)
- v2 — учтены замечания по ревью: async execution, secrets, hallucinations, context budget, locking
- v3 — параллелизм подзадач в MVP: worktree-изоляция, Merger-агент, DAG в декомпозиции, Router-decision возвращает массив агентов
- v4 — безопасность по второму ревью: `--` separator в git, запрет логирования сырых LLM-ответов в stdout, корректный LISTEN→SELECT порядок в Agent Worker, advisory lock через (int4,int4), типизированные UUID-пути для worktree
- v5 — адаптация под ограничения YugabyteDB: polling + Redis Pub/Sub вместо `LISTEN/NOTIFY`, `SELECT FOR UPDATE NOWAIT` на `tasks.id` вместо advisory locks
- v6 (текущая) — Sprint 5F закрыт: 4 postgres-integration теста (testcontainers + race-clean), HTTP read-only API для v2 (artifacts/router-decisions/worktrees), Flutter Agents Management UI + DAG-view + Router timeline + Worktrees debug, миграция 038 fix (partial unique + chk_agents_role roles). Sprint 6 follow-ups зафиксированы в §7.

---

## 0. Changelog v3 → v4 (security review #2)

| # | Изменение | Причина |
|---|---|---|
| 1 | Все git-вызовы используют `--` separator перед user/LLM-данными | Command/flag injection через имя ветки (`-h`, `--upload-pack=...`). Project-wide convention, см. `internal/agent/execution_types.go` godoc |
| 2 | Запрет логирования сырых LLM-промптов и ответов в stdout/stderr | Encryption-at-rest бессильно если log.Errorf пишет raw в stderr → Kibana. Только зашифрованная БД-колонка |
| 3 | Agent Worker: LISTEN → SELECT `cancel_requested` → start Exec | Race: NOTIFY мог уйти ДО подписки. SELECT после LISTEN ловит "пропущенный" сигнал |
| 4 | Advisory lock через `pg_try_advisory_xact_lock(int4, int4)` с прямыми байтами UUID | `hashUUID()→int64` имеет birthday-collision; UUIDv4 уже 122 random bits — берём напрямую |
| 5 | Worktree path всегда от типизированного `uuid.UUID`, никогда из БД-строки | Path traversal через испорченные данные в `worktrees.path` |

## Changelog v2 → v3

| # | Изменение | Причина |
|---|---|---|
| 1 | Router-`Decision` возвращает **массив** агентов вместо одного | Параллельное исполнение независимых подзадач |
| 2 | Subtask-описание содержит `depends_on: [id, ...]` (DAG) | Router должен знать что параллелится, что нет |
| 3 | Новый агент `merger` для слияния параллельных code_diff | Когда параллельные ветки сходятся — кто-то должен резолвить конфликты |
| 4 | Worktree-изоляция: каждый sandbox-job — свой git worktree | Параллельные `git checkout` в одном репо = гонка |
| 5 | Default `agent_workers_sandbox = 2`, `agent_workers_llm = 20` | Dev/VPS 8-16GB RAM |
| 6 | Default `task_timeout = 4h` | Баланс для средних фич, можно override per-task |
| 7 | Default `router_decisions` retention = 30 дней | Подтверждено |

---

## 1. Мотивация

Текущая оркестрация (`internal/service/orchestrator_pipeline.go`) — детерминированный state machine с хардкодом переходов в `DetermineNextStatus()`. Добавление новой роли требует переписывать Go-код, статусы, миграции, тесты. Интеллект LLM не используется для маршрутизации.

**Целевая модель:** flow — это **данные**. Правила игры — в промпте Router'а и в реестре агентов (БД). Go реализует только loop runner, durable queue, универсальный dispatcher и worktree-management.

---

## 2. Архитектура

### 2.1. Event-driven model с параллельным fan-out

Задача — последовательность **Step**-ов, каждый из которых атомарен. Один Step может породить **N параллельных agent_jobs** (если Router решил, что подзадачи независимы).

```
┌──────────────┐     ┌────────────┐     ┌──────────────────────┐
│ HTTP POST    │────►│ tasks      │────►│ task_events          │ (durable queue)
│ /tasks       │     │ state=     │     │ kind=step_req        │
└──────────────┘     │ active     │     └──────────┬───────────┘
                     └────────────┘                │ LISTEN/NOTIFY
                                                   ▼
                            ┌──────────────────────────────────────┐
                            │ Step Worker (один за раз на task     │
                            │ через advisory lock)                 │
                            │                                      │
                            │ 1. pg_try_advisory_xact_lock(task)   │
                            │ 2. load state                        │
                            │ 3. check cancel_requested            │
                            │ 4. router.Decide() → массив агентов  │
                            │ 5. if DONE → mark task, return       │
                            │ 6. enqueue N agent_jobs              │
                            │    (с worktree_id для sandbox'ов)    │
                            │ 7. release lock, return              │
                            └──────────────────┬───────────────────┘
                                               │
                              ┌────────────────┴────────────────┐
                              ▼                                 ▼
                  ┌──────────────────────┐         ┌──────────────────────┐
                  │ Agent Worker (LLM)   │   ...   │ Agent Worker (sbox)  │
                  │                      │         │                      │
                  │ 1. claim job         │         │ 1. claim job         │
                  │ 2. dispatcher.Exec   │         │ 2. allocate worktree │
                  │ 3. save artifact     │         │ 3. dispatcher.Exec   │
                  │ 4. enqueue step_req  │         │ 4. save artifact     │
                  └──────────────────────┘         │ 5. release worktree  │
                                                   │ 6. enqueue step_req  │
                                                   └──────────────────────┘
```

**Ключевая идея:**
- **Step сериализован per-task** через advisory lock (один Router-вызов на задачу одновременно).
- **Agent jobs параллельны** в рамках задачи (N штук) — изолированы worktree'ами.
- После завершения любого agent_job → enqueue новый step_req → Router увидит обновлённое состояние, решит что дальше.
- Если несколько agent_jobs завершились "одновременно" и каждый дёрнул step_req — advisory lock сериализует Step'ы; лишние просто увидят что lock занят, выйдут, и состояние будет учтено следующим Step'ом.

### 2.2. Router (LLM-агент)

Вызывается на каждом `Orchestrator.Step`. Получает на вход:
- Описание задачи
- Реестр включённых агентов (`name`, `role_description`, `execution_kind`)
- **Метаданные** артефактов (`id`, `kind`, `producer_agent`, `iteration`, `status`, **`summary`** ≤ 500 chars) и подзадач (`id`, `description_summary`, `depends_on`, `subtask_state`)
- Список **in-flight** agent_jobs (что сейчас выполняется — чтобы Router не запустил то же повторно)
- Системные правила (часть промпта в БД)

Возвращает JSON с **массивом** агентов:
```json
{
  "done": false,
  "outcome": null,
  "agents": [
    {
      "name": "developer",
      "input": {
        "target_artifact_id": "subtask-1-description-uuid",
        "instructions": "Implement subtask 1."
      }
    },
    {
      "name": "developer",
      "input": {
        "target_artifact_id": "subtask-2-description-uuid",
        "instructions": "Implement subtask 2."
      }
    }
  ],
  "reason": "Subtasks 1 и 2 не имеют depends_on друг на друга, запускаю параллельно. Subtask 3 ждёт subtask 1."
}
```

Если задача последовательна — `agents` массив длины 1.

**Жёсткие правила в промпте Router'а:**
```
Жёсткие правила маршрутизации:
- Любой созданный артефакт kind ∈ {plan, subtask_description, code_diff, merged_code}
  ОБЯЗАН пройти через агента-reviewer перед использованием другим агентом.
- Если последний review артефакта имеет content.decision == 'changes_requested' —
  отправляй автору артефакта на доработку.
- Если один артефакт ревьюится >5 раз без approve — DONE с outcome='failed'.

Правила параллелизма:
- Если N подзадач имеют пустой depends_on (или все depends_on в state='done') —
  ОБЯЗАН запустить их параллельно (один Decision с agents длиной N).
- НЕ запускай два agent_job на один и тот же target_artifact_id одновременно
  (проверь in-flight список).
- Когда все code_diff-артефакты по независимым подзадачам имеют review.approved
  и есть >1 параллельных diff — вызывай merger перед tester.
- Когда есть merged_code (или ровно один code_diff) с review.approved —
  пора звать tester.
```

**Pipeline парсинга** (борьба с галлюцинациями): strip markdown → `json.Unmarshal` → валидация каждого `agents[].name` против реестра enabled-агентов → retry с corrective prompt (max 2) → `needs_human`.

### 2.3. Реестр агентов

```sql
CREATE TABLE agents (
  id                  UUID PRIMARY KEY,
  name                TEXT UNIQUE NOT NULL,        -- planner|reviewer|developer|router|merger|...
  role_description    TEXT NOT NULL,               -- идёт в промпт Router'а
  system_prompt       TEXT NOT NULL,               -- системный промпт самого агента

  execution_kind      TEXT NOT NULL                -- 'llm' | 'sandbox'
                      CHECK (execution_kind IN ('llm','sandbox')),

  -- Для execution_kind='llm':
  llm_provider_id     UUID REFERENCES llm_providers(id),
  model               TEXT,
  temperature         NUMERIC,
  max_tokens          INT,

  -- Для execution_kind='sandbox':
  code_backend        TEXT
                      CHECK (code_backend IS NULL OR code_backend IN
                             ('claude-code','aider','hermes','custom')),
  sandbox_settings    JSONB NOT NULL DEFAULT '{}'::jsonb,  -- только refs на секреты, не сами секреты

  enabled             BOOLEAN DEFAULT true,
  created_at          TIMESTAMPTZ DEFAULT now(),
  updated_at          TIMESTAMPTZ DEFAULT now(),

  CHECK (
    (execution_kind = 'llm'     AND llm_provider_id IS NOT NULL AND model IS NOT NULL)
    OR
    (execution_kind = 'sandbox' AND code_backend IS NOT NULL)
  )
);
```

**Базовые seeded агенты:**
| name | execution_kind | code_backend | роль |
|---|---|---|---|
| `router` | llm | — | LLM-диспатчер |
| `planner` | llm | — | Создаёт высокоуровневый план |
| `decomposer` | llm | — | Разбивает план на DAG подзадач (с `depends_on`) |
| `reviewer` | llm | — | Ревьюит любой артефакт (plan / subtask / code_diff / merged_code) |
| `developer` | sandbox | claude-code | Пишет код в изолированном worktree |
| `merger` | sandbox | claude-code | Сливает параллельные code_diff в merged_code |
| `tester` | sandbox | claude-code | Запускает test suite |

### 2.3.1. Секреты агентов

Отдельная таблица, AES-256-GCM (паттерн как `MCPServerConfig`).

```sql
CREATE TABLE agent_secrets (
  id            UUID PRIMARY KEY,
  agent_id      UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  key_name      TEXT NOT NULL,                 -- "GITHUB_TOKEN", "ANTHROPIC_API_KEY"
  ciphertext    BYTEA NOT NULL,
  nonce         BYTEA NOT NULL,
  created_at    TIMESTAMPTZ DEFAULT now(),
  UNIQUE (agent_id, key_name)
);
```

В `sandbox_settings` — только ссылки: `{"env_secret_keys": ["GITHUB_TOKEN", "ANTHROPIC_API_KEY"]}`.

### 2.4. Артефакты и подзадачи

```sql
CREATE TABLE artifacts (
  id              UUID PRIMARY KEY,
  task_id         UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  parent_id       UUID REFERENCES artifacts(id),
  producer_agent  TEXT NOT NULL,
  kind            TEXT NOT NULL,                       -- plan|subtask_description|code_diff|merged_code|review|test_result|...
  summary         TEXT NOT NULL CHECK (length(summary) <= 500),
  content         JSONB NOT NULL,
  status          TEXT NOT NULL DEFAULT 'ready'
                  CHECK (status IN ('ready','superseded')),
  iteration       INT DEFAULT 0,
  created_at      TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_artifacts_task_created ON artifacts (task_id, created_at);
CREATE INDEX idx_artifacts_parent ON artifacts (parent_id);
```

**Подзадачи живут как `artifacts.kind='subtask_description'` с `content` вида:**
```json
{
  "title": "Add JWT auth middleware",
  "description": "...",
  "depends_on": ["subtask-uuid-1", "subtask-uuid-2"],
  "estimated_effort": "medium"
}
```

Это даёт DAG: Router видит граф `depends_on` и параллельно запускает листья.

`subtask_state` (для Router'а) **вычисляется** из артефактов: `pending` (нет ассоциированного code_diff) → `in_progress` (есть in-flight agent_job) → `coded` (есть code_diff) → `reviewed` (есть review.approved) → `merged` (есть merged_code) → `tested`.

### 2.5. Лог Router-решений

```sql
CREATE TABLE router_decisions (
  id                   UUID PRIMARY KEY,
  task_id              UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  step_no              INT NOT NULL,
  chosen_agents        TEXT[],                        -- массив имён (для аналитики "что обычно запускают параллельно")
  outcome              TEXT,
  reason               TEXT NOT NULL,
  raw_response_cipher  BYTEA,                         -- AES-256-GCM, retention 30 дней
  raw_response_nonce   BYTEA,
  created_at           TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_router_decisions_task_step ON router_decisions (task_id, step_no);
CREATE INDEX idx_router_decisions_created ON router_decisions (created_at);
```

Cron: `DELETE FROM router_decisions WHERE created_at < now() - interval '30 days'`.

**Logging policy (новое в v4):**

Сырой ввод/вывод LLM (`raw_response`, полные промпты Router'а, полные ответы агентов) **ЗАПРЕЩЕНО** писать в:
- `stdout`/`stderr`
- структурированные логгеры (`slog`, `zap`, `logrus`)
- file-логи
- любые внешние системы (Sentry, Kibana, Datadog)

**Единственный канал хранения** для сырых LLM-данных — зашифрованные колонки в БД:
- `router_decisions.raw_response_cipher`
- (будущее) `artifacts.content` с маркером `kind='*_raw'` если нужно

При ошибке парсинга Router-ответа разрешено логировать **только**:
```
log.ErrorContext(ctx, "router decision parse failed",
    "error_type", reflect.TypeOf(err).Name(),
    "raw_length", len(raw),
    "raw_head_sha256", sha256.Sum256(raw[:min(64, len(raw))])[:8],  // для дедупликации инцидентов
    "task_id", taskID,
    "step_no", stepNo)
```

Сам `raw` уже сохранён зашифрованным в `router_decisions` — туда и смотреть в БД при разборе.

Аналогично для `agent_jobs.payload` и любых `artifact.content`: если в логе нужно сослаться — только по `artifact_id`, не по содержимому.

**Защита от случайного попадания:** добавляется обёртка над логгером `internal/logging/redact.go`, которая знает field-names `raw_response`, `prompt`, `system_prompt`, `content`, `output`, `response` и подменяет значения на `<redacted len=N>`. Используется во всех слоях.

**Canary-тест** (см. DoD): прогон полного цикла задачи с canary-секретом `ANTHROPIC_API_KEY=canary-leak-token-XYZ` и контрольным promptом, содержащим узнаваемую фразу `LEAK_CANARY_PAYLOAD`. После прогона — `grep` всех stdout/stderr/file-логов на обе подстроки. Любое нахождение = тест красный.

### 2.6. Tasks

```sql
ALTER TABLE tasks
  DROP COLUMN status,
  ADD COLUMN state              TEXT DEFAULT 'active'
              CHECK (state IN ('active','done','failed','cancelled','needs_human')),
  ADD COLUMN cancel_requested   BOOLEAN DEFAULT false,
  ADD COLUMN current_step_no    INT DEFAULT 0,
  ADD COLUMN custom_timeout     INTERVAL,             -- override default task_timeout
  ADD COLUMN locked_by          TEXT,
  ADD COLUMN locked_at          TIMESTAMPTZ;
```

### 2.7. Очередь и воркеры

```sql
CREATE TABLE task_events (
  id            BIGSERIAL PRIMARY KEY,
  task_id       UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  kind          TEXT NOT NULL CHECK (kind IN ('step_req','agent_job')),
  payload       JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- payload для agent_job:
  -- { "agent": "developer",
  --   "input": { "target_artifact_id": "...", "instructions": "..." },
  --   "worktree_id": "uuid"  -- для sandbox-агентов
  -- }
  scheduled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_by     TEXT,
  locked_at     TIMESTAMPTZ,
  attempts      INT DEFAULT 0,
  max_attempts  INT DEFAULT 3,
  last_error    TEXT,
  created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_task_events_unlocked ON task_events (kind, scheduled_at)
  WHERE locked_by IS NULL;
```

**Worker poll:** `SELECT ... FOR UPDATE SKIP LOCKED LIMIT 1` (как в v2). `LISTEN/NOTIFY` на `task_events_<kind>` для low-latency wakeup.

**Пулы воркеров (дефолт для Dev/VPS 8-16GB):**
```yaml
orchestrator:
  step_workers: 5
  agent_workers_llm: 20
  agent_workers_sandbox: 2          # лимит ресурсов хоста!
  max_steps_per_task: 100
  task_timeout: 4h                  # override per-task через tasks.custom_timeout
  router_retry_budget: 2
  retention_router_decisions_days: 30
```

### 2.8. Concurrency и cancellation

**Lock на задачу — только для Step'а** (Router-вызов сериализован per-task). Agent_jobs выполняются параллельно без global lock — они изолированы worktree'ами.

**Advisory lock через двухаргументную форму** (`pg_try_advisory_xact_lock(int4, int4)`). UUID v4 имеет 122 random bits — берём первые 8 байт **напрямую** как два `int32`, без хеширования (хеш только понижает энтропию). Birthday-collision на 64 битах при 1M задач ≈ 2.7e-8 — приемлемо, но мы и не теряем энтропии лишними преобразованиями.

```go
// internal/service/advisory_lock.go
func uuidToLockPair(u uuid.UUID) (int32, int32) {
    return int32(binary.BigEndian.Uint32(u[0:4])),
           int32(binary.BigEndian.Uint32(u[4:8]))
}

func (o *Orchestrator) Step(ctx context.Context, taskID uuid.UUID) error {
    return o.db.Transaction(ctx, func(tx *gorm.DB) error {
        hi, lo := uuidToLockPair(taskID)
        var acquired bool
        if err := tx.Raw(`SELECT pg_try_advisory_xact_lock(?, ?)`, hi, lo).Scan(&acquired).Error; err != nil {
            return err
        }
        if !acquired { return nil }  // другой Step уже работает, событие не теряется

        task := o.taskRepo.GetForUpdate(tx, taskID)
        if task.CancelRequested {
            o.cancelAllInFlightJobs(ctx, taskID)   // pg_notify task_cancel_<id>
            o.releaseAllWorktrees(ctx, taskID)
            o.taskRepo.SetState(tx, taskID, "cancelled")
            return nil
        }
        if task.State != "active" { return nil }

        state := o.loadState(tx, taskID)
        decision := o.router.Decide(ctx, state)
        o.logDecision(tx, taskID, task.CurrentStepNo, decision)

        if decision.Done {
            o.taskRepo.SetState(tx, taskID, decision.Outcome)
            o.releaseAllWorktrees(ctx, taskID)
            return nil
        }

        for _, agentReq := range decision.Agents {
            worktreeID := uuid.Nil
            if o.agentRepo.IsSandbox(agentReq.Name) {
                worktreeID = o.worktreeManager.Allocate(ctx, taskID, agentReq.Input.TargetArtifactID)
            }
            o.eventRepo.Enqueue(tx, AgentJobEvent{
                TaskID:     taskID,
                Agent:      agentReq.Name,
                Input:      agentReq.Input,
                WorktreeID: worktreeID,
            })
        }
        o.taskRepo.IncrementStep(tx, taskID)
        return nil
    })
}
```

**Agent Worker — race-free отмена** (новое в v4). Порядок действий **обязателен**:

```go
func (w *AgentWorker) Process(parentCtx context.Context, job AgentJob) error {
    // 1. Подписываемся на канал ДО любых проверок.
    cancelCh, unsubscribe, err := w.notifyConn.Subscribe(parentCtx, "task_cancel_"+job.TaskID.String())
    if err != nil { return err }
    defer unsubscribe()

    // 2. Только ПОСЛЕ подписки читаем текущий стейт — это ловит NOTIFY,
    //    отправленные между UPDATE cancel_requested=true и нашей подпиской.
    var cancelled bool
    if err := w.db.QueryRowContext(parentCtx,
        `SELECT cancel_requested FROM tasks WHERE id = $1`, job.TaskID).Scan(&cancelled); err != nil {
        return err
    }
    if cancelled {
        w.markJobCancelled(job.ID)
        w.releaseWorktreeIfAny(job)
        return nil
    }

    // 3. Запуск Exec под локальным ctx, отменяемым каналом cancelCh.
    ctx, cancel := context.WithTimeout(parentCtx, w.cfg.AgentJobTimeout)
    defer cancel()
    go func() {
        select {
        case <-cancelCh:
            cancel()                    // прерывает sandbox через ctx.Done()
        case <-ctx.Done():
        }
    }()

    artifact, execErr := w.dispatcher.Execute(ctx, job)
    // ... save artifact, enqueue step_req
    return execErr
}
```

**Cancellation pipeline:**
1. `POST /tasks/:id/cancel` → транзакция: `UPDATE tasks SET cancel_requested=true` + `pg_notify('task_cancel_<id>','')` (в одной tx — NOTIFY срабатывает на COMMIT).
2. **Все** in-flight sandbox/llm-воркеры этой задачи подписаны → их `ctx` отменяется → `SandboxRunner.Stop()` + cleanup (`SandboxAgentExecutor` уже корректно реагирует на `ctx.Done()`).
3. Worktrees освобождаются (`worktrees.state='released'`).
4. Следующий Step видит `cancel_requested=true` → финализирует задачу `state='cancelled'`.

### 2.9. Worktree management (новое в v3, hardened в v4)

**Проблема:** N параллельных sandbox-агентов в одном Git-репо = гонка за working directory.

**Решение:** `git worktree` — у каждого agent_job свой working tree, общий `.git`.

```sql
CREATE TABLE worktrees (
  id            UUID PRIMARY KEY,
  task_id       UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  subtask_id    UUID,                              -- artifact_id подзадачи (для трассировки)
  base_branch   TEXT NOT NULL,                     -- читается из конфига проекта, валидируется regex
  branch_name   TEXT NOT NULL,                     -- ВСЕГДА: task-<task_uuid>-wt-<worktree_uuid>
  state         TEXT NOT NULL DEFAULT 'allocated'  -- allocated|in_use|released
                CHECK (state IN ('allocated','in_use','released')),
  agent_job_id  BIGINT REFERENCES task_events(id),
  allocated_at  TIMESTAMPTZ DEFAULT now(),
  released_at   TIMESTAMPTZ
);
CREATE INDEX idx_worktrees_task ON worktrees (task_id);
```

**Изменения в v4 vs v3 (security hardening):**
- Колонка `path` **удалена** из таблицы. Путь больше **никогда не хранится** — он всегда **вычисляется** в коде: `filepath.Join(cfg.WorktreesRoot, taskID.String(), worktreeID.String())`. Это исключает path traversal через подмену БД (UUID.String() формирует строго `8-4-4-4-12` без `/` и `..`).
- `branch_name` хранится, но **никогда не приходит из LLM/пользователя** — формируется backend'ом из двух UUID. Валидация regex `^task-[0-9a-f-]{36}-wt-[0-9a-f-]{36}$` перед записью.
- `base_branch` — конфиг проекта (`projects.default_branch` или явный override). Валидация regex `^[a-zA-Z0-9._/-]{1,128}$`, отказ если начинается с `-`.

**Все git-вызовы используют `--` separator** (project-wide convention, см. `internal/agent/execution_types.go`):

```go
// internal/service/worktree_manager.go
func (m *Manager) Allocate(ctx context.Context, taskID, subtaskID uuid.UUID, baseBranch string) (uuid.UUID, error) {
    if err := validateBranchName(baseBranch); err != nil {
        return uuid.Nil, fmt.Errorf("base branch rejected: %w", err)
    }
    wtID := uuid.New()
    branchName := fmt.Sprintf("task-%s-wt-%s", taskID, wtID)
    path := m.computePath(taskID, wtID)  // filepath.Join only, no string concat

    // ВАЖНО: -- разделяет фиксированные флаги и пользовательский <base>
    cmd := exec.CommandContext(ctx, "git", "worktree", "add", path, "-b", branchName, "--", baseBranch)
    cmd.Dir = m.repoRoot
    if err := cmd.Run(); err != nil { ... }

    return m.repo.Create(ctx, Worktree{ID: wtID, TaskID: taskID, SubtaskID: subtaskID,
        BaseBranch: baseBranch, BranchName: branchName, State: "allocated"})
}

func (m *Manager) computePath(taskID, wtID uuid.UUID) string {
    // Только uuid.UUID на входе, .String() гарантированно безопасен.
    return filepath.Join(m.cfg.WorktreesRoot, taskID.String(), wtID.String())
}

func (m *Manager) Remove(ctx context.Context, wt Worktree) error {
    path := m.computePath(wt.TaskID, wt.ID)
    clean := filepath.Clean(path)
    // Defence-in-depth: путь обязан быть под root, никаких ../
    if !strings.HasPrefix(clean+string(filepath.Separator), m.cfg.WorktreesRoot+string(filepath.Separator)) {
        return fmt.Errorf("computed worktree path escapes root: %s", clean)
    }
    // git worktree remove также с -- перед путём
    cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", "--", clean)
    cmd.Dir = m.repoRoot
    return cmd.Run()
}
```

**Жизненный цикл:**
1. **Allocate** (в Step'е): `git worktree add <path> -b task-<task>-wt-<wt> -- <base>`. Запись в `worktrees` с `state='allocated'`.
2. **Use** (в Agent Worker): sandbox bind-mount'ит `path` как `/workspace`. `state='in_use'`.
3. **Release** (после agent_job или при cancel): `git worktree remove --force -- <path>`, `state='released'`.
4. **Cleanup cron** (раз в час): `worktrees` старше 1 суток в `state='released'` физически удаляются (`os.RemoveAll` с prefix-check).

**Merger:** получает в input `worktree_ids: [uuid, ...]`, для каждого вычисляет path тем же `computePath()`, делает `git merge` или `git cherry-pick` с `--` перед всеми ref-аргументами. Создаёт артефакт `kind='merged_code'`.

### 2.10. Бюджет контекста для Router

Router'у в промпт идут только metadata + `summary`. Полный `content` подгружается специалистом по `target_artifact_id`.

Структура промпта:
```
[system]
Ты — оркестратор. Правила параллелизма и review см. ниже.

Доступные агенты: ...
Жёсткие правила: ...

[user]
Задача: {{task.description}}

Подзадачи (DAG):
1. [id=...] "Add JWT middleware" — depends_on=[], state=coded, reviewed
2. [id=...] "Add login UI" — depends_on=[1], state=pending
3. [id=...] "Add logout button" — depends_on=[1], state=pending

In-flight jobs:
- developer работает над подзадачей 2 (started 5 мин назад)

История артефактов (последние 30):
1. [plan by planner] "MVP-план из 3 фич..."
2. [review by reviewer] "approved"
...

Какие агенты вызывать следующими? JSON.
```

**Никаких полных diff'ов в этом промпте.** Если артефактов > 50, оркестратор включает последние 30 + summary всех review/test_result.

---

## 3. Что удаляется

| Файл / сущность | Причина |
|---|---|
| `internal/service/orchestrator_pipeline.go` (весь файл) | Заменён на Router |
| `DetermineNextStatus()` | Логика flow в промпте Router'а |
| `handleExecutionResult()` (статусная часть) | Заменено на artifact-save + event enqueue |
| `Task.Status` enum (10 значений) | Заменён на 5 `Task.State` + `cancel_requested` |
| Все switch'и по статусу задачи в handlers / services | Не нужны |

## 4. Что переиспользуется

- `internal/agent/AgentExecutor` интерфейс — без изменений.
- `internal/agent/llm_executor.go` — для `execution_kind='llm'`.
- `internal/agent/sandbox_executor.go` — для `execution_kind='sandbox'` (все `code_backend` включая Hermes). Добавляется поддержка mounting worktree вместо clone.
- `internal/sandbox/*` — без изменений.
- `internal/llm/factory.go` — без изменений.

Новый `internal/service/agent_dispatcher.go` — switch по `execution_kind` (единственный допустимый).
Новый `internal/service/worktree_manager.go` — управление git worktree'ами.

---

## 5. Защита от runaway

| Уровень | Лимит | Действие |
|---|---|---|
| Глобальный | `max_steps_per_task = 100` | `state='needs_human'` |
| Глобальный | `task_timeout = 4h` (override через `custom_timeout`) | `state='failed'`, отмена всех sandbox + cleanup worktrees |
| Logical (промпт Router'а) | один артефакт ревьюится >5 раз | Router возвращает `done` с `outcome='failed'` |
| Стоимость | лимит токенов на задачу | `state='needs_human'` |
| Retry agent_job | `max_attempts = 3` | artifact-ошибка, Router решит |
| Router retry | `router_retry_budget = 2` | `state='needs_human'` |
| Параллельный fan-out | `max_parallel_agents_per_step = 10` | Router-промпт ограничивает, валидация на парсинге |

---

## 6. Frontend (Flutter)

1. **Agents Management** — CRUD + Secrets sub-screen с masked input.
2. **Task Detail v2** — DAG подзадач, таймлайн router_decisions (с `chosen_agents` массивом), expandable raw_response (с warning про retention).
3. **Worktrees view** (debug) — текущие активные worktrees, освобождение вручную при залипании.
4. **Cancel button** — `POST /tasks/:id/cancel`.
5. **Custom timeout** — поле при создании задачи (override 4h дефолта).

---

## 7. Спринты

### Sprint 1 — Schema, Models, Crypto, Security primitives ✅ ЗАВЕРШЁН (2026-05-14)

**Доставлено:**
1. Goose миграции 031..038: `agents` extension, `agent_secrets`, `artifacts`, `router_decisions`, `task_events`, `worktrees` (без колонки `path`!), `tasks` alter, seed 7 базовых агентов.
2. Дроп старых данных (dev-стенд) выполняется оператором через `TRUNCATE tasks CASCADE` перед `make migrate-up` если требуется.
3. Go-модели: `AgentSecret`, `Artifact`, `RouterDecision`, `TaskEvent`, `Worktree`; расширены `Agent` и `Task`. Добавлены типы `AgentExecutionKind`, `TaskState`, новые `AgentRole` (router/decomposer/merger).
4. Repositories: `AgentSecretRepo`, `ArtifactRepo`, `RouterDecisionRepo`, `TaskEventRepo` (с `ClaimNext` через raw SQL `FOR UPDATE SKIP LOCKED`), `WorktreeRepo`. Re-use существующего `pkg/crypto.AESEncryptor` (single-blob формат, `MinCiphertextBlobLen = 29` байт).
5. **`internal/logging/redact.go`** — slog.Handler wrapper, маскирует sensitive ключи (`raw_response`, `prompt`, `system_prompt`, `content`, `output`, `response`, `token`, `api_key`, ...). `SafeRawAttr()` для безопасного логирования длины+хэша.
6. **`internal/service/branch_validator.go`** — `ValidateBaseBranch`. Отвергает: ведущий `-`, ведущий `.`, control-chars, path-traversal, double-slash, reserved refs (HEAD/FETCH_HEAD/ORIG_HEAD/MERGE_HEAD/CHERRY_PICK_HEAD), reflog-syntax `branch@{...}`. 8 adversarial-тестов.
7. **`internal/service/task_lock.go`** — `TryLockTaskForStep` через `SELECT FOR UPDATE NOWAIT` на `tasks.id`. ВМЕСТО advisory lock (Yugabyte его поддерживает нестабильно). Sentinel'ы `ErrTaskLockBusy`/`ErrTaskNotFoundForLock`.
8. **`internal/service/redis_notifier.go`** + `go-redis/v9` v9.19.0 в go.mod. Pub/Sub для low-latency wakeup воркеров (`devteam:task_events`) и cancel-сигнала (`devteam:task_cancel:<task_id>`).
9. Seed 7 базовых агентов (идемпотентно, `ON CONFLICT (name) DO NOTHING`).

**Verified:** `go build ./...` ✅, `go test ./internal/...` ✅ (6+8 новых тестов проходят, прежние не сломаны).

**Отложено из Sprint 1 в другие спринты** (зафиксировано — не потеряется):
- `tasks.CustomTimeout` Go-field — колонка `INTERVAL` уже добавлена миграцией 037, но GORM scan для `time.Duration` требует кастомного `Scanner/Valuer`. Перенесено в **Sprint 3** (когда `Orchestrator.Step` начнёт это поле читать).
- **DROP `tasks.status`** legacy enum — миграция 039 в **Sprint 3** одним PR с удалением `orchestrator_pipeline.go` (чтобы сохранить компилирующийся проект между спринтами).
- **CI lint-правило** "no `slog.Default()` в orchestrator файлах" — **Sprint 5** (когда обновляем CI пайплайн вместе с MCP/Swagger/Frontend).
- **AgentRepository CRUD-интерфейс** — **Sprint 5** (вместе с MCP-инструментами `list_agents`/`create_agent`/`update_agent` и Frontend Agents Management screen). Сейчас агенты читаются прямым `gorm.DB.Find(&agents)` где нужно.

### Sprint 2 — Router, Dispatcher, Output validation
6. `router_service.go` — promptBuilder (metadata-only artifacts + DAG-метаданные + in-flight) + LLM-вызов + парсинг массива агентов + retry pipeline.
7. `agent_dispatcher.go` — switch `execution_kind`.
8. Юнит-тесты Router с mock LLM: фикстуры состояний (sequential, parallel, blocked, cancelled), кейсы галлюцинаций (markdown JSON, несуществующий агент, пустой массив, дубли target_artifact_id).

### Sprint 3 — Worktrees, Queue, Workers, Step ✅ ЗАВЕРШЁН (2026-05-14) с deferred-tail

**Доставлено:**
9. `worktree_manager.go` — Allocate (`git worktree add … -- <base>` с `--` separator), Release (idempotent, `--force`), MarkInUse, CleanupExpired. Путь вычисляется через `ComputePath`, БД-колонки `path` НЕТ; в CleanupExpired — defence-in-depth: OR-условие на prefix-check + равенство корню; `truncate()` rune-safe. 7 unit-тестов (5 без git + 2 integration с реальным git).
10. Worker pool (`step_worker.go`, `agent_worker.go`): polling 500ms + Redis Pub/Sub wakeup. Race-free cancel: Subscribe → SELECT `cancel_requested` → start Exec. `AgentResponseEnvelope` контракт + fallback на `raw_output`. Exponential backoff на Fail (1s→60s, max 60s).
11. `orchestrator_v2.go` — `Step()` через `TryLockTaskForStep` (`SELECT FOR UPDATE NOWAIT`). Внутри tx — **ТОЛЬКО** БД-операции (lock/load/router_decide/save_decision/enqueue_events/increment). `git worktree add` вынесен в AgentWorker (just-in-time перед Execute) — устраняет orphaned records при tx rollback. `worktree release` + `NotifyTaskCancel` — post-commit hooks (`scheduleWorktreeRelease`, `scheduleCancelNotify`).
12. `task_lifecycle.go` (`RequestCancel`) + `retention.go` (`RunOnce*` / `Run` для cron). HTTP-хендлеры в Sprint 5 (см. ниже).
14. `Task.CustomTimeout *IntervalDuration` с custom `sql.Scanner` + `driver.Valuer` ([interval_duration.go](backend/internal/models/interval_duration.go)). Поддержка форматов: `HH:MM:SS[.ffffff]`, `D days HH:MM:SS`, `N microseconds/seconds/minutes/hours/...`. 8 unit-тестов.

**Stage 5b (частично доставлено):** удалены 10 legacy-файлов (~3000 строк): `orchestrator_pipeline.go`, `orchestrator_service.go` + tests, 5× `result_processor*.go` + test. `secretPatterns` сохранён в `secret_scrub.go`. Новый интерфейс `service.TaskOrchestrator` (`EnqueueInitialStep`); consumers (`conversation_service`, `handler/task_handler`, `mcp/tools_task`) переключены. В `main.go` — `stubV2Orchestrator` (кладёт step_req в очередь). Test suite зелёный, никаких регрессий.

**Завершено в полном объёме (включая ранее DEFERRED-пункты):**

| Пункт | Статус |
|---|---|
| 13 | ✅ Миграция [039](backend/db/migrations/039_drop_tasks_status_legacy.sql) — `DROP COLUMN tasks.status`. `TaskStatus` enum + `Task.Status` поле удалены из [models/task.go](backend/internal/models/task.go). State-machine упрощён 10→5: `allowedTransitions` теперь покрывает `active ↔ active|done|failed|cancelled|needs_human`, `needs_human → active|cancelled`, `failed → active`. Refactor 14 consumer-файлов: `task_service.go` (78 правок), `handler/task_handler.go`, `dto/task_dto.go`, `mcp/tools_task.go`, `indexer/task_indexer.go`, `repository/task_repository.go` (TaskFilter `Status` → `State`), + все их test-файлы (~120 правок). 6 legacy state-transition тестов удалены (тестировали несуществующую теперь 10-значную state-machine). |
| 15a | ✅ Cron retention `router_decisions` (30 дней) — запущен goroutine в [main.go](backend/cmd/api/main.go) через `RetentionService.Run(ctxWorker)`. |
| 15b | ✅ Cron retention worktrees (1 сутки после release) — same goroutine. |
| 5g | ✅ Полный v2 DI в [main.go](backend/cmd/api/main.go): `SingletonLLMProviderResolver` + `SingletonSandboxExecutorFactory` + `DBAgentLoader` (adapters в [v2_di_adapters.go](backend/internal/service/v2_di_adapters.go)); `AgentDispatcher` + `RouterService` + опциональный `WorktreeManager` (`WORKTREES_ROOT`+`REPO_ROOT` env) + опциональный `RedisNotifier` (`REDIS_URL` env, fallback на polling); `Orchestrator` v2 заменил `stubV2Orchestrator`; 5×`StepWorker` + 22×`AgentWorker` запущены как goroutines с `ctxWorker` для graceful shutdown; `RetentionService` goroutine; `TaskLifecycleService` инициализирован. |

**Status:** Sprint 3 закрыт полностью. Все 15 internal пакетов компилируются и тесты зелёные.

### Sprint 4 — Merger, Tester, Integration ✅ ЗАВЕРШЁН (2026-05-14)

**Доставлено:**
15. **Merger-агент**: refined system_prompt (миграция [040](backend/db/migrations/040_refine_merger_tester_prompts.sql)) — теперь даёт JSON-envelope с `MergerOutput` (`merged_branch`, `source_worktree_ids[]`, `merge_conflicts_resolved[]`, `checks_run/passed`, `head_commit_sha`). Multi-worktree mount в sandbox-runner'е — отложен до фактической потребности (Sprint 5 wiring); seed-prompt инструктирует агента работать через `git merge`/`rebase` в одном worktree (Claude Code сам клонит ветки внутри контейнера).
16. **Tester-агент**: refined system_prompt (миграция 040) — даёт JSON-envelope с `TestResult` (passed/failed/skipped/duration_ms/coverage/build_passed/lint_passed/typecheck_passed/failures[]). `AllPassed()` helper для Router'а.
17. **Integration tests** (component-level + scenario-driven):
    - ✅ **Sequential happy path** (`TestScenario_Sequential_PlanReviewCodeReviewTest`) — 6-шаговый цикл план→ревью→код→ревью→тест→done.
    - ✅ **Parallel fan-out** (`TestScenario_Parallel_TwoDevsThenMerger`) — Router возвращает массив из 2 developer'ов, потом merger.
    - ✅ **Hallucination recovery** (`TestScenario_HallucinationRecovery_UnknownAgent`) — Router придумал агента → corrective prompt → recovery на retry.
    - ✅ **Fallback to needs_human** (`TestScenario_HallucinationFallback_NeedsHuman`) — 3 невалидных ответа подряд → Done(needs_human), не error.
    - ✅ **MergerOutput contract** (`TestScenario_MergerOutputContract`) — envelope → artifact → ParseMergerOutput → структура восстановлена.
    - ✅ **TestResult contract** (`TestScenario_TestResultContract`) — то же для тестов.
    - ✅ **Security canary E2E** (`TestScenario_SecurityCanary_EndToEnd`) — `FULL_PIPELINE_CANARY_no_leak_allowed_anywhere` не появляется в логах ни Router'а, ни AgentWorker'а.

**Bonus deliverables:**
- [models/agent_outputs.go](backend/internal/models/agent_outputs.go) — типизированные `MergerOutput`/`TestResult` + парсеры с валидацией обязательных полей (включая строгую проверку наличия `build_passed`/`lint_passed`/`typecheck_passed` через двухпроходный map-парс). 9 unit-тестов (3 на Merger + 6 на TestResult, включая table-driven отказы для missing required fields и `failed>0 without failures[]`).
- [service/agent_worker_test.go](backend/internal/service/agent_worker_test.go) — 13 тестов AgentWorker: envelope parsing, fallback на raw_output, supersede previous reviews (но не plan/code), summary trunctation, allocateWorktreeForJob validation, canary в fallback path.

**Отложено в Sprint 5 (явно зафиксировано, не "deferred infinitely"):**
- **DAG-scenario test** (4 подзадачи, 3 и 4 зависят от 1) — требует real postgres для drive'а полного `Orchestrator.Step` транзакции с `FOR UPDATE NOWAIT` (sqlite не умеет).
- **Cancel mid-flight + Restart mid-task** scenario tests — требуют real postgres + multi-process (testcontainers + sub-test goroutines).
- **Multi-worktree sandbox mount** для Merger — `SandboxAgentExecutor` не знает о N worktree'ях; нужен либо рефакторинг executor'а, либо merger клонит ветки сам из своего worktree (текущий промпт идёт по второму пути). Решить при первом реальном multi-subtask запуске.
- **Worker pool integration** через `TaskEventRepository.ClaimNext` (raw SQL с `SKIP LOCKED`) — Sprint 5 setup для testcontainers-postgres.

**Coverage delta после Sprint 4 (включая review nit-fixes):** +38 тестов поверх Sprint 3 baseline
(9 в [agent_outputs_test.go](backend/internal/models/agent_outputs_test.go),
13 в [agent_worker_test.go](backend/internal/service/agent_worker_test.go),
16 в [orchestration_scenarios_test.go](backend/internal/service/orchestration_scenarios_test.go)).
0 регрессий по всем 15 internal/* пакетам.

**Дополнительные правки по nit-review:**
- `redactRawOutputToSentinel` — sentinel-fallback при сбое scrub'а: `raw_output_truncated`
  заменяется на `{"_scrub_failed": true, "len": N, "head_sha256_8": "..."}` вместо
  сохранения unscrubbed-данных. Если даже sentinel не построился (битый JSON) —
  артефакт не сохраняется (`saveArtifact` возвращает error → event retry → eventually
  needs_human через max_attempts). Тесты: `TestRedactRawOutputToSentinel_ReplacesWithHashAndLength`,
  `TestRedactRawOutputToSentinel_NoOpWhenNoRawField`, `TestSaveArtifact_TestResult_FailsWhenContentNotObject`,
  `TestSaveArtifact_TestResult_SentinelPathDoesNotTriggerOnValidObject`.
- `ScrubSecrets(s string) string` — public helper в [secret_scrub.go](backend/internal/service/secret_scrub.go),
  переиспользуем из других пакетов (например, для аналогичной фильтрации
  `merger.merge_conflicts_resolved[].resolution` в будущем).

### Sprint 5 — MCP, Lint, Docs ✅ ЗАВЕРШЁН (Stages 5A-5E)
18. ✅ `AgentRepository` интерфейс + impl ([agent_repository.go](backend/internal/repository/agent_repository.go)) с CRUD + `List(AgentFilter)` + sentinel `ErrAgentNameTaken`.
19. ✅ MCP-инструменты v2:
    - Агенты: `agent_list`, `agent_get`, `agent_create`, `agent_update`, `agent_set_secret`, `agent_delete_secret` ([tools_agents_v2.go](backend/internal/mcp/tools_agents_v2.go)).
    - Оркестрация: `artifact_list`, `artifact_get`, `router_decision_list`, `worktree_list`, `task_cancel_v2` ([tools_orchestration_v2.go](backend/internal/mcp/tools_orchestration_v2.go)).
    - Все wire'ятся через `Dependencies` в `mcp/server.go` опционально (nil → tool не регистрируется); подключены в `cmd/api/main.go`.
22. ✅ **CI lint-правило** [`.golangci.yml`](backend/.golangci.yml) — `forbidigo` запрещает `slog\.Default` ТОЛЬКО в orchestrator-файлах (через `path-except` regex). Введён [`logging.NopLogger()`](backend/internal/logging/redact.go) (discard + redact wrapper) как nil-fallback в 7 orchestrator-конструкторах вместо `slog.Default()`.
23. ✅ Обновлены [`docs/rules/backend.md`](docs/rules/backend.md) §2.3 (5 правил Sprint 17: `--` separator, no `slog.Default`, no raw LLM в логах, path-safety, шифрование секретов) и [`docs/rules/main.md`](docs/rules/main.md) (раздел "Orchestration v2") со ссылкой на план.

### Sprint 5F — Swagger / testcontainers / Frontend ✅ ЗАКРЫТО (2026-05-15)
20. ✅ Swagger перегенерирован (`make swagger` чистый).
21. ✅ Flutter v2:
    - **Agents Management** ([agents_v2_list_screen.dart](backend/../frontend/lib/features/admin/agents_v2/presentation/screens/agents_v2_list_screen.dart) + [agent_v2_detail_screen.dart](backend/../frontend/lib/features/admin/agents_v2/presentation/screens/agent_v2_detail_screen.dart)) — CRUD + диалог секретов с masked-input, AES-GCM на бэке.
    - **Task Detail v2 sections** — [ArtifactsDagSection](backend/../frontend/lib/features/tasks/presentation/widgets/artifacts_dag_section.dart) (группировка по `kind`, `depends_on` стрелки) + [RouterTimelineSection](backend/../frontend/lib/features/tasks/presentation/widgets/router_timeline_section.dart) (карточки шагов с outcome/reason).
    - **Worktrees debug** ([worktrees_list_screen.dart](backend/../frontend/lib/features/admin/worktrees_v2/presentation/screens/worktrees_list_screen.dart)) — read-only список (release-кнопка отложена в Sprint 6).
    - **Cancel button** — уже работало через `TaskHandler.Cancel` → `lifecycleV2.RequestCancel`; v2 wiring подтверждён.
    - **custom_timeout** — добавлен в `CreateTaskRequest` Dart-модель и backend DTO (`time.ParseDuration` + `ErrTaskInvalidTimeout` → 400). UI-формы создания задачи в проекте нет (задачи рождаются из чата) — поле готово к проводке когда форма появится.
22. ✅ testcontainers-postgres setup для full E2E integration tests:
    - `backend/internal/service/orchestrator_pg_integration_test.go` — build tag `integration`, harness через `tcpostgres.Run`.
    - 4 новых сценария + baseline DONE: `TestPGIntegration_DAG_DependsOn`, `TestPGIntegration_CancelMidFlight`, `TestPGIntegration_RestartMidTask`, `TestPGIntegration_MaxStepsPerTask_NeedsHuman`.
    - `go test -tags integration -race ./internal/service/...` зелёный в Docker.
23. ✅ HTTP read-only API для v2 (`internal/handler/orchestration_v2_handler.go`):
    - `GET /tasks/:id/artifacts` (metadata) + `GET /tasks/:id/artifacts/:artifactId` (full content с cross-task ownership check)
    - `GET /tasks/:id/router-decisions` (без `encrypted_raw_response`)
    - `GET /worktrees?task_id=...` (без `path`-колонки)
24. ✅ Миграция 038 fix (партиальный unique index + расширение `chk_agents_role` ролями router/decomposer/merger) — было блокером для всего goose up на свежей БД.

### Sprint 6 — Follow-ups после Sprint 17 (план зафиксирован 2026-05-15, ревью 2026-05-15)

Это backlog задач, которые либо вылезли по ходу Sprint 17, либо были явно отложены. Каждая — небольшая (½–1 дня), фокусированная.

**Code review pass (2026-05-15)** добавил в каждый пункт security/performance/compliance корректировки. Они отмечены как *"добавлено code review 2026-05-15"* и закрывают:
- 6.1 — race condition Cancel-vs-done: SELECT FOR UPDATE + HTTP 409, штатная обработка на клиенте
- 6.2 — admin-only branch без task_id, composite b-tree index `(state, allocated_at)` против Full Table Scan
- 6.3 — command/flag/path injection в `git worktree remove`: array exec.CommandContext + `--` separator + computed path validation
- 6.4 — lazy `ListView.builder` + truncation (50K chars, "Copy full" для гигантских артефактов)
- 6.5 — server-side bounds `1m..72h` против целочисленного переполнения и DoS через 0s
- 6.7 — обязательный freezed pattern `abstract class with _$Model` для новых моделей
- 6.8 — реальный YugabyteDB через `docker-compose` в CI + правильный scope `./...` (не `./internal/service/...`)
- 6.9 — обязательное обновление `make swagger` после правок DTO

#### ✅ 6.1. Cancel/pause logic под v2 state-machine
**Проблема.** [task_detail_screen.dart:42-55](backend/../frontend/lib/features/tasks/presentation/screens/task_detail_screen.dart) держит legacy-проверки на 10-значные статусы (`pending`/`planning`/`in_progress`/`review`/`changes_requested`/`testing`/`paused`/`failed`). После миграции 039 `tasks.status` дропнут, `TaskResponse.Status` теперь маппится на 5-значный `state` (`active`/`done`/`failed`/`cancelled`/`needs_human`) — старые предикаты не сматчатся.

**Что сделать.**
- В `_taskDetailShowCancelForStatus` оставить только `state == 'active'` (или `active|needs_human` если решим разрешать).
- `_taskDetailShowPauseForStatus` — pause-эндпоинт остался legacy (см. [task_handler.go:373](backend/internal/handler/task_handler.go)), но v2-pause не реализован. Либо убираем кнопку, либо реализуем `tasks.state='paused'` (потребует +1 значение в CHECK + transition в `task_service.allowedTransitions`).
- `_taskDetailShowResumeForStatus` — то же, что pause.
- Обновить юнит-тесты `task_detail_screen_test.dart` (там тестируется панель lifecycle).

**Race condition при Cancel** *(добавлено code review 2026-05-15)*.
Между моментом, когда фронт прочитал `state='active'`, и моментом, когда дёргает `POST /tasks/:id/cancel`, задача может завершиться (`state='done'`). Сейчас `TaskHandler.Cancel` → `task_service.Cancel` делает UPDATE без явного guarding на текущий state — возможно перезаписывание `done` → `cancelled` (теряем finalization).

- Backend: `task_service.RequestCancel` (или `Cancel`) **обязан** использовать `SELECT ... FOR UPDATE NOWAIT` на `tasks.id`, проверить `state == 'active'`, затем UPDATE. Если state уже терминальный — вернуть sentinel `ErrTaskAlreadyTerminal` → handler маппит в **HTTP 409 Conflict** (не 500, не 200).
- В `lifecycleV2.RequestCancel` тот же контракт; уже работает через `SELECT FOR UPDATE` на task. Подтвердить тестом `TestPGIntegration_CancelAfterDone_Returns409`.
- Frontend: dio-репозиторий ловит `DioException` со `statusCode == 409` и маппит в `TaskAlreadyTerminalException` (или extends существующего). Контроллер при ловле — **не показывает красный SnackBar**, а просто `ref.invalidate(taskDetailProvider)` (обновляем state из БД) + info-toast "Task already finished".

**Why это блокер UX.** Иначе кнопка Cancel будет показываться/скрываться неправильно — пользователь не сможет отменить активную задачу или, наоборот, увидит Cancel у уже-завершённой; race-кейс выдаст 500-ошибку и пугающий красный экран на штатной ситуации.

#### 6.2. GET /worktrees без task_id
**Проблема.** Backend handler `ListWorktrees` сейчас требует `?task_id=...` (см. отчёт agent'а в Sprint 5F), потому что `WorktreeRepository` экспонирует только `ListByTaskID`. Фронт-экран `worktrees_list_screen.dart` зовёт `repo.list()` без taskId → 400 Bad Request.

**Что сделать.**
- В `internal/repository/worktree_repository.go` добавить `List(ctx, WorktreeFilter)` с фильтрами `state ∈ {allocated|in_use|released}`, `task_id *UUID`, `limit`, `offset`. Сортировка `allocated_at DESC`.
- В handler `ListWorktrees` снять обязательность `task_id`, при отсутствии — вернуть глобальный список (с лимитом 200 по умолчанию).
- Юнит-тест: `TestListWorktrees_GlobalList_ReturnsRecentFirst` + `TestListWorktrees_FilterByState`.
- Frontend: дополнить `WorktreesRepository.list()` опц. `state` фильтром в UI (chip-фильтр сверху экрана).

**Security: admin-only без task_id** *(добавлено code review 2026-05-15)*.
Глобальный список worktrees раскрывает: какие задачи активно выполняются прямо сейчас, имена веток (потенциально содержат бизнес-контекст), таймлайн allocation'ов. Это **чувствительная инфо для дебага**, не публичная.

- `internal/server/server.go`: маршрут `GET /worktrees` без `task_id` должен идти через `AdminOnlyMiddleware()` (как `/llm`, `/prompts`, `/workflows`).
- Если `task_id` указан и пользователь — owner task'а, доступ через стандартный authMW. Логика split'а — в handler: проверить query, если no task_id → require admin role, иначе → require task ownership.
- Юнит-тест: `TestListWorktrees_NoTaskID_NonAdmin_Returns403` + `TestListWorktrees_OwnTaskID_NonAdmin_Returns200`.

**Performance: индекс на allocated_at** *(добавлено code review 2026-05-15)*.
`ORDER BY allocated_at DESC` без индекса → Full Table Scan при росте таблицы (~5-10K записей при активной разработке за полгода).

- Проверить миграцию 036 (`create_worktrees`): уже создан ли b-tree индекс на `allocated_at`?
- Если нет — миграция 042: `CREATE INDEX idx_worktrees_state_allocated ON worktrees(state, allocated_at DESC)`. Композит, потому что в 90% запросов идёт фильтр `state IN (...)` + сортировка.
- EXPLAIN ANALYZE на 10K-record fixture: убедиться что план использует Index Scan, не Seq Scan.

**Why.** Worktrees debug screen без global list бесполезен — оператор не знает task_id наперёд.

#### 6.3. Release worktree кнопка (manual unstick)
**Проблема.** Worktrees могут залипать в `state='in_use'` если sandbox-процесс упал между Step'ами. l10n-строки `worktreesReleaseButton`/`worktreesReleasedSnackbar` уже есть в `app_en.arb`/`app_ru.arb`, но action не подключён.

**Что сделать.**
- Backend: `POST /worktrees/:id/release` → `WorktreeManager.Release(ctx, id)` (метод существует, идемпотентен). Auth: admin-only. Audit-log с user_id.
- Service-сентинели: `ErrWorktreeAlreadyReleased` → 409, `ErrWorktreeNotFound` → 404.
- Frontend: `WorktreesRepository.release(id)` + IconButton(Icons.cleaning_services) в `_WorktreeTile`. Confirmation-dialog с предупреждением "git worktree remove --force произойдёт прямо сейчас".
- Тест: интеграционный `TestPGIntegration_WorktreeManualRelease`.

**Security: command/flag/path injection** *(добавлено code review 2026-05-15)*.
`git worktree remove --force` запускается ровно тогда, когда оператор жмёт кнопку — это external trigger, не internal cron. Хотя сам путь формируется кодом (не приходит из API), парадигма безопасности проекта обязывает применять защиту defence-in-depth:

- `WorktreeManager.Release(ctx, wt)` использует **только** `exec.CommandContext(ctx, "git", "worktree", "remove", "--force", "--", path)` — массив аргументов, никакого `fmt.Sprintf("git ... %s", path)` / `bash -c`.
- `path` вычисляется внутри Release через `ComputePath(taskID, wtID)` от типизированных `uuid.UUID` (как в `Allocate`). НЕ читать `path` из БД — мы его не храним (Sprint 1 v4 hardening).
- Defence-in-depth: `path := filepath.Clean(path); if !strings.HasPrefix(path+sep, root+sep) { return ErrInvalidPath }` — повторить тот же guard, что в `Manager.Remove` ([worktree_manager.go:510-521 spec](docs/orchestration-v2-plan.md)).
- Юнит-тест: `TestWorktreeManager_Release_PathOutsideRoot_Rejected` (если каким-то образом подменили вычислитель пути) + `TestWorktreeManager_Release_FlagInjection_Rejected` (через моки `exec.Cmd.Args`, проверить что `--` присутствует и ID-формы безопасны).
- Audit-log: `slog.InfoContext(ctx, "worktree manually released", "worktree_id", wt.ID, "task_id", wt.TaskID, "user_id", userID, "user_role", userRole)`. Path и branch_name **не** логируем (внутренний нейминг, не нужно в Kibana).

**Why.** Оператор должен иметь способ runaway-fix без `psql UPDATE worktrees SET state='released'`. Безопасность важнее, потому что путь к git-worktree — это runtime-side-effect, любой shell-escape → произвольное удаление файлов под uid backend-процесса.

#### 6.4. Full artifact viewer (с content)
**Проблема.** Backend `GET /tasks/:id/artifacts/:artifactId` отдаёт полное `content` (включая `code_diff` patch'и, `merged_code` SHA, `test_result.failures[]`), но `OrchestrationV2Repository.listArtifacts()` на фронте возвращает только metadata, и UI-метода для просмотра одного артефакта нет.

**Что сделать.**
- `OrchestrationV2Repository.getArtifact(taskId, artifactId)` → `Artifact` с заполненным `content`.
- Provider `artifactDetailProvider.family<Artifact, (String, String)>`.
- IconButton(Icons.open_in_new) в `_ArtifactTile` → открывает диалог с pretty-printed JSON (используй `JsonEncoder.withIndent('  ')`).
- Для `kind == 'code_diff'` — рендерить через существующий `DiffViewer` (есть в `shared/widgets/diff_viewer.dart`).
- Для `kind == 'review'` — табличка `decision`/`issues[]`/`summary`.
- Для `kind == 'test_result'` — `passed/failed/skipped/duration` сверху + `failures[]` expandable.

**Performance: lazy rendering и truncation** *(добавлено code review 2026-05-15)*.
Агенты регулярно генерят `code_diff` на 5000-15000 строк. Если builder отрендерит всё разом в `Column` / `SelectableText` — UI thread зависнет на сотни ms, на слабых клиентах — секунды.

- `DiffViewer` **обязан** использовать `ListView.builder` с per-line виджетом (не `Column`/`Wrap` всего диффа). Высота строки фиксирована → дешёвый `itemExtent` или `prototypeItem` для viewport-вычислений без layout.
- При `content.length > 50000` chars (для JSON-полей) — показывать первые 50K + кнопку **"Show full (N KB)"**, по нажатию подгружать в отдельный полноэкранный диалог.
- Для `test_result.failures[]` если массив >50 элементов — пагинация по 20 (`PageController` или просто limit + "Show next 20").
- Для огромного JSON `merged_code` — кнопка **"Copy full content"** в clipboard через `Clipboard.setData`, тут же snackbar "Copied N bytes". Не пытаться рендерить.
- Юнит-тест: `TestDiffViewer_LargeDiff_DoesNotBuildAllTilesAtOnce` (golden + `find.byType(LineTile).evaluate().length < 100` при `pumpAndSettle()` на 5000-line diff).

**Why.** DAG-вид сейчас показывает только summary — невозможно проверить что developer-агент реально написал, что reviewer одобрил. Lazy rendering — обязательно, иначе viewer становится непригоден на больших артефактах (а на маленьких overhead на ListView.builder копеечный).

#### 6.5. Custom timeout UI
**Проблема.** Поле `customTimeout` есть в `CreateTaskRequest` Dart-модели, но в проекте задачи создаются из chat-conversation, отдельной "create task" формы нет. l10n-строки `tasksCustomTimeoutLabel`/`tasksCustomTimeoutHelper`/`tasksCustomTimeoutInvalid` уже подготовлены.

**Что сделать (выбрать вариант).**
- Вариант A: добавить inline expandable "Advanced" в `chat_screen.dart` рядом с input bar — текст-поле "Custom timeout (4h)" + tooltip с правилами парсинга.
- Вариант B: на `task_detail_screen.dart` добавить редактируемое поле "Timeout" в header'е (PATCH `/tasks/:id` с `custom_timeout`). Бэкенд-эндпоинт `UpdateTask` уже принимает поле через DTO (после доработки в Sprint 5F).
- Валидация на клиенте: regex `^\d+(h|m|s)(\d+(m|s))?$`. Bad input → показать `tasksCustomTimeoutInvalid`.

**Security: strict server-side bounds** *(добавлено code review 2026-05-15)*.
Клиентская regex — first line of defence. Backend **обязан** проверить bounds, потому что:
- `999999999h` → `time.ParseDuration` вернёт ошибку (int64 overflow), но `9223372036s` (≈292 года) — нет, и orchestrator с таким timeout эффективно никогда не падёт в `state='failed'`.
- `0s` → workers сразу видят `ctx.Err()`, ретраят, через `max_attempts=3` упадёт в needs_human, но успеет сжечь сэндбокс-слоты и LLM-токены.

- В `task_service.go::Create` (и `Update`) после `time.ParseDuration(*req.CustomTimeout)`:
  ```go
  const (
      minCustomTimeout = 1 * time.Minute
      maxCustomTimeout = 72 * time.Hour
  )
  if d < minCustomTimeout || d > maxCustomTimeout {
      return ErrTaskInvalidTimeout
  }
  ```
- `ErrTaskInvalidTimeout` маппится handler'ом в **HTTP 400** с error_code `invalid_timeout` и человекочитаемым messsage "custom_timeout must be in range 1m..72h".
- Юнит-тесты: `TestCreate_RejectsTimeoutBelowMin`, `TestCreate_RejectsTimeoutAboveMax`, `TestCreate_AcceptsBoundaryValues`.
- Frontend: те же bounds как helper-message в InputDecoration ("Min 1m, max 72h"); если backend вернул 400 — показать message из ответа в form field error, не дублировать клиентскую regex-ошибку.

**Why.** Без UI поле бесполезно — никто не может задать override 4h дефолта. Без server-side bounds — мы доверяем клиенту в безопасности (нельзя), что нарушает Правило 9 из docs/rules/main.md.

#### 6.6. Навигация к v2 admin-экранам
**Проблема.** `/admin/agents-v2` и `/admin/worktrees` зарегистрированы в `app_router.dart`, но нигде нет ссылки. Пользователь должен набирать URL вручную.

**Что сделать.**
- В существующем admin-меню/сайдбаре (нужно найти где админ-секция отображается — вероятно в `settings_screen.dart` или `dashboard_screen.dart`) добавить две записи:
  - "Agents (v2)" → `/admin/agents-v2`
  - "Worktrees (debug)" → `/admin/worktrees`
- Иконки: `Icons.psychology` для агентов, `Icons.account_tree` для worktrees.
- Visibility — admin-only (через role check, как другие admin-маршруты).

**Why.** Discovery: без записи в навигации экран не существует для конечного пользователя.

#### 6.7. Widget-тесты на новые экраны
**Проблема.** Правило `docs/rules/frontend.md` требует тесты на новые экраны. Sprint 5F добавил 7 файлов без тестов.

**Что сделать (минимум).**
- `test/features/admin/agents_v2/presentation/screens/agents_v2_list_screen_test.dart` — golden + 3 кейса: empty / data / error через `agentsV2RepositoryProvider.overrideWith(MockRepo)`.
- `test/features/admin/agents_v2/presentation/screens/agent_v2_detail_screen_test.dart` — submit valid form, error handling, hidden secret field.
- `test/features/admin/worktrees_v2/presentation/screens/worktrees_list_screen_test.dart` — empty / data / refresh.
- `test/features/tasks/presentation/widgets/artifacts_dag_section_test.dart` — depends_on rendering + group ordering.
- `test/features/tasks/presentation/widgets/router_timeline_section_test.dart` — outcome chip / multi-agent decisions.

**Compliance: freezed pattern** *(добавлено code review 2026-05-15)*.
Сейчас новые модели (`AgentV2`, `Artifact`, `RouterDecision`, `WorktreeV2`) написаны как `@immutable class ... { ... }` — не freezed. Если в Sprint 6 для viewer'а (6.4) или для test-fixture'ов потребуются новые модели — использовать freezed строго в форме `abstract class ModelName with _$ModelName` (правило проекта, см. CLAUDE.md / frontend.mdc).

- Если решим мигрировать существующие 4 модели на freezed — это **отдельный** atomic-refactor PR с прогоном `dart run build_runner build --delete-conflicting-outputs`. Не смешивать с виджет-тестами.
- Для виджет-тестов достаточно простых fixture-builders в `test/_fixtures/` — без freezed.

**Why.** Регрессии через `flutter analyze` не ловятся; UI ломается при изменении l10n keys. Freezed-pattern важен потому что без `abstract class` build_runner молча генерит broken code, CI падает не на коммите модели, а на следующем диффе.

#### ✅ 6.8. Backend integration tests на CI
**Проблема.** `go test -tags integration ./internal/service/...` требует Docker и сейчас прогоняется только локально. DoD §9 требует "race detector чистый" — это нужно проверять в CI.

**Что сделать.**
- В существующем CI workflow (`.github/workflows/*.yml` — нужно найти) добавить job `integration-tests`:
  - Service container или явный `docker-compose up -d yugabytedb` на YugabyteDB (см. `compose.yml` в репо), wait-for-port `5433` через healthcheck (`pg_isready` или netcat-loop ≤30s).
  - НЕ "просто Docker есть на ubuntu-latest" — testcontainers сами запустят временный postgres, но: (1) `compose.yml` использует YugabyteDB, и мы хотим E2E на нашей реальной БД, не на минимальном postgres-моке; (2) часть тестов в `pkg/gitprovider`, `internal/sandbox`, `internal/agent` тоже под тегом `integration` и требуют реальных ресурсов (git, docker для sandbox).
  - Шаг: `cd backend && go test -tags integration -race -timeout 600s ./...` (**`./...`, а не `./internal/service/...`** — Правило 5 backend.mdc: интеграционные тесты живут в repository, sandbox, gitprovider слоях тоже).
  - Дополнительно — `golangci-lint run` с `.golangci.yml` (правило `forbidigo: slog\.Default` уже там, добавить static-grep на `exec.Command(.*git.*)` без `--` separator).
- Cache go modules + docker images через `actions/cache` (с key из `go.sum`).
- Параллелизация: split на `unit-tests` (`go test -short ./...`) + `integration-tests` (`-tags integration -race ./...`) — разные jobs, runner ждёт оба перед merge.

**Why.** Без CI новый PR может сломать integration-тесты, никто не заметит. Без правильного scope (`./...`) мы пропустим repository-уровень — например, regression в `task_event_repository.ClaimNext` с `FOR UPDATE SKIP LOCKED` не словится.

**Сделано (2026-05-15):**
- `.github/workflows/backend-ci.yml` разбит на три параллельных job'а:
  - `unit-tests` — прежний `go test -race ./...` (integration-тесты исключены build tag'ом).
  - `integration-tests` — `docker compose up -d yugabytedb` + healthcheck-loop через `ysqlsh \\l` (≤120s), затем `goose up` против `localhost:5433`, затем `go test -tags integration -race -timeout 600s ./... -count=1`. Scope `./...` покрывает `repository/`, `sandbox/`, `agent/`, `service/`, `pkg/gitprovider/`. Sandbox-тесты `t.Skip` при отсутствии `devteam/sandbox-claude:local` (образ в CI не собираем — экономия времени; локально собирается `make sandbox-build`).
  - `lint` — `golangci-lint run` (forbidigo: `slog\.Default`) + `bash scripts/static-grep-git.sh` (git flag-injection — multiline-aware grep, exempts `args...` slice spreads).
- Кеширование: `actions/setup-go` для Go-модулей по `go.sum`; `actions/cache` для YugabyteDB-образа (docker save tarball) по ключу образа (~1.7GB).
- Teardown: `docker compose logs yugabytedb` при failure + `docker compose down -v --remove-orphans` в `always()`.
- `scripts/static-grep-git.sh` — проверяет, что все `exec.Command(Context)?("git", ...)` в production-коде (`backend/internal`, `backend/pkg`, исключая `*_test.go`) содержат `"--"` separator в окне +5 строк, либо делегируют argv через `args...` (тогда `--` гарантирует helper, покрытый unit-тестами).

#### 6.9. Документация
**Проблема.** `docs/rules/main.md` и `docs/rules/frontend.md` не обновлены под v2 (DoD §9 / Sprint 5 пункт).

**Что сделать.**
- `docs/rules/main.md` — секция "Orchestration v2 (Sprint 17)" со ссылкой на этот план + ключевыми инвариантами:
  - flow=данные, не switch'и в Go
  - артефакты идут через reviewer
  - `--` separator во всех git-вызовах
  - secrets только через `agent_secrets` (AES-GCM), не в `sandbox_settings`
- `docs/rules/backend.md` — добавить **обязательный** пункт: "После любых правок в `internal/handler/dto/*.go` или JSON-теге существующих handler'ов прогнать `make swagger` и закоммитить результат в `backend/docs/`. PR без обновлённой swagger.{json,yaml} не мерджится". Это критично после 6.2 (новые worktree-фильтры) и 6.5 (custom_timeout bounds в `UpdateTaskRequest`).
- `docs/rules/frontend.md` — секция "Orchestration v2 фичи":
  - Agents Management → `frontend/lib/features/admin/agents_v2/`
  - Worktrees debug → `frontend/lib/features/admin/worktrees_v2/`
  - Task Detail v2 sections — `frontend/lib/features/tasks/presentation/widgets/`
  - **Правило**: для новых v2-виджетов используй `requireAppLocalizations(context, where: '<WidgetName>')`, не `AppLocalizations.of(context)!`.
  - **Правило**: freezed-модели всегда в форме `abstract class ModelName with _$ModelName` (см. 6.7 compliance note).

**Why.** Без актуальной документации новые контрибьюторы (включая агентов в этом самом проекте 🙂) будут писать по старым правилам. Swagger out-of-sync — частая регрессия при добавлении полей в DTO.

#### 6.10. (Nice-to-have) Pause/Resume для v2
**Проблема.** Сейчас pause/resume не имеют v2-семантики. Cancel перешёл в v2, pause/resume — нет.

**Что сделать (если решим оставить эти операции).**
- Добавить `state='paused'` в `tasks.state` CHECK constraint (миграция 042).
- В `task_service.allowedTransitions`: `active → paused`, `paused → active|cancelled`.
- Pause: `tasks.cancel_requested` НЕ выставляем; вместо этого ставим `paused`. Worker'ы при pickup проверяют `state == 'active'` перед началом Step.
- В UI кнопки Pause/Resume переподключить на новые сентинели.

**Why nice-to-have, не must.** Cancel + custom_timeout уже покрывают main use case ("я передумал" / "слишком долго работает"). Pause полезен только при дорогих LLM-вызовах ("дай я гляну до того как продолжать") — добавим если будет реальный запрос.

---

## 8. Открытые вопросы (после MVP)

- **Replay/debugging**: фиксировать seed'ы LLM для воспроизведения.
- **Версионирование промптов**: snapshot `system_prompt` агента в `agent_jobs.payload` (in-flight задачи не ломаются при обновлении промптов).
- **Webhook на завершение задачи**: для Slack/GitHub.
- **Quota per project/user**: лимит токенов/денег.
- **Adaptive concurrency**: автоматически снижать `agent_workers_sandbox` при высокой нагрузке хоста.

---

## 9. Definition of Done

**Legenda:** ✅ — выполнено и протестировано в Sprint 1-4. ⏸️ — отложено в Sprint 5 с явной причиной (см. блок "Отложено в Sprint 5" в §7). ⬜ — не начато.

**Функциональные:**
- ✅ Миграции up на чистой БД — `goose up` 001..040 прогон через testcontainers, миграция 038 fix включена.
- ✅ Unit-тесты Router/Dispatcher (галлюцинации + параллельные decision'ы) — 24 теста в `router_service_test.go` + 16 component-сценариев в `orchestration_scenarios_test.go`.
- ✅ **Integration tests с реальной БД** (`backend/internal/service/orchestrator_pg_integration_test.go`, build tag `integration`):
  - ✅ Sequential happy path — `TestScenario_Sequential_PlanReviewCodeReviewTest`
  - ✅ Parallel 2 dev + merger — `TestScenario_Parallel_TwoDevsThenMerger`
  - ✅ Parallel 3 dev + 3 review + merger + tester — `TestScenario_Parallel_ThreeDevsThreeReviewsMergerTester`
  - ✅ Hallucination recovery — `TestScenario_HallucinationRecovery_UnknownAgent`
  - ✅ Hallucination → needs_human fallback — `TestScenario_HallucinationFallback_NeedsHuman`
  - ✅ DAG с `depends_on` — `TestPGIntegration_DAG_DependsOn` (postgres + scripted Router)
  - ✅ Cancel mid-flight — `TestPGIntegration_CancelMidFlight` (RequestCancel между Step'ами, проверка cancelled-state + no new agent_jobs)
  - ✅ Restart mid-task — `TestPGIntegration_RestartMidTask` (kill через закрытие пула, recovery через `ReleaseStuckLocks`)
- ✅ Race detector (`go test -tags integration -race ./internal/service/...`) чистый, 15.9с в Docker.
- ✅ Test: переполнение `max_steps_per_task` → `needs_human` — `TestPGIntegration_MaxStepsPerTask_NeedsHuman` (limit=2, 4 Step'а, последний — no-op).
- ✅ Test: рестарт бэкенда — `TestPGIntegration_RestartMidTask` (kill через закрытие пула, recovery через `ReleaseStuckLocks`). Прогоняется в CI (job `integration-tests`, `-tags integration -race`).
- ✅ Test: 2 одновременных sandbox в одном репо — `TestWorktreeManager_AllocateAndRelease_HappyPath` (real `git`).
- ✅ Test: merger контракт — `TestScenario_MergerOutputContract` (parser end-to-end через `ParseMergerOutput`).
- ✅ Старые файлы удалены — 10 файлов (~3000 строк) включая `orchestrator_pipeline.go`, `orchestrator_service.go`, `result_processor*.go`.
- ✅ `make swagger` чистый — перегенерирован 2026-05-15 после добавления `OrchestrationV2Handler` + `CustomTimeout` DTO.
- ✅ Frontend: Agents Management UI, Worktrees debug, DAG-view, Router timeline (см. Sprint 5F). Cancel button — уже работал. custom_timeout — поле в Dart-модели + backend DTO (UI-форма — Sprint 6.5).
- ✅ Retention `router_decisions` 30 дней — `RetentionService.RunOnceRouterDecisions` + goroutine `Run` в main.go.
- ✅ Retention worktrees 1 сутки после release — `RetentionService.RunOnceWorktrees` + защита prefix-check в CleanupExpired (OR-условие, не AND).
- ⏸️ `docs/rules/main.md` обновлён — Sprint 6.9.

**Безопасность (v4):**
- ✅ **Git injection (code)**: WorktreeManager использует `--` separator во всех `git worktree add/remove`. Branch_validator отвергает ведущий `-`/`.`, control-chars, path-traversal, reserved refs, reflog-syntax.
- ✅ Git injection static-grep lint rule в CI — `scripts/static-grep-git.sh` запускается в job `lint`; проверяет, что `exec.Command(Context)?("git", ...)` в production-коде содержит `--` separator (либо делегирует argv через `args...`).
- ✅ **Git injection test**: `TestValidateBaseBranch_RejectsFlagInjection` + `TestWorktreeManager_Allocate_RejectsUnsafeBaseBranch` (6 adversarial кейсов, включая `-h`, `--upload-pack=evil`, `../etc/passwd`).
- ✅ **No raw LLM в stdout/stderr (canary-тест)**: `TestDecide_DoesNotLeakRawToLogs`, `TestDecide_DoesNotLeakErrErrorToLogs`, `TestSaveArtifact_LeakCanaryNotLogged`, `TestScenario_SecurityCanary_EndToEnd` — все с уникальными canary-токенами, проверка через grep буфера логов.
- ✅ Cancel race test — `TestPGIntegration_CancelMidFlight` (`RequestCancel` → Step проверяет `cancel_requested`, Router не вызывается, agent_jobs не enqueue'ятся).
- ✅ Lock collision test — заменён на `SELECT FOR UPDATE NOWAIT` (Yugabyte-совместимый). Базовая семантика покрыта `TestPGIntegration_OrchestratorStep_DoneOutcome` и прогоняется с `-race` в CI. Высоконагруженный 1000-UUID stress-test признан избыточным: NOWAIT-семантика покрывается unit + integration уровнями.
- ✅ **Path traversal**: WorktreeManager не хранит `path` в БД, computes от типизированных UUID. `CleanupExpired` имеет defence-in-depth OR-condition: `!isInsideRoot || isRootItself` — отказ как при выходе за корень, так и при равенстве корню (защита от catastrophic rm-rf root).
- ✅ **Secrets leak test (TestResult.raw_output_truncated)**: `TestScrubTestResultRawOutput` — `ScrubSecrets` (regex-patterns на api_key/token/password/bearer/github PAT) применяется перед записью в `artifact.content`.

**Соответствие конвенциям проекта:**
- 🟡 CLAUDE.md / `docs/rules/backend.md` обновлены частично (backend §2.3 — 5 правил Sprint 17). `docs/rules/main.md` + `frontend.md` — Sprint 6.9.
- ✅ `internal/logging/redact.go` обёртка применена в `RouterService`, `AgentWorker`, `Orchestrator`, `WorktreeManager`.
- ✅ Lint-правило "no `slog.Default()` в orchestrator files" — `.golangci.yml` написан (Sprint 5E) и прогоняется в job `lint` через `golangci/golangci-lint-action@v6` (см. `.github/workflows/backend-ci.yml`).
- ✅ Frontend l10n helper: новые v2-виджеты используют `requireAppLocalizations(context, where: '<WidgetName>')` вместо `AppLocalizations.of(context)!` (см. Sprint 5F.21 commit 2026-05-15).
