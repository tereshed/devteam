# Orchestration v2 — LLM-driven, flow-as-data

Статус: **PLAN / pending approval (revision 3)**
Дата: 2026-05-14
История:
- v1 — MVP-набросок (детерминированные точки вызова Router)
- v2 — учтены замечания по ревью: async execution, secrets, hallucinations, context budget, locking
- v3 — параллелизм подзадач в MVP: worktree-изоляция, Merger-агент, DAG в декомпозиции, Router-decision возвращает массив агентов
- v4 — безопасность по второму ревью: `--` separator в git, запрет логирования сырых LLM-ответов в stdout, корректный LISTEN→SELECT порядок в Agent Worker, advisory lock через (int4,int4), типизированные UUID-пути для worktree
- v5 (текущая) — адаптация под ограничения YugabyteDB: polling + Redis Pub/Sub вместо `LISTEN/NOTIFY` (не поддерживается YSQL), `SELECT FOR UPDATE NOWAIT` на `tasks.id` вместо advisory locks (нестабильно в распределённой среде)

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

### Sprint 3 — Worktrees, Queue, Workers, Step
9. `worktree_manager.go` — allocate/release через `git worktree`, retention cron. Все git-команды с `--` separator перед user/LLM-аргументами. `ComputePath` от `Worktree.ID + TaskID` (из модели уже есть в Sprint 1).
10. Worker pool: `task_events` через `SKIP LOCKED` polling (~500ms интервал) + **Redis Pub/Sub** wakeup (`SubscribeTaskEvents`) для low-latency. Race-free pattern в Agent Worker'е: Subscribe → SELECT `cancel_requested` → start Exec.
11. `orchestrator_v2.go` — `Step()` использует `service.TryLockTaskForStep` (`SELECT FOR UPDATE NOWAIT` на `tasks.id`), cancel check, параллельный fan-out N `agent_jobs`.
12. HTTP-хендлеры: `POST /tasks` (создание + первый `step_req` event), `POST /tasks/:id/cancel` (UPDATE `cancel_requested=true` + `NotifyTaskCancel`).
13. Миграция 039 — `DROP COLUMN tasks.status` + `CHECK chk_tasks_status`. Одновременно удалить legacy: `orchestrator_pipeline.go`, `DetermineNextStatus`, статусная часть `handleExecutionResult`, `TaskStatus` enum + все его константы из `models/task.go`, все switch'и по `Task.Status` в handlers/services.
14. Добавить Go-field `Task.CustomTimeout *time.Duration` с кастомным `sql.Scanner`/`driver.Valuer` для INTERVAL ↔ time.Duration маппинга.
15. Cron: retention `router_decisions` (30 дней через `RouterDecisionRepository.DeleteOlderThan`) + retention worktrees (1 сутки после release через `WorktreeRepository.ListForCleanup` + `os.RemoveAll` с prefix-check).

### Sprint 4 — Merger, Tester, Integration
15. Merger-агент: промпт + sandbox-runner с поддержкой множественных worktree mounts.
16. Tester-агент: запуск test suite, парсинг результата в `test_result` артефакт.
17. Integration tests:
    - **Sequential**: задача → план → 1 подзадача → код → ревью → тест → done.
    - **Parallel**: задача → план → 3 независимых подзадачи → 3 параллельных Developer'а → 3 параллельных review → Merger → tester → done.
    - **DAG**: 4 подзадачи где 3 и 4 зависят от 1 — должна быть фаза параллели (1, 2) затем (3, 4).
    - **Cancel mid-flight**: пользователь отменяет когда работают 2 параллельных sandbox'а — оба останавливаются, worktrees освобождаются.
    - **Restart mid-task**: kill бэкенда → задача продолжается с того же шага.
    - **Hallucination**: Router возвращает невалидный JSON / несуществующего агента → retry → recovery.

### Sprint 5 — MCP, Swagger, Frontend, CI
18. `AgentRepository` интерфейс + impl (CRUD по `models.Agent`) — нужен для Frontend Agents Management.
19. MCP-инструменты: `list_agents`, `create_agent`, `update_agent`, `set_agent_secret`, `list_artifacts`, `get_router_decisions`, `list_worktrees`, `cancel_task`.
20. Swagger обновить (`make swagger`).
21. Flutter: Agents Management, Task Detail v2 (DAG view, router timeline), Worktrees debug screen, Cancel button, custom_timeout поле.
22. **CI lint-правило** в `golangci.yml`: forbid `slog.Default()` в `internal/service/router_*`, `internal/service/orchestrator_*`, `internal/service/agent_dispatcher.go` — все должны получать redact-обёрнутый логгер через DI.
23. Обновить `docs/rules/main.md` и `docs/rules/backend.md` (правило `--` separator в git, no-raw-log policy).

---

## 8. Открытые вопросы (после MVP)

- **Replay/debugging**: фиксировать seed'ы LLM для воспроизведения.
- **Версионирование промптов**: snapshot `system_prompt` агента в `agent_jobs.payload` (in-flight задачи не ломаются при обновлении промптов).
- **Webhook на завершение задачи**: для Slack/GitHub.
- **Quota per project/user**: лимит токенов/денег.
- **Adaptive concurrency**: автоматически снижать `agent_workers_sandbox` при высокой нагрузке хоста.

---

## 9. Definition of Done

**Функциональные:**
- [ ] Миграции up + down на чистой БД
- [ ] Unit-тесты Router/Dispatcher (включая галлюцинации и параллельные decision'ы)
- [ ] Integration tests: sequential, parallel, DAG, cancel, restart, hallucination — все зелёные
- [ ] Race detector (`go test -race`) чистый
- [ ] Test: переполнение `max_steps_per_task` → `needs_human`, sandbox'ы остановлены, worktrees освобождены
- [ ] Test: рестарт бэкенда в середине задачи → продолжение после старта воркеров
- [ ] Test: 2 одновременных sandbox-агента в одном репо не мешают друг другу (worktree-изоляция)
- [ ] Test: merger корректно объединяет 2 параллельных code_diff
- [ ] Старые файлы удалены
- [ ] `make swagger` чистый, `make test-all` зелёный
- [ ] Frontend: создание агента через UI → Router подхватывает без рестарта
- [ ] Frontend: cancel button работает end-to-end
- [ ] Frontend: DAG-view отрисовывает зависимости подзадач
- [ ] Retention `router_decisions` 30 дней работает (unit-тест cron)
- [ ] Retention worktrees 1 сутки после release работает
- [ ] `docs/rules/main.md` обновлён

**Безопасность (v4):**
- [ ] **Git injection**: статический grep по проекту — каждый `exec.Command*("git", ...)` с не-фиксированными аргументами имеет `--` перед ними. Lint-правило в CI.
- [ ] **Git injection test**: попытка передать `base_branch="-h"` или `"--upload-pack=evil"` → отвергнуто валидатором ДО `exec.Command`.
- [ ] **No raw LLM в stdout/stderr (canary-тест)**: прогон полного цикла задачи с canary-секретом `ANTHROPIC_API_KEY=canary-leak-token-XYZ` и фразой `LEAK_CANARY_PAYLOAD` в промпте — `grep` всех логов на обе подстроки даёт 0 матчей.
- [ ] **Cancel race test**: симуляция — `UPDATE cancel_requested=true` + `NOTIFY` отправлены ДО того, как Agent Worker дошёл до `Subscribe`. Тест проверяет, что Worker всё равно отменяет (через SELECT после LISTEN).
- [ ] **Advisory lock test**: 1000 задач с искусственно сконструированными UUID-ами, у которых старшие 8 байт намеренно конфликтуют — задачи с разными ID не должны блокировать друг друга при использовании `(int4,int4)` формы лока.
- [ ] **Path traversal test**: ручная подмена строки в БД (тест с прямым SQL UPDATE) `worktrees.branch_name='../../etc/passwd'` → `Remove()` отвергает с ошибкой "computed path escapes root" (хотя путь больше не из БД, defence-in-depth тест нужен).
- [ ] **Secrets leak test**: canary-секрет `ANTHROPIC_API_KEY=test-leak-canary-2026` прогоняется через sandbox; `grep` логов, артефактов, router_decisions (после расшифровки) — не находит plaintext.

**Соответствие конвенциям проекта:**
- [ ] CLAUDE.md / `docs/rules/backend.md` обновлены под новые правила (`--` separator везде, no-raw-log policy)
- [ ] `internal/logging/redact.go` обёртка применена во всех слоях, lint-правило запрещает использовать `slog.Default()` напрямую в файлах оркестрации
