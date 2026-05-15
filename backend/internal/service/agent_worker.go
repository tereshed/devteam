package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
	db           *gorm.DB
	eventRepo    repository.TaskEventRepository
	artifactRepo repository.ArtifactRepository
	dispatcher   AgentDispatcher
	worktreeMgr  *WorktreeManager
	notifier     *RedisNotifier // может быть nil
	logger       *slog.Logger
	cfg          AgentWorkerConfig
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

	// Загружаем агента (актуальный snapshot — system_prompt/model могли обновить).
	var agentRec models.Agent
	if err := w.db.WithContext(execCtx).Where("name = ?", payload.AgentName).First(&agentRec).Error; err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("load agent %q: %w", payload.AgentName, err))
		return
	}

	// Sandbox-агенту нужен worktree. Allocate ЗДЕСЬ (just-in-time), а не в Orchestrator.Step,
	// чтобы git-команды не оказывались внутри Step-транзакции (см. orchestrator_v2.go §Step).
	//
	// Контракт payload (формируется в orchestrator_v2.enqueueOneAgentJob для sandbox-агента):
	//   payload.Input["_base_branch"] — строка с базовой веткой проекта.
	//
	// Если WorktreeID уже задан (повторный pickup после рестарта) — переводим в in_use,
	// не аллоцируем заново.
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox && w.worktreeMgr != nil {
		if payload.WorktreeID == nil {
			wt, err := w.allocateWorktreeForJob(execCtx, ev, &payload)
			if err != nil {
				w.failEvent(parentCtx, ev, fmt.Errorf("allocate worktree: %w", err))
				return
			}
			payload.WorktreeID = &wt.ID
		}
		if err := w.worktreeMgr.MarkInUse(execCtx, *payload.WorktreeID, ev.ID); err != nil {
			w.failEvent(parentCtx, ev, fmt.Errorf("mark worktree in_use: %w", err))
			return
		}
	}

	// Загружаем task для description/title (executor их использует).
	var task models.Task
	if err := w.db.WithContext(execCtx).Where("id = ?", ev.TaskID).First(&task).Error; err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("load task %s: %w", ev.TaskID, err))
		return
	}

	executor, err := w.dispatcher.BuildExecutor(execCtx, &agentRec)
	if err != nil {
		w.failEvent(parentCtx, ev, fmt.Errorf("build executor: %w", err))
		return
	}

	in := w.buildExecutionInput(&task, &agentRec, payload.Input)

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
	if err := w.saveArtifact(parentCtx, ev.TaskID, &agentRec, result); err != nil {
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
	if err := w.eventRepo.Complete(parentCtx, ev.ID); err != nil {
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
	if err := w.eventRepo.Complete(ctx, ev.ID); err != nil {
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
	if ferr := w.eventRepo.Fail(ctx, ev.ID, truncate(err.Error(), 512), backoff); ferr != nil {
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
func (w *AgentWorker) saveArtifact(ctx context.Context, taskID uuid.UUID, agentRec *models.Agent, result *agent.ExecutionResult) error {
	envelope, ok := parseAgentEnvelope(result)
	if !ok {
		// Fallback — агент не следовал формату. Сохраняем как raw_output.
		w.logger.WarnContext(ctx, "agent did not return envelope, saving raw_output fallback",
			"agent", agentRec.Name, "task_id", taskID,
			logging.SafeRawAttr([]byte(result.Output)))
		envelope = AgentResponseEnvelope{
			Kind:    "raw_output",
			Summary: fallbackSummary(result),
			Content: rawOutputContent(result),
		}
	}
	if envelope.Summary == "" {
		envelope.Summary = fallbackSummary(result)
	}
	if !models.ValidateArtifactSummary(envelope.Summary) {
		// Усекаем (rune-based) до 500 — артефакт-validator также rune-based.
		envelope.Summary = truncateRunesForArtifact(envelope.Summary, 500)
	}

	// Если это review/changes_requested на каком-то артефакте — пометим прошлые
	// review той же кали kind+parent как superseded (новая итерация).
	if envelope.ParentArtifactID != nil && envelope.Kind == string(models.ArtifactKindReview) {
		if _, err := w.artifactRepo.SupersedePrevious(ctx, taskID, envelope.ParentArtifactID,
			models.ArtifactKindReview); err != nil {
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
	if err := w.artifactRepo.Create(ctx, art); err != nil {
		return err
	}
	return nil
}

// parseAgentEnvelope пытается распарсить ArtifactsJSON (если есть) или Output как envelope.
func parseAgentEnvelope(result *agent.ExecutionResult) (AgentResponseEnvelope, bool) {
	var env AgentResponseEnvelope
	// 1. ArtifactsJSON (если LLMAgentExecutor нашёл ```json ... ``` блок).
	if len(result.ArtifactsJSON) > 0 {
		if err := json.Unmarshal(result.ArtifactsJSON, &env); err == nil && env.Kind != "" {
			return env, true
		}
	}
	// 2. Output как голый JSON.
	if result.Output != "" {
		if err := json.Unmarshal([]byte(result.Output), &env); err == nil && env.Kind != "" {
			return env, true
		}
	}
	return AgentResponseEnvelope{}, false
}

func fallbackSummary(result *agent.ExecutionResult) string {
	if result.Summary != "" {
		return truncateRunesForArtifact(result.Summary, 500)
	}
	return truncateRunesForArtifact(result.Output, 500)
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
	if err := w.eventRepo.Enqueue(ctx, ev); err != nil {
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

func (w *AgentWorker) buildExecutionInput(task *models.Task, agentRec *models.Agent, input map[string]any) agent.ExecutionInput {
	inputJSON, _ := json.Marshal(input)

	in := agent.ExecutionInput{
		TaskID:            task.ID.String(),
		ProjectID:         task.ProjectID.String(),
		Title:             task.Title,
		Description:       task.Description,
		ContextJSON:       inputJSON,
		AgentID:           agentRec.ID.String(),
		AgentName:         agentRec.Name,
		Role:              string(agentRec.Role),
		Model:             derefString(agentRec.Model),
		PromptSystem:      derefString(agentRec.SystemPrompt),
		StructuredContext: inputJSON,
		Temperature:       agentRec.Temperature,
		MaxTokens:         agentRec.MaxTokens,
	}
	if agentRec.CodeBackend != nil {
		in.CodeBackend = string(*agentRec.CodeBackend)
	}
	return in
}
