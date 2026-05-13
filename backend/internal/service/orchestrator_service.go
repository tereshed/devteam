package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/tidwall/sjson"
	"gorm.io/datatypes"
)

var (
	ErrOrchestratorTaskNotFound          = errors.New("orchestrator: task not found")
	ErrOrchestratorProjectNotFound       = errors.New("orchestrator: project not found")
	ErrOrchestratorAgentNotFound         = errors.New("orchestrator: agent not found")
	ErrOrchestratorNoAgentAssigned       = errors.New("orchestrator: no agent assigned to task")
	ErrOrchestratorInvalidRole           = errors.New("orchestrator: invalid agent role for this state")
	ErrOrchestratorIterationLimitReached = errors.New("orchestrator: iteration limit reached")
	// ErrOrchestratorInvalidUserMessage — нет видимого пользовательского текста в задаче (title/description).
	ErrOrchestratorInvalidUserMessage = errors.New("orchestrator: invalid user message")
	// ErrStepRestarted — шаг отменён для перезапуска (correct / откат статуса в БД); не переводить задачу в cancelled.
	ErrStepRestarted = errors.New("orchestrator: step restarted")
)

// TaskSandboxStopper принудительно останавливает изолированный runtime по ID задачи (задача 6.7: cancel).
type TaskSandboxStopper interface {
	StopTask(ctx context.Context, taskID string) error
}

// OrchestratorOption настраивает оркестратор (тесты, расширения).
type OrchestratorOption func(*orchestratorService)

// WithStepPollInterval задаёт интервал опроса БД на pause/cancel внутри шага; 0 — отключить опрос.
func WithStepPollInterval(d time.Duration) OrchestratorOption {
	return func(s *orchestratorService) {
		s.stepPollInterval = d
	}
}

// WithGracefulPauseTimeout задаёт таймаут «мягкой» паузы перед отменой шага (тесты могут ставить мало).
func WithGracefulPauseTimeout(d time.Duration) OrchestratorOption {
	return func(s *orchestratorService) {
		s.gracefulPauseTimeout = d
	}
}

// WithTeamRepository позволяет оркестратору автоматически переключать
// task.AssignedAgentID на агента следующей по pipeline роли при Transition.
// Без этого опционала pipeline остаётся с первоначальным агентом до конца.
func WithTeamRepository(teamRepo repository.TeamRepository) OrchestratorOption {
	return func(s *orchestratorService) {
		s.teamRepo = teamRepo
	}
}

// WithPullRequestPublisher включает автосоздание PR после перехода задачи в completed.
// Если publisher == nil, шаг тихо пропускается.
func WithPullRequestPublisher(p PullRequestPublisher) OrchestratorOption {
	return func(s *orchestratorService) {
		s.prPublisher = p
	}
}

// FreeClaudeProxyHealthChecker — fail-fast проверка прокси при старте оркестратора (Sprint 15.19).
// Если хоть один агент использует CodeBackend=claude-code-via-proxy, проверяем прокси /healthz.
type FreeClaudeProxyHealthChecker interface {
	Check(ctx context.Context) error
}

// WithFreeClaudeProxyHealthChecker подключает проверку прокси.
func WithFreeClaudeProxyHealthChecker(checker FreeClaudeProxyHealthChecker) OrchestratorOption {
	return func(s *orchestratorService) {
		s.proxyHealthChecker = checker
	}
}

// OrchestratorService управляет жизненным циклом выполнения задач через агентов.
type OrchestratorService interface {
	// ProcessTask запускает или продолжает выполнение задачи.
	ProcessTask(ctx context.Context, taskID uuid.UUID) error

	// Start запускает фоновые процессы оркестратора (очистка зомби-задач).
	Start(ctx context.Context) error
}

type orchestratorService struct {
	taskRepo        repository.TaskRepository
	taskMsgRepo     repository.TaskMessageRepository
	workflowRepo    repository.WorkflowRepository
	projectSvc      ProjectService
	txManager       repository.TransactionManager
	llmExecutor     agent.AgentExecutor
	sandboxExecutor agent.AgentExecutor
	taskSvc         TaskService
	pipeline        PipelineEngine
	contextBuilder  ContextBuilder
	codeIndexer     indexer.CodeIndexer

	sandboxStop TaskSandboxStopper
	controlBus  *UserTaskControlBus
	teamRepo    repository.TeamRepository
	prPublisher PullRequestPublisher
	// Sprint 15.19 — fail-fast health check для free-claude-proxy.
	proxyHealthChecker FreeClaudeProxyHealthChecker

	// Настройки
	zombieTimeout          time.Duration
	gracefulPauseTimeout   time.Duration
	stepPollInterval       time.Duration
	zombieTicker           *time.Ticker
	stepMu                 sync.Mutex
	stepCancelByTask       map[uuid.UUID]context.CancelCauseFunc
}

// NewOrchestratorService создает новый экземпляр OrchestratorService.
func NewOrchestratorService(
	taskRepo repository.TaskRepository,
	taskMsgRepo repository.TaskMessageRepository,
	workflowRepo repository.WorkflowRepository,
	projectSvc ProjectService,
	txManager repository.TransactionManager,
	llmExecutor agent.AgentExecutor,
	sandboxExecutor agent.AgentExecutor,
	taskSvc TaskService,
	pipeline PipelineEngine,
	contextBuilder ContextBuilder,
	codeIndexer indexer.CodeIndexer,
	sandboxStop TaskSandboxStopper,
	controlBus *UserTaskControlBus,
	opts ...OrchestratorOption,
) OrchestratorService {
	s := &orchestratorService{
		taskRepo:             taskRepo,
		taskMsgRepo:          taskMsgRepo,
		workflowRepo:         workflowRepo,
		projectSvc:           projectSvc,
		txManager:            txManager,
		llmExecutor:          llmExecutor,
		sandboxExecutor:      sandboxExecutor,
		taskSvc:              taskSvc,
		pipeline:             pipeline,
		contextBuilder:       contextBuilder,
		codeIndexer:          codeIndexer,
		sandboxStop:          sandboxStop,
		controlBus:           controlBus,
		zombieTimeout:        1 * time.Hour,
		gracefulPauseTimeout: 30 * time.Second,
		stepPollInterval:     400 * time.Millisecond,
		stepCancelByTask:     make(map[uuid.UUID]context.CancelCauseFunc),
	}
	for _, o := range opts {
		if o != nil {
			o(s)
		}
	}
	return s
}

func (s *orchestratorService) Start(ctx context.Context) error {
	slog.Info("Starting OrchestratorService...")

	if s.controlBus != nil {
		s.controlBus.SubscribeCommands(s.handleControlCommand)
	}

	// Sprint 15.19 — fail-fast health check для free-claude-proxy.
	// Чекер подключается в main.go ИСКЛЮЧИТЕЛЬНО при FREE_CLAUDE_PROXY_ENABLED=true
	// (см. buildFreeClaudeProxyHealthChecker). Никакой проверки «есть ли claude-code-via-proxy
	// агент» здесь нет; ответственность — на операторе/конфиге.
	if s.proxyHealthChecker != nil {
		hcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := s.proxyHealthChecker.Check(hcCtx)
		cancel()
		if err != nil {
			return fmt.Errorf("free-claude-proxy health check failed: %w", err)
		}
		slog.Info("free-claude-proxy: health check passed")
	}

	// Первичная очистка
	if err := s.recoverZombieTasks(ctx); err != nil {
		slog.Error("Initial zombie recovery failed", "error", err)
	}

	// Запуск периодической очистки
	s.zombieTicker = time.NewTicker(5 * time.Minute)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic in zombie recovery ticker", "error", r)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				if s.zombieTicker != nil {
					s.zombieTicker.Stop()
				}
				return
			case <-s.zombieTicker.C:
				if err := s.recoverZombieTasks(ctx); err != nil {
					slog.Error("Periodic zombie recovery failed", "error", err)
				}
			}
		}
	}()

	return nil
}

func (s *orchestratorService) recoverZombieTasks(ctx context.Context) error {
	// Ищем задачи в активных статусах, которые не обновлялись дольше zombieTimeout
	now := time.Now().UTC()
	cutoff := now.Add(-s.zombieTimeout)

	// Статусы, которые мы считаем "активными" и требующими восстановления
	activeStatuses := []models.TaskStatus{
		models.TaskStatusPlanning,
		models.TaskStatusInProgress,
		models.TaskStatusReview,
		models.TaskStatusTesting,
	}

	filter := repository.TaskFilter{
		Statuses:        activeStatuses,
		UpdatedAtBefore: &cutoff,
		Limit:           100, // Обрабатываем пачками
	}

	tasks, _, err := s.taskRepo.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list active tasks for recovery: %w", err)
	}

	for i := range tasks {
		t := &tasks[i]
		slog.Warn("Zombie task detected", "project_id", t.ProjectID, "task_id", t.ID, "last_updated", t.UpdatedAt)

		// Переводим в Failed с объяснением
		errMsg := "Task timed out (zombie detection)"
		_, err := s.taskSvc.Transition(ctx, t.ID, models.TaskStatusFailed, TransitionOpts{
			ErrorMessage: &errMsg,
		})
		if err != nil {
			slog.Error("Failed to transition zombie task to failed", "project_id", t.ProjectID, "task_id", t.ID, "error", err)
		}
	}

	return nil
}

func (s *orchestratorService) registerStepCancel(taskID uuid.UUID, cancel context.CancelCauseFunc) {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	s.stepCancelByTask[taskID] = cancel
}

func (s *orchestratorService) clearStepCancel(taskID uuid.UUID) {
	s.stepMu.Lock()
	defer s.stepMu.Unlock()
	delete(s.stepCancelByTask, taskID)
}

func (s *orchestratorService) runStepCancelCause(taskID uuid.UUID, cause error) {
	s.stepMu.Lock()
	fn := s.stepCancelByTask[taskID]
	s.stepMu.Unlock()
	if fn != nil {
		fn(cause)
	}
}

func (s *orchestratorService) stopSandboxForTask(ctx context.Context, taskID uuid.UUID) {
	if s.sandboxStop == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.sandboxStop.StopTask(cleanupCtx, taskID.String()); err != nil {
		slog.Warn("StopTask failed", "task_id", taskID, "error", err)
	}
}

func (s *orchestratorService) handleControlCommand(ctx context.Context, cmd UserTaskControlCommand) {
	task, err := s.taskRepo.GetByID(ctx, cmd.TaskID)
	if err != nil {
		slog.Debug("task control: task not found", "task_id", cmd.TaskID, "error", err)
		return
	}
	if _, err := s.projectSvc.GetByID(ctx, cmd.UserID, cmd.UserRole, task.ProjectID); err != nil {
		slog.Warn("task control command forbidden", "task_id", cmd.TaskID, "user_id", cmd.UserID, "error", err)
		return
	}
	switch cmd.Kind {
	case UserTaskControlCancel:
		s.runStepCancelCause(cmd.TaskID, context.Canceled)
		s.stopSandboxForTask(ctx, cmd.TaskID)
	case UserTaskControlPause:
		// Graceful pause по таймауту обрабатывается при опросе БД в executeStep
	case UserTaskControlCorrect:
		s.runStepCancelCause(cmd.TaskID, ErrStepRestarted)
		s.stopSandboxForTask(ctx, cmd.TaskID)
	case UserTaskControlResume:
		// Состояние уже в БД; новый цикл ProcessTask запускается из HTTP Resume
	default:
		slog.Debug("task control: unknown kind", "kind", cmd.Kind)
	}
	if s.controlBus != nil {
		s.controlBus.PublishOutcome(ctx, TaskControlOutcome{
			TaskID:    task.ID,
			ProjectID: task.ProjectID,
			Kind:      cmd.Kind,
			Detail:    "ack",
		})
	}
}

func (s *orchestratorService) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	// 1. Загрузка задачи
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return ErrOrchestratorTaskNotFound
		}
		return err
	}

	// 2. Внутренний прогон оркестратора: доступ к проекту с системной ролью (ProcessTask вызывается после проверок API).
	project, err := s.projectSvc.GetByID(ctx, uuid.Nil, models.RoleAdmin, task.ProjectID)
	if err != nil {
		return ErrOrchestratorProjectNotFound
	}

	slog.Info("Processing task", "project_id", project.ID, "task_id", task.ID, "status", task.Status)

	// 3. Основной цикл выполнения
	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping task processing", "project_id", project.ID, "task_id", task.ID)
			cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
			_, transitionErr := s.taskSvc.Transition(cleanupCtx, task.ID, models.TaskStatusCancelled, TransitionOpts{})
			cancelCleanup()
			if transitionErr != nil {
				slog.Error("Failed to transition task to cancelled", "project_id", project.ID, "task_id", task.ID, "error", transitionErr)
			}
			return ctx.Err()
		default:
		}

		// task поднят до цикла и обновляется в конце каждой итерации (без лишнего GetByID в начале).
		if s.isTerminalStatus(task.Status) {
			slog.Info("Task reached terminal status", "project_id", project.ID, "task_id", task.ID, "status", task.Status)
			return nil
		}

		if task.Status == models.TaskStatusPaused || task.Status == models.TaskStatusCancelled {
			slog.Info("Task is paused or cancelled", "project_id", project.ID, "task_id", task.ID, "status", task.Status)
			return nil
		}

		err = s.executeStep(ctx, task, project)
		if err != nil {
			return s.handleProcessTaskError(ctx, project.ID, task.ID, err)
		}

		task, err = s.taskRepo.GetByID(ctx, taskID)
		if err != nil {
			return err
		}
	}
}

func (s *orchestratorService) handleProcessTaskError(ctx context.Context, projectID, taskID uuid.UUID, execErr error) error {
	if errors.Is(execErr, ErrStepRestarted) {
		return nil
	}
	slog.Error("Failed to execute task step", "project_id", projectID, "task_id", taskID, "error", execErr)

	cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCleanup()

	task, gerr := s.taskRepo.GetByID(ctx, taskID)
	if gerr == nil {
		if task.Status == models.TaskStatusPaused || task.Status == models.TaskStatusCancelled {
			return nil
		}
	}

	cancelLike := errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, agent.ErrExecutionCancelled)
	if cancelLike {
		t2, e2 := s.taskRepo.GetByID(cleanupCtx, taskID)
		if e2 == nil && (t2.Status == models.TaskStatusPaused || t2.Status == models.TaskStatusCancelled) {
			return nil
		}
		slog.Info("Task cancelled by user or timeout", "project_id", projectID, "task_id", taskID)
		_, transitionErr := s.taskSvc.Transition(cleanupCtx, taskID, models.TaskStatusCancelled, TransitionOpts{})
		if transitionErr != nil {
			slog.Error("Failed to transition task to cancelled", "project_id", projectID, "task_id", taskID, "error", transitionErr)
		}
		return execErr
	}

	errMsg := execErr.Error()
	_, transitionErr := s.taskSvc.Transition(cleanupCtx, taskID, models.TaskStatusFailed, TransitionOpts{
		ErrorMessage: &errMsg,
	})
	if transitionErr != nil {
		slog.Error("Failed to transition task to failed", "project_id", projectID, "task_id", taskID, "error", transitionErr)
	}
	return execErr
}

func (s *orchestratorService) isTerminalStatus(status models.TaskStatus) bool {
	return status == models.TaskStatusCompleted || status == models.TaskStatusFailed
}

// pollIndicatesStepRestart — Correct на другом инстансе меняет статус (например review→in_progress); шаг нужно перезапустить.
func pollIndicatesStepRestart(orig, cur models.TaskStatus) bool {
	if cur != models.TaskStatusInProgress {
		return false
	}
	switch orig {
	case models.TaskStatusReview, models.TaskStatusTesting, models.TaskStatusChangesRequested:
		return true
	default:
		return false
	}
}

func (s *orchestratorService) executeStep(ctx context.Context, task *models.Task, project *models.Project) error {
	origStatus := task.Status
	stepCtx, cancelStep := context.WithCancelCause(ctx)
	defer cancelStep(context.Canceled)

	s.registerStepCancel(task.ID, cancelStep)
	defer s.clearStepCancel(task.ID)

	if !taskHasVisibleUserContent(task) {
		return ErrOrchestratorInvalidUserMessage
	}

	executor, input, err := s.prepareExecution(stepCtx, task, project)
	if err != nil {
		return err
	}

	type stepOutcome struct {
		result *agent.ExecutionResult
		err    error
	}
	done := make(chan stepOutcome, 1)

	go func() {
		var result *agent.ExecutionResult
		var execErr error
		maxRetries := 3
		for i := 0; i < maxRetries; i++ {
			func() {
				defer func() {
					if r := recover(); r != nil {
						execErr = fmt.Errorf("agent panic: %v", r)
					}
				}()
				result, execErr = executor.Execute(stepCtx, *input)
			}()

			if execErr == nil {
				break
			}

			if errors.Is(execErr, agent.ErrRateLimit) && i < maxRetries-1 {
				backoff := time.Duration(1<<i) * time.Second
				slog.Warn("Agent rate limited, retrying", "project_id", project.ID, "task_id", task.ID, "retry", i+1, "backoff", backoff)
				select {
				case <-stepCtx.Done():
					done <- stepOutcome{nil, stepCtx.Err()}
					return
				case <-time.After(backoff):
					continue
				}
			}
			break
		}
		if execErr != nil {
			done <- stepOutcome{nil, execErr}
			return
		}
		done <- stepOutcome{result, nil}
	}()

	var poll *time.Ticker
	if s.stepPollInterval > 0 {
		poll = time.NewTicker(s.stepPollInterval)
		defer poll.Stop()
	}

	var pauseGraceStart *time.Time

	for {
		if poll == nil {
			select {
			case <-ctx.Done():
				cancelStep(context.Canceled)
				s.stopSandboxForTask(ctx, task.ID)
				out := <-done
				return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)
			case out := <-done:
				return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)
			}
		}
		select {
		case <-ctx.Done():
			cancelStep(context.Canceled)
			s.stopSandboxForTask(ctx, task.ID)
			out := <-done
			return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)

		case out := <-done:
			return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)

		case <-poll.C:
			t, err := s.taskRepo.GetByID(ctx, task.ID)
			if err != nil {
				continue
			}
			if pollIndicatesStepRestart(origStatus, t.Status) {
				cancelStep(ErrStepRestarted)
				s.stopSandboxForTask(ctx, task.ID)
				out := <-done
				return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)
			}
			switch t.Status {
			case models.TaskStatusCancelled:
				cancelStep(context.Canceled)
				s.stopSandboxForTask(ctx, task.ID)
				out := <-done
				return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)
			case models.TaskStatusPaused:
				if pauseGraceStart == nil {
					now := time.Now()
					pauseGraceStart = &now
				}
				if time.Since(*pauseGraceStart) > s.gracefulPauseTimeout {
					cancelStep(context.Canceled)
					s.stopSandboxForTask(ctx, task.ID)
					out := <-done
					return s.finishStepExecution(ctx, stepCtx, task, out.result, out.err)
				}
			}
		}
	}
}

func (s *orchestratorService) finishStepExecution(ctx context.Context, stepCtx context.Context, task *models.Task, result *agent.ExecutionResult, execErr error) error {
	if execErr != nil {
		if stepCtx != nil && errors.Is(context.Cause(stepCtx), ErrStepRestarted) {
			return nil
		}
		if errors.Is(execErr, ErrStepRestarted) {
			return nil
		}
		t, err := s.taskRepo.GetByID(ctx, task.ID)
		if err == nil {
			if t.Status == models.TaskStatusPaused || t.Status == models.TaskStatusCancelled {
				return nil
			}
		}
		return fmt.Errorf("agent execution failed after retries: %w", execErr)
	}

	t, err := s.taskRepo.GetByID(ctx, task.ID)
	if err != nil {
		return err
	}
	if t.Status == models.TaskStatusPaused || t.Status == models.TaskStatusCancelled {
		return nil
	}
	return s.handleExecutionResult(ctx, t, result)
}

func taskHasVisibleUserContent(task *models.Task) bool {
	if task == nil {
		return false
	}
	return strings.TrimSpace(task.Title) != "" || strings.TrimSpace(task.Description) != ""
}

func (s *orchestratorService) prepareExecution(ctx context.Context, task *models.Task, project *models.Project) (agent.AgentExecutor, *agent.ExecutionInput, error) {
	// Получаем агента
	if task.AssignedAgentID == nil {
		return nil, nil, ErrOrchestratorNoAgentAssigned
	}

	assignedAgent, err := s.workflowRepo.GetAgentByID(ctx, *task.AssignedAgentID)
	if err != nil {
		return nil, nil, ErrOrchestratorAgentNotFound
	}

	// Выбираем Executor на основе роли и CodeBackend.
	// Orchestrator и Planner — всегда LLM (короткое декомпозирующее рассуждение).
	// Developer/Tester/Reviewer — sandbox, если у агента выставлен code_backend != custom:
	// reviewer'у нужен полный контекст репозитория (Read/Glob/Bash), а не только diff.
	var executor agent.AgentExecutor
	switch assignedAgent.Role {
	case models.AgentRolePlanner, models.AgentRoleOrchestrator:
		executor = s.llmExecutor
	case models.AgentRoleDeveloper, models.AgentRoleTester, models.AgentRoleReviewer:
		if assignedAgent.CodeBackend != nil && *assignedAgent.CodeBackend != models.CodeBackendCustom {
			executor = s.sandboxExecutor
		} else {
			executor = s.llmExecutor
		}
	default:
		executor = s.llmExecutor
	}

	// Сборка контекста
	input, err := s.contextBuilder.Build(ctx, task, assignedAgent, project)
	if err != nil {
		return nil, nil, err
	}

	// Векторный поиск для контекста кода (Задача 9.11)
	if assignedAgent.RequiresCodeContext && s.codeIndexer != nil {
		query := task.GetSearchQuery()
		if query != "" {
			searchCtx, cancelSearch := context.WithTimeout(ctx, 5*time.Second)
			defer cancelSearch()

			chunks, err := s.codeIndexer.SearchContext(searchCtx, project.ID, query, 15)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, indexer.ErrIndexNotReady) {
					slog.Warn("SearchContext skipped", "task_id", task.ID, "project_id", project.ID, "reason", err)
				} else {
					slog.Error("SearchContext failed", "task_id", task.ID, "project_id", project.ID, "error", err)
				}
			} else if len(chunks) > 0 {
				// Передаем чанки в ContextBuilder для финальной обработки и вставки в промпт
				if cb, ok := s.contextBuilder.(interface {
					WithCodeChunks(input *agent.ExecutionInput, chunks []indexer.Chunk) error
				}); ok {
					if err := cb.WithCodeChunks(input, chunks); err != nil {
						slog.Error("Failed to add code chunks to context", "task_id", task.ID, "error", err)
					}
				}
			}
		}
	}

	return executor, input, nil
}

// pipelineRoleForStatus — ожидаемая роль агента для следующего шага.
// Для терминальных статусов и pending возвращает "".
func pipelineRoleForStatus(status models.TaskStatus) models.AgentRole {
	switch status {
	case models.TaskStatusPlanning:
		return models.AgentRolePlanner
	case models.TaskStatusInProgress, models.TaskStatusChangesRequested:
		return models.AgentRoleDeveloper
	case models.TaskStatusReview:
		return models.AgentRoleReviewer
	case models.TaskStatusTesting:
		return models.AgentRoleTester
	default:
		return ""
	}
}

// resolveNextAgentID лукапит в команде проекта активного агента нужной роли.
// Возвращает nil, если teamRepo не сконфигурирован, роль терминальная или агент не найден —
// в этом случае оркестратор сохраняет текущий AssignedAgentID (поведение до фикса).
func (s *orchestratorService) resolveNextAgentID(ctx context.Context, projectID uuid.UUID, nextStatus models.TaskStatus) *uuid.UUID {
	if s.teamRepo == nil {
		return nil
	}
	role := pipelineRoleForStatus(nextStatus)
	if role == "" {
		return nil
	}
	team, err := s.teamRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		slog.Warn("resolveNextAgentID: team not found", "project_id", projectID, "error", err)
		return nil
	}
	for i := range team.Agents {
		a := &team.Agents[i]
		if a.Role == role && a.IsActive {
			id := a.ID
			return &id
		}
	}
	slog.Warn("resolveNextAgentID: no active agent for role", "project_id", projectID, "role", role)
	return nil
}

func (s *orchestratorService) handleExecutionResult(ctx context.Context, task *models.Task, result *agent.ExecutionResult) error {
	// Определяем следующий статус
	nextStatus, err := s.pipeline.DetermineNextStatus(task, result)
	if err != nil {
		return fmt.Errorf("pipeline step failed: %w", err)
	}

	// Обновляем счетчик итераций в контексте задачи, если переходим в ChangesRequested
	newContext := task.Context
	if nextStatus == models.TaskStatusChangesRequested {
		count := s.pipeline.GetIterationCount(task)
		newContextBytes, err := sjson.SetBytes(task.Context, "iteration_count", count+1)
		if err != nil {
			return fmt.Errorf("failed to update iteration_count: %w", err)
		}
		newContext = datatypes.JSON(newContextBytes)
	}

	// Выполняем переход и сохранение сообщения агента в одной транзакции
	transitionErr := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		senderID := uuid.Nil
		if task.AssignedAgentID != nil {
			senderID = *task.AssignedAgentID
		}
		msg := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeAgent,
			SenderID:    senderID,
			Content:     result.Output,
			MessageType: models.MessageTypeResult,
			Metadata:    datatypes.JSON(result.ArtifactsJSON),
		}
		if err := s.taskMsgRepo.Create(txCtx, msg); err != nil {
			return fmt.Errorf("failed to save agent message: %w", err)
		}

		// 2. Выполняем переход статуса
		opts := TransitionOpts{
			Result:    &result.Output,
			Artifacts: (*datatypes.JSON)(&result.ArtifactsJSON),
		}
		// Передаем обновленный контекст (счетчик итераций) в БД
		if nextStatus == models.TaskStatusChangesRequested {
			opts.Context = &newContext
		}
		// Переключаем задачу на агента следующей по pipeline роли (см. WithTeamRepository).
		if nextAgentID := s.resolveNextAgentID(txCtx, task.ProjectID, nextStatus); nextAgentID != nil {
			opts.AssignedAgentID = nextAgentID
		}

		slog.Info("Transitioning task", "project_id", task.ProjectID, "task_id", task.ID, "from", task.Status, "to", nextStatus)

		_, err = s.taskSvc.Transition(txCtx, task.ID, nextStatus, opts)
		return err
	})

	if transitionErr != nil {
		return transitionErr
	}

	// После успешного перехода в completed — открываем PR в git-провайдере проекта.
	// Ошибки PR не валят pipeline (задача уже completed); только лог.
	if nextStatus == models.TaskStatusCompleted && s.prPublisher != nil {
		go func(taskID uuid.UUID, projectID uuid.UUID) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in PR publisher", "task_id", taskID, "panic", r)
				}
			}()
			prCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			t, err := s.taskRepo.GetByID(prCtx, taskID)
			if err != nil {
				slog.Warn("PR publisher: task fetch failed", "task_id", taskID, "error", err)
				return
			}
			proj, err := s.projectSvc.GetByID(prCtx, uuid.Nil, models.RoleAdmin, projectID)
			if err != nil {
				slog.Warn("PR publisher: project fetch failed", "task_id", taskID, "error", err)
				return
			}
			pr, err := s.prPublisher.Publish(prCtx, t, proj)
			if err != nil {
				if errors.Is(err, ErrPullRequestSkipped) {
					slog.Info("PR publisher: skipped", "task_id", taskID, "reason", err)
					return
				}
				slog.Error("PR publisher: failed", "task_id", taskID, "error", err)
				return
			}
			_ = pr // pr.Number / pr.HTMLURL уже залогированы в Publish()
		}(task.ID, task.ProjectID)
	}

	return nil
}
