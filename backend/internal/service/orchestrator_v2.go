package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// Снижен со 100 до 40: на практике 100 шагов недостижимо для здоровой задачи, а как
	// предохранитель 100 слишком высок — задача 1.1 крутилась 37 шагов без сходимости и
	// уперлась бы в лимит только через час+. 40 покрывает декомпозицию на ~7-10 подзадач
	// с ревью и парой итераций, но ловит зацикливание заметно раньше.
	MaxStepsPerTask int

	// MaxDeadJobsPerTask — сколько agent_job'ов задачи могут окончательно «умереть»
	// (исчерпать retry) до того, как Orchestrator детерминированно эскалирует задачу в
	// needs_human, не вызывая Router. Защищает от петли «sandbox падает по OOM → Router
	// переназначает того же агента → снова OOM». 0/отрицательное — выключено.
	MaxDeadJobsPerTask int

	// MaxSameAgentRepeats — сколько подряд идущих Router-решений могут выбрать ОДИН И ТОТ ЖЕ
	// непустой набор агентов до того, как Orchestrator детерминированно эскалирует задачу в
	// needs_human, не вызывая Router. Предохранитель против петли «Router переназначает того
	// же агента, тот снова кладёт raw_output, задача не сходится» (см. инцидент SupportAggent:
	// router на каждом шаге считал задачу «только созданной» и снова звал support). Ни один
	// здоровый flow не выбирает идентичный одиночный набор агентов N раз подряд (developer↔
	// reviewer чередуются, параллельные подзадачи идут одним решением). 0/отрицательное —
	// выключено. Основная защита — память Router'а о прошлых решениях (RecentDecisions в
	// промпте); этот backstop — последний рубеж, если LLM игнорирует историю.
	MaxSameAgentRepeats int

	// DefaultBaseBranch — fallback если Project.GitDefaultBranch пуст.
	// Обычно "main".
	DefaultBaseBranch string

	// CIGate (Sprint 22) — после открытия MR оркестратор дожидается вердикта CI-пайплайна
	// этого MR и при красном пайплайне переводит done→needs_human (со ссылкой на упавший
	// джоб). Ловит ложный passed тестера (sandbox не воспроизводит весь проектный CI).
	// CIGateEnabled=false → шаг пропускается (прежнее поведение: done сразу после открытия PR).
	CIGateEnabled      bool
	CIGatePollInterval time.Duration // период опроса статуса пайплайна
	CIGateTimeout      time.Duration // потолок ожидания терминального статуса
}

// DefaultOrchestratorConfig возвращает разумные дефолты MVP.
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		WorkerID:            "orchestrator-default",
		MaxStepsPerTask:     40,
		MaxDeadJobsPerTask:  3,
		MaxSameAgentRepeats: 4,
		DefaultBaseBranch:   "main",
		CIGateEnabled:       true,
		CIGatePollInterval:  30 * time.Second,
		CIGateTimeout:       25 * time.Minute,
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
	notifier     *RedisNotifier  // опционально — может быть nil в minimal-setup
	bus          events.EventBus // опционально (nil в тестах) — live-апдейты UI через HubBridge
	logger       *slog.Logger
	cfg          OrchestratorConfig
	// prPublisher — опционально. Если задан, на done открывается PR (ground-truth-гейт):
	// PR открылся → done достоверен; не открылся → задача уходит в needs_human (см.
	// schedulePullRequestPublish). nil → прежнее поведение (без PR, без гейта).
	prPublisher PullRequestPublisher
}

// SetPullRequestPublisher включает ground-truth-гейт завершения: при done оркестратор
// post-commit открывает PR с веткой задачи. Если PR открыть нельзя (нет git-credential у
// проекта, ветка не на remote, ошибка провайдера) — задача переводится в needs_human, т.к.
// "done" без приземлённого в репозиторий изменения недостоверен. Вызывать один раз при сборке.
func (o *Orchestrator) SetPullRequestPublisher(p PullRequestPublisher) {
	o.prPublisher = p
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
	bus events.EventBus,
	logger *slog.Logger,
	cfg OrchestratorConfig,
) *Orchestrator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	if cfg.MaxStepsPerTask <= 0 {
		cfg.MaxStepsPerTask = 40
	}
	if cfg.MaxDeadJobsPerTask < 0 {
		cfg.MaxDeadJobsPerTask = 0
	}
	if cfg.MaxSameAgentRepeats < 0 {
		cfg.MaxSameAgentRepeats = 0
	}
	if cfg.DefaultBaseBranch == "" {
		cfg.DefaultBaseBranch = "main"
	}
	return &Orchestrator{
		db: db, artifactRepo: artifactRepo, eventRepo: eventRepo,
		decisionRepo: decisionRepo, worktreeMgr: worktreeMgr, routerSvc: routerSvc,
		notifier: notifier, bus: bus, logger: logger, cfg: cfg,
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

		// 6.6. Dead-jobs backstop. Детерминированный предохранитель: если слишком много
		// agent_job'ов окончательно умерло (OOM/timeout/crash), нет смысла тратить Router-LLM
		// вызов — почти наверняка он либо переназначит того же агента (снова падение), либо
		// сам эскалирует. Эскалируем в needs_human напрямую. Защищает от петли, которая в
		// задаче 1.1 раздула прогон до 37 шагов.
		if o.cfg.MaxDeadJobsPerTask > 0 && len(state.DeadJobs) >= o.cfg.MaxDeadJobsPerTask {
			o.logger.WarnContext(ctx, "too many dead agent_jobs, escalating to needs_human",
				"task_id", task.ID, "dead_jobs", len(state.DeadJobs), "max", o.cfg.MaxDeadJobsPerTask)
			postCommit = append(postCommit, o.scheduleWorktreeRelease(task.ID))
			return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateNeedsHuman,
				fmt.Sprintf("%d agent jobs exhausted retries (likely sandbox OOM/timeout); human inspection required", len(state.DeadJobs)))
		}

		// 6.7. Repeated-dispatch backstop. Если Router N раз подряд выбрал ОДИН И ТОТ ЖЕ
		// непустой набор агентов — задача не сходится (Router зациклился, переназначая того же
		// агента). Эскалируем в needs_human, не вызывая LLM. Последний рубеж: основную защиту
		// даёт память Router'а (RecentDecisions в промпте), но если LLM её игнорирует — ловим
		// здесь. См. инцидент SupportAggent (router на каждом шаге считал задачу «только
		// созданной»). Использует уже загруженный state.RecentDecisions.
		if reps, agentSet := repeatedDispatchRun(state.RecentDecisions); o.cfg.MaxSameAgentRepeats > 0 && reps >= o.cfg.MaxSameAgentRepeats {
			o.logger.WarnContext(ctx, "same agent set re-dispatched too many times, escalating to needs_human",
				"task_id", task.ID, "agents", agentSet, "repeats", reps, "max", o.cfg.MaxSameAgentRepeats)
			postCommit = append(postCommit, o.scheduleWorktreeRelease(task.ID))
			return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateNeedsHuman,
				fmt.Sprintf("router re-dispatched the same agent(s) [%s] %d times without convergence; human inspection required", agentSet, reps))
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

		// 8.5. Live-апдейт UI: публикуем RouterDecisionCreated ПОСЛЕ commit'а (в postCommit),
		// чтобы не пушить решение, которое откатится при rollback. Без этого Router-таймлайн
		// и execution-граф не обновляются до ручного рефреша. Захватываем значения в копии —
		// task мутируется ниже (step_no++).
		eventProjectID, eventStepNo, eventDecision := task.ProjectID, task.CurrentStepNo, decision
		postCommit = append(postCommit, func(ctx context.Context) {
			o.publishRouterDecision(ctx, eventProjectID, taskID, eventStepNo, &eventDecision)
		})

		// 9. Если Router сказал Done — финализируем задачу. Worktree-release —
		//    post-commit, чтобы git-операции не сидели в транзакции.
		if decision.Done {
			newState, err := mapOutcomeToTaskState(decision.Outcome)
			if err != nil {
				return err
			}
			postCommit = append(postCommit, o.scheduleWorktreeRelease(taskID))
			// Ground-truth-гейт: для done открываем PR post-commit. Если PR открыть не удастся —
			// schedulePullRequestPublish переведёт задачу в needs_human (изменения не приземлены).
			// nil-publisher → хук no-op (прежнее поведение).
			if newState == models.TaskStateDone {
				postCommit = append(postCommit, o.schedulePullRequestPublish(taskID))
			}
			return o.finalizeTaskInTx(ctx, tx, &task, newState, decision.Reason)
		}

		// 10. Fan-out: enqueue'им agent_jobs. Worktree-allocation НЕ делаем здесь
		//    (вынесено в AgentWorker just-in-time перед Execute), чтобы git-команды
		//    не оказывались внутри tx и orphaned-records не накапливались при rollback.
		if err := o.enqueueAgentJobs(ctx, tx, &task, &decision); err != nil {
			return err
		}

		// 11. Инкремент step_no — ВСЕГДА (по одному на каждый вызов Router'а, включая
		//    «ожидание» с пустым fan-out). step_no обязан быть уникальным и монотонным по
		//    времени: и router_decisions.step_no — ключ ноды в execution-графе фронта, и
		//    окна привязки артефактов строятся по порядку step_no. Если не инкрементить на
		//    ожиданиях, появляются строки с одинаковым step_no, порядок по времени ломается,
		//    окна перекрываются и один артефакт рендерится на двух нодах (баг UI). Цена —
		//    ожидания тоже тратят бюджет MaxStepsPerTask, но это некритично: реальную защиту
		//    от петель дают dead-job backstop и фикс пустого вывода, а не счётчик шагов.
		if err := tx.Model(&models.Task{}).Where("id = ?", taskID).
			Update("current_step_no", task.CurrentStepNo+1).Error; err != nil {
			return fmt.Errorf("increment step_no: %w", err)
		}

		o.logger.InfoContext(ctx, "orchestrator step completed",
			"task_id", taskID,
			"step_no", task.CurrentStepNo,
			"fan_out", len(decision.Agents),
			"waiting", len(decision.Agents) == 0,
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

// schedulePullRequestPublish — post-commit ground-truth-гейт завершения задачи.
//
// Зачем: пайплайн определяет done по самоотчётам LLM-агентов (developer/reviewer/tester
// пишут «готово/approved/passed»). Без независимой проверки агент может отчитаться об успехе,
// не запушив результат — задача станет done, а в репозитории ничего не приземлится.
// Хук пытается открыть PR из ветки задачи: открылся → done достоверен, ссылку кладём в result;
// не открылся (нет git-credential у проекта, ветка не на remote, ошибка провайдера) →
// переводим задачу в needs_human с честной причиной. nil-publisher → no-op (прежнее поведение).
func (o *Orchestrator) schedulePullRequestPublish(taskID uuid.UUID) func(context.Context) {
	return func(ctx context.Context) {
		if o.prPublisher == nil {
			return
		}
		var task models.Task
		if err := o.db.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
			o.logger.ErrorContext(ctx, "pr-gate: load task failed", "task_id", taskID, "error", err)
			return
		}
		var project models.Project
		if err := o.db.WithContext(ctx).
			Preload("GitCredential").
			Preload("Repositories", func(db *gorm.DB) *gorm.DB {
				return db.Order("sort_order ASC, created_at ASC")
			}).
			Preload("Repositories.GitCredential").
			First(&project, "id = ?", task.ProjectID).Error; err != nil {
			o.logger.ErrorContext(ctx, "pr-gate: load project failed", "task_id", taskID, "error", err)
			return
		}

		// Гейт применяем ТОЛЬКО к задачам, реально менявшим код (есть code_diff). Review-/
		// planning-only задачи кода не трогают → ветки с коммитами нет, PR-ить нечего, и
		// done без PR для них корректен. Иначе CreatePullRequest падает («no commits between
		// base and head») и задача ошибочно уходит в needs_human.
		if !o.taskHasCodeChanges(ctx, taskID) {
			o.logger.InfoContext(ctx, "pr-gate: no code_diff — task done without PR (review/planning-only)", "task_id", taskID)
			return
		}

		// Мульти-репо: определяем затронутые репозитории по code_diff/merged_code артефактам.
		// Пусто → одно-репо/legacy путь (один PR по полям проекта, прежнее поведение).
		touched := o.resolveTouchedRepos(ctx, taskID, &project)
		if len(touched) == 0 {
			o.publishSingleRepoPR(ctx, taskID, &task, &project)
			return
		}
		o.publishMultiRepoPRs(ctx, taskID, &task, &project, touched)
	}
}

// publishSingleRepoPR — прежнее одно-репо поведение гейта (один PR по полям проекта).
func (o *Orchestrator) publishSingleRepoPR(ctx context.Context, taskID uuid.UUID, task *models.Task, project *models.Project) {
	pr, err := o.prPublisher.Publish(ctx, task, project)
	if err != nil {
		branch := ""
		if task.BranchName != nil {
			branch = *task.BranchName
		}
		// MR уже открыт (идемпотентный повтор done-гейта) — не провал: переходим к CI-gate
		// и гейтим пайплайн ветки.
		if errors.Is(err, gitprovider.ErrConflict) {
			o.logger.InfoContext(ctx, "pr-gate: MR already exists — proceeding to CI-gate", "task_id", taskID)
			o.startCIGate(project, taskID, []ciGateTarget{{branch: branch, slug: "project"}})
			return
		}
		reason := fmt.Sprintf(
			"Задача не завершена: не удалось открыть PR — изменения не приземлены в репозиторий (%v). "+
				"Привяжите git-credential к проекту и убедитесь, что ветка %q запушена в remote.",
			err, branch,
		)
		o.logger.WarnContext(ctx, "pr-gate: PR not opened → downgrading done→needs_human",
			"task_id", taskID, "error", err)
		o.downgradeToNeedsHuman(ctx, taskID, reason)
		return
	}

	o.logger.InfoContext(ctx, "pr-gate: PR opened for completed task",
		"task_id", taskID, "pr_number", pr.Number, "pr_url", pr.HTMLURL)
	if uerr := o.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).
		Update("result", fmt.Sprintf("PR #%d: %s", pr.Number, pr.HTMLURL)).Error; uerr != nil {
		o.logger.WarnContext(ctx, "pr-gate: store PR url failed", "task_id", taskID, "error", uerr)
	}

	// CI-gate (Sprint 22): дождаться вердикта пайплайна MR; красный → done→needs_human.
	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}
	o.startCIGate(project, taskID, []ciGateTarget{{branch: branch, prURL: pr.HTMLURL, slug: "project"}})
}

// publishMultiRepoPRs открывает по PR на каждый затронутый репозиторий. Задача done только
// если по всем затронутым репо PR успешно открыт (либо репо local/без PR — тогда пропуск
// не считается провалом). Любая реальная ошибка → needs_human. Открытые PR персистятся в
// task_pull_requests, агрегированный список — в task.result.
func (o *Orchestrator) publishMultiRepoPRs(ctx context.Context, taskID uuid.UUID, task *models.Task, project *models.Project, repos []*models.ProjectRepository) {
	type opened struct {
		slug   string
		number int
		url    string
	}
	var ok []opened
	var failures []string
	var ciTargets []ciGateTarget
	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}

	for _, repo := range repos {
		pr, err := o.prPublisher.PublishForRepo(ctx, task, project, repo)
		if err != nil {
			// Пропуск (local-провайдер / нечего пушить) не считаем провалом гейта —
			// для такого репо PR не требуется.
			if errors.Is(err, ErrPullRequestSkipped) {
				o.logger.InfoContext(ctx, "pr-gate: repo skipped (no PR needed)",
					"task_id", taskID, "repo_slug", repo.Slug, "reason", err.Error())
				continue
			}
			// MR уже открыт (идемпотентный повтор) — не провал; гейтим пайплайн ветки этого репо.
			if errors.Is(err, gitprovider.ErrConflict) {
				o.logger.InfoContext(ctx, "pr-gate: MR already exists — will CI-gate",
					"task_id", taskID, "repo_slug", repo.Slug)
				ciTargets = append(ciTargets, ciGateTarget{repo: repo, branch: branch, slug: repo.Slug})
				continue
			}
			failures = append(failures, fmt.Sprintf("%s: %v", repo.Slug, err))
			continue
		}
		ok = append(ok, opened{slug: repo.Slug, number: pr.Number, url: pr.HTMLURL})
		ciTargets = append(ciTargets, ciGateTarget{repo: repo, branch: branch, prURL: pr.HTMLURL, slug: repo.Slug})
		tpr := &models.TaskPullRequest{TaskID: taskID, RepoSlug: repo.Slug, PRNumber: pr.Number, PRURL: pr.HTMLURL}
		if uerr := o.db.WithContext(ctx).
			Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "task_id"}, {Name: "repo_slug"}}, DoNothing: true}).
			Create(tpr).Error; uerr != nil {
			o.logger.WarnContext(ctx, "pr-gate: persist task_pull_request failed",
				"task_id", taskID, "repo_slug", repo.Slug, "error", uerr)
		}
	}

	if len(failures) > 0 {
		reason := fmt.Sprintf(
			"Задача не завершена: не удалось открыть PR в части репозиториев — изменения не приземлены (%s). "+
				"Привяжите git-credential к этим репозиториям и убедитесь, что ветка запушена в remote.",
			strings.Join(failures, "; "),
		)
		o.logger.WarnContext(ctx, "pr-gate: multi-repo PR failures → downgrading done→needs_human",
			"task_id", taskID, "failures", strings.Join(failures, "; "))
		o.downgradeToNeedsHuman(ctx, taskID, reason)
		return
	}

	if len(ok) > 0 {
		parts := make([]string, 0, len(ok))
		for _, r := range ok {
			parts = append(parts, fmt.Sprintf("%s: PR #%d %s", r.slug, r.number, r.url))
		}
		result := strings.Join(parts, "\n")
		o.logger.InfoContext(ctx, "pr-gate: multi-repo PRs opened", "task_id", taskID, "count", len(ok))
		if uerr := o.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).
			Update("result", result).Error; uerr != nil {
			o.logger.WarnContext(ctx, "pr-gate: store PR urls failed", "task_id", taskID, "error", uerr)
		}
	} else {
		o.logger.InfoContext(ctx, "pr-gate: no new PRs opened (all skipped or already exist)", "task_id", taskID)
	}

	// CI-gate (Sprint 22): дождаться вердиктов пайплайнов открытых/существующих MR; любой красный → needs_human.
	o.startCIGate(project, taskID, ciTargets)
}

// downgradeToNeedsHuman переводит задачу done→needs_human с честной причиной.
func (o *Orchestrator) downgradeToNeedsHuman(ctx context.Context, taskID uuid.UUID, reason string) {
	if uerr := o.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"state":         models.TaskStateNeedsHuman,
			"error_message": reason,
			"updated_at":    time.Now(),
		}).Error; uerr != nil {
		o.logger.ErrorContext(ctx, "pr-gate: downgrade update failed", "task_id", taskID, "error", uerr)
	}
}

// resolveTouchedRepos возвращает репозитории проекта, затронутые задачей (по code_diff/
// merged_code артефактам и их repo_slug). Пусто, если у проекта нет репо-реестра (legacy)
// либо нет code-артефактов. code_diff без repo_slug относим к primary-репо.
func (o *Orchestrator) resolveTouchedRepos(ctx context.Context, taskID uuid.UUID, project *models.Project) []*models.ProjectRepository {
	if project == nil || len(project.Repositories) == 0 {
		return nil
	}
	arts, err := o.artifactRepo.ListByTaskID(ctx, taskID, false)
	if err != nil {
		o.logger.WarnContext(ctx, "pr-gate: list artifacts failed for touched-repo resolution", "task_id", taskID, "error", err)
		return nil
	}
	byID := make(map[uuid.UUID]*models.Artifact, len(arts))
	for i := range arts {
		byID[arts[i].ID] = &arts[i]
	}
	slugSet := make(map[string]bool)
	hasCode := false
	for i := range arts {
		a := &arts[i]
		if a.Kind != models.ArtifactKindCodeDiff && a.Kind != models.ArtifactKindMergedCode {
			continue
		}
		hasCode = true
		slug := resolveSlugFromArtifact(a, byID)
		if slug == "" {
			if pr := project.PrimaryRepo(); pr != nil {
				slug = pr.Slug
			}
		}
		if slug != "" {
			slugSet[slug] = true
		}
	}
	if !hasCode {
		return nil
	}
	repos := make([]*models.ProjectRepository, 0, len(slugSet))
	for slug := range slugSet {
		if r := project.RepoBySlug(slug); r != nil {
			repos = append(repos, r)
		}
	}
	return repos
}

// resolveSlugFromArtifact ищет repo_slug в content артефакта, поднимаясь по цепочке
// parent_id (code_diff → subtask_description, где repo_slug проставлен decomposer'ом).
func resolveSlugFromArtifact(a *models.Artifact, byID map[uuid.UUID]*models.Artifact) string {
	cur := a
	for depth := 0; depth < 6 && cur != nil; depth++ {
		if s := extractRepoSlug(cur.Content); s != "" {
			return s
		}
		if cur.ParentID == nil {
			return ""
		}
		cur = byID[*cur.ParentID]
	}
	return ""
}

// taskHasCodeChanges — были ли у задачи реальные изменения кода (артефакт code_diff).
// Используется PR-гейтом: PR требуется только для задач, менявших код. При ошибке чтения
// артефактов возвращаем true (консервативно: лучше потребовать PR, чем по-тихому
// объявить done незалендженный код).
func (o *Orchestrator) taskHasCodeChanges(ctx context.Context, taskID uuid.UUID) bool {
	arts, err := o.artifactRepo.ListMetadataByTaskID(ctx, taskID, false)
	if err != nil {
		o.logger.WarnContext(ctx, "pr-gate: list artifacts failed, assuming code changes", "task_id", taskID, "error", err)
		return true
	}
	for _, a := range arts {
		if a.Kind == models.ArtifactKindCodeDiff {
			return true
		}
	}
	return false
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

	// Dead jobs: agent_job'ы, исчерпавшие retry (OOM/timeout/crash). Router должен их
	// видеть, чтобы эскалировать вместо вечного переназначения (см. router prompt).
	deadJobs, err := o.eventRepo.ListDeadByTaskID(ctx, task.ID)
	if err != nil {
		return RouterState{}, fmt.Errorf("load dead jobs: %w", err)
	}

	// Репозитории проекта (мульти-репо) — для секции `# Repositories` в промпте Router'а.
	var repos []models.ProjectRepository
	if err := tx.WithContext(ctx).
		Where("project_id = ?", task.ProjectID).
		Order("sort_order ASC, created_at ASC").
		Find(&repos).Error; err != nil {
		return RouterState{}, fmt.Errorf("load project repositories: %w", err)
	}

	// Владелец проекта — его per-user ключи LLM (user_llm_credentials)
	// приоритетнее env при вызове Router-LLM.
	var owner struct{ UserID uuid.UUID }
	if err := tx.WithContext(ctx).
		Model(&models.Project{}).
		Select("user_id").
		Where("id = ?", task.ProjectID).
		Take(&owner).Error; err != nil {
		return RouterState{}, fmt.Errorf("load project owner: %w", err)
	}

	// Недавние Router-решения — память о прошлых шагах (кого уже запускали). Грузим последние
	// recentDecisionsLimit (DESC), затем разворачиваем в ASC для хронологичного рендера. Тот же
	// срез использует backstop повторных назначений (см. detectRepeatedDispatch).
	var recent []models.RouterDecision
	if err := tx.WithContext(ctx).
		Where("task_id = ?", task.ID).
		Order("step_no DESC").
		Limit(recentDecisionsLimit).
		Find(&recent).Error; err != nil {
		return RouterState{}, fmt.Errorf("load recent router decisions: %w", err)
	}
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	return RouterState{
		Task:            task,
		TeamID:          teamID,
		Agents:          agents,
		Artifacts:       artifacts,
		InFlight:        inflight,
		DeadJobs:        deadJobs,
		StepNo:          task.CurrentStepNo,
		MaxSteps:        o.cfg.MaxStepsPerTask,
		Repositories:    repos,
		RecentDecisions: recent,
		OwnerUserID:     owner.UserID.String(),
	}, nil
}

// recentDecisionsLimit — сколько последних Router-решений подтягивать в RouterState для
// памяти Router'а (секция `# Recent routing history`) и для backstop'а повторных назначений.
// Достаточно, чтобы покрыть и историю для LLM, и окно MaxSameAgentRepeats (которое заметно
// меньше). Больше — лишний раздув промпта.
const recentDecisionsLimit = 8

// repeatedDispatchRun возвращает длину хвоста подряд идущих Router-решений с ОДНИМ И ТЕМ ЖЕ
// непустым набором выбранных агентов (множество, без учёта порядка) и человекочитаемую метку
// этого набора. decisions ожидаются в хронологическом порядке (ASC по step_no). Решения с
// пустым набором агентов («ожидание» при in-flight job'ах) прерывают серию и не учитываются —
// они не являются повторным назначением. Используется repeated-dispatch backstop'ом (см. Step)
// для детекта зацикливания Router'а (переназначение одного и того же агента без сходимости).
func repeatedDispatchRun(decisions []models.RouterDecision) (int, string) {
	run := 0
	var key, label string
	for i := len(decisions) - 1; i >= 0; i-- {
		agents := []string(decisions[i].ChosenAgents)
		if len(agents) == 0 {
			break
		}
		sorted := append([]string(nil), agents...)
		sort.Strings(sorted)
		k := strings.Join(sorted, ",")
		if run == 0 {
			key = k
			label = strings.Join(agents, ", ")
		} else if k != key {
			break
		}
		run++
	}
	return run, label
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

// publishRouterDecision шлёт RouterDecisionCreated в EventBus (для live-апдейта UI через
// HubBridge). No-op если bus не сконфигурирован (тесты/minimal-setup). Ошибки публикации
// не критичны — UI всегда может сделать ручной рефреш.
func (o *Orchestrator) publishRouterDecision(ctx context.Context, projectID, taskID uuid.UUID, stepNo int, d *Decision) {
	if o.bus == nil || projectID == uuid.Nil {
		return
	}
	chosen := make([]string, 0, len(d.Agents))
	for _, a := range d.Agents {
		chosen = append(chosen, a.Name)
	}
	o.bus.Publish(ctx, events.RouterDecisionCreated{
		ProjectID:    projectID,
		TaskID:       taskID,
		StepNo:       stepNo,
		ChosenAgents: chosen,
		Done:         d.Done,
		Outcome:      string(d.Outcome),
		Reason:       d.Reason,
		OccurredAt:   time.Now(),
	})
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
		// Мульти-репо: определяем целевой репозиторий подзадачи по repo_slug из target-артефакта
		// (subtask_description или его потомков). Кладём _repo_slug; base_branch берём из
		// default-ветки этого репо (если у задачи нет явного branch override).
		repoBaseBranch := baseBranch
		if slug := o.resolveRepoSlugForJob(ctx, tx, req.Input); slug != "" {
			payload.Input["_repo_slug"] = slug
			if task.BranchName == nil || *task.BranchName == "" {
				if b, ok := o.repoDefaultBranch(ctx, tx, task.ProjectID, slug); ok {
					repoBaseBranch = b
				}
			}
		}
		payload.Input["_base_branch"] = repoBaseBranch
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

// resolveRepoSlugForJob определяет repo_slug подзадачи по target_artifact_id(s) из input.
// Поддерживает обе формы: target_artifact_id (строка) и target_artifact_ids (массив) —
// последнюю шлёт merger, передавая все одобренные code_diff'ы разом. Для каждого
// артефакта-кандидата поднимается по цепочке parent_id (subtask_description → code_diff →
// review → ...), пока не найдёт repo_slug в content. Возвращает первый найденный slug.
// Пусто, если репо не указан (одно-репо проект / decomposer не проставил slug) — вызывающий
// код откатывается на primary-репо.
func (o *Orchestrator) resolveRepoSlugForJob(ctx context.Context, tx *gorm.DB, input map[string]any) string {
	for _, idStr := range targetArtifactIDsFromInput(input) {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		for depth := 0; depth < 6 && id != uuid.Nil; depth++ {
			var art models.Artifact
			if err := tx.WithContext(ctx).Where("id = ?", id).First(&art).Error; err != nil {
				break
			}
			if slug := extractRepoSlug(art.Content); slug != "" {
				return slug
			}
			if art.ParentID == nil {
				break
			}
			id = *art.ParentID
		}
	}
	return ""
}

// targetArtifactIDsFromInput извлекает id артефактов из input, поддерживая singular
// (target_artifact_id) и plural (target_artifact_ids) формы — ср. AgentRequest.RawTargetArtifactIDs.
func targetArtifactIDsFromInput(input map[string]any) []string {
	if input == nil {
		return nil
	}
	var ids []string
	if raw, ok := input["target_artifact_id"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			ids = append(ids, s)
		}
	}
	if raw, ok := input["target_artifact_ids"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok && s != "" {
					ids = append(ids, s)
				}
			}
		}
	}
	return ids
}

// repoDefaultBranch возвращает валидную default-ветку репо проекта по slug.
func (o *Orchestrator) repoDefaultBranch(ctx context.Context, tx *gorm.DB, projectID uuid.UUID, slug string) (string, bool) {
	var repo models.ProjectRepository
	if err := tx.WithContext(ctx).Where("project_id = ? AND slug = ?", projectID, slug).First(&repo).Error; err != nil {
		return "", false
	}
	if repo.GitDefaultBranch == "" || ValidateBaseBranch(repo.GitDefaultBranch) != nil {
		return "", false
	}
	return repo.GitDefaultBranch, true
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

// FailTaskExhausted переводит задачу в failed, когда её step_req окончательно умер
// (StepWorker исчерпал retry-бюджет на Orchestrator.Step — обычно инфра/внешняя ошибка:
// LLM 5xx, лимит ключа, серия 40001). Без этого мёртвый step_req никем не будит
// Orchestrator.Step, и задача навсегда залипает в active: штатный Resume её не берёт
// (он работает из failed/needs_human/paused). Перевод в failed делает ситуацию видимой
// в UI и resumable после устранения причины.
//
// Идемпотентно и безопасно к гонкам: берёт тот же per-task lock, что и Step (NOWAIT).
// Если задачу держит другой воркер (значит она не «застряла») или она уже не active
// (финализирована/приостановлена) — no-op. Worktree-release — post-commit, как в Step.
func (o *Orchestrator) FailTaskExhausted(ctx context.Context, taskID uuid.UUID, reason string) error {
	if taskID == uuid.Nil {
		return fmt.Errorf("orchestrator.FailTaskExhausted: taskID is required")
	}

	var postCommit []func(context.Context)
	err := o.db.Transaction(func(tx *gorm.DB) error {
		if err := TryLockTaskForStep(ctx, tx, taskID); err != nil {
			if errors.Is(err, ErrTaskLockBusy) || errors.Is(err, ErrTaskNotFoundForLock) {
				return nil
			}
			return err
		}
		var task models.Task
		if err := tx.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			return fmt.Errorf("load task %s: %w", taskID, err)
		}
		if task.State != models.TaskStateActive {
			return nil
		}
		postCommit = append(postCommit, o.scheduleWorktreeRelease(taskID))
		return o.finalizeTaskInTx(ctx, tx, &task, models.TaskStateFailed, reason)
	})
	if err != nil {
		return err
	}
	for _, hook := range postCommit {
		hook(ctx)
	}
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

// detectCycle ловит петлю «ревью → changes_requested → переделка → снова ТО ЖЕ замечание»
// и в implement-фазе (developer↔reviewer по code_diff), и в decompose-фазе
// (decomposer↔reviewer по subtask_description). Свежие changes_requested-ревью
// группируются по ИДЕНТИЧНОСТИ цели, и внутри группы сравниваются комментарии: 3 подряд
// почти одинаковых замечания (Jaccard > 0.8) на одну цель = задача не сходится → петля.
//
// Цель ревью определяется по его родительскому артефакту:
//   - code_diff           → набор изменённых файлов (стабилен между ревизиями диффа);
//   - subtask_description → стабильный id подзадачи из content (артефакты пересоздаются
//     на каждой ревизии с новым UUID, но id подзадачи постоянен — это переживает
//     interleaving нескольких подзадач, которое ломало старую логику «последние 3 ревью»);
//   - иное                → UUID родителя.
//
// approved/escalate-ревью на цель «сбрасывает» её счётчик (цель сошлась — более ранние
// отклонения не считаем; идём newest-first, поэтому первый же approve закрывает группу).
// Порог сходства 0.8 отсекает ложные срабатывания: переделки с НОВЫМИ замечаниями (реальный
// прогресс) имеют низкую схожесть и петлёй не считаются.
func (o *Orchestrator) detectCycle(ctx context.Context, tx *gorm.DB, taskID uuid.UUID) (bool, string, error) {
	const (
		reviewWindow     = 12  // сколько последних ревью смотрим (запас на interleaving подзадач)
		minRepeats       = 3   // одинаковых замечаний подряд на одну цель → петля
		similarityThresh = 0.8 // порог Jaccard «то же самое замечание»
	)

	var reviews []models.Artifact
	if err := tx.WithContext(ctx).
		Where("task_id = ? AND kind = ?", taskID, models.ArtifactKindReview).
		Order("created_at DESC").
		Limit(reviewWindow).
		Find(&reviews).Error; err != nil {
		return false, "", err
	}
	if len(reviews) < minRepeats {
		return false, "", nil
	}

	// extractReview — decision + объединённый текст замечаний ревью.
	extractReview := func(art models.Artifact) (decision, comment string) {
		var rc struct {
			Decision string `json:"decision"`
			Issues   []struct {
				Comment string `json:"comment"`
			} `json:"issues"`
		}
		if err := json.Unmarshal(art.Content, &rc); err != nil {
			return "", ""
		}
		var parts []string
		for _, iss := range rc.Issues {
			if c := strings.TrimSpace(iss.Comment); c != "" {
				parts = append(parts, c)
			}
		}
		return rc.Decision, strings.Join(parts, " ")
	}

	fileRe := regexp.MustCompile(`diff --git a/([^\s]+) b/`)
	// targetKey — стабильный ключ цели ревью (см. doc функции).
	targetKey := func(rev models.Artifact) string {
		if rev.ParentID == nil {
			return ""
		}
		var parent models.Artifact
		if err := tx.WithContext(ctx).Where("id = ?", *rev.ParentID).First(&parent).Error; err != nil {
			return ""
		}
		switch parent.Kind {
		case models.ArtifactKindCodeDiff:
			var w struct {
				Diff      string `json:"diff"`
				RawOutput string `json:"raw_output"`
			}
			_ = json.Unmarshal(parent.Content, &w)
			text := w.Diff
			if text == "" {
				text = w.RawOutput
			}
			seen := make(map[string]bool)
			var files []string
			for _, m := range fileRe.FindAllStringSubmatch(text, -1) {
				if len(m) > 1 && !seen[m[1]] {
					seen[m[1]] = true
					files = append(files, m[1])
				}
			}
			if len(files) == 0 {
				return "codediff:" + parent.ID.String()
			}
			sort.Strings(files)
			return "files:" + strings.Join(files, ",")
		case models.ArtifactKindSubtaskDescription:
			var w struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(parent.Content, &w)
			if strings.TrimSpace(w.ID) != "" {
				return "subtask:" + w.ID
			}
			return "artifact:" + parent.ID.String()
		default:
			return "artifact:" + parent.ID.String()
		}
	}

	// Группируем комментарии отклонений по цели (newest-first). approved по цели → группа
	// закрыта (converged): её более ранние отклонения уже не считаем.
	grouped := make(map[string][]string)
	converged := make(map[string]bool)
	for _, rev := range reviews { // newest → oldest
		key := targetKey(rev)
		if key == "" {
			continue
		}
		decision, comment := extractReview(rev)
		if decision != "changes_requested" {
			converged[key] = true
			continue
		}
		if converged[key] || comment == "" {
			continue
		}
		grouped[key] = append(grouped[key], comment)
	}

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

	// Детерминированный порядок обхода групп (стабильный reason при нескольких застрявших целях).
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		comments := grouped[key]
		if len(comments) < minRepeats {
			continue
		}
		// 3 самых свежих отклонения (comments[0..2], newest-first).
		sim1 := jaccardSimilarity(comments[0], comments[1])
		sim2 := jaccardSimilarity(comments[1], comments[2])
		if sim1 > similarityThresh && sim2 > similarityThresh {
			reason := fmt.Sprintf(
				"router stuck: reviewer raised the same changes_requested %d× on target [%s] (similarity %.0f%%/%.0f%%) without convergence",
				minRepeats, key, sim1*100, sim2*100)
			return true, reason, nil
		}
	}

	return false, "", nil
}
