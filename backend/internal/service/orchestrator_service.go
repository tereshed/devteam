package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/agent"
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
)

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

	// Настройки
	zombieTimeout time.Duration
	zombieTicker  *time.Ticker
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
) OrchestratorService {
	return &orchestratorService{
		taskRepo:        taskRepo,
		taskMsgRepo:     taskMsgRepo,
		workflowRepo:    workflowRepo,
		projectSvc:      projectSvc,
		txManager:       txManager,
		llmExecutor:     llmExecutor,
		sandboxExecutor: sandboxExecutor,
		taskSvc:         taskSvc,
		pipeline:        pipeline,
		contextBuilder:  contextBuilder,
		zombieTimeout:   1 * time.Hour,
	}
}

func (s *orchestratorService) Start(ctx context.Context) error {
	slog.Info("Starting OrchestratorService...")

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

func (s *orchestratorService) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	// 1. Загрузка задачи
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return ErrOrchestratorTaskNotFound
		}
		return err
	}

	// 2. Проверка прав (Tenant Isolation)
	project, err := s.projectSvc.GetByID(ctx, uuid.Nil, models.RoleAdmin, task.ProjectID)
	if err != nil {
		return ErrOrchestratorProjectNotFound
	}

	slog.Info("Processing task", "project_id", project.ID, "task_id", task.ID, "status", task.Status)

	// 3. Основной цикл выполнения
	for {
		// Проверяем контекст на отмену (Safe Cancellation)
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping task processing", "project_id", project.ID, "task_id", task.ID)
			// Graceful Shutdown: переводим задачу в Cancelled через detached context
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, transitionErr := s.taskSvc.Transition(cleanupCtx, task.ID, models.TaskStatusCancelled, TransitionOpts{})
			if transitionErr != nil {
				slog.Error("Failed to transition task to cancelled", "project_id", project.ID, "task_id", task.ID, "error", transitionErr)
			}
			return ctx.Err()
		default:
		}

		// Если задача в терминальном статусе — выходим
		if s.isTerminalStatus(task.Status) {
			slog.Info("Task reached terminal status", "project_id", project.ID, "task_id", task.ID, "status", task.Status)
			return nil
		}

		// Если задача в паузе или отменена — выходим
		if task.Status == models.TaskStatusPaused || task.Status == models.TaskStatusCancelled {
			slog.Info("Task is paused or cancelled", "project_id", project.ID, "task_id", task.ID, "status", task.Status)
			return nil
		}

		// Выполняем один шаг пайплайна
		err := s.executeStep(ctx, task, project)
		if err != nil {
			slog.Error("Failed to execute task step", "project_id", project.ID, "task_id", task.ID, "error", err)

			// Graceful Shutdown: при отмене контекста переводим задачу в Cancelled
			// Используем detached context с таймаутом для гарантированного сохранения статуса
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.Info("Task cancelled by user or timeout", "project_id", project.ID, "task_id", task.ID)
				_, transitionErr := s.taskSvc.Transition(cleanupCtx, task.ID, models.TaskStatusCancelled, TransitionOpts{})
				if transitionErr != nil {
					slog.Error("Failed to transition task to cancelled", "project_id", project.ID, "task_id", task.ID, "error", transitionErr)
				}
				return err
			}

			// Другие ошибки - переводим в Failed
			errMsg := err.Error()
			_, transitionErr := s.taskSvc.Transition(cleanupCtx, task.ID, models.TaskStatusFailed, TransitionOpts{
				ErrorMessage: &errMsg,
			})
			if transitionErr != nil {
				slog.Error("Failed to transition task to failed", "project_id", project.ID, "task_id", task.ID, "error", transitionErr)
			}
			return err
		}

		// Перечитываем задачу для следующей итерации
		task, err = s.taskRepo.GetByID(ctx, taskID)
		if err != nil {
			return err
		}
	}
}

func (s *orchestratorService) isTerminalStatus(status models.TaskStatus) bool {
	return status == models.TaskStatusCompleted || status == models.TaskStatusFailed
}

func (s *orchestratorService) executeStep(ctx context.Context, task *models.Task, project *models.Project) error {
	// 1. Подготовка (Context Builder)
	executor, input, err := s.prepareExecution(ctx, task, project)
	if err != nil {
		return err
	}

	// 2. Выполнение (Agent Executor) с ретраями при инфраструктурных ошибках
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
			result, execErr = executor.Execute(ctx, *input)
		}()

		if execErr == nil {
			break
		}

		// Проверяем, стоит ли ретраить (только RateLimit или временные ошибки)
		if errors.Is(execErr, agent.ErrRateLimit) && i < maxRetries-1 {
			backoff := time.Duration(1<<i) * time.Second
			slog.Warn("Agent rate limited, retrying", "project_id", project.ID, "task_id", task.ID, "retry", i+1, "backoff", backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		break
	}

	if execErr != nil {
		return fmt.Errorf("agent execution failed after retries: %w", execErr)
	}

	// 3. Обработка результата и переход (Pipeline Engine)
	return s.handleExecutionResult(ctx, task, result)
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

	// Выбираем Executor на основе роли и CodeBackend
	var executor agent.AgentExecutor
	switch assignedAgent.Role {
	case models.AgentRolePlanner, models.AgentRoleReviewer, models.AgentRoleOrchestrator:
		executor = s.llmExecutor
	case models.AgentRoleDeveloper, models.AgentRoleTester:
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

	return executor, input, nil
}

func (s *orchestratorService) handleExecutionResult(ctx context.Context, task *models.Task, result *agent.ExecutionResult) error {
	// Определяем следующий статус
	nextStatus, err := s.pipeline.DetermineNextStatus(task, result)
	if err != nil {
		return err
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
	return s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
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

		slog.Info("Transitioning task", "project_id", task.ProjectID, "task_id", task.ID, "from", task.Status, "to", nextStatus)

		_, err = s.taskSvc.Transition(txCtx, task.ID, nextStatus, opts)
		return err
	})
}
