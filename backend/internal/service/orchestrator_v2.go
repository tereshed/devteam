package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/devteam/backend/internal/logging"
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
		logger = logging.NopLogger()
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

		// 5.5. Loop detection
		isLoop, loopReason, err := o.detectCycle(ctx, tx, task.ID)
		if err != nil {
			o.logger.WarnContext(ctx, "failed to run loop detector", "task_id", task.ID, "error", err.Error())
		} else if isLoop {
			o.logger.WarnContext(ctx, "loop detected, escalating to needs_human",
				"task_id", task.ID, "reason", loopReason)
			postCommit = append(postCommit, o.scheduleWorktreeRelease(task.ID))
			return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateNeedsHuman,
				fmt.Sprintf("loop detected: %s", loopReason))
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
	var teamID uuid.UUID
	if task.TeamID != nil {
		teamID = *task.TeamID
	} else {
		var err error
		teamID, err = getProjectTeamID(tx, task.ProjectID)
		if err != nil {
			return RouterState{}, fmt.Errorf("find project team: %w", err)
		}
	}

	// Все enabled-агенты, ИМЕЮЩИЕ непустое role_description.
	// Фильтруем по (team_id = teamID OR team_id IS NULL) и дедуплицируем, предпочитая командных агентов.
	var loaded []*models.Agent
	if err := tx.WithContext(ctx).
		Where("(team_id = ? OR (team_id IS NULL AND user_id IS NULL)) AND role <> ? AND is_active = ? AND role_description IS NOT NULL AND role_description <> ''", teamID, string(models.AgentRoleAssistant), true).
		Find(&loaded).Error; err != nil {
		return RouterState{}, fmt.Errorf("load agents: %w", err)
	}

	agentsMap := make(map[string]*models.Agent)
	for _, a := range loaded {
		existing, ok := agentsMap[a.Name]
		if !ok {
			agentsMap[a.Name] = a
		} else {
			// Если существующий агент глобальный, а текущий привязан к команде — заменяем командным
			if existing.TeamID == nil && a.TeamID != nil {
				agentsMap[a.Name] = a
			}
		}
	}

	agents := make([]*models.Agent, 0, len(agentsMap))
	for _, a := range agentsMap {
		agents = append(agents, a)
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
		TeamID:    teamID,
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
	var teamID uuid.UUID
	var err error
	if task.TeamID != nil {
		teamID = *task.TeamID
	} else {
		teamID, err = getProjectTeamID(tx, task.ProjectID)
		if err != nil {
			return fmt.Errorf("find project team: %w", err)
		}
	}

	// Загружаем агента чтобы понять llm vs sandbox.
	var agentRec models.Agent
	err = tx.WithContext(ctx).Where("team_id = ? AND name = ? AND is_active = ?", teamID, req.Name, true).
		First(&agentRec).Error
	if err != nil {
		// Fallback to global agent
		if errGlobal := tx.WithContext(ctx).Where("team_id IS NULL AND name = ? AND is_active = ?", req.Name, true).
			First(&agentRec).Error; errGlobal != nil {
			return fmt.Errorf("load agent %q: %w", req.Name, err)
		}
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

// resolveBaseBranch — порядок: task.BranchName → Project.GitDefaultBranch → cfg.DefaultBaseBranch.
// Также валидирует результат через ValidateBaseBranch (защита от мусора в БД).
func (o *Orchestrator) resolveBaseBranch(ctx context.Context, tx *gorm.DB, task *models.Task) (string, error) {
	if task.BranchName != nil && *task.BranchName != "" {
		if err := ValidateBaseBranch(*task.BranchName); err == nil {
			return *task.BranchName, nil
		} else {
			o.logger.WarnContext(ctx, "task branch name invalid, falling back", "task_id", task.ID, "branch_name", *task.BranchName, "error", err.Error())
		}
	}

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

func getProjectTeamID(db *gorm.DB, projectID uuid.UUID) (uuid.UUID, error) {
	var team models.Team
	// Try development team first
	if err := db.Where("project_id = ? AND type = ?", projectID, "development").First(&team).Error; err == nil {
		return team.ID, nil
	}
	// Fallback to any team of this project
	if err := db.Where("project_id = ?", projectID).First(&team).Error; err == nil {
		return team.ID, nil
	}
	return uuid.Nil, fmt.Errorf("no team found for project %s", projectID)
}

// detectCycle checks if the agent workflow has entered a loop (e.g. infinite developer <-> reviewer updates).
// A loop is detected if:
// 1. There are at least 3 review artifacts with decision='changes_requested'.
// 2. There are at least 3 code_diff artifacts.
// 3. The list of modified files in the last 3 code_diffs is identical.
// 4. The Jaccard similarity of reviewer issues across the last 3 reviews is > 80%.
func (o *Orchestrator) detectCycle(ctx context.Context, tx *gorm.DB, taskID uuid.UUID) (bool, string, error) {
	// 1. Fetch last 3 ready/superseded reviews with decision = 'changes_requested'
	var reviews []models.Artifact
	if err := tx.WithContext(ctx).
		Where("task_id = ? AND kind = ?", taskID, models.ArtifactKindReview).
		Order("created_at DESC").
		Limit(3).
		Find(&reviews).Error; err != nil {
		return false, "", err
	}

	if len(reviews) < 3 {
		return false, "", nil
	}

	// 2. Fetch last 3 ready/superseded code_diff artifacts
	var diffs []models.Artifact
	if err := tx.WithContext(ctx).
		Where("task_id = ? AND kind = ?", taskID, models.ArtifactKindCodeDiff).
		Order("created_at DESC").
		Limit(3).
		Find(&diffs).Error; err != nil {
		return false, "", err
	}

	if len(diffs) < 3 {
		return false, "", nil
	}

	// Helper to extract reviewer comments and ensure all decisions are 'changes_requested'
	extractCommentsAndValidate := func(art models.Artifact) ([]string, bool) {
		var rc struct {
			Decision string `json:"decision"`
			Issues   []struct {
				Comment string `json:"comment"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(art.Content, &rc); err != nil {
			return nil, false
		}
		if rc.Decision != "changes_requested" {
			return nil, false
		}
		var comments []string
		for _, iss := range rc.Issues {
			if c := strings.TrimSpace(iss.Comment); c != "" {
				comments = append(comments, c)
			}
		}
		return comments, true
	}

	// Validate reviews decisions and extract comments
	var allComments [][]string
	for _, rev := range reviews {
		comments, ok := extractCommentsAndValidate(rev)
		if !ok || len(comments) == 0 {
			return false, "", nil
		}
		allComments = append(allComments, comments)
	}

	// Helper to extract changed files from diff/raw_output inside artifact content
	extractChangedFiles := func(art models.Artifact) []string {
		var wrapper struct {
			RawOutput string `json:"raw_output"`
			Diff      string `json:"diff"`
		}
		_ = json.Unmarshal(art.Content, &wrapper)
		text := wrapper.Diff
		if text == "" {
			text = wrapper.RawOutput
		}
		re := regexp.MustCompile(`diff --git a/([^\s]+) b/`)
		matches := re.FindAllStringSubmatch(text, -1)

		var files []string
		seen := make(map[string]bool)
		for _, m := range matches {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				files = append(files, m[1])
			}
		}
		return files
	}

	// Extract files and compare
	files0 := extractChangedFiles(diffs[0])
	files1 := extractChangedFiles(diffs[1])
	files2 := extractChangedFiles(diffs[2])

	equalStringSlices := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		m := make(map[string]int)
		for _, x := range a {
			m[x]++
		}
		for _, x := range b {
			m[x]--
			if m[x] < 0 {
				return false
			}
		}
		return true
	}

	filesMatch := false
	if len(files0) > 0 && len(files1) > 0 && len(files2) > 0 {
		if equalStringSlices(files0, files1) && equalStringSlices(files1, files2) {
			filesMatch = true
		}
	} else {
		// Fallback: if we couldn't parse any git files (e.g. no git output captured),
		// assume they match to rely solely on similarity of comments.
		filesMatch = true
	}

	if !filesMatch {
		return false, "", nil
	}

	// Jaccard similarity helpers
	getWords := func(s string) map[string]bool {
		words := make(map[string]bool)
		f := func(c rune) bool {
			return !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= 'А' && c <= 'Я') || (c >= 'а' && c <= 'я') || (c >= '0' && c <= '9') || c == '_')
		}
		fields := strings.FieldsFunc(strings.ToLower(s), f)
		for _, w := range fields {
			if len(w) > 2 {
				words[w] = true
			}
		}
		return words
	}

	jaccardSimilarity := func(s1, s2 string) float64 {
		w1 := getWords(s1)
		w2 := getWords(s2)
		if len(w1) == 0 && len(w2) == 0 {
			return 1.0
		}
		intersection := 0
		for w := range w1 {
			if w2[w] {
				intersection++
			}
		}
		union := len(w1)
		for w := range w2 {
			if !w1[w] {
				union++
			}
		}
		return float64(intersection) / float64(union)
	}

	str0 := strings.Join(allComments[0], " ")
	str1 := strings.Join(allComments[1], " ")
	str2 := strings.Join(allComments[2], " ")

	sim1 := jaccardSimilarity(str0, str1)
	sim2 := jaccardSimilarity(str1, str2)

	if sim1 > 0.8 && sim2 > 0.8 {
		filesStr := strings.Join(files0, ", ")
		if filesStr == "" {
			filesStr = "unknown files"
		}
		reason := fmt.Sprintf("repeated reviewer comments (similarity %.1f%% and %.1f%%) on same files [%s]", sim1*100, sim2*100, filesStr)
		return true, reason, nil
	}

	return false, "", nil
}
