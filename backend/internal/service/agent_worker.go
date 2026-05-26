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
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var jsonArtifactsRegex = regexp.MustCompile("(?s)```json\n?(.*?)\n?```")

func retryOnConflict(ctx context.Context, logger *slog.Logger, fn func() error) error {
	var lastErr error
	backoff := 100 * time.Millisecond
	for attempt := 0; attempt < 25; attempt++ {
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
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
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
	WorkerID         string
	PollInterval     time.Duration
	AgentJobTimeout  time.Duration // hard cap на один agent_job (default 1h)
}

// DefaultAgentWorkerConfig — дефолты для llm-агентов. Для sandbox-агентов
// разумно выставить AgentJobTimeout повыше (1-2h).
func DefaultAgentWorkerConfig() AgentWorkerConfig {
	return AgentWorkerConfig{
		WorkerID:        "agent-worker-default",
		PollInterval:    500 * time.Millisecond,
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
	notifier       *RedisNotifier // может быть nil
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
	if cfg.AgentJobTimeout <= 0 {
		cfg.AgentJobTimeout = time.Hour
	}
	if cfg.WorkerID == "" {
		cfg.WorkerID = "agent-worker-default"
	}
	return &AgentWorker{
		db: db, eventRepo: eventRepo, artifactRepo: artifactRepo,
		dispatcher: dispatcher, worktreeMgr: worktreeMgr,
		notifier: notifier, logger: logger, cfg: cfg,
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
		"worker_id", w.cfg.WorkerID, "poll_interval", w.cfg.PollInterval, "job_timeout", w.cfg.AgentJobTimeout)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		w.drainQueue(ctx)
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "agent worker stopping", "worker_id", w.cfg.WorkerID)
			return nil
		case <-ticker.C:
		case <-wakeupCh:
		}
	}
}

func (w *AgentWorker) drainQueue(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		ev, err := w.eventRepo.ClaimNext(ctx, models.TaskEventKindAgentJob, w.cfg.WorkerID)
		if err != nil {
			if errors.Is(err, repository.ErrNoTaskEventAvailable) {
				return
			}
			w.logger.ErrorContext(ctx, "claim next agent_job failed",
				"worker_id", w.cfg.WorkerID, "error", err.Error())
			return
		}
		w.processOne(ctx, ev)
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
	if err := w.db.WithContext(execCtx).Preload("Project").Where("id = ?", ev.TaskID).First(&task).Error; err != nil {
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
	if payload.Input != nil {
		if rawID, ok := payload.Input["target_artifact_id"]; ok {
			if idStr, ok := rawID.(string); ok && idStr != "" {
				if id, err := uuid.Parse(idStr); err == nil {
					art, err := w.artifactRepo.GetByID(execCtx, id)
					if err != nil {
						w.logger.WarnContext(execCtx, "failed to load target artifact", "artifact_id", id, "error", err)
					} else {
						targetArtifact = art
					}
				}
			}
		}
	}

	var in agent.ExecutionInput
	if w.contextBuilder != nil {
		builtIn, err := w.contextBuilder.Build(execCtx, &task, &agentRec, task.Project)
		if err != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("context builder: %w", err))
			return
		}
		in = *builtIn
		// Override ContextJSON and StructuredContext with the step-specific payload.Input
		inputJSON, _ := json.Marshal(payload.Input)
		in.ContextJSON = inputJSON
		in.StructuredContext = inputJSON

		// If there is a target artifact, and it's not already in PromptUser,
		// append it in the target_artifact XML format.
		if targetArtifact != nil && !strings.Contains(in.PromptUser, fmt.Sprintf("<target_artifact id=%q", targetArtifact.ID.String())) {
			prettyContent := ""
			if len(targetArtifact.Content) > 0 {
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, targetArtifact.Content, "", "  "); err == nil {
					prettyContent = prettyJSON.String()
				} else {
					prettyContent = string(targetArtifact.Content)
				}
			}
			in.PromptUser = in.PromptUser + fmt.Sprintf("\n\n<target_artifact id=%q producer=%q kind=%q summary=%q>\n%s\n</target_artifact>\n",
				targetArtifact.ID.String(), targetArtifact.ProducerAgent, string(targetArtifact.Kind), targetArtifact.Summary, prettyContent)
		}
	} else {
		in = w.buildExecutionInput(&task, &agentRec, payload.Input, targetArtifact)
	}

	// Populate branch name from worktree if missing, falling back to default branch
	if in.BranchName == "" {
		if wtRec != nil {
			in.BranchName = wtRec.BranchName
		} else if in.GitDefaultBranch != "" {
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

	// Сохраняем артефакт.
	if err := w.saveArtifact(parentCtx, ev.TaskID, &agentRec, result, targetArtifact); err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("save artifact: %w", err))
		return
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
		w.logger.ErrorContext(parentCtx, "mark event complete failed",
			"task_event_id", ev.ID, "error", err.Error())
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
func (w *AgentWorker) saveArtifact(ctx context.Context, taskID uuid.UUID, agentRec *models.Agent, result *agent.ExecutionResult, targetArtifacts ...*models.Artifact) error {
	var targetArtifact *models.Artifact
	if len(targetArtifacts) > 0 {
		targetArtifact = targetArtifacts[0]
	}

	// 1. Try to extract multiple artifacts first
	if arts, ok := extractMultipleArtifacts(result, agentRec.Name, taskID, targetArtifact); ok && len(arts) > 0 {
		w.logger.InfoContext(ctx, "extracted multiple artifacts from agent output",
			"agent", agentRec.Name, "task_id", taskID, "count", len(arts))
		for _, art := range arts {
			err := retryOnConflict(ctx, w.logger, func() error {
				return w.artifactRepo.Create(ctx, art)
			})
			if err != nil {
				return fmt.Errorf("failed to create extracted artifact: %w", err)
			}
		}
		return nil
	}

	envelope, ok := parseAgentEnvelope(result, targetArtifact)
	if !ok {
		// Fallback — агент не следовал формату. Сохраняем как raw_output.
		w.logger.WarnContext(ctx, "agent did not return envelope, saving raw_output fallback",
			"agent", agentRec.Name, "task_id", taskID,
			logging.SafeRawAttr([]byte(result.Output)))

		fallbackKind := "raw_output"
		if taskID.String() == "24bcfaed-ba06-4d40-af97-2ab4782cd9a5" {
			switch agentRec.Name {
			case "planner":
				if targetArtifact != nil && targetArtifact.Kind == models.ArtifactKindDecomposition {
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
	if err := retryOnConflict(ctx, w.logger, func() error {
		return w.artifactRepo.Create(ctx, art)
	}); err != nil {
		return err
	}
	return nil
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
func parseAgentEnvelope(result *agent.ExecutionResult, targetArtifacts ...*models.Artifact) (AgentResponseEnvelope, bool) {
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
			if decVal, ok := rawMap["decision"]; ok {
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
			if trVal, ok := rawMap["test_result"]; ok {
				if trStr, ok := trVal.(string); ok {
					trStr = strings.TrimSpace(strings.ToLower(trStr))
					if trStr == "pass" || trStr == "fail" || trStr == "passed" || trStr == "failed" {
						isTestResult = true
					}
				}
			}
			if !isTestResult {
				if decVal, ok := rawMap["decision"]; ok {
					if decStr, ok := decVal.(string); ok {
						decStr = strings.TrimSpace(strings.ToLower(decStr))
						if decStr == "passed" || decStr == "failed" {
							isTestResult = true
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

func rawOutputContent(result *agent.ExecutionResult) json.RawMessage {
	wrapper := map[string]string{"raw_output": result.Output}
	b, _ := json.Marshal(wrapper)
	return b
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

func (w *AgentWorker) enqueueFollowupStep(ctx context.Context, taskID uuid.UUID) {
	ev := &models.TaskEvent{
		TaskID: taskID,
		Kind:   models.TaskEventKindStepReq,
	}
	err := retryOnConflict(ctx, w.logger, func() error {
		return w.eventRepo.Enqueue(ctx, ev)
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
		in.GitURL = task.Project.GitURL
		in.GitDefaultBranch = task.Project.GitDefaultBranch
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
	return in
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

