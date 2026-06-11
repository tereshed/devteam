package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var jsonArtifactsRegex = regexp.MustCompile("(?s)```json\n?(.*?)\n?```")

// jitterDuration возвращает d с случайным разбросом ±25%. Расфазирует пул
// воркеров: без джиттера N goroutine'ов, стартовавших в одном цикле, опрашивают
// очередь синхронно и бьют Yugabyte залпами. Общий хелпер для step/agent воркеров.
func jitterDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	delta := int64(d) / 4 // ±25%
	return d - time.Duration(delta) + time.Duration(rand.Int63n(2*delta+1))
}

func retryOnConflict(ctx context.Context, logger *slog.Logger, fn func() error) error {
	var lastErr error
	backoff := 100 * time.Millisecond
	// Бюджет ретраев расширен (было 25): при широком fan-out (напр. decomposer → 7 reviewer'ов)
	// конкурентные записи артефактов/SupersedePrevious дают штормы 40001, длящиеся минутами.
	// Больше попыток + чуть выше потолок backoff = переживаем шторм, не роняя job в orphan/fail.
	for attempt := 0; attempt < 40; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40001" && pgErr.Code != "40P01") {
			return err
		}

		if logger != nil {
			logger.WarnContext(ctx, "database conflict, retrying operation", "attempt", attempt, "code", pgErr.Code, "error", err.Error())
		}

		// jitter ±25%
		jitterRange := backoff / 2
		sleep := backoff - jitterRange/2 + time.Duration(rand.Int63n(int64(jitterRange)+1))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}
		backoff *= 2
		if backoff > 3*time.Second {
			backoff = 3 * time.Second
		}
	}
	return lastErr
}

// agent_worker.go — Sprint 17 / Orchestration v2 — пул воркеров типа agent_job.
//
// Воркер забирает task_event(kind=agent_job) → запускает агента → сохраняет артефакт
// → enqueue'ит step_req → помечает event Complete.
//
// Race-free отмена (план §2.8):
//   1. ОБЯЗАТЕЛЬНО: Subscribe Redis(task_cancel:<id>) ДО SELECT cancel_requested.
//   2. SELECT cancel_requested — если уже true, abort.
//   3. Start agent с ctx, привязанным к Redis-каналу.
// Без этого порядка можно пропустить NOTIFY, отправленный между UPDATE и Subscribe.

// AgentResponseEnvelope — стандартный контракт ответа агента-исполнителя.
//
// Каждый агент в своём system_prompt инструктируется выдавать JSON этого формата
// (см. seed migration 038). Воркер парсит Output → AgentResponseEnvelope →
// models.Artifact.
//
// Если агент вернул не-envelope JSON или вообще не-JSON, воркер делает
// fallback-артефакт kind='raw_output' c summary=truncated output (Sprint 4
// научит агентов давать корректный envelope; пока это резервный путь).
type AgentResponseEnvelope struct {
	Kind             string          `json:"kind"`
	Summary          string          `json:"summary"`
	ParentArtifactID *uuid.UUID      `json:"parent_artifact_id,omitempty"`
	Content          json.RawMessage `json:"content,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to tolerate invalid or short UUID formats for parent_artifact_id.
func (e *AgentResponseEnvelope) UnmarshalJSON(data []byte) error {
	type Alias AgentResponseEnvelope
	aux := &struct {
		ParentArtifactID *string `json:"parent_artifact_id,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ParentArtifactID != nil && *aux.ParentArtifactID != "" {
		if pID, err := uuid.Parse(*aux.ParentArtifactID); err == nil {
			e.ParentArtifactID = &pID
		} else {
			// Tolerated short ID or invalid UUID format. Leave as nil (can be resolved from context targetArtifact).
			slog.Warn("envelope unmarshal: invalid uuid for parent_artifact_id, leaving nil", "value", *aux.ParentArtifactID)
		}
	}
	return nil
}

// AgentWorkerConfig — настройки одного воркера.
type AgentWorkerConfig struct {
	WorkerID string
	// PollInterval — минимальная пауза между опросами (нижняя граница backoff'а).
	PollInterval time.Duration
	// MaxPollInterval — верхняя граница адаптивного backoff'а на пустой очереди.
	// См. StepWorkerConfig.MaxPollInterval.
	MaxPollInterval time.Duration
	AgentJobTimeout time.Duration // hard cap на один agent_job (default 1h)
}

// DefaultAgentWorkerConfig — дефолты для llm-агентов. Для sandbox-агентов
// разумно выставить AgentJobTimeout повыше (1-2h).
func DefaultAgentWorkerConfig() AgentWorkerConfig {
	return AgentWorkerConfig{
		WorkerID:        "agent-worker-default",
		PollInterval:    500 * time.Millisecond,
		MaxPollInterval: 5 * time.Second,
		AgentJobTimeout: time.Hour,
	}
}

// AgentWorker — один воркер пула agent_job.
type AgentWorker struct {
	db             *gorm.DB
	eventRepo      repository.TaskEventRepository
	artifactRepo   repository.ArtifactRepository
	dispatcher     AgentDispatcher
	worktreeMgr    *WorktreeManager
	notifier       *RedisNotifier  // может быть nil
	bus            events.EventBus // может быть nil — live-апдейты UI через HubBridge
	logger         *slog.Logger
	cfg            AgentWorkerConfig
	contextBuilder ContextBuilder
}

// NewAgentWorker — конструктор.
func NewAgentWorker(
	db *gorm.DB,
	eventRepo repository.TaskEventRepository,
	artifactRepo repository.ArtifactRepository,
	dispatcher AgentDispatcher,
	worktreeMgr *WorktreeManager,
	notifier *RedisNotifier,
	bus events.EventBus,
	logger *slog.Logger,
	cfg AgentWorkerConfig,
	contextBuilder ContextBuilder,
) *AgentWorker {
	if logger == nil {
		logger = logging.NopLogger()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.MaxPollInterval <= 0 {
		cfg.MaxPollInterval = 5 * time.Second
	}
	if cfg.MaxPollInterval < cfg.PollInterval {
		cfg.MaxPollInterval = cfg.PollInterval
	}
	if cfg.AgentJobTimeout <= 0 {
		cfg.AgentJobTimeout = time.Hour
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "agent-worker-default"
	}
	return &AgentWorker{
		db: db, eventRepo: eventRepo, artifactRepo: artifactRepo,
		dispatcher: dispatcher, worktreeMgr: worktreeMgr,
		notifier: notifier, bus: bus, logger: logger, cfg: cfg,
		contextBuilder: contextBuilder,
	}
}

// Run блокирует до ctx.Done(). Структурно идентичен StepWorker.Run.
func (w *AgentWorker) Run(ctx context.Context) error {
	var wakeupCh <-chan struct{}
	if w.notifier != nil {
		pubsub := w.notifier.SubscribeTaskEvents(ctx)
		defer pubsub.Close()
		ch := make(chan struct{}, 64)
		go func() {
			defer close(ch)
			for msg := range pubsub.Channel() {
				if msg.Payload != string(models.TaskEventKindAgentJob) {
					continue
				}
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}()
		wakeupCh = ch
	}

	w.logger.InfoContext(ctx, "agent worker started",
		"worker_id", w.cfg.WorkerID, "poll_interval", w.cfg.PollInterval,
		"max_poll_interval", w.cfg.MaxPollInterval, "job_timeout", w.cfg.AgentJobTimeout)

	// Адаптивный backoff — см. StepWorker.Run.
	delay := w.cfg.PollInterval

	for {
		worked := w.drainQueue(ctx)
		if worked {
			delay = w.cfg.PollInterval
		} else {
			delay *= 2
			if delay > w.cfg.MaxPollInterval {
				delay = w.cfg.MaxPollInterval
			}
		}

		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "agent worker stopping", "worker_id", w.cfg.WorkerID)
			return nil
		case <-time.After(jitterDuration(delay)):
		case <-wakeupCh:
			delay = w.cfg.PollInterval
		}
	}
}

func (w *AgentWorker) drainQueue(ctx context.Context) bool {
	worked := false
	for {
		if ctx.Err() != nil {
			return worked
		}
		ev, err := w.eventRepo.ClaimNext(ctx, models.TaskEventKindAgentJob, w.cfg.WorkerID)
		if err != nil {
			if errors.Is(err, repository.ErrNoTaskEventAvailable) {
				return worked
			}
			w.logger.ErrorContext(ctx, "claim next agent_job failed",
				"worker_id", w.cfg.WorkerID, "error", err.Error())
			return worked
		}
		w.processOne(ctx, ev)
		worked = true
	}
}

// processOne обрабатывает один agent_job event с race-free cancel поддержкой.
func (w *AgentWorker) processOne(parentCtx context.Context, ev *models.TaskEvent) {
	// Распаковка payload.
	var payload models.AgentJobPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("unmarshal payload: %w", err))
		return
	}

	w.logger.InfoContext(parentCtx, "agent_job claimed",
		"worker_id", w.cfg.WorkerID,
		"task_event_id", ev.ID,
		"task_id", ev.TaskID,
		"agent", payload.AgentName,
	)

	// ──────────────────────────────────────────────────────────────────────
	// Race-free отмена (план §2.8):
	// 1. Subscribe Redis cancel ДО SELECT (если notifier есть).
	// 2. SELECT cancel_requested — ловит NOTIFY, ушедший до Subscribe.
	// 3. Если уже cancelled — abort до Execute (не тратим LLM/sandbox).
	// ──────────────────────────────────────────────────────────────────────
	var cancelCh <-chan struct{}
	if w.notifier != nil {
		pubsub := w.notifier.SubscribeTaskCancel(parentCtx, ev.TaskID)
		defer pubsub.Close()
		ch := make(chan struct{}, 1)
		go func() {
			for msg := range pubsub.Channel() {
				_ = msg
				select {
				case ch <- struct{}{}:
				default:
				}
			}
			close(ch)
		}()
		cancelCh = ch
	}

	if cancelled, err := w.checkCancelRequested(parentCtx, ev.TaskID); err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("check cancel_requested: %w", err))
		return
	} else if cancelled {
		w.handleCancellation(parentCtx, ev, &payload, "cancel_requested=true before exec start")
		return
	}

	// Sprint 17 / 6.10: Pause/Resume v2. Если задача не в active (paused, needs_human,
	// done/failed/cancelled), не запускаем агента: помечаем event как Complete и пинаем
	// Orchestrator.Step. Step увидит state и либо выйдет (paused/needs_human),
	// либо подтвердит финализацию. Resume позже создаст новый step_req и Router
	// пересчитает следующее действие на актуальном артефакт-state.
	if state, err := w.checkTaskState(parentCtx, ev.TaskID); err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("check task state: %w", err))
		return
	} else if state != models.TaskStateActive {
		w.handleNonActiveSkip(parentCtx, ev, state)
		return
	}

	// Ctx с таймаутом + cancel-hook через Redis-канал.
	execCtx, cancel := context.WithTimeout(parentCtx, w.cfg.AgentJobTimeout)
	defer cancel()
	if cancelCh != nil {
		go func() {
			select {
			case <-cancelCh:
				cancel()
			case <-execCtx.Done():
			}
		}()
	}

	// Загружаем task для description/title (executor их использует).
	var task models.Task
	if err := w.db.WithContext(execCtx).
		Preload("Project").
		Preload("Project.GitCredential").
		Preload("Project.Repositories", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		Preload("Project.Repositories.GitCredential").
		Where("id = ?", ev.TaskID).First(&task).Error; err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("load task %s: %w", ev.TaskID, err))
		return
	}

	var teamID uuid.UUID
	var err error
	if task.TeamID != nil {
		teamID = *task.TeamID
	} else {
		teamID, err = getProjectTeamID(w.db, task.ProjectID)
		if err != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("find project team: %w", err))
			return
		}
	}

	// Загружаем агента (актуальный snapshot — system_prompt/model могли обновить).
	// Пытаемся загрузить командного агента, с фоллбеком на глобального.
	var agentRec models.Agent
	err = w.db.WithContext(execCtx).Preload("Prompt").Where("team_id = ? AND name = ?", teamID, payload.AgentName).First(&agentRec).Error
	if err != nil {
		// Fallback to global agent
		if errGlobal := w.db.WithContext(execCtx).Preload("Prompt").Where("team_id IS NULL AND name = ?", payload.AgentName).First(&agentRec).Error; errGlobal != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("load agent %q: %w", payload.AgentName, err))
			return
		}
	}

	// Sandbox-агенту нужен worktree. Allocate ЗДЕСЬ (just-in-time), а не в Orchestrator.Step,
	// чтобы git-команды не оказывались внутри Step-транзакции (см. orchestrator_v2.go §Step).
	//
	// Контракт payload (формируется в orchestrator_v2.enqueueOneAgentJob для sandbox-агента):
	//   payload.Input["_base_branch"] — строка с базовой веткой проекта.
	//
	// Если WorktreeID уже задан (повторный pickup после рестарта) — переводим в in_use,
	// не аллоцируем заново.
	var wtRec *models.Worktree
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox && w.worktreeMgr != nil {
		if payload.WorktreeID == nil {
			wt, err := w.allocateWorktreeForJob(execCtx, ev, &payload)
			if err != nil {
				w.failEvent(parentCtx, ev, fmt.Errorf("allocate worktree: %w", err))
				return
			}
			payload.WorktreeID = &wt.ID
			wtRec = wt
		} else {
			var wt models.Worktree
			if err := w.db.WithContext(execCtx).Where("id = ?", *payload.WorktreeID).First(&wt).Error; err != nil {
				w.failEvent(parentCtx, ev, fmt.Errorf("load worktree %s: %w", *payload.WorktreeID, err))
				return
			}
			wtRec = &wt
		}
		if err := w.worktreeMgr.MarkInUse(execCtx, *payload.WorktreeID, ev.ID); err != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("mark worktree in_use: %w", err))
			return
		}
	}

	executor, err := w.dispatcher.BuildExecutor(execCtx, &agentRec)
	if err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("build executor: %w", err))
		return
	}

	var targetArtifact *models.Artifact
	var targetArtifacts []*models.Artifact
	if payload.Input != nil {
		var rawIDs []string

		if rawSingular, ok := payload.Input["target_artifact_id"]; ok {
			if s, ok := rawSingular.(string); ok && s != "" {
				rawIDs = append(rawIDs, s)
			}
		}

		if rawPlural, ok := payload.Input["target_artifact_ids"]; ok {
			if arr, ok := rawPlural.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok && s != "" {
						rawIDs = append(rawIDs, s)
					}
				}
			}
		}

		for _, idStr := range rawIDs {
			var art *models.Artifact
			if id, err := uuid.Parse(idStr); err == nil {
				if a, err := w.artifactRepo.GetByID(execCtx, id); err != nil {
					w.logger.WarnContext(execCtx, "failed to load target artifact", "artifact_id", id, "error", err)
				} else {
					art = a
				}
			} else {
				if a := w.resolveArtifactByPrefix(execCtx, ev.TaskID, idStr); a != nil {
					w.logger.WarnContext(execCtx, "resolved truncated target_artifact_id by prefix",
						"raw", idStr, "resolved_id", a.ID, "task_id", ev.TaskID)
					art = a
				}
			}

			if art != nil {
				targetArtifacts = append(targetArtifacts, art)
			}
		}

		if len(targetArtifacts) > 0 {
			targetArtifact = targetArtifacts[0]
		}
	}

	// Если это агент-декомпозитор и на вход пришёл уже одобренный артефакт декомпозиции,
	// то вместо вызова LLM/песочницы мы можем автоматически распарсить его subtasks и создать
	// индивидуальные артефакты subtask_description. Это предотвращает бесконечные циклы
	// и экономит ресурсы.
	if agentRec.Name == "decomposer" && targetArtifact != nil && targetArtifact.Kind == models.ArtifactKindDecomposition {
		// Повторный dispatch decomposer'а на уже готовую декомпозицию: не гоняем LLM/sandbox,
		// а разбиваем её на subtask_description напрямую (идемпотентно). Только если в
		// декомпозиции реально есть subtasks — иначе проваливаемся в обычное исполнение.
		if len(extractSubtasks(targetArtifact.Content)) > 0 {
			w.logger.InfoContext(execCtx, "bypassing decomposer execution: splitting decomposition artifact directly", "artifact_id", targetArtifact.ID)
			if _, err := w.splitDecomposition(parentCtx, task.Project, ev.TaskID, targetArtifact.ID, targetArtifact.Content); err != nil {
				w.failEvent(parentCtx, ev, fmt.Errorf("split decomposition: %w", err))
				return
			}

			// Releasing worktree if allocated (decomposer might have had one)
			if payload.WorktreeID != nil && w.worktreeMgr != nil {
				if err := w.worktreeMgr.Release(parentCtx, *payload.WorktreeID); err != nil {
					w.logger.WarnContext(parentCtx, "release worktree after split success failed",
						"worktree_id", *payload.WorktreeID, "error", err.Error())
				}
			}

			// Mark event complete
			if err := retryOnConflict(parentCtx, w.logger, func() error {
				return w.eventRepo.Complete(parentCtx, ev.ID)
			}); err != nil {
				w.logger.ErrorContext(parentCtx, "mark event complete failed",
					"task_event_id", ev.ID, "error", err.Error())
			}

			// Live-апдейт UI: созданы subtask_description артефакты.
			w.publishArtifactCreated(parentCtx, task.ProjectID, ev.TaskID, "decomposer")

			w.enqueueFollowupStep(parentCtx, ev.TaskID)
			return
		}
	}

	// Fix 2 (multi-repo guard, defence-in-depth): developer в multi-repo проекте не должен
	// запускаться против primary-репо вслепую. Покрывает ОБА пути диспатча:
	//   - подзадача декомпозиции (есть target-артефакт): _repo_slug обязан прийти от
	//     decomposer/orchestrator, иначе выводим из текста артефакта;
	//   - прямой запуск router-правилом (7) без target-артефакта (инцидент 82150066:
	//     «MCP Yandex Tracker» с инструкцией «в репозитории mcp-servers» молча ушла в
	//     primary bot-service): выводим из instructions/title/description задачи.
	// Инференс матчит slug, display_name и basename git_url (slug primary — «main», но в
	// тексте задачи пишут «bot-service»). Не вышло однозначно — фейлим job (retry → router
	// → needs_human): тихий primary-дефолт превращает ошибку маршрутизации в PR не в том репо.
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox &&
		agentRec.Role == models.AgentRoleDeveloper && task.Project != nil {
		if slugs := projectRepoSlugs(task.Project); len(slugs) >= 2 {
			slug := stringFromAny(payload.Input["_repo_slug"])
			if slug == "" || task.Project.RepoBySlug(slug) == nil {
				inferText := repoInferTextFromArtifacts(targetArtifacts) + "\n" +
					stringFromAny(payload.Input["instructions"]) + "\n" +
					task.Title + "\n" + task.Description
				if inf := inferRepoSlugFromProject(inferText, task.Project); inf != "" {
					if payload.Input == nil {
						payload.Input = map[string]any{}
					}
					payload.Input["_repo_slug"] = inf
					w.logger.WarnContext(execCtx, "developer repo_slug missing; inferred from job context",
						"task_id", ev.TaskID, "inferred_repo_slug", inf)
					slug = inf
				}
			}
			if slug == "" || task.Project.RepoBySlug(slug) == nil {
				w.failEvent(parentCtx, ev, fmt.Errorf(
					"multi-repo project but developer job has no resolvable repo_slug (repos: %v); refusing to run against primary repo to avoid wrong-repo edits", slugs))
				return
			}
		}
	}

	// Reviewer/Tester/Merger в multi-repo: рабочее репо обязано совпадать с репо
	// проверяемого артефакта — иначе sandbox клонирует primary, ревью/тесты идут не
	// в том дереве, а entrypoint пушит пустую мусорную ветку в primary (инцидент
	// e7f807ba: reviewer работал в bot-service при коде в mcp-servers). Слаг
	// наследуется от target-артефакта (code_diff штампуется при сохранении), затем
	// инференс по текстам; не определилось — primary как прежде (роли читающие,
	// fail-loud тут избыточен: ревью по diff-артефакту валидно и из primary).
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox &&
		(agentRec.Role == models.AgentRoleReviewer || agentRec.Role == models.AgentRoleTester || agentRec.Role == models.AgentRoleMerger) &&
		task.Project != nil && len(projectRepoSlugs(task.Project)) >= 2 &&
		stringFromAny(payload.Input["_repo_slug"]) == "" {
		slug := ""
		for _, ta := range targetArtifacts {
			if ta == nil {
				continue
			}
			if s := extractRepoSlug(ta.Content); s != "" && task.Project.RepoBySlug(s) != nil {
				slug = s
				break
			}
		}
		if slug == "" {
			inferText := repoInferTextFromArtifacts(targetArtifacts) + "\n" + task.Title + "\n" + task.Description
			if s := inferRepoSlugFromProject(inferText, task.Project); s != "" && task.Project.RepoBySlug(s) != nil {
				slug = s
			}
		}
		if slug != "" {
			if payload.Input == nil {
				payload.Input = map[string]any{}
			}
			payload.Input["_repo_slug"] = slug
			w.logger.InfoContext(execCtx, "repo_slug inherited from target artifact/context",
				"task_id", ev.TaskID, "role", string(agentRec.Role), "repo_slug", slug)
		}
	}

	var in agent.ExecutionInput
	if w.contextBuilder != nil {
		// Мульти-репо: целевой репозиторий подзадачи (по _repo_slug из payload, иначе primary).
		targetRepo := resolveTargetRepo(task.Project, payload.Input)
		builtIn, err := w.contextBuilder.Build(execCtx, &task, &agentRec, task.Project, targetRepo)
		if err != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("context builder: %w", err))
			return
		}
		in = *builtIn
		// Override ContextJSON and StructuredContext with the step-specific payload.Input
		inputJSON, _ := json.Marshal(payload.Input)
		in.ContextJSON = inputJSON
		in.StructuredContext = inputJSON

		// If there are target artifacts, append them in the target_artifact XML format.
		for _, art := range targetArtifacts {
			if !strings.Contains(in.PromptUser, fmt.Sprintf("<target_artifact id=%q", art.ID.String())) {
				prettyContent := ""
				if len(art.Content) > 0 {
					var prettyJSON bytes.Buffer
					if err := json.Indent(&prettyJSON, art.Content, "", "  "); err == nil {
						prettyContent = prettyJSON.String()
					} else {
						prettyContent = string(art.Content)
					}
				}
				in.PromptUser = in.PromptUser + fmt.Sprintf("\n\n<target_artifact id=%q producer=%q kind=%q summary=%q>\n%s\n</target_artifact>\n",
					art.ID.String(), art.ProducerAgent, string(art.Kind), art.Summary, prettyContent)
			}
		}
	} else {
		in = w.buildExecutionInput(&task, &agentRec, payload.Input, targetArtifact)
	}

	// Ключи владельца проекта (user_llm_credentials) приоритетнее env при
	// LLM-вызовах (planner/decomposer/...); пусто → fallback на env-ключ.
	if task.Project != nil {
		in.OwnerUserID = task.Project.UserID.String()
	}

	// Finalize branch name for sandbox agent execution
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox {
		switch agentRec.Role {
		case models.AgentRoleDeveloper:
			// Developer always works on the unique isolated branch of its worktree
			if wtRec != nil {
				in.BranchName = wtRec.BranchName
			}
		case models.AgentRoleReviewer, models.AgentRoleTester:
			// Reviewer/Tester must work on the branch of the target artifact under test/review
			if resolvedBranch, err := w.resolveBranchNameForArtifact(execCtx, targetArtifact); err == nil && resolvedBranch != "" {
				in.BranchName = resolvedBranch
			} else if task.BranchName != nil && *task.BranchName != "" {
				in.BranchName = *task.BranchName
			}
		case models.AgentRoleMerger:
			// Merger merges changes into the task-level branch
			if task.BranchName != nil && *task.BranchName != "" {
				in.BranchName = *task.BranchName
			}
		default:
			// Fallback: use worktree's branch if available, else task branch
			if wtRec != nil {
				in.BranchName = wtRec.BranchName
			} else if task.BranchName != nil && *task.BranchName != "" {
				in.BranchName = *task.BranchName
			}
		}
	}

	// Double check fallback
	if in.BranchName == "" {
		if in.GitDefaultBranch != "" {
			in.BranchName = in.GitDefaultBranch
		} else {
			in.BranchName = "main"
		}
	}
	in.ExecutionID = fmt.Sprintf("%d", ev.ID)
	result, execErr := executor.Execute(execCtx, in)
	if execErr != nil {
		// Отмена через ctx.Done() — это не infrastructure failure.
		if errors.Is(execErr, context.Canceled) {
			w.handleCancellation(parentCtx, ev, &payload, "agent ctx cancelled during exec")
			return
		}
		w.failEvent(parentCtx, ev, fmt.Errorf("agent execute: %w", execErr))
		return
	}
	if result == nil {
		w.failEvent(parentCtx, ev, errors.New("agent returned nil result"))
		return
	}

	// Sandbox OOM/timeout/crash или агент, вернувший пустой вывод — это инфраструктурный
	// сбой, а НЕ валидный результат. Раньше такой исход молча сохранялся как ready-артефакт
	// (например code_diff с content {"raw_output":""}), и Router принимал его за «работа
	// сделана», после чего бесконечно переназначал того же агента (разбор задачи 1.1:
	// ~10 из 22 code_diff были пустыми «failed», что и раздуло прогон до 37 шагов).
	// Помечаем event как failed: сработает retry/backoff, а при исчерпании попыток job
	// «умирает» и Router увидит его в разделе Failed jobs (см. enqueueFollowupStep ниже).
	if agentResultUnusable(result) {
		// НЕ утверждаем «OOM» — причиной может быть OOM, timeout, краш CLI или ошибка
		// entrypoint'а (exit!=0 без вывода). Точную причину смотреть в логах контейнера
		// (SANDBOX_KEEP_ON_FAILURE=1 сохраняет упавший sandbox: docker logs <id>).
		w.failEvent(parentCtx, ev, fmt.Errorf(
			"agent produced no usable result (success=%v, output_len=%d, artifacts_len=%d): sandbox exited without usable output (check sandbox logs)",
			result.Success, len(result.Output), len(result.ArtifactsJSON)))
		return
	}

	// Сохраняем артефакт. Слаг (после multi-repo guard — всегда разрешённый)
	// штампуется в code-артефакты, чтобы PR-гейт открыл MR в правильном репо.
	if err := w.saveArtifact(parentCtx, ev.TaskID, &agentRec, result, wtRec, stringFromAny(payload.Input["_repo_slug"]), targetArtifact); err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("save artifact: %w", err))
		return
	}

	// Live-апдейт UI: артефакт(ы) сохранены — пушим событие, чтобы фронт обновил
	// список артефактов и execution-граф без ручного рефреша.
	w.publishArtifactCreated(parentCtx, task.ProjectID, ev.TaskID, agentRec.Name)

	// Детерминированный split: если decomposer в первом прогоне выдал decomposition с
	// subtasks — сразу раскладываем её на subtask_description, не дожидаясь повторного
	// dispatch'а Router'ом. Иначе «умный» Router может уйти к developer'ам мимо подзадач,
	// и code_diff'ы окажутся orphaned (без parent), а поток — сплющенным.
	if agentRec.Name == "decomposer" {
		if err := w.splitLatestDecomposition(parentCtx, task.Project, ev.TaskID); err != nil {
			// Невалидная декомпозиция (мульти-репо без resolvable repo_slug): фейлим event,
			// чтобы decomposer перегенерил с правильным repo_slug, а не разложил на primary.
			w.failEvent(parentCtx, ev, fmt.Errorf("decomposition rejected: %w", err))
			return
		}
	}

	// Releasing worktree после успешного завершения (содержимое уже закоммичено агентом).
	if payload.WorktreeID != nil && w.worktreeMgr != nil {
		if err := w.worktreeMgr.Release(parentCtx, *payload.WorktreeID); err != nil {
			w.logger.WarnContext(parentCtx, "release worktree after success failed",
				"worktree_id", *payload.WorktreeID, "error", err.Error())
		}
	}

	// Mark event complete.
	if err := retryOnConflict(parentCtx, w.logger, func() error {
		return w.eventRepo.Complete(parentCtx, ev.ID)
	}); err != nil {
		// Артефакт уже сохранён, но событие не удалось пометить done (исчерпан бюджет 40001).
		// НЕ оставляем его залоченным: иначе осиротеет навсегда (ClaimNext берёт только
		// не-locked; до lease-реклейма провисит ~90 мин). Делаем Fail → воркер переотработает
		// (новый артефакт supersede'ит старый через SupersedePrevious), либо при исчерпании
		// max_attempts job станет dead и Router увидит его в Failed. НЕ enqueue'им followup.
		w.logger.ErrorContext(parentCtx, "mark event complete failed → failing event to avoid orphan lock",
			"task_event_id", ev.ID, "error", err.Error())
		w.failEvent(parentCtx, ev, fmt.Errorf("mark complete: %w", err))
		return
	}

	// Пнём Orchestrator.Step — Router решит что дальше.
	w.enqueueFollowupStep(parentCtx, ev.TaskID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Cancel-flow и failure-обработка
// ─────────────────────────────────────────────────────────────────────────────

// handleCancellation — корректно прибирается при отмене: помечает event как Complete
// (не retry'им — отмена не баг), освобождает worktree, enqueue'ит step_req для
// финализации задачи в Orchestrator.Step.
func (w *AgentWorker) handleCancellation(ctx context.Context, ev *models.TaskEvent, payload *models.AgentJobPayload, reason string) {
	w.logger.InfoContext(ctx, "agent_job cancelled",
		"worker_id", w.cfg.WorkerID, "task_event_id", ev.ID,
		"task_id", ev.TaskID, "reason", reason)

	if payload.WorktreeID != nil && w.worktreeMgr != nil {
		if err := w.worktreeMgr.Release(ctx, *payload.WorktreeID); err != nil {
			w.logger.WarnContext(ctx, "release worktree on cancel failed",
				"worktree_id", *payload.WorktreeID, "error", err.Error())
		}
	}
	if err := retryOnConflict(ctx, w.logger, func() error {
		return w.eventRepo.Complete(ctx, ev.ID)
	}); err != nil {
		w.logger.ErrorContext(ctx, "mark cancelled event complete failed",
			"task_event_id", ev.ID, "error", err.Error())
	}
	w.enqueueFollowupStep(ctx, ev.TaskID)
}

// failEvent — fail+backoff. При окончательной смерти (attempts >= max) — событие
// остаётся в БД, idx_task_events_pollable его уже не возвращает; следующий step_req
// даст Router'у шанс обработать ситуацию.
func (w *AgentWorker) failEvent(ctx context.Context, ev *models.TaskEvent, err error) {
	w.logger.ErrorContext(ctx, "agent_job failed",
		"worker_id", w.cfg.WorkerID, "task_event_id", ev.ID,
		"task_id", ev.TaskID, "attempt", ev.Attempts+1, "error", err.Error())

	backoff := time.Duration(1<<ev.Attempts) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	ferr := retryOnConflict(ctx, w.logger, func() error {
		return w.eventRepo.Fail(ctx, ev.ID, truncate(err.Error(), 512), backoff)
	})
	if ferr != nil {
		w.logger.ErrorContext(ctx, "mark agent_job as failed",
			"task_event_id", ev.ID, "error", ferr.Error())
	}

	// Если это была последняя попытка — job окончательно «умер» (attempts >= max_attempts,
	// idx_task_events_pollable его больше не вернёт). Сам по себе мёртвый job ничем не
	// разбудит Orchestrator.Step, и задача зависнет. Пингуем step_req, чтобы Router
	// переоценил ситуацию: он увидит job в Failed jobs и сможет эскалировать в needs_human
	// вместо вечного переназначения того же агента на тот же артефакт.
	if ev.Attempts+1 >= ev.MaxAttempts {
		w.logger.WarnContext(ctx, "agent_job exhausted retries (dead), pinging orchestrator",
			"worker_id", w.cfg.WorkerID, "task_event_id", ev.ID,
			"task_id", ev.TaskID, "attempts", ev.Attempts+1, "max_attempts", ev.MaxAttempts)
		w.enqueueFollowupStep(ctx, ev.TaskID)
	}
}

// checkCancelRequested — atomic SELECT после Redis-подписки (race-free pattern).
func (w *AgentWorker) checkCancelRequested(ctx context.Context, taskID uuid.UUID) (bool, error) {
	var cancelled bool
	err := w.db.WithContext(ctx).
		Raw(`SELECT cancel_requested FROM tasks WHERE id = ?`, taskID).
		Scan(&cancelled).Error
	if err != nil {
		return false, err
	}
	return cancelled, nil
}

// checkTaskState — текущий state задачи. Используется при pickup, чтобы пропустить
// agent_job если задача в paused/needs_human/terminal (см. Sprint 17 / 6.10).
func (w *AgentWorker) checkTaskState(ctx context.Context, taskID uuid.UUID) (models.TaskState, error) {
	var state models.TaskState
	err := w.db.WithContext(ctx).
		Raw(`SELECT state FROM tasks WHERE id = ?`, taskID).
		Scan(&state).Error
	if err != nil {
		return "", err
	}
	return state, nil
}

// handleNonActiveSkip — задача не в active (paused/needs_human/terminal). Worktree
// ещё не аллоцирован (state-check идёт ДО allocate), поэтому никакой очистки не
// требуется. Помечаем event как Complete (без attempts++) и пинаем Orchestrator
// чтобы он отработал текущее state (например: дождался Resume или подтвердил
// финализацию). На Resume orchestrator пересчитает Router-decision на актуальном
// artifact-state.
func (w *AgentWorker) handleNonActiveSkip(ctx context.Context, ev *models.TaskEvent, state models.TaskState) {
	w.logger.InfoContext(ctx, "agent_job skipped — task not active",
		"worker_id", w.cfg.WorkerID, "task_event_id", ev.ID,
		"task_id", ev.TaskID, "task_state", string(state))

	if err := w.eventRepo.Complete(ctx, ev.ID); err != nil {
		w.logger.ErrorContext(ctx, "mark skipped event complete failed",
			"task_event_id", ev.ID, "error", err.Error())
	}
	w.enqueueFollowupStep(ctx, ev.TaskID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Артефакты
// ─────────────────────────────────────────────────────────────────────────────

// saveArtifact парсит ExecutionResult.Output как AgentResponseEnvelope; если не
// получилось — делает fallback-артефакт kind='raw_output' с truncated summary.
// Дополнительно: для review-артефактов вызывает SupersedePrevious чтобы прошлые
// итерации перевести в superseded.
func (w *AgentWorker) saveArtifact(ctx context.Context, taskID uuid.UUID, agentRec *models.Agent, result *agent.ExecutionResult, wtRec *models.Worktree, repoSlug string, targetArtifacts ...*models.Artifact) error {
	var targetArtifact *models.Artifact
	if len(targetArtifacts) > 0 {
		targetArtifact = targetArtifacts[0]
	}

	// 1. Try to extract multiple artifacts first
	if arts, ok := extractMultipleArtifacts(result, agentRec.Name, taskID, targetArtifact); ok && len(arts) > 0 {
		w.logger.InfoContext(ctx, "extracted multiple artifacts from agent output",
			"agent", agentRec.Name, "task_id", taskID, "count", len(arts))
		for _, art := range arts {
			stampRepoSlugIntoArtifact(art, repoSlug)
			err := retryOnConflict(ctx, w.logger, func() error {
				return w.artifactRepo.Create(ctx, art)
			})
			if err != nil {
				return fmt.Errorf("failed to create extracted artifact: %w", err)
			}
		}
		return nil
	}

	envelope, ok := parseAgentEnvelope(result, agentRec.Name, targetArtifact)
	if !ok {
		// Fallback — агент не следовал формату. Сохраняем как raw_output.
		w.logger.WarnContext(ctx, "agent did not return envelope, saving raw_output fallback",
			"agent", agentRec.Name, "task_id", taskID,
			logging.SafeRawAttr([]byte(result.Output)))

		fallbackKind := "raw_output"
		switch agentRec.Name {
		case "planner":
			if targetArtifact != nil && (targetArtifact.Kind == models.ArtifactKindDecomposition || targetArtifact.Kind == models.ArtifactKindPlan) {
				fallbackKind = string(models.ArtifactKindPlan)
			} else {
				fallbackKind = string(models.ArtifactKindSubtaskDescription)
			}
		case "developer":
			fallbackKind = string(models.ArtifactKindCodeDiff)
		case "reviewer":
			fallbackKind = string(models.ArtifactKindReview)
		case "tester":
			fallbackKind = string(models.ArtifactKindTestResult)
		case "merger":
			fallbackKind = string(models.ArtifactKindMergedCode)
		}

		var parentID *uuid.UUID
		if targetArtifact != nil {
			parentID = &targetArtifact.ID
		}

		envelope = AgentResponseEnvelope{
			Kind:             fallbackKind,
			Summary:          fallbackSummary(result, targetArtifact, agentRec.Name),
			ParentArtifactID: parentID,
			Content:          rawOutputContent(result),
		}
	}
	if envelope.Summary == "" {
		envelope.Summary = fallbackSummary(result, targetArtifact, agentRec.Name)
	}
	if !models.ValidateArtifactSummary(envelope.Summary) {
		// Усекаем (rune-based) до 500 — артефакт-validator также rune-based.
		envelope.Summary = truncateRunesForArtifact(envelope.Summary, 500)
	}

	// Если это review/changes_requested на каком-то артефакте — пометим прошлые
	// review той же кали kind+parent как superseded (новая итерация).
	if envelope.ParentArtifactID != nil && envelope.Kind == string(models.ArtifactKindReview) {
		err := retryOnConflict(ctx, w.logger, func() error {
			_, err := w.artifactRepo.SupersedePrevious(ctx, taskID, envelope.ParentArtifactID, models.ArtifactKindReview)
			return err
		})
		if err != nil {
			w.logger.WarnContext(ctx, "supersede previous reviews failed",
				"task_id", taskID, "parent_id", *envelope.ParentArtifactID, "error", err.Error())
		}
	}

	content := envelope.Content
	if len(content) == 0 {
		content = json.RawMessage(`{}`)
	}

	// Для code_diff подменяем рукописный diff и ветку на реальные данные из sandbox.
	// LLM склонен «конспектировать» diff в JSON (хунки вида `@@ imports @@` вместо
	// `@@ -165,6 +165,7 @@`, «(new file) — full content added» без тела), что git apply
	// не берёт, и merger вынужден переписывать руками. Реальный full.diff (git diff --cached
	// origin/<base>, см. entrypoint) — машинно-применим, поэтому он первичен.
	if envelope.Kind == string(models.ArtifactKindCodeDiff) {
		var contentMap map[string]any
		if err := json.Unmarshal(content, &contentMap); err == nil {
			if realDiff := sandboxRealDiff(result); realDiff != "" {
				contentMap["diff"] = realDiff
			}
			if wtRec != nil {
				contentMap["branch_name"] = wtRec.BranchName
			}
			if updatedContent, err := json.Marshal(contentMap); err == nil {
				content = updatedContent
			}
		}
	}

	// Sprint 4 review fix §1: scrubbing для test_result.raw_output_truncated.
	// Tester может включить stack trace / env-dump в raw_output; пройдёмся
	// secret_scrub'ом перед записью в artifact.content (jsonb незашифрован).
	//
	// Sprint 4 review fix §2 (stricter): на ОШИБКЕ scrub'а raw_output_truncated
	// ЗАМЕНЯЕТСЯ на sentinel {"_scrub_failed": true, "len": N, "head_sha256_8": "..."}.
	// Безопасность > availability: лучше потерять детали падений, чем pers'нуть
	// потенциально-сырой env-dump в jsonb. Остальные поля test_result (счётчики,
	// boolean'ы checks) сохраняются как есть — их семантика не sensitive.
	if envelope.Kind == string(models.ArtifactKindTestResult) {
		scrubbed, err := scrubTestResultRawOutput(content)
		if err != nil {
			w.logger.WarnContext(ctx, "test_result raw_output scrub failed, replacing with sentinel",
				"task_id", taskID, "error", err.Error())
			if redacted, rerr := redactRawOutputToSentinel(content); rerr == nil {
				content = redacted
			} else {
				// Даже sentinel-замена не вышла — это критично, артефакт не сохраняем.
				return fmt.Errorf("scrub failed and sentinel redaction failed: scrub=%w, redact=%v", err, rerr)
			}
		} else {
			content = scrubbed
		}
	}

	art := &models.Artifact{
		TaskID:        taskID,
		ParentID:      envelope.ParentArtifactID,
		ProducerAgent: agentRec.Name,
		Kind:          models.ArtifactKind(envelope.Kind),
		Summary:       envelope.Summary,
		Content:       datatypes.JSON(content),
		Status:        models.ArtifactStatusReady,
	}
	stampRepoSlugIntoArtifact(art, repoSlug)
	if err := retryOnConflict(ctx, w.logger, func() error {
		return w.artifactRepo.Create(ctx, art)
	}); err != nil {
		return err
	}
	return nil
}

// stampRepoSlugIntoArtifact — проставляет repo_slug в content code-артефакта, если его
// там ещё нет. Без этого прямой developer-диспатч (router-правило 7, без декомпозиции)
// производит code_diff БЕЗ repo_slug в цепочке артефактов, и PR-гейт
// (resolveTouchedRepos) молча относит его к primary-репо — инцидент f1d9549e: код уехал
// в mcp-servers, а пустой MR открылся в bot-service. Слаг берётся из job payload
// (_repo_slug), который к этому моменту уже разрешён multi-repo guard'ом.
func stampRepoSlugIntoArtifact(art *models.Artifact, slug string) {
	if slug == "" || art == nil ||
		(art.Kind != models.ArtifactKindCodeDiff && art.Kind != models.ArtifactKindMergedCode) {
		return
	}
	var m map[string]any
	if len(art.Content) == 0 || json.Unmarshal(art.Content, &m) != nil || m == nil {
		m = map[string]any{}
	}
	if s, _ := m["repo_slug"].(string); s != "" {
		return // уже проставлен (например, decomposer-цепочкой)
	}
	m["repo_slug"] = slug
	if b, err := json.Marshal(m); err == nil {
		art.Content = datatypes.JSON(b)
	}
}

// resolveBranchNameForArtifact traverses up the artifact parent chain to find the git branch
// name associated with the changes. Finds "branch_name" from code_diff or "merged_branch" from merged_code.
func (w *AgentWorker) resolveBranchNameForArtifact(ctx context.Context, art *models.Artifact) (string, error) {
	curr := art
	for curr != nil {
		if curr.Kind == models.ArtifactKindCodeDiff {
			var contentMap map[string]any
			if err := json.Unmarshal(curr.Content, &contentMap); err == nil {
				if b, ok := contentMap["branch_name"].(string); ok && b != "" {
					return b, nil
				}
			}
			// Fallback to worktree lookup using parent_id (which is subtask_description id)
			var wt models.Worktree
			if curr.ParentID != nil {
				if err := w.db.WithContext(ctx).Where("subtask_id = ?", *curr.ParentID).Order("allocated_at DESC").First(&wt).Error; err == nil {
					return wt.BranchName, nil
				}
			}
			break
		}
		if curr.Kind == models.ArtifactKindMergedCode {
			var out models.MergerOutput
			if err := json.Unmarshal(curr.Content, &out); err == nil && out.MergedBranch != "" {
				return out.MergedBranch, nil
			}
			break
		}
		if curr.ParentID == nil {
			break
		}
		parent, err := w.artifactRepo.GetByID(ctx, *curr.ParentID)
		if err != nil {
			return "", err
		}
		curr = parent
	}
	return "", nil
}

// extractJSON extracts a JSON substring from the output, handling markdown blocks and preambles.
func extractJSON(output string) (string, bool) {
	if output == "" {
		return "", false
	}
	// 1. Try raw JSON as a whole.
	if json.Valid([]byte(output)) {
		return output, true
	}
	// 2. Try markdown json block.
	matches := jsonArtifactsRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		jsonStr := strings.TrimSpace(matches[1])
		if json.Valid([]byte(jsonStr)) {
			return jsonStr, true
		}
	}
	// 3. Try to find the outermost JSON object by matching { and }.
	firstBrace := strings.Index(output, "{")
	lastBrace := strings.LastIndex(output, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		candidate := output[firstBrace : lastBrace+1]
		if json.Valid([]byte(candidate)) {
			return candidate, true
		}
	}
	// 4. Try to find the outermost JSON array by matching [ and ].
	firstBracket := strings.Index(output, "[")
	lastBracket := strings.LastIndex(output, "]")
	if firstBracket != -1 && lastBracket != -1 && lastBracket > firstBracket {
		candidate := output[firstBracket : lastBracket+1]
		if json.Valid([]byte(candidate)) {
			return candidate, true
		}
	}
	return "", false
}

// parseAgentEnvelope пытается распарсить ArtifactsJSON (если есть) или Output как envelope.
func parseAgentEnvelope(result *agent.ExecutionResult, agentName string, targetArtifacts ...*models.Artifact) (AgentResponseEnvelope, bool) {
	var targetArtifact *models.Artifact
	if len(targetArtifacts) > 0 {
		targetArtifact = targetArtifacts[0]
	}

	var env AgentResponseEnvelope
	// 1. ArtifactsJSON (если LLMAgentExecutor нашёл ```json ... ``` блок).
	if len(result.ArtifactsJSON) > 0 {
		if err := json.Unmarshal(result.ArtifactsJSON, &env); err == nil && env.Kind != "" {
			if env.ParentArtifactID == nil && targetArtifact != nil {
				id := targetArtifact.ID
				env.ParentArtifactID = &id
			}
			return env, true
		}
	}
	// 2. Try extraction using extractJSON helper.
	var rawJSON string
	if result.Output != "" {
		if extracted, ok := extractJSON(result.Output); ok {
			rawJSON = extracted
			if err := json.Unmarshal([]byte(extracted), &env); err == nil && env.Kind != "" {
				if env.ParentArtifactID == nil && targetArtifact != nil {
					id := targetArtifact.ID
					env.ParentArtifactID = &id
				}
				return env, true
			}
		}
	}

	// 3. Try parsing Output/Markdown JSON as direct review/test_result structures (Sprint 17 / sandbox formats).
	if rawJSON != "" {
		var rawMap map[string]interface{}
		if err := json.Unmarshal([]byte(rawJSON), &rawMap); err == nil {
			// Check if it's a direct review (has decision with review values)
			if decVal, ok := rawMap["decision"]; ok && (agentName == "reviewer" || agentName == "planner") {
				if decStr, ok := decVal.(string); ok {
					decStr = strings.TrimSpace(strings.ToLower(decStr))
					if decStr == "approve" || decStr == "approved" || decStr == "changes_requested" || decStr == "escalate_to_planner" {
						if decStr == "approve" {
							decStr = "approved"
						}
						env.Kind = "review"
						if sumVal, ok := rawMap["summary"]; ok {
							if sumStr, ok := sumVal.(string); ok && sumStr != "" {
								env.Summary = sumStr
							}
						}
						if env.Summary == "" {
							env.Summary = fmt.Sprintf("Review decision: %s", decStr)
						}
						if targetArtifact != nil {
							id := targetArtifact.ID
							env.ParentArtifactID = &id
						}
						if parentVal, ok := rawMap["parent_artifact_id"]; ok {
							if parentStr, ok := parentVal.(string); ok && parentStr != "" {
								if pID, err := uuid.Parse(parentStr); err == nil {
									env.ParentArtifactID = &pID
								}
							}
						}
						env.Content = json.RawMessage(rawJSON)
						return env, true
					}
				}
			}

			// Check if it's a direct test_result (has test_result or decision=passed/failed)
			isTestResult := false
			if agentName == "tester" {
				if trVal, ok := rawMap["test_result"]; ok {
					if trStr, ok := trVal.(string); ok {
						trStr = strings.TrimSpace(strings.ToLower(trStr))
						if trStr == "pass" || trStr == "fail" || trStr == "passed" || trStr == "failed" || trStr == "blocked" {
							isTestResult = true
						}
					}
				}
				if !isTestResult {
					if decVal, ok := rawMap["decision"]; ok {
						if decStr, ok := decVal.(string); ok {
							decStr = strings.TrimSpace(strings.ToLower(decStr))
							if decStr == "passed" || decStr == "failed" || decStr == "blocked" {
								isTestResult = true
							}
						}
					}
				}
			}

			if isTestResult {
				env.Kind = "test_result"
				if sumVal, ok := rawMap["summary"]; ok {
					if sumStr, ok := sumVal.(string); ok && sumStr != "" {
						env.Summary = sumStr
					}
				}
				if env.Summary == "" {
					decStr := ""
					if decVal, ok := rawMap["decision"]; ok {
						if ds, ok := decVal.(string); ok {
							decStr = ds
						}
					}
					trStr := ""
					if trVal, ok := rawMap["test_result"]; ok {
						if ts, ok := trVal.(string); ok {
							trStr = ts
						}
					}
					env.Summary = fmt.Sprintf("Test result: %s (decision: %s)", trStr, decStr)
				}
				if targetArtifact != nil {
					id := targetArtifact.ID
					env.ParentArtifactID = &id
				}
				env.Content = json.RawMessage(rawJSON)
				return env, true
			}
		}
	}

	return AgentResponseEnvelope{}, false
}

func fallbackSummary(result *agent.ExecutionResult, targetArtifact *models.Artifact, agentName string) string {
	statusSuffix := "completed successfully"
	if !result.Success {
		statusSuffix = "failed"
	}
	if targetArtifact != nil {
		return truncateRunesForArtifact(fmt.Sprintf("%s for %s (%s) (%s)", agentName, targetArtifact.Kind, targetArtifact.ID.String()[:8], statusSuffix), 500)
	}
	return truncateRunesForArtifact(fmt.Sprintf("%s execution (%s)", agentName, statusSuffix), 500)
}

// agentResultUnusable — true, если результат агента нельзя превращать в ready-артефакт:
// либо исполнение неуспешно (sandbox OOM/timeout/crash → Success=false), либо вывод
// пустой и нет ArtifactsJSON. Такой исход должен идти в failEvent (retry/backoff →
// смерть job'а), а НЕ сохраняться как code_diff{"raw_output":""}: иначе Router примет
// пустышку за выполненную работу и зациклится на переназначениях (разбор задачи 1.1).
func agentResultUnusable(result *agent.ExecutionResult) bool {
	if result == nil {
		return true
	}
	if !result.Success {
		return true
	}
	return strings.TrimSpace(result.Output) == "" && len(result.ArtifactsJSON) == 0
}

func rawOutputContent(result *agent.ExecutionResult) json.RawMessage {
	wrapper := map[string]string{"raw_output": result.Output}
	b, _ := json.Marshal(wrapper)
	return b
}

// sandboxRealDiff извлекает реальный unified diff, собранный sandbox-entrypoint'ом
// (full.diff = git diff --cached origin/<base>), из ExecutionResult.ArtifactsJSON.
// SandboxAgentExecutor кладёт туда {"diff","commit_hash","branch_name"} БЕЗ поля kind;
// LLM-executor же кладёт в ArtifactsJSON готовый envelope с kind. Поэтому при наличии
// kind возвращаем пусто — чтобы не трогать LLM-only путь и не подменять diff там, где
// реального full.diff из песочницы нет.
func sandboxRealDiff(result *agent.ExecutionResult) string {
	if result == nil || len(result.ArtifactsJSON) == 0 {
		return ""
	}
	var sb struct {
		Kind string `json:"kind"`
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(result.ArtifactsJSON, &sb); err != nil {
		return ""
	}
	if sb.Kind != "" {
		return ""
	}
	return sb.Diff
}

func truncateRunesForArtifact(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

// ─────────────────────────────────────────────────────────────────────────────
// Step-followup enqueue
// ─────────────────────────────────────────────────────────────────────────────

// resolveArtifactByPrefix ищет артефакт задачи по префиксу его UUID (страховка от
// обрезанных id из Router'а). Возвращает полный артефакт ТОЛЬКО при ровно одном
// совпадении; при 0 или неоднозначности — nil (лучше остаться без target, чем привязать
// к чужому артефакту). Требует префикс ≥ 4 символов, чтобы не матчить наугад.
func (w *AgentWorker) resolveArtifactByPrefix(ctx context.Context, taskID uuid.UUID, prefix string) *models.Artifact {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if len(prefix) < 4 {
		return nil
	}
	arts, err := w.artifactRepo.ListMetadataByTaskID(ctx, taskID, false)
	if err != nil {
		w.logger.WarnContext(ctx, "prefix resolve: list artifacts failed", "task_id", taskID, "error", err.Error())
		return nil
	}
	var matchID uuid.UUID
	matches := 0
	for _, a := range arts {
		if strings.HasPrefix(strings.ToLower(a.ID.String()), prefix) {
			matchID = a.ID
			matches++
		}
	}
	if matches != 1 {
		return nil
	}
	full, err := w.artifactRepo.GetByID(ctx, matchID)
	if err != nil {
		return nil
	}
	return full
}

// extractSubtasks достаёт список подзадач из content декомпозиции. Поддерживает обе формы,
// которые встречаются в выводе decomposer'а: {"subtasks":[...]} и {"content":{"subtasks":[...]}}.
func extractSubtasks(content datatypes.JSON) []any {
	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		return nil
	}
	if v, ok := m["subtasks"].([]any); ok {
		return v
	}
	if inner, ok := m["content"].(map[string]any); ok {
		if v, ok := inner["subtasks"].([]any); ok {
			return v
		}
	}
	return nil
}

// extractRepoSlug достаёт repo_slug подзадачи из content артефакта (мульти-репо).
// Поддерживает обе формы: {"repo_slug":"..."} и {"content":{"repo_slug":"..."}}.
func extractRepoSlug(content datatypes.JSON) string {
	var m map[string]any
	if err := json.Unmarshal(content, &m); err != nil {
		return ""
	}
	if s, ok := m["repo_slug"].(string); ok && s != "" {
		return s
	}
	if inner, ok := m["content"].(map[string]any); ok {
		if s, ok := inner["repo_slug"].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// ErrDecompositionRepoSlugMissing — мульти-репо проект, но у подзадачи декомпозиции нет
// разрешимого repo_slug (ни явного из каталога, ни выводимого из текста). Декомпозиция
// отвергается (split не создаёт subtask_description), event фейлится → retry/backoff, при
// исчерпании Router эскалирует в needs_human. Лучше пере-сгенерировать декомпозицию, чем
// молча разложить подзадачи на primary-репо и наредактировать не тот репозиторий.
var ErrDecompositionRepoSlugMissing = errors.New("decomposition subtask has no resolvable repo_slug in multi-repo project")

// projectRepoSlugs — список slug'ов репозиториев проекта (пусто для legacy/одно-репо).
func projectRepoSlugs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	out := make([]string, 0, len(project.Repositories))
	for i := range project.Repositories {
		if s := strings.TrimSpace(project.Repositories[i].Slug); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// inferRepoSlugFromText возвращает ЕДИНСТВЕННЫЙ известный slug, встречающийся в тексте как
// отдельный токен (регистронезависимо). Пусто, если совпадений ноль или больше одного:
// неоднозначность — это сигнал отвергнуть/эскалировать, а не угадывать (иначе рискуем
// разложить работу на чужой репозиторий).
func inferRepoSlugFromText(text string, slugs []string) string {
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	found := ""
	for _, s := range slugs {
		ls := strings.ToLower(strings.TrimSpace(s))
		if ls == "" {
			continue
		}
		if containsWholeToken(lower, ls) {
			if found != "" && found != s {
				return "" // неоднозначно
			}
			found = s
		}
	}
	return found
}

// inferRepoSlugFromProject — как inferRepoSlugFromText, но матчит не только slug,
// а ещё display_name и basename git_url: slug primary-репо обычно «main», тогда как
// в тексте задачи пишут имя репозитория («bot-service»). Совпадение с двумя разными
// репозиториями → "" (неоднозначно).
func inferRepoSlugFromProject(text string, project *models.Project) string {
	if text == "" || project == nil {
		return ""
	}
	lower := strings.ToLower(text)
	found := ""
	for i := range project.Repositories {
		repo := &project.Repositories[i]
		if strings.TrimSpace(repo.Slug) == "" || !repoMatchesText(lower, repo) {
			continue
		}
		if found != "" && found != repo.Slug {
			return "" // неоднозначно
		}
		found = repo.Slug
	}
	return found
}

// repoMatchesText — упоминается ли репозиторий в тексте (по slug, display_name
// или basename git_url). lowerText ожидается в нижнем регистре.
func repoMatchesText(lowerText string, repo *models.ProjectRepository) bool {
	for _, tok := range []string{repo.Slug, repo.DisplayName, gitURLBasename(repo.GitURL)} {
		lt := strings.ToLower(strings.TrimSpace(tok))
		if lt == "" {
			continue
		}
		if containsWholeToken(lowerText, lt) {
			return true
		}
	}
	return false
}

// gitURLBasename — последний сегмент git URL без .git: «.../pai/bot-service.git» → «bot-service».
func gitURLBasename(u string) string {
	u = strings.TrimSuffix(strings.TrimSpace(u), ".git")
	if i := strings.LastIndexAny(u, "/:"); i >= 0 {
		u = u[i+1:]
	}
	return u
}

// containsWholeToken — есть ли needle в haystack как целый токен (границы — не [a-z0-9_-]).
// Оба аргумента ожидаются в нижнем регистре.
func containsWholeToken(haystack, needle string) bool {
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		leftOK := i == 0 || !isSlugTokenByte(haystack[i-1])
		right := i + len(needle)
		rightOK := right >= len(haystack) || !isSlugTokenByte(haystack[right])
		if leftOK && rightOK {
			return true
		}
		from = i + 1
	}
}

func isSlugTokenByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_'
}

// repoInferTextFromArtifacts — конкатенация summary+title+description целевых артефактов для
// вывода repo_slug, когда он не проставлен явно (defence-in-depth на стороне воркера).
func repoInferTextFromArtifacts(arts []*models.Artifact) string {
	var b strings.Builder
	for _, a := range arts {
		if a == nil {
			continue
		}
		b.WriteString(a.Summary)
		b.WriteByte('\n')
		if len(a.Content) == 0 {
			continue
		}
		var m map[string]any
		if json.Unmarshal(a.Content, &m) != nil {
			continue
		}
		b.WriteString(stringFromAny(m["title"]))
		b.WriteByte('\n')
		b.WriteString(stringFromAny(m["description"]))
		b.WriteByte('\n')
		if inner, ok := m["content"].(map[string]any); ok {
			b.WriteString(stringFromAny(inner["title"]))
			b.WriteByte('\n')
			b.WriteString(stringFromAny(inner["description"]))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// splitDecomposition разбивает артефакт decomposition на индивидуальные subtask_description
// (по одному на каждый элемент content.subtasks), привязывая их parent_id к декомпозиции.
// ИДЕМПОТЕНТНО: если для этой декомпозиции subtask_description уже созданы — ничего не делает
// (bypass при повторном dispatch и first-pass auto-split не должны плодить дубли). Возвращает
// число созданных артефактов.
func (w *AgentWorker) splitDecomposition(ctx context.Context, project *models.Project, taskID, decompositionID uuid.UUID, content datatypes.JSON) (int, error) {
	// Идемпотентность: пропускаем, если уже разбивали.
	if existing, err := w.artifactRepo.ListMetadataByTaskID(ctx, taskID, false); err == nil {
		for _, a := range existing {
			if a.Kind == models.ArtifactKindSubtaskDescription && a.ParentID != nil && *a.ParentID == decompositionID {
				return 0, nil
			}
		}
	}

	subtasks := extractSubtasks(content)
	if len(subtasks) == 0 {
		return 0, nil
	}

	// Мульти-репо: каждая подзадача обязана иметь валидный repo_slug. Сначала
	// валидируем/нормализуем ВСЕ подзадачи (без частичного создания артефактов): если slug
	// отсутствует/невалиден — пробуем вывести его из текста подзадачи; если не вышло —
	// отвергаем всю декомпозицию (ErrDecompositionRepoSlugMissing). Так developer не уедет
	// на primary-репо вслепую.
	slugs := projectRepoSlugs(project)
	multiRepo := len(slugs) >= 2

	type preparedSubtask struct {
		title string
		bytes []byte
	}
	prepared := make([]preparedSubtask, 0, len(subtasks))
	for _, stVal := range subtasks {
		stMap, ok := stVal.(map[string]any)
		if !ok {
			continue
		}
		title := "Subtask"
		if t, ok := stMap["title"].(string); ok && t != "" {
			title = t
		}
		if multiRepo {
			slug := strings.TrimSpace(stringFromAny(stMap["repo_slug"]))
			if slug == "" || !containsStr(slugs, slug) {
				inferText := title + "\n" + stringFromAny(stMap["description"])
				if inf := inferRepoSlugFromText(inferText, slugs); inf != "" {
					w.logger.WarnContext(ctx, "decomposer omitted/invalid repo_slug; inferred from subtask text",
						"task_id", taskID, "subtask", truncate(title, 80), "given_repo_slug", slug, "inferred_repo_slug", inf)
					slug = inf
				} else {
					return 0, fmt.Errorf("%w: subtask %q (project repos: %v, given: %q)",
						ErrDecompositionRepoSlugMissing, truncate(title, 80), slugs, slug)
				}
			}
			stMap["repo_slug"] = slug // нормализуем в content артефакта → downstream резолв тривиален
		}
		subtaskBytes, err := json.Marshal(stMap)
		if err != nil {
			return 0, fmt.Errorf("marshal subtask: %w", err)
		}
		prepared = append(prepared, preparedSubtask{title: title, bytes: subtaskBytes})
	}

	created := 0
	for _, p := range prepared {
		parentID := decompositionID
		art := &models.Artifact{
			TaskID:        taskID,
			ParentID:      &parentID,
			ProducerAgent: "decomposer",
			Kind:          models.ArtifactKindSubtaskDescription,
			Summary:       p.title,
			Content:       datatypes.JSON(p.bytes),
			Status:        models.ArtifactStatusReady,
		}
		if !models.ValidateArtifactSummary(art.Summary) {
			art.Summary = truncateRunesForArtifact(art.Summary, 500)
		}
		if err := retryOnConflict(ctx, w.logger, func() error {
			return w.artifactRepo.Create(ctx, art)
		}); err != nil {
			return created, fmt.Errorf("create subtask_description: %w", err)
		}
		created++
	}
	return created, nil
}

// splitLatestDecomposition находит самую свежую decomposition задачи и разбивает её на подзадачи
// (детерминированно, сразу после первого прогона decomposer'а — не дожидаясь повторного dispatch
// Router'ом). Без этого «умный» Router может уйти к developer'ам мимо подзадач, и code_diff'ы
// окажутся без parent (orphaned), а поток — «сплющенным». No-op если декомпозиции нет или она
// уже разбита.
func (w *AgentWorker) splitLatestDecomposition(ctx context.Context, project *models.Project, taskID uuid.UUID) error {
	arts, err := w.artifactRepo.ListByTaskID(ctx, taskID, true)
	if err != nil {
		w.logger.WarnContext(ctx, "auto-split: list artifacts failed", "task_id", taskID, "error", err.Error())
		return nil
	}
	var latest *models.Artifact
	for i := range arts {
		if arts[i].Kind == models.ArtifactKindDecomposition {
			latest = &arts[i] // ListByTaskID отсортирован по created_at — последнее совпадение самое свежее
		}
	}
	if latest == nil {
		return nil
	}
	n, err := w.splitDecomposition(ctx, project, taskID, latest.ID, latest.Content)
	if err != nil {
		// Невалидная декомпозиция (мульти-репо без resolvable repo_slug) пробрасывается наверх:
		// caller зафейлит event → retry/backoff перегенерит decomposer'а. Прочие ошибки
		// (инфраструктурные) логируем и не валим job — поведение как раньше.
		if errors.Is(err, ErrDecompositionRepoSlugMissing) {
			return err
		}
		w.logger.WarnContext(ctx, "auto-split decomposition failed", "task_id", taskID, "error", err.Error())
		return nil
	}
	if n > 0 {
		projectID := uuid.Nil
		if project != nil {
			projectID = project.ID
		}
		w.logger.InfoContext(ctx, "auto-split decomposition into subtasks",
			"task_id", taskID, "decomposition_id", latest.ID, "subtasks", n)
		w.publishArtifactCreated(ctx, projectID, taskID, "decomposer")
	}
	return nil
}

// publishArtifactCreated шлёт ArtifactCreated в EventBus для live-апдейта UI (HubBridge →
// фронт рефетчит список артефактов задачи). No-op если bus не сконфигурирован. Гранулярность —
// на уровне задачи+продюсера: id/kind конкретного артефакта не отслеживаем здесь (один
// agent_job может создать несколько артефактов), фронт всё равно перезапрашивает весь список.
func (w *AgentWorker) publishArtifactCreated(ctx context.Context, projectID, taskID uuid.UUID, producer string) {
	if w.bus == nil || projectID == uuid.Nil {
		return
	}
	w.bus.Publish(ctx, events.ArtifactCreated{
		ProjectID:     projectID,
		TaskID:        taskID,
		ProducerAgent: producer,
		OccurredAt:    time.Now(),
	})
}

func (w *AgentWorker) enqueueFollowupStep(ctx context.Context, taskID uuid.UUID) {
	// Дедуп-enqueue: при завершении N параллельных job'ов не плодим N step_req'ов —
	// Router'у достаточно одного прогона на актуальном состоянии. Коалесцирование убирает
	// бесполезные «ожидающие» вызовы дорогой LLM-модели без потери решений (см. репозиторий).
	err := retryOnConflict(ctx, w.logger, func() error {
		return w.eventRepo.EnqueueFollowupStepReq(ctx, taskID)
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "enqueue follow-up step_req failed",
			"task_id", taskID, "error", err.Error())
		return
	}
	if w.notifier != nil {
		if err := w.notifier.NotifyTaskEvent(ctx, string(models.TaskEventKindStepReq)); err != nil {
			w.logger.WarnContext(ctx, "follow-up step_req NOTIFY failed",
				"task_id", taskID, "error", err.Error())
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// redactRawOutputToSentinel — fallback на отказ scrub'а: ЗАМЕНЯЕТ raw_output_truncated
// на безопасный sentinel (длина + хэш первых 64 байт). Если поля не было — возвращает
// content без изменений (нечего редактировать).
//
// Sprint 4 review fix §2: stricter policy "безопасность > availability". Эта функция
// должна СПРАВЛЯТЬСЯ всегда (это просто json marshal); если она тоже упала — caller
// возвращает ошибку и отказывается сохранять артефакт целиком.
func redactRawOutputToSentinel(content []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("unmarshal content for sentinel: %w", err)
	}
	rawField, ok := m["raw_output_truncated"]
	if !ok {
		return content, nil // нечего редактировать
	}
	// Различаем "пустой valid string" и "non-string (агент нарушил контракт)" —
	// семантика разная: первая — нормальный edge-case (tester не сохранил output),
	// вторая — баг в промпте/агенте, видим из отдельного флага в sentinel.
	var raw string
	typeMismatch := false
	if err := json.Unmarshal(rawField, &raw); err != nil {
		typeMismatch = true
		raw = ""
	}
	// Тот же primitive что и logging.SafeRawAttr — длина + sha256[:8].
	hash := sha256.Sum256([]byte(raw)[:min(64, len(raw))])
	sentinel := map[string]any{
		"_scrub_failed": true,
		"len":           len(raw),
		"head_sha256_8": hex.EncodeToString(hash[:8]),
	}
	if typeMismatch {
		sentinel["_type_mismatch"] = true
	}
	sentinelBytes, err := json.Marshal(sentinel)
	if err != nil {
		return nil, fmt.Errorf("marshal sentinel: %w", err)
	}
	m["raw_output_truncated"] = sentinelBytes
	return json.Marshal(m)
}

// min — для Go <1.21 (хотя у нас новее, оставим locally чтобы не зависеть).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// scrubTestResultRawOutput — Sprint 4 review fix §1.
// Распаковывает content как map, прогоняет raw_output_truncated через ScrubSecrets,
// упаковывает обратно. Если поля нет — возвращает content без изменений.
//
// Альтернатива (ParseTestResult + повторная сериализация) теряла бы неизвестные
// поля контракта; map-подход сохраняет всё что прислал агент.
func scrubTestResultRawOutput(content []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("unmarshal content: %w", err)
	}
	rawField, ok := m["raw_output_truncated"]
	if !ok {
		return content, nil
	}
	var raw string
	if err := json.Unmarshal(rawField, &raw); err != nil {
		// Поле есть, но это не строка — пропускаем (агент сломал контракт, но не наше дело).
		return content, nil
	}
	scrubbed := ScrubSecrets(raw)
	if scrubbed == raw {
		return content, nil // ничего не изменилось — экономим re-marshal
	}
	scrubbedBytes, err := json.Marshal(scrubbed)
	if err != nil {
		return nil, fmt.Errorf("marshal scrubbed: %w", err)
	}
	m["raw_output_truncated"] = scrubbedBytes
	return json.Marshal(m)
}

// allocateWorktreeForJob — just-in-time allocation для sandbox-агента.
// Распаковывает base_branch из payload.Input и зовёт WorktreeManager.Allocate.
//
// Если ev.TaskID не задан или base_branch отсутствует — возвращает ошибку (caller
// помечает event как failed, retry-семантика очереди подберёт).
func (w *AgentWorker) allocateWorktreeForJob(ctx context.Context, ev *models.TaskEvent, payload *models.AgentJobPayload) (*models.Worktree, error) {
	if payload == nil || payload.Input == nil {
		return nil, fmt.Errorf("payload.Input is nil; cannot resolve base_branch for worktree alloc")
	}
	rawBase, ok := payload.Input["_base_branch"]
	if !ok {
		return nil, fmt.Errorf("payload.Input[_base_branch] missing")
	}
	baseBranch, ok := rawBase.(string)
	if !ok || baseBranch == "" {
		return nil, fmt.Errorf("payload.Input[_base_branch] is not a non-empty string")
	}

	// subtask_id — опциональный; если в Input есть target_artifact_id, используем его
	// для трассировки (worktrees.subtask_id).
	subtaskID := uuid.Nil
	if raw, ok := payload.Input["target_artifact_id"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			if id, err := uuid.Parse(s); err == nil {
				subtaskID = id
			}
		}
	}

	return w.worktreeMgr.Allocate(ctx, ev.TaskID, subtaskID, baseBranch)
}

func (w *AgentWorker) buildExecutionInput(task *models.Task, agentRec *models.Agent, input map[string]any, targetArtifact *models.Artifact) agent.ExecutionInput {
	inputJSON, _ := json.Marshal(input)

	var promptParts []string
	if agentRec.Prompt != nil && strings.TrimSpace(agentRec.Prompt.Template) != "" {
		promptParts = append(promptParts, agentRec.Prompt.Template)
	}
	if agentRec.SystemPrompt != nil && strings.TrimSpace(*agentRec.SystemPrompt) != "" {
		promptParts = append(promptParts, *agentRec.SystemPrompt)
	}
	promptSystem := strings.Join(promptParts, "\n\n")
	modelName := derefString(agentRec.Model)
	if modelName == "" && agentRec.ExecutionKind == models.AgentExecutionKindSandbox && len(agentRec.CodeBackendSettings) > 0 {
		if settings, err := decodeCodeBackendSettings(agentRec.CodeBackendSettings); err == nil && settings.Model != "" {
			modelName = settings.Model
		}
	}

	in := agent.ExecutionInput{
		TaskID:            task.ID.String(),
		ProjectID:         task.ProjectID.String(),
		Title:             task.Title,
		Description:       task.Description,
		ContextJSON:       inputJSON,
		AgentID:           agentRec.ID.String(),
		AgentName:         agentRec.Name,
		Role:              string(agentRec.Role),
		Model:             modelName,
		PromptSystem:      promptSystem,
		StructuredContext: inputJSON,
		Temperature:       agentRec.Temperature,
		MaxTokens:         agentRec.MaxTokens,
	}
	if task.Project != nil {
		// Мульти-репо: целевой репозиторий подзадачи определяется по _repo_slug из payload
		// (его проставляет orchestrator из repo_slug подзадачи), иначе primary-репо, иначе
		// старые поля проекта (бэк-компат с одно-репо моделью).
		if targetRepo := resolveTargetRepo(task.Project, input); targetRepo != nil {
			in.GitURL = targetRepo.GitURL
			in.GitDefaultBranch = targetRepo.GitDefaultBranch
			// Соседние репозитории (read-only) — контракты/типы для согласования API↔UI.
			for i := range task.Project.Repositories {
				sib := &task.Project.Repositories[i]
				if sib.Slug == targetRepo.Slug {
					continue
				}
				in.SiblingRepos = append(in.SiblingRepos, agent.SiblingRepo{
					Slug:   sib.Slug,
					GitURL: sib.GitURL,
					Branch: sib.GitDefaultBranch,
				})
			}
		} else {
			in.GitURL = task.Project.GitURL
			in.GitDefaultBranch = task.Project.GitDefaultBranch
		}
	}
	if task.BranchName != nil {
		in.BranchName = *task.BranchName
	}
	if agentRec.ProviderKind != nil {
		in.Provider = string(*agentRec.ProviderKind)
	}
	if agentRec.CodeBackend != nil {
		in.CodeBackend = string(*agentRec.CodeBackend)
	}
	if targetArtifact != nil {
		prettyContent := ""
		if len(targetArtifact.Content) > 0 {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, targetArtifact.Content, "", "  "); err == nil {
				prettyContent = prettyJSON.String()
			} else {
				prettyContent = string(targetArtifact.Content)
			}
		}
		in.PromptUser = fmt.Sprintf("\n\n<target_artifact id=%q producer=%q kind=%q summary=%q>\n%s\n</target_artifact>\n",
			targetArtifact.ID.String(), targetArtifact.ProducerAgent, string(targetArtifact.Kind), targetArtifact.Summary, prettyContent)
	}
	// Мульти-репо: каталог репозиториев проекта во входном контексте агента (decomposer
	// проставляет repo_slug подзадачам; developer понимает доступные соседние репо).
	if task.Project != nil {
		if catalog := renderRepoCatalog(task.Project.Repositories); catalog != "" {
			in.PromptUser = catalog + in.PromptUser
		}
	}
	return in
}

// resolveTargetRepo выбирает целевой репозиторий подзадачи: по _repo_slug из payload,
// иначе primary-репо проекта. Возвращает nil, если у проекта нет загруженных репозиториев
// (тогда вызывающий код откатывается на старые поля Project.Git*).
func resolveTargetRepo(project *models.Project, input map[string]any) *models.ProjectRepository {
	if project == nil || len(project.Repositories) == 0 {
		return nil
	}
	if input != nil {
		if raw, ok := input["_repo_slug"]; ok {
			if slug, ok := raw.(string); ok && slug != "" {
				if repo := project.RepoBySlug(slug); repo != nil {
					return repo
				}
			}
		}
	}
	return project.PrimaryRepo()
}

// renderRepoCatalog рендерит секцию `# Repositories` для входного контекста агента.
// Пусто, если репозиториев меньше двух (одно-репо проекту каталог не нужен).
func renderRepoCatalog(repos []models.ProjectRepository) string {
	if len(repos) < 2 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Repositories\n")
	b.WriteString("Проект состоит из нескольких git-репозиториев. Каждая подзадача относится РОВНО к одному репо (repo_slug). Кросс-репо фичу декомпозируй на подзадачи с depends_on.\n")
	for _, repo := range repos {
		marker := ""
		if repo.IsPrimary {
			marker = " (primary)"
		}
		desc := strings.TrimSpace(repo.RoleDescription)
		if desc == "" {
			desc = repo.DisplayName
		}
		fmt.Fprintf(&b, "- slug=%s%s: %s\n", repo.Slug, marker, desc)
	}
	b.WriteString("\n")
	return b.String()
}

// extractMultipleArtifacts attempts to extract multiple artifacts from result.Output or result.ArtifactsJSON.
func extractMultipleArtifacts(result *agent.ExecutionResult, agentName string, taskID uuid.UUID, targetArtifact *models.Artifact) ([]*models.Artifact, bool) {
	var rawJSON string
	if len(result.ArtifactsJSON) > 0 && json.Valid(result.ArtifactsJSON) {
		rawJSON = string(result.ArtifactsJSON)
	} else if result.Output != "" {
		if extracted, ok := extractJSON(result.Output); ok {
			rawJSON = extracted
		}
	}

	if rawJSON == "" || !json.Valid([]byte(rawJSON)) {
		return nil, false
	}

	// Try parsing as a map with "artifacts" key
	var rawMap map[string]json.RawMessage
	var listRaw json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &rawMap); err == nil {
		if artsRaw, ok := rawMap["artifacts"]; ok {
			listRaw = artsRaw
		}
	}

	// If not "artifacts" map, check if it's directly an array
	if len(listRaw) == 0 {
		var rawArray []json.RawMessage
		if err := json.Unmarshal([]byte(rawJSON), &rawArray); err == nil {
			listRaw = json.RawMessage(rawJSON)
		}
	}

	if len(listRaw) == 0 {
		return nil, false
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(listRaw, &items); err != nil || len(items) == 0 {
		return nil, false
	}

	var arts []*models.Artifact
	for _, item := range items {
		// Extract kind
		kindStr := "subtask_description"
		if kVal, ok := item["kind"]; ok {
			if ks, ok := kVal.(string); ok && ks != "" {
				kindStr = ks
			}
		}

		// Extract summary
		sumStr := ""
		if sVal, ok := item["summary"]; ok {
			if ss, ok := sVal.(string); ok && ss != "" {
				sumStr = ss
			}
		}
		if sumStr == "" {
			if titleVal, ok := item["title"]; ok {
				if ts, ok := titleVal.(string); ok && ts != "" {
					sumStr = ts
				}
			}
		}
		if sumStr == "" {
			sumStr = fmt.Sprintf("Artifact of kind %s", kindStr)
		}

		// Extract parent ID
		var parentID *uuid.UUID
		if targetArtifact != nil {
			id := targetArtifact.ID
			parentID = &id
		}
		for _, key := range []string{"parent", "parent_id", "parent_artifact_id"} {
			if pVal, ok := item[key]; ok {
				if ps, ok := pVal.(string); ok && ps != "" {
					if pUUID, err := uuid.Parse(ps); err == nil {
						parentID = &pUUID
					}
				}
			}
		}

		// Marshal the item itself back to json to be used as content
		contentBytes, _ := json.Marshal(item)

		art := &models.Artifact{
			TaskID:        taskID,
			ParentID:      parentID,
			ProducerAgent: agentName,
			Kind:          models.ArtifactKind(kindStr),
			Summary:       sumStr,
			Content:       datatypes.JSON(contentBytes),
			Status:        models.ArtifactStatusReady,
		}
		// Validate and truncate summary
		if !models.ValidateArtifactSummary(art.Summary) {
			art.Summary = truncateRunesForArtifact(art.Summary, 500)
		}

		arts = append(arts, art)
	}

	return arts, true
}
