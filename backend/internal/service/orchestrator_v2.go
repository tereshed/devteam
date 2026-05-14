package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// orchestrator_v2.go — Sprint 17 / Orchestration v2 — Orchestrator.Step.
//
// Атомарный шаг оркестратора: lock → router.Decide → enqueue agent_jobs / финализация.
// Вся durable-доставка работы — через task_events (см. репозиторий + воркеры).
// LLM-driven flow — через RouterService (см. router_service.go).
//
// Контракт ошибок:
//   - nil: шаг успешно завершён (агент-job'ы enqueued, или задача финализирована,
//     или задача lock'нута другим воркером — это нормально, мы молча выходим).
//   - error: инфраструктурный сбой (БД, dispatcher). Caller (StepWorker) пометит
//     соответствующий task_event как failed для retry через очередь.
//
// Транзакционность: ВСЁ — внутри одной БД-транзакции. Если что-то после
// router.Decide упадёт (например, enqueue agent_job), router_decision НЕ
// сохранится, и следующий step_req заново попросит Router'а. Это безопасно
// (idempotent), но потенциально дорого по LLM-вызовам — снижается через
// надёжный repo.

// OrchestratorConfig — настройки шага.
type OrchestratorConfig struct {
	// WorkerID — идентификатор процесса для observability (попадает в logs).
	// Обычно "hostname-pid" или подобное, выставляется в cmd/api/main.go.
	WorkerID string

	// MaxStepsPerTask — hard safety limit; при превышении задача → needs_human.
	// Из docs/orchestration-v2-plan.md §5: default 100.
	MaxStepsPerTask int

	// DefaultBaseBranch — fallback если Project.GitDefaultBranch пуст.
	// Обычно "main".
	DefaultBaseBranch string
}

// DefaultOrchestratorConfig возвращает разумные дефолты MVP.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		WorkerID:          "orchestrator-default",
		MaxStepsPerTask:   100,
		DefaultBaseBranch: "main",
	}
}

// Orchestrator — ядро оркестрации Sprint 17.
type Orchestrator struct {
	db           *gorm.DB
	artifactRepo repository.ArtifactRepository
	eventRepo    repository.TaskEventRepository
	decisionRepo repository.RouterDecisionRepository
	worktreeMgr  *WorktreeManager
	routerSvc    *RouterService
	notifier     *RedisNotifier // опционально — может быть nil в minimal-setup
	logger       *slog.Logger
	cfg          OrchestratorConfig
}

// NewOrchestrator — конструктор. logger ОБЯЗАН быть с redact-обёрткой.
func NewOrchestrator(
	db *gorm.DB,
	artifactRepo repository.ArtifactRepository,
	eventRepo repository.TaskEventRepository,
	decisionRepo repository.RouterDecisionRepository,
	worktreeMgr *WorktreeManager,
	routerSvc *RouterService,
	notifier *RedisNotifier,
	logger *slog.Logger,
	cfg OrchestratorConfig,
) *Orchestrator {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxStepsPerTask <= 0 {
		cfg.MaxStepsPerTask = 100
	}
	if cfg.DefaultBaseBranch == "" {
		cfg.DefaultBaseBranch = "main"
	}
	return &Orchestrator{
		db: db, artifactRepo: artifactRepo, eventRepo: eventRepo,
		decisionRepo: decisionRepo, worktreeMgr: worktreeMgr, routerSvc: routerSvc,
		notifier: notifier, logger: logger, cfg: cfg,
	}
}

// EnqueueInitialStep кладёт первый step_req для свежесозданной задачи.
// Вызывается POST /tasks хендлером.
func (o *Orchestrator) EnqueueInitialStep(ctx context.Context, taskID uuid.UUID) error {
	ev := &models.TaskEvent{
		TaskID: taskID,
		Kind:   models.TaskEventKindStepReq,
	}
	if err := o.eventRepo.Enqueue(ctx, ev); err != nil {
		return fmt.Errorf("enqueue initial step: %w", err)
	}
	// Best-effort Redis-NOTIFY — если упадёт, polling всё равно подхватит.
	if o.notifier != nil {
		if err := o.notifier.NotifyTaskEvent(ctx, string(models.TaskEventKindStepReq)); err != nil {
			o.logger.WarnContext(ctx, "notifier publish failed (polling will pick up)",
				"task_id", taskID, "error", err.Error())
		}
	}
	return nil
}

// Step — один шаг оркестратора для taskID. См. контракт ошибок в шапке файла.
//
// Архитектура (после ревью §2.1/§2.2):
//   - Внутри tx ТОЛЬКО БД-операции: lock, load state, save router_decision, enqueue
//     agent_jobs, increment step. БЕЗ git, БЕЗ внешнего I/O.
//   - Worktree-аллокация перенесена в AgentWorker (just-in-time перед Execute),
//     это устраняет git-команды из tx и orphaned records при rollback.
//   - Worktree-RELEASE при finalizeTask происходит ПОСЛЕ tx.Commit'а (см. ниже).
//   - Router.Decide остаётся внутри tx, т.к. сериализация per-task Step'а критична
//     для консистентности. LLM-call длится секунды (не минуты как git), приемлемо.
func (o *Orchestrator) Step(ctx context.Context, taskID uuid.UUID) error {
	if taskID == uuid.Nil {
		return fmt.Errorf("orchestrator.Step: taskID is required")
	}

	// Финализация задачи может потребовать release worktree'ев — собираем post-commit
	// callbacks внутри транзакции и выполняем их ПОСЛЕ успешного commit'а.
	// Если tx упадёт — release не выполнится; следующий Step увидит state и сделает повторно.
	var postCommit []func(context.Context)

	err := o.db.Transaction(func(tx *gorm.DB) error {
		// 1. Lock задачи через SELECT FOR UPDATE NOWAIT — сериализация per-task.
		if err := TryLockTaskForStep(ctx, tx, taskID); err != nil {
			if errors.Is(err, ErrTaskLockBusy) {
				// Другой воркер уже работает с этой задачей — это нормальная ситуация,
				// событие в очереди остаётся для следующей попытки. Возвращаем nil
				// чтобы caller'у не повторять enqueue.
				o.logger.DebugContext(ctx, "task locked by another worker, skipping",
					"task_id", taskID, "worker_id", o.cfg.WorkerID)
				return nil
			}
			if errors.Is(err, ErrTaskNotFoundForLock) {
				// Задача была удалена между enqueue и pick — событие может быть прибрано.
				return nil
			}
			return err
		}

		// 2. Загружаем задачу для чтения её state/flags. Lock уже взят выше.
		var task models.Task
		if err := tx.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("load task %s: %w", taskID, err)
		}

		// 3. Уже финализирована — выходим (могло финализироваться другим Step'ом).
		if task.State != models.TaskStateActive {
			return nil
		}

		// 4. Пользователь запросил отмену — финализируем; release worktrees + NOTIFY
		//    выполняем ПОСЛЕ commit'а (см. postCommit ниже).
		if task.CancelRequested {
			postCommit = append(postCommit, o.scheduleWorktreeRelease(taskID), o.scheduleCancelNotify(taskID))
			return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateCancelled, "user cancelled task")
		}

		// 5. Hard safety: max steps. При превышении → needs_human.
		if task.CurrentStepNo >= o.cfg.MaxStepsPerTask {
			o.logger.WarnContext(ctx, "max_steps_per_task exceeded, escalating",
				"task_id", taskID, "step_no", task.CurrentStepNo, "max", o.cfg.MaxStepsPerTask)
			postCommit = append(postCommit, o.scheduleWorktreeRelease(taskID))
			return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateNeedsHuman,
				fmt.Sprintf("max_steps_per_task=%d exceeded", o.cfg.MaxStepsPerTask))
		}

		// 6. Загружаем state для Router'а (metadata only, без content артефактов).
		state, err := o.loadRouterState(ctx, tx, &task)
		if err != nil {
			return fmt.Errorf("load router state: %w", err)
		}

		// 7. Router.Decide — может вернуть Done или массив агентов.
		// Внутри уже есть retry-пайплайн при галлюцинациях; ошибка отсюда — инфра.
		decision, err := o.routerSvc.Decide(ctx, state)
		if err != nil {
			return fmt.Errorf("router decide: %w", err)
		}

		// 8. Сохраняем router_decision (для UI/аналитики/отладки).
		if err := o.saveRouterDecision(ctx, tx, taskID, task.CurrentStepNo, &decision); err != nil {
			return fmt.Errorf("save router decision: %w", err)
		}

		// 9. Если Router сказал Done — финализируем задачу. Worktree-release —
		//    post-commit, чтобы git-операции не сидели в транзакции.
		if decision.Done {
			newState, err := mapOutcomeToTaskState(decision.Outcome)
			if err != nil {
				return err
			}
			postCommit = append(postCommit, o.scheduleWorktreeRelease(taskID))
			return o.finalizeTaskInTx(ctx, tx, &task, newState, decision.Reason)
		}

		// 10. Fan-out: enqueue'им agent_jobs. Worktree-allocation НЕ делаем здесь
		//    (вынесено в AgentWorker just-in-time перед Execute), чтобы git-команды
		//    не оказывались внутри tx и orphaned-records не накапливались при rollback.
		if err := o.enqueueAgentJobs(ctx, tx, &task, &decision); err != nil {
			return err
		}

		// 11. Инкремент step_no.
		if err := tx.Model(&models.Task{}).Where("id = ?", taskID).
			Update("current_step_no", task.CurrentStepNo+1).Error; err != nil {
			return fmt.Errorf("increment step_no: %w", err)
		}

		o.logger.InfoContext(ctx, "orchestrator step completed",
			"task_id", taskID,
			"step_no", task.CurrentStepNo,
			"fan_out", len(decision.Agents),
		)
		return nil
	})

	// Post-commit hooks (worktree-release и др. внешнее I/O). Если tx упал — pending
	// hooks НЕ выполняем: следующий Step увидит state задачи и сделает повторно.
	if err != nil {
		return err
	}
	for _, hook := range postCommit {
		hook(ctx)
	}
	return nil
}

// scheduleWorktreeRelease возвращает post-commit callback для освобождения всех
// worktree'ев задачи. Вызывается ВНЕ tx-callback'а внутри Step после успешного commit'а.
// Если worktreeMgr=nil (мини-конфиг для тестов), возвращает no-op.
func (o *Orchestrator) scheduleWorktreeRelease(taskID uuid.UUID) func(context.Context) {
	if o.worktreeMgr == nil {
		return func(context.Context) {}
	}
	return func(ctx context.Context) {
		o.releaseAllWorktreesForTask(ctx, taskID)
	}
}

// scheduleCancelNotify — post-commit callback для Redis-сигнала отмены. Будит in-flight
// AgentWorker'ов, которые слушают канал task_cancel:<id>. Если notifier=nil — no-op.
//
// Важно отправлять ПОСЛЕ commit'а: иначе подписчики могли бы увидеть NOTIFY раньше
// чем state=cancelled в БД и среагировать на неконсистентное состояние.
func (o *Orchestrator) scheduleCancelNotify(taskID uuid.UUID) func(context.Context) {
	if o.notifier == nil {
		return func(context.Context) {}
	}
	return func(ctx context.Context) {
		if err := o.notifier.NotifyTaskCancel(ctx, taskID); err != nil {
			o.logger.WarnContext(ctx, "post-commit cancel NOTIFY failed",
				"task_id", taskID, "error", err.Error())
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// State loading
// ─────────────────────────────────────────────────────────────────────────────

func (o *Orchestrator) loadRouterState(ctx context.Context, tx *gorm.DB, task *models.Task) (RouterState, error) {
	// Все enabled-агенты (Router их видит в реестре).
	var agents []*models.Agent
	if err := tx.WithContext(ctx).Where("is_active = ?", true).Find(&agents).Error; err != nil {
		return RouterState{}, fmt.Errorf("load agents: %w", err)
	}

	// Артефакты — только metadata, только status=ready (Router их видит в истории).
	artifacts, err := o.artifactRepo.ListMetadataByTaskID(ctx, task.ID, true)
	if err != nil {
		return RouterState{}, fmt.Errorf("load artifacts metadata: %w", err)
	}

	// In-flight: pending agent_jobs текущей задачи. Router'у важно знать что уже
	// запущено, чтобы не дублировать.
	allPending, err := o.eventRepo.ListPendingByTaskID(ctx, task.ID)
	if err != nil {
		return RouterState{}, fmt.Errorf("load in-flight events: %w", err)
	}
	inflight := make([]models.TaskEvent, 0, len(allPending))
	for _, ev := range allPending {
		if ev.Kind == models.TaskEventKindAgentJob {
			inflight = append(inflight, ev)
		}
	}

	return RouterState{
		Task:      task,
		Agents:    agents,
		Artifacts: artifacts,
		InFlight:  inflight,
		StepNo:    task.CurrentStepNo,
		MaxSteps:  o.cfg.MaxStepsPerTask,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Decision persistence
// ─────────────────────────────────────────────────────────────────────────────

func (o *Orchestrator) saveRouterDecision(ctx context.Context, tx *gorm.DB, taskID uuid.UUID, stepNo int, d *Decision) error {
	chosen := make(pq.StringArray, 0, len(d.Agents))
	for _, a := range d.Agents {
		chosen = append(chosen, a.Name)
	}

	rd := &models.RouterDecision{
		ID:           uuid.New(),
		TaskID:       taskID,
		StepNo:       stepNo,
		ChosenAgents: chosen,
		Reason:       d.Reason,
	}
	if d.Done {
		outcome := d.Outcome
		rd.Outcome = &outcome
	}
	// encrypted_raw_response пока не сохраняем — потребует Encryptor через DI,
	// добавляется в Sprint 4 (см. docs/orchestration-v2-plan.md §2.5).

	// Используем tx.Create вместо decisionRepo.Create, чтобы запись попала в общую
	// транзакцию Step'а. Repo используется в cron-job'ах вне транзакций.
	if err := tx.WithContext(ctx).Create(rd).Error; err != nil {
		return fmt.Errorf("create router_decision: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Fan-out: enqueue agent_job events (with worktree alloc for sandbox)
// ─────────────────────────────────────────────────────────────────────────────

func (o *Orchestrator) enqueueAgentJobs(ctx context.Context, tx *gorm.DB, task *models.Task, d *Decision) error {
	// base_branch резолвим один раз и кладём в payload каждого job'а; AgentWorker
	// использует его для git worktree add (just-in-time allocation).
	baseBranch, err := o.resolveBaseBranch(ctx, tx, task)
	if err != nil {
		return fmt.Errorf("resolve base branch: %w", err)
	}

	for i := range d.Agents {
		agentReq := &d.Agents[i]
		if err := o.enqueueOneAgentJob(ctx, tx, task, agentReq, baseBranch); err != nil {
			return fmt.Errorf("enqueue agent[%d]=%s: %w", i, agentReq.Name, err)
		}
	}

	// NOTIFY — best-effort. Если упадёт, polling всё равно подберёт jobs.
	if o.notifier != nil {
		if err := o.notifier.NotifyTaskEvent(ctx, string(models.TaskEventKindAgentJob)); err != nil {
			o.logger.WarnContext(ctx, "agent_job NOTIFY failed (polling will pick up)",
				"task_id", task.ID, "error", err.Error())
		}
	}
	return nil
}

// enqueueOneAgentJob — ТОЛЬКО БД-операции. Worktree-аллокация здесь НЕ происходит:
// AgentWorker сам аллоцирует worktree just-in-time перед Execute, чтобы:
//  1. Не держать git-команды внутри Step-транзакции (избежать tx connection pool exhaustion).
//  2. Не создавать orphaned worktree-записи + worktree-каталоги на диске при rollback tx.
//
// Для sandbox-агента в payload кладётся base_branch — AgentWorker по нему аллоцирует.
func (o *Orchestrator) enqueueOneAgentJob(ctx context.Context, tx *gorm.DB, task *models.Task, req *AgentRequest, baseBranch string) error {
	// Загружаем агента чтобы понять llm vs sandbox.
	var agentRec models.Agent
	if err := tx.WithContext(ctx).Where("name = ? AND is_active = ?", req.Name, true).
		First(&agentRec).Error; err != nil {
		return fmt.Errorf("load agent %q: %w", req.Name, err)
	}

	payload := models.AgentJobPayload{
		AgentName: req.Name,
		Input:     req.Input,
	}
	// Для sandbox-агента кладём base_branch в Input — AgentWorker распакует и аллоцирует
	// worktree до запуска Execute. WorktreeID НЕ заполняем здесь — это сделает AgentWorker
	// после успешной allocation, обновив payload в payload-helpers (или передав worktree_id
	// в ExecutionInput напрямую).
	if agentRec.ExecutionKind == models.AgentExecutionKindSandbox {
		if payload.Input == nil {
			payload.Input = map[string]any{}
		}
		payload.Input["_base_branch"] = baseBranch
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	ev := &models.TaskEvent{
		TaskID:  task.ID,
		Kind:    models.TaskEventKindAgentJob,
		Payload: payloadJSON,
	}
	if err := tx.WithContext(ctx).Create(ev).Error; err != nil {
		return fmt.Errorf("create task_event: %w", err)
	}
	return nil
}

// resolveBaseBranch — порядок: Project.GitDefaultBranch → cfg.DefaultBaseBranch.
// Также валидирует результат через ValidateBaseBranch (защита от мусора в БД).
func (o *Orchestrator) resolveBaseBranch(ctx context.Context, tx *gorm.DB, task *models.Task) (string, error) {
	var project models.Project
	if err := tx.WithContext(ctx).Where("id = ?", task.ProjectID).First(&project).Error; err != nil {
		o.logger.WarnContext(ctx, "load project failed, falling back to cfg default branch",
			"task_id", task.ID, "project_id", task.ProjectID, "error", err.Error())
		return o.cfg.DefaultBaseBranch, ValidateBaseBranch(o.cfg.DefaultBaseBranch)
	}

	candidate := project.GitDefaultBranch
	if candidate == "" {
		candidate = o.cfg.DefaultBaseBranch
	}
	if err := ValidateBaseBranch(candidate); err != nil {
		return "", fmt.Errorf("invalid base branch in project/config: %w", err)
	}
	return candidate, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Финализация задачи
// ─────────────────────────────────────────────────────────────────────────────

// finalizeTaskInTx — ТОЛЬКО БД-операции: обновление state/completed_at/error_message.
// Worktree-release и Redis NOTIFY вызываются ОТДЕЛЬНО (post-commit) через
// scheduleWorktreeRelease + scheduleCancelNotify в Step.
//
// Это устраняет два анти-паттерна:
//   - git worktree remove внутри tx (длинное I/O блокирует connection pool)
//   - NOTIFY до commit'а (подписчики могли бы среагировать на ещё не зафиксированный state)
func (o *Orchestrator) finalizeTaskInTx(ctx context.Context, tx *gorm.DB, task *models.Task, newState models.TaskState, reason string) error {
	now := time.Now()
	updates := map[string]any{
		"state":      newState,
		"updated_at": now,
	}
	if newState == models.TaskStateDone || newState == models.TaskStateFailed ||
		newState == models.TaskStateCancelled {
		updates["completed_at"] = now
	}
	if reason != "" {
		updates["error_message"] = reason
	}

	if err := tx.WithContext(ctx).Model(&models.Task{}).
		Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update task state to %s: %w", newState, err)
	}

	o.logger.InfoContext(ctx, "task finalized (post-commit cleanup pending)",
		"task_id", task.ID, "new_state", newState, "reason", reason)
	return nil
}

func (o *Orchestrator) releaseAllWorktreesForTask(ctx context.Context, taskID uuid.UUID) {
	wts, err := o.worktreeMgr.ListByTaskID(ctx, taskID)
	if err != nil {
		o.logger.WarnContext(ctx, "list worktrees for release failed",
			"task_id", taskID, "error", err.Error())
		return
	}
	for _, wt := range wts {
		if wt.State == models.WorktreeStateReleased {
			continue
		}
		if err := o.worktreeMgr.Release(ctx, wt.ID); err != nil {
			o.logger.WarnContext(ctx, "release worktree failed",
				"task_id", taskID, "worktree_id", wt.ID, "error", err.Error())
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// mapOutcomeToTaskState маппит RouterDecisionOutcome → models.TaskState.
// Контракт: только один источник правды (этот helper). Если в БД появится новый
// outcome (router_decisions.outcome CHECK) — нужно явно добавить case.
func mapOutcomeToTaskState(o models.RouterDecisionOutcome) (models.TaskState, error) {
	switch o {
	case models.RouterDecisionOutcomeDone:
		return models.TaskStateDone, nil
	case models.RouterDecisionOutcomeFailed:
		return models.TaskStateFailed, nil
	case models.RouterDecisionOutcomeCancelled:
		return models.TaskStateCancelled, nil
	case models.RouterDecisionOutcomeNeedsHuman:
		return models.TaskStateNeedsHuman, nil
	default:
		return "", fmt.Errorf("orchestrator: unknown router outcome %q", o)
	}
}
