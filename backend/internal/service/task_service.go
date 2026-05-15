package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/metrics"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/async"
	"github.com/tidwall/sjson"
	"gorm.io/datatypes"
)

// TODO(7.x): перейти на transactional outbox для гарантии at-least-once доставки.

var (
	ErrTaskNotFound           = errors.New("task not found")
	ErrTaskInvalidTitle       = errors.New("task title is required")
	ErrTaskInvalidPriority    = errors.New("invalid task priority")
	ErrTaskInvalidStatus      = errors.New("invalid task status")
	ErrTaskInvalidTransition  = errors.New("invalid status transition")
	ErrTaskTerminalStatus     = errors.New("task is in terminal status")
	// ErrTaskAlreadyTerminal — race condition при Cancel: задача уже завершилась
	// (done/failed/cancelled) или её прямо сейчас финализирует другой процесс.
	// Маппится в HTTP 409 Conflict с error_code task_already_terminal.
	ErrTaskAlreadyTerminal = errors.New("task is already in terminal state")
	ErrTaskConcurrentUpdate   = errors.New("task was modified concurrently, please retry")
	ErrTaskParentNotFound     = errors.New("parent task not found")
	ErrAgentNotInTeam         = errors.New("agent does not belong to project team")
	ErrTaskMessageNotFound    = errors.New("task message not found")
	ErrTaskMessageInvalidType = errors.New("invalid message type")
	ErrTaskInvalidTimeout     = errors.New("custom_timeout must be in range 1m..72h")
)

const (
	taskServiceDefaultLimit = 50
	taskServiceMaxLimit     = 200
	taskTitleMaxLen         = 500

	// minCustomTimeout / maxCustomTimeout — server-side bounds для per-task
	// override task_timeout (см. orchestration-v2-plan.md §6.5). Меньше 1m → DoS
	// через мгновенный ctx.Err() в каждом step'е (сжигает sandbox-слоты + LLM-
	// токены до needs_human). Больше 72h → orchestrator практически никогда
	// не упадёт в failed; верхняя граница пресекает int64-overflow в духе
	// `9223372036s` (≈292 года), который time.ParseDuration принимает как валидное.
	minCustomTimeout = 1 * time.Minute
	maxCustomTimeout = 72 * time.Hour
)

// parseCustomTimeout валидирует и парсит строку custom_timeout. Возвращает nil без
// ошибки если строка пуста (поле опциональное). Формат — стандартный time.ParseDuration.
func parseCustomTimeout(s string) (*models.IntervalDuration, error) {
	if s == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, ErrTaskInvalidTimeout
	}
	if d < minCustomTimeout || d > maxCustomTimeout {
		return nil, ErrTaskInvalidTimeout
	}
	iv := models.IntervalDuration(d)
	return &iv, nil
}

// allowedTransitions — Sprint 17 / Orchestration v2: упрощённый state-machine 6 состояний.
// Прежняя 10-значная pipeline-таблица (pending|planning|in_progress|review|...) сводится
// к высокоуровневым state'ам. Внутреннее течение active-задачи (через какие фазы
// прошла) отражается в artifacts, не в state.
//
// Переходы:
//   active       → done | failed | cancelled | needs_human | paused
//   paused       → active | cancelled (Sprint 17 / 6.10 — Pause/Resume v2)
//   needs_human  → active | cancelled (resume или отмена оператором)
//   failed       → active (retry с теми же параметрами)
//   done | cancelled — терминальные
var allowedTransitions = map[models.TaskState][]models.TaskState{
	models.TaskStateActive: {
		// Active → Active разрешён: метаданные task'а (assigned_agent, result,
		// artifacts, branch, context) могут обновляться без смены state.
		// В legacy 10-state модели это были переходы planning↔in_progress↔review etc.
		models.TaskStateActive,
		models.TaskStateDone,
		models.TaskStateFailed,
		models.TaskStateCancelled,
		models.TaskStateNeedsHuman,
		models.TaskStatePaused,
	},
	models.TaskStatePaused: {
		models.TaskStateActive,
		models.TaskStateCancelled,
	},
	models.TaskStateNeedsHuman: {
		models.TaskStateActive,
		models.TaskStateCancelled,
	},
	models.TaskStateFailed: {
		models.TaskStateActive,
	},
}

// TransitionOpts опции программного перехода статуса (оркестратор).
type TransitionOpts struct {
	AssignedAgentID *uuid.UUID
	Result          *string
	ErrorMessage    *string
	Artifacts       *datatypes.JSON
	BranchName      *string
	// Context обновленный JSON контекст задачи (например, iteration_count)
	Context *datatypes.JSON
}

// TaskService бизнес-логика задач и state machine.
type TaskService interface {
	Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error)
	GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error)
	List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error)
	Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error)
	Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error

	Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error)
	Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error)
	Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error)
	Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error)

	Transition(ctx context.Context, taskID uuid.UUID, newState models.TaskState, opts TransitionOpts) (*models.Task, error)

	AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error)
	ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error)

	Close() error
}

type taskService struct {
	taskRepo    repository.TaskRepository
	taskMsgRepo repository.TaskMessageRepository
	projectSvc  ProjectService
	teamSvc     TeamService
	txManager   repository.TransactionManager
	bus         events.EventBus
	indexer     indexer.TaskIndexer
	logger      *slog.Logger
	wg          sync.WaitGroup
}

// NewTaskService создаёт сервис задач.
func NewTaskService(
	taskRepo repository.TaskRepository,
	taskMsgRepo repository.TaskMessageRepository,
	projectSvc ProjectService,
	teamSvc TeamService,
	txManager repository.TransactionManager,
	bus events.EventBus,
	indexer indexer.TaskIndexer,
	logger *slog.Logger,
) TaskService {
	return &taskService{
		taskRepo:    taskRepo,
		taskMsgRepo: taskMsgRepo,
		projectSvc:  projectSvc,
		teamSvc:     teamSvc,
		txManager:   txManager,
		bus:         bus,
		indexer:     indexer,
		logger:      logger,
	}
}

func (s *taskService) Close() error {
	s.wg.Wait()
	return nil
}

func canTransition(from, to models.TaskState) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

func isTerminalTaskState(s models.TaskState) bool {
	return s == models.TaskStateDone || s == models.TaskStateCancelled
}

func normalizeTaskServicePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = taskServiceDefaultLimit
	}
	if limit > taskServiceMaxLimit {
		limit = taskServiceMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func mapTaskRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrTaskNotFound) {
		return ErrTaskNotFound
	}
	if errors.Is(err, repository.ErrTaskConcurrentUpdate) {
		return ErrTaskConcurrentUpdate
	}
	return err
}

func mapTaskMessageRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrTaskMessageNotFound) {
		return ErrTaskMessageNotFound
	}
	return err
}

func validateTaskTitle(title string) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return ErrTaskInvalidTitle
	}
	if len(t) > taskTitleMaxLen {
		return ErrTaskInvalidTitle
	}
	return nil
}

func parseTaskPriority(s string) (models.TaskPriority, error) {
	if strings.TrimSpace(s) == "" {
		return models.TaskPriorityMedium, nil
	}
	p := models.TaskPriority(s)
	if !p.IsValid() {
		return "", ErrTaskInvalidPriority
	}
	return p, nil
}

func parseTaskState(s string) (models.TaskState, error) {
	st := models.TaskState(strings.TrimSpace(s))
	if !st.IsValid() {
		return "", ErrTaskInvalidStatus
	}
	return st, nil
}

// applyTimestampsOnStateChange проставляет started_at/completed_at в зависимости
// от целевого state. Sprint 17: 10 статусов → 5 state'ов; pending+in_progress→active
// сворачиваются (одна логика "started_at"). pending→active "reopen" моделируется как
// явный Resume (failed/needs_human → active), где completed_at сбрасывается.
func applyTimestampsOnStateChange(task *models.Task, from, to models.TaskState) {
	now := time.Now().UTC()
	switch to {
	case models.TaskStateActive:
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
		// Resume (failed|needs_human|paused → active): сбрасываем completed_at, чтобы
		// заново отметить терминал при следующем переходе. paused не выставляет
		// completed_at (это не финиш), но из failed/needs_human возможно надо чистить.
		if from == models.TaskStateFailed ||
			from == models.TaskStateNeedsHuman ||
			from == models.TaskStatePaused {
			task.CompletedAt = nil
		}
	case models.TaskStateDone, models.TaskStateFailed, models.TaskStateCancelled:
		task.CompletedAt = &now
	}
}

func (s *taskService) checkAgentInTeam(ctx context.Context, projectID, agentID uuid.UUID) error {
	team, err := s.teamSvc.GetByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}
	for i := range team.Agents {
		if team.Agents[i].ID == agentID {
			return nil
		}
	}
	return ErrAgentNotInTeam
}

func (s *taskService) checkTaskAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, task *models.Task) error {
	_, err := s.projectSvc.GetByID(ctx, userID, userRole, task.ProjectID)
	return err
}

func (s *taskService) publishEventsWithTime(ctx context.Context, userRole models.UserRole, task *models.Task, prevState models.TaskState, msg *models.TaskMessage, occurredAt time.Time) {
	// Отвязываем от родительского ctx, чтобы закрытая вкладка клиента
	// не заблокировала доставку остальным подписчикам проекта.
	pubCtx := context.WithoutCancel(ctx)

	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	// Публикуем TaskStatusChanged ТОЛЬКО при реальном переходе
	if task != nil && task.State != prevState {
		agentRole := ""
		if task.AssignedAgent != nil {
			agentRole = string(task.AssignedAgent.Role)
		}

		// TODO(7.x): перейти на transactional outbox для гарантии at-least-once доставки.
		s.bus.Publish(pubCtx, events.TaskStatusChanged{
			ProjectID:       task.ProjectID,
			TaskID:          task.ID,
			ParentTaskID:    task.ParentTaskID,
			Previous:        string(prevState),
			Current:         string(task.State),
			AssignedAgentID: task.AssignedAgentID,
			AgentRole:       agentRole,
			ErrorMessage:    getSafeErrorMessage(task),
			OccurredAt:      occurredAt,
		})
	}

	// Публикуем TaskMessageCreated ТОЛЬКО если сообщение создано
	if msg != nil && task != nil {
		senderRole := ""
		if msg.SenderType == models.SenderTypeUser {
			senderRole = string(userRole)
		} else if msg.SenderType == models.SenderTypeAgent && task.AssignedAgent != nil && task.AssignedAgent.ID == msg.SenderID {
			senderRole = string(task.AssignedAgent.Role)
		}

		// TODO(7.x): перейти на transactional outbox для гарантии at-least-once доставки.
		var metadata map[string]any
		_ = json.Unmarshal(msg.Metadata, &metadata)

		s.bus.Publish(pubCtx, events.TaskMessageCreated{
			ProjectID:   task.ProjectID,
			TaskID:      msg.TaskID,
			MessageID:   msg.ID,
			SenderType:  string(msg.SenderType),
			SenderID:    msg.SenderID,
			SenderRole:  senderRole,
			MessageType: string(msg.MessageType),
			Content:     msg.Content,
			Metadata:    metadata,
			OccurredAt:  msg.CreatedAt,
		})
	}
}

func (s *taskService) publishEvents(ctx context.Context, userRole models.UserRole, task *models.Task, prevState models.TaskState, msg *models.TaskMessage) {
	s.publishEventsWithTime(ctx, userRole, task, prevState, msg, time.Now().UTC())
}

func getSafeErrorMessage(task *models.Task) string {
	if task.State == models.TaskStateFailed && task.ErrorMessage != nil {
		return *task.ErrorMessage
	}
	return ""
}

func (s *taskService) listRequestToFilter(projectID uuid.UUID, req dto.ListTasksRequest) (repository.TaskFilter, error) {
	limit, offset := normalizeTaskServicePagination(req.Limit, req.Offset)
	f := repository.TaskFilter{
		ProjectID:       &projectID,
		Limit:           limit,
		Offset:          offset,
		OrderBy:         req.OrderBy,
		OrderDir:        req.OrderDir,
		RootOnly:        req.RootOnly,
		BranchName:      req.BranchName,
		Search:          req.Search,
		AssignedAgentID: req.AssignedAgentID,
		ParentTaskID:    req.ParentTaskID,
	}
	if req.Status != nil && *req.Status != "" {
		st, err := parseTaskState(*req.Status)
		if err != nil {
			return f, err
		}
		f.State = &st
	}
	for _, raw := range req.Statuses {
		st, err := parseTaskState(raw)
		if err != nil {
			return f, err
		}
		f.States = append(f.States, st)
	}
	if req.Priority != nil && *req.Priority != "" {
		pr, err := parseTaskPriority(*req.Priority)
		if err != nil {
			return f, err
		}
		f.Priority = &pr
	}
	if req.CreatedByType != nil && *req.CreatedByType != "" {
		ct := models.CreatedByType(*req.CreatedByType)
		if !ct.IsValid() {
			return f, fmt.Errorf("invalid created_by_type")
		}
		f.CreatedByType = &ct
		f.CreatedByID = req.CreatedByID
		if req.CreatedByID == nil {
			return f, fmt.Errorf("created_by_id is required with created_by_type")
		}
	}
	return f, nil
}

func (s *taskService) indexTaskAsync(ctx context.Context, task *models.Task) {
	if task == nil {
		return
	}
	// indexer не сконфигурирован (dev/test без Weaviate) → пропускаем индексирование,
	// чтобы async.ExecuteWithRetry не упал в nil pointer panic.
	if s.indexer == nil {
		return
	}

	// Глубокое копирование задачи для предотвращения data race
	taskCopy := *task
	if task.Result != nil {
		res := *task.Result
		taskCopy.Result = &res
	}
	if task.ErrorMessage != nil {
		errStr := *task.ErrorMessage
		taskCopy.ErrorMessage = &errStr
	}
	if task.BranchName != nil {
		bn := *task.BranchName
		taskCopy.BranchName = &bn
	}
	if task.StartedAt != nil {
		sa := *task.StartedAt
		taskCopy.StartedAt = &sa
	}
	if task.CompletedAt != nil {
		ca := *task.CompletedAt
		taskCopy.CompletedAt = &ca
	}
	// Context и Artifacts (datatypes.JSON — это []byte)
	if task.Context != nil {
		taskCopy.Context = make(datatypes.JSON, len(task.Context))
		copy(taskCopy.Context, task.Context)
	}
	if task.Artifacts != nil {
		taskCopy.Artifacts = make(datatypes.JSON, len(task.Artifacts))
		copy(taskCopy.Artifacts, task.Artifacts)
	}

	async.ExecuteWithRetry(ctx, &s.wg, async.TaskOptions{
		Timeout: 2 * time.Minute,
		Retries: 3,
		LogTags: map[string]any{
			"task_id":    taskCopy.ID,
			"project_id": taskCopy.ProjectID,
			"action":     "index_task",
		},
		OnSuccess: func() {
			metrics.IncAsyncTask("index_task", "success")
		},
		OnFailure: func(err error) {
			metrics.IncAsyncTask("index_task", "error")
		},
	}, func(idxCtx context.Context) error {
		return s.indexer.IndexTaskFromModel(idxCtx, &taskCopy)
	})
}

func (s *taskService) deleteTaskAsync(ctx context.Context, taskID uuid.UUID) {
	if s.indexer == nil {
		return
	}
	async.ExecuteWithRetry(ctx, &s.wg, async.TaskOptions{
		Timeout: 1 * time.Minute,
		Retries: 3,
		LogTags: map[string]any{
			"task_id": taskID,
			"action":  "delete_task",
		},
		OnSuccess: func() {
			metrics.IncAsyncTask("delete_task", "success")
		},
		OnFailure: func(err error) {
			metrics.IncAsyncTask("delete_task", "error")
		},
	}, func(idxCtx context.Context) error {
		return s.indexer.DeleteTask(idxCtx, taskID)
	})
}

func (s *taskService) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	if _, err := s.projectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	if err := validateTaskTitle(req.Title); err != nil {
		return nil, err
	}
	priority, err := parseTaskPriority(req.Priority)
	if err != nil {
		return nil, err
	}
	ctxJSON := req.Context
	if len(ctxJSON) == 0 {
		ctxJSON = datatypes.JSON([]byte("{}"))
	}
	// CustomTimeout — per-task override task_timeout. Бэкенд-обязанность —
	// проверять bounds (см. parseCustomTimeout): клиентскому regex'у нельзя
	// доверять как единственному гарду.
	var customTimeout *models.IntervalDuration
	if req.CustomTimeout != nil {
		ct, parseErr := parseCustomTimeout(*req.CustomTimeout)
		if parseErr != nil {
			return nil, parseErr
		}
		customTimeout = ct
	}
	task := &models.Task{
		ProjectID:     projectID,
		Title:         strings.TrimSpace(req.Title),
		Description:   req.Description,
		State:         models.TaskStateActive,
		Priority:      priority,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   userID,
		Context:       ctxJSON,
		Artifacts:     datatypes.JSON([]byte("{}")),
		BranchName:    req.BranchName,
		CustomTimeout: customTimeout,
	}

	var created *models.Task
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if req.ParentTaskID != nil {
			parent, err := s.taskRepo.GetByID(txCtx, *req.ParentTaskID)
			if err != nil {
				if errors.Is(err, repository.ErrTaskNotFound) {
					return ErrTaskParentNotFound
				}
				return mapTaskRepoErr(err)
			}
			if parent.ProjectID != projectID {
				return ErrTaskParentNotFound
			}
			task.ParentTaskID = req.ParentTaskID
		}
		if req.AssignedAgentID != nil {
			if err := s.checkAgentInTeam(txCtx, projectID, *req.AssignedAgentID); err != nil {
				return err
			}
			task.AssignedAgentID = req.AssignedAgentID
		}
		if err := s.taskRepo.Create(txCtx, task); err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return fmt.Errorf("assigned agent not found: %w", err)
			}
			return err
		}
		created = task
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEvents(ctx, userRole, created, "", nil)
	s.indexTaskAsync(ctx, created)
	return created, nil
}

func (s *taskService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *taskService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	if _, err := s.projectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, 0, err
	}
	filter, err := s.listRequestToFilter(projectID, req)
	if err != nil {
		return nil, 0, err
	}
	return s.taskRepo.List(ctx, filter)
}

func (s *taskService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	var (
		updated    *models.Task
		prevState models.TaskState
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		task, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		expectedStatus := task.State
		expectedUpdatedAt := task.UpdatedAt
		prevState = task.State

		if req.Title != nil {
			if err := validateTaskTitle(*req.Title); err != nil {
				return err
			}
			task.Title = strings.TrimSpace(*req.Title)
		}
		if req.Description != nil {
			task.Description = *req.Description
		}
		if req.Priority != nil {
			p, err := parseTaskPriority(*req.Priority)
			if err != nil {
				return err
			}
			task.Priority = p
		}
		if req.ClearAssignedAgent {
			task.AssignedAgentID = nil
		} else if req.AssignedAgentID != nil {
			if err := s.checkAgentInTeam(txCtx, task.ProjectID, *req.AssignedAgentID); err != nil {
				return err
			}
			task.AssignedAgentID = req.AssignedAgentID
		}
		if req.BranchName != nil {
			task.BranchName = req.BranchName
		}
		if req.CustomTimeout != nil {
			if *req.CustomTimeout == "" {
				task.CustomTimeout = nil
			} else {
				ct, parseErr := parseCustomTimeout(*req.CustomTimeout)
				if parseErr != nil {
					return parseErr
				}
				task.CustomTimeout = ct
			}
		}
		if req.Status != nil {
			newState, err := parseTaskState(*req.Status)
			if err != nil {
				return err
			}
			if newState != task.State {
				if isTerminalTaskState(task.State) {
					return ErrTaskTerminalStatus
				}
				// Sprint 17: легаси-ограничение "review/testing → in_progress только через correct API"
				// больше не применимо в 5-state модели — этот guard убран. Все правомерные переходы
				// в active разрешаются стандартным canTransition (needs_human/failed → active).
				if !canTransition(task.State, newState) {
					return ErrTaskInvalidTransition
				}
				task.State = newState
				applyTimestampsOnStateChange(task, prevState, newState)
			}
		}
		if err := s.taskRepo.Update(txCtx, task, expectedStatus, expectedUpdatedAt); err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return fmt.Errorf("assigned agent not found: %w", err)
			}
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevState, nil, occurredAt)
	if isTerminalTaskState(updated.State) {
		s.indexTaskAsync(ctx, updated)
	}
	return updated, nil
}

func (s *taskService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return err
	}
	if err := s.taskRepo.Delete(ctx, taskID); err != nil {
		return err
	}
	s.deleteTaskAsync(ctx, taskID)
	return nil
}

func (s *taskService) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevState models.TaskState
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// SELECT ... FOR UPDATE NOWAIT — pause-vs-finalization race: если строку
		// держит воркер прямо сейчас, не блокируем UI, отдаём 409.
		task, err := s.taskRepo.GetByIDForUpdate(txCtx, taskID)
		if err != nil {
			if errors.Is(err, repository.ErrTaskLocked) {
				return ErrTaskAlreadyTerminal
			}
			return mapTaskRepoErr(err)
		}
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		if isTerminalTaskState(task.State) {
			return ErrTaskAlreadyTerminal
		}
		expectedStatus := task.State
		expectedUpdatedAt := task.UpdatedAt
		// Sprint 17 / 6.10: Pause → state='paused' (не needs_human). Воркеры при pickup
		// видят non-active state и пропускают шаг до Resume.
		if !canTransition(task.State, models.TaskStatePaused) {
			return ErrTaskInvalidTransition
		}
		prevState = task.State
		task.State = models.TaskStatePaused
		if err := s.taskRepo.Update(txCtx, task, expectedStatus, expectedUpdatedAt); err != nil {
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevState, nil, occurredAt)
	if isTerminalTaskState(updated.State) {
		s.indexTaskAsync(ctx, updated)
	}
	return updated, nil
}

func (s *taskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevState  models.TaskState
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// SELECT ... FOR UPDATE NOWAIT — защита от race condition cancel-vs-finalization.
		// Если строку прямо сейчас держит worker (финализирует задачу) — NOWAIT даст 55P03;
		// трактуем как «задача уже завершается», возвращаем ErrTaskAlreadyTerminal (HTTP 409).
		task, err := s.taskRepo.GetByIDForUpdate(txCtx, taskID)
		if err != nil {
			if errors.Is(err, repository.ErrTaskLocked) {
				return ErrTaskAlreadyTerminal
			}
			return mapTaskRepoErr(err)
		}
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		// После lock state не изменится до COMMIT — но если он уже terminal,
		// этот же 409 сигнализирует фронту что cancel опоздал.
		if isTerminalTaskState(task.State) {
			return ErrTaskAlreadyTerminal
		}
		expectedStatus := task.State
		expectedUpdatedAt := task.UpdatedAt
		if !canTransition(task.State, models.TaskStateCancelled) {
			return ErrTaskInvalidTransition
		}
		prevState = task.State
		task.State = models.TaskStateCancelled
		applyTimestampsOnStateChange(task, prevState, models.TaskStateCancelled)
		if err := s.taskRepo.Update(txCtx, task, expectedStatus, expectedUpdatedAt); err != nil {
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevState, nil, occurredAt)
	if isTerminalTaskState(updated.State) {
		s.indexTaskAsync(ctx, updated)
	}
	return updated, nil
}

func (s *taskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevState models.TaskState
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		task, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		expectedStatus := task.State
		expectedUpdatedAt := task.UpdatedAt
		// Sprint 17 / 6.10: Resume теперь поддерживает paused (новое v2-состояние) дополнительно
		// к legacy needs_human/failed. allowedTransitions гарантирует корректность переходов.
		if task.State != models.TaskStateNeedsHuman &&
			task.State != models.TaskStateFailed &&
			task.State != models.TaskStatePaused {
			return ErrTaskInvalidTransition
		}
		if !canTransition(task.State, models.TaskStateActive) {
			return ErrTaskInvalidTransition
		}
		prevState = task.State
		task.State = models.TaskStateActive
		applyTimestampsOnStateChange(task, prevState, models.TaskStateActive)
		if err := s.taskRepo.Update(txCtx, task, expectedStatus, expectedUpdatedAt); err != nil {
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevState, nil, occurredAt)
	if isTerminalTaskState(updated.State) {
		s.indexTaskAsync(ctx, updated)
	}
	return updated, nil
}

func (s *taskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	sanitized, err := ValidateAndSanitizeUserCorrection(text)
	if err != nil {
		return nil, err
	}

	var (
		updated    *models.Task
		prevState models.TaskState
		msg        *models.TaskMessage
		occurredAt time.Time
	)

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		task, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		if isTerminalTaskState(task.State) ||
			task.State == models.TaskStateNeedsHuman ||
			task.State == models.TaskStatePaused {
			return ErrTaskInvalidTransition
		}

		newContextBytes, err := sjson.SetBytes(task.Context, "user_correction", sanitized)
		if err != nil {
			return fmt.Errorf("failed to patch task context: %w", err)
		}
		newContextBytes, err = sjson.SetBytes(newContextBytes, "user_correction_at", time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("failed to patch task context: %w", err)
		}
		newContext := datatypes.JSON(newContextBytes)

		m := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeUser,
			SenderID:    userID,
			Content:     FormatCorrectionForPrompt(sanitized),
			MessageType: models.MessageTypeFeedback,
			Metadata:    datatypes.JSON([]byte("{}")),
		}
		if err := s.taskMsgRepo.Create(txCtx, m); err != nil {
			return mapTaskRepoErr(err)
		}
		msg = m

		prevState = task.State
		expectedUpdatedAt := task.UpdatedAt
		// Sprint 17: коллапс 10→5 убрал распознавание review→in_progress/changes_requested→in_progress
		// (всё active). Correct теперь обновляет ТОЛЬКО context; state остаётся прежним.
		// Если задача в needs_human/failed — оператор сам Resume'ит её отдельным вызовом.
		nextState := prevState

		task.Context = newContext
		if nextState != prevState {
			if !canTransition(prevState, nextState) {
				return ErrTaskInvalidTransition
			}
			task.State = nextState
			applyTimestampsOnStateChange(task, prevState, nextState)
		}

		if err := s.taskRepo.Update(txCtx, task, prevState, expectedUpdatedAt); err != nil {
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevState, msg, occurredAt)
	s.indexTaskAsync(ctx, updated)
	return updated, nil
}

func (s *taskService) Transition(ctx context.Context, taskID uuid.UUID, newState models.TaskState, opts TransitionOpts) (*models.Task, error) {
	if !newState.IsValid() {
		return nil, ErrTaskInvalidStatus
	}

	var (
		updated    *models.Task
		from       models.TaskState
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		task, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		from = task.State
		expectedUpdatedAt := task.UpdatedAt
		if isTerminalTaskState(from) {
			return ErrTaskTerminalStatus
		}
		if !canTransition(from, newState) {
			return ErrTaskInvalidTransition
		}
		if opts.AssignedAgentID != nil {
			if err := s.checkAgentInTeam(txCtx, task.ProjectID, *opts.AssignedAgentID); err != nil {
				return err
			}
			task.AssignedAgentID = opts.AssignedAgentID
		}
		if opts.Result != nil {
			task.Result = opts.Result
		}
		if opts.ErrorMessage != nil {
			task.ErrorMessage = opts.ErrorMessage
		}
		if opts.Artifacts != nil {
			art := *opts.Artifacts
			if len(art) == 0 {
				art = datatypes.JSON([]byte("{}"))
			}
			task.Artifacts = art
		}
		if opts.BranchName != nil {
			task.BranchName = opts.BranchName
		}
		if opts.Context != nil {
			task.Context = *opts.Context
		}
		task.State = newState
		applyTimestampsOnStateChange(task, from, newState)
		if err := s.taskRepo.Update(txCtx, task, from, expectedUpdatedAt); err != nil {
			if errors.Is(err, repository.ErrAgentNotFound) {
				return fmt.Errorf("assigned agent not found: %w", err)
			}
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, models.RoleUser, updated, from, nil, occurredAt)
	if isTerminalTaskState(updated.State) {
		s.indexTaskAsync(ctx, updated)
	}
	return updated, nil
}

func (s *taskService) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	var (
		createdMsg *models.TaskMessage
		task       *models.Task
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		t, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		task = t
		if err := s.checkTaskAccess(txCtx, userID, userRole, task); err != nil {
			return err
		}
		mt := models.MessageType(req.MessageType)
		if !mt.IsValid() {
			return ErrTaskMessageInvalidType
		}
		meta := req.Metadata
		if len(meta) == 0 {
			meta = datatypes.JSON([]byte("{}"))
		}
		msg := &models.TaskMessage{
			TaskID:      task.ID,
			SenderType:  models.SenderTypeUser,
			SenderID:    userID,
			Content:     req.Content,
			MessageType: mt,
			Metadata:    meta,
		}
		if err := s.taskMsgRepo.Create(txCtx, msg); err != nil {
			return mapTaskRepoErr(err)
		}
		m, err := s.taskMsgRepo.GetByID(txCtx, msg.ID)
		if err != nil {
			return mapTaskMessageRepoErr(err)
		}
		createdMsg = m
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEvents(ctx, userRole, task, task.State, createdMsg)

	// Индексируем только важные типы сообщений
	mt := createdMsg.MessageType
	if mt == models.MessageTypeResult || mt == models.MessageTypeSummary || mt == models.MessageTypeFeedback {
		s.indexTaskAsync(ctx, task)
	}

	return createdMsg, nil
}

func (s *taskService) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, 0, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeTaskServicePagination(req.Limit, req.Offset)
	f := repository.TaskMessageFilter{
		Limit:  limit,
		Offset: offset,
	}
	if req.MessageType != nil && *req.MessageType != "" {
		mt := models.MessageType(*req.MessageType)
		if !mt.IsValid() {
			return nil, 0, ErrTaskMessageInvalidType
		}
		f.MessageType = &mt
	}
	if req.SenderType != nil && *req.SenderType != "" {
		st := models.SenderType(*req.SenderType)
		if !st.IsValid() {
			return nil, 0, fmt.Errorf("invalid sender_type")
		}
		f.SenderType = &st
	}
	return s.taskMsgRepo.ListByTaskID(ctx, taskID, f)
}
