package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
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
	ErrTaskConcurrentUpdate   = errors.New("task was modified concurrently, please retry")
	ErrTaskParentNotFound     = errors.New("parent task not found")
	ErrAgentNotInTeam         = errors.New("agent does not belong to project team")
	ErrTaskMessageNotFound    = errors.New("task message not found")
	ErrTaskMessageInvalidType = errors.New("invalid message type")
)

const (
	taskServiceDefaultLimit = 50
	taskServiceMaxLimit     = 200
	taskTitleMaxLen         = 500
)

var allowedTransitions = map[models.TaskStatus][]models.TaskStatus{
	models.TaskStatusPending:          {models.TaskStatusPlanning, models.TaskStatusCancelled},
	models.TaskStatusPlanning:         {models.TaskStatusInProgress, models.TaskStatusFailed, models.TaskStatusCancelled, models.TaskStatusPaused},
	models.TaskStatusInProgress:       {models.TaskStatusReview, models.TaskStatusFailed, models.TaskStatusCancelled, models.TaskStatusPaused},
	models.TaskStatusReview:           {models.TaskStatusTesting, models.TaskStatusChangesRequested, models.TaskStatusInProgress, models.TaskStatusFailed, models.TaskStatusCancelled, models.TaskStatusPaused},
	models.TaskStatusChangesRequested: {models.TaskStatusInProgress, models.TaskStatusCancelled, models.TaskStatusPaused},
	models.TaskStatusTesting:          {models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusInProgress, models.TaskStatusCancelled, models.TaskStatusPaused},
	models.TaskStatusPaused:           {models.TaskStatusPending},
	models.TaskStatusFailed:           {models.TaskStatusPending},
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

	Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error)

	AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error)
	ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error)
}

type taskService struct {
	taskRepo    repository.TaskRepository
	taskMsgRepo repository.TaskMessageRepository
	projectSvc  ProjectService
	teamSvc     TeamService
	txManager   repository.TransactionManager
	bus         events.EventBus
}

// NewTaskService создаёт сервис задач.
func NewTaskService(
	taskRepo repository.TaskRepository,
	taskMsgRepo repository.TaskMessageRepository,
	projectSvc ProjectService,
	teamSvc TeamService,
	txManager repository.TransactionManager,
	bus events.EventBus,
) TaskService {
	return &taskService{
		taskRepo:    taskRepo,
		taskMsgRepo: taskMsgRepo,
		projectSvc:  projectSvc,
		teamSvc:     teamSvc,
		txManager:   txManager,
		bus:         bus,
	}
}

func canTransition(from, to models.TaskStatus) bool {
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

func isTerminalTaskStatus(s models.TaskStatus) bool {
	return s == models.TaskStatusCompleted || s == models.TaskStatusCancelled
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

func parseTaskStatus(s string) (models.TaskStatus, error) {
	st := models.TaskStatus(strings.TrimSpace(s))
	if !st.IsValid() {
		return "", ErrTaskInvalidStatus
	}
	return st, nil
}

func applyTimestampsOnStatusChange(task *models.Task, _from, to models.TaskStatus) {
	now := time.Now().UTC()
	switch to {
	case models.TaskStatusInProgress:
		if task.StartedAt == nil {
			task.StartedAt = &now
		}
	case models.TaskStatusCompleted, models.TaskStatusFailed, models.TaskStatusCancelled:
		task.CompletedAt = &now
	case models.TaskStatusPending:
		task.CompletedAt = nil
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

func (s *taskService) publishEventsWithTime(ctx context.Context, userRole models.UserRole, task *models.Task, prevStatus models.TaskStatus, msg *models.TaskMessage, occurredAt time.Time) {
	// Отвязываем от родительского ctx, чтобы закрытая вкладка клиента
	// не заблокировала доставку остальным подписчикам проекта.
	pubCtx := context.WithoutCancel(ctx)

	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	// Публикуем TaskStatusChanged ТОЛЬКО при реальном переходе
	if task != nil && task.Status != prevStatus {
		agentRole := ""
		if task.AssignedAgent != nil {
			agentRole = string(task.AssignedAgent.Role)
		}

		// TODO(7.x): перейти на transactional outbox для гарантии at-least-once доставки.
		s.bus.Publish(pubCtx, events.TaskStatusChanged{
			ProjectID:       task.ProjectID,
			TaskID:          task.ID,
			ParentTaskID:    task.ParentTaskID,
			Previous:        string(prevStatus),
			Current:         string(task.Status),
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

func (s *taskService) publishEvents(ctx context.Context, userRole models.UserRole, task *models.Task, prevStatus models.TaskStatus, msg *models.TaskMessage) {
	s.publishEventsWithTime(ctx, userRole, task, prevStatus, msg, time.Now().UTC())
}

func getSafeErrorMessage(task *models.Task) string {
	if task.Status == models.TaskStatusFailed && task.ErrorMessage != nil {
		return *task.ErrorMessage
	}
	return ""
}

func (s *taskService) listRequestToFilter(projectID uuid.UUID, req dto.ListTasksRequest) (repository.TaskFilter, error) {
	limit, offset := normalizeTaskServicePagination(req.Limit, req.Offset)
	f := repository.TaskFilter{
		ProjectID:    &projectID,
		Limit:        limit,
		Offset:       offset,
		OrderBy:      req.OrderBy,
		OrderDir:     req.OrderDir,
		RootOnly:     req.RootOnly,
		BranchName:   req.BranchName,
		Search:       req.Search,
		AssignedAgentID: req.AssignedAgentID,
		ParentTaskID: req.ParentTaskID,
	}
	if req.Status != nil && *req.Status != "" {
		st, err := parseTaskStatus(*req.Status)
		if err != nil {
			return f, err
		}
		f.Status = &st
	}
	for _, raw := range req.Statuses {
		st, err := parseTaskStatus(raw)
		if err != nil {
			return f, err
		}
		f.Statuses = append(f.Statuses, st)
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
	task := &models.Task{
		ProjectID:     projectID,
		Title:         strings.TrimSpace(req.Title),
		Description:   req.Description,
		Status:        models.TaskStatusPending,
		Priority:      priority,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   userID,
		Context:       ctxJSON,
		Artifacts:     datatypes.JSON([]byte("{}")),
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
		prevStatus models.TaskStatus
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
		expectedStatus := task.Status
		expectedUpdatedAt := task.UpdatedAt
		prevStatus = task.Status

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
		if req.Status != nil {
			newStatus, err := parseTaskStatus(*req.Status)
			if err != nil {
				return err
			}
			if newStatus != task.Status {
				if isTerminalTaskStatus(task.Status) {
					return ErrTaskTerminalStatus
				}
				// Переход review/testing → in_progress только через API correct (задача 6.7).
				if newStatus == models.TaskStatusInProgress {
					if task.Status == models.TaskStatusReview || task.Status == models.TaskStatusTesting {
						return ErrTaskInvalidTransition
					}
				}
				if !canTransition(task.Status, newStatus) {
					return ErrTaskInvalidTransition
				}
				task.Status = newStatus
				applyTimestampsOnStatusChange(task, prevStatus, newStatus)
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

	s.publishEventsWithTime(ctx, userRole, updated, prevStatus, nil, occurredAt)
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
	return s.taskRepo.Delete(ctx, taskID)
}

func (s *taskService) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevStatus models.TaskStatus
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
		expectedStatus := task.Status
		expectedUpdatedAt := task.UpdatedAt
		if !canTransition(task.Status, models.TaskStatusPaused) {
			return ErrTaskInvalidTransition
		}
		prevStatus = task.Status
		task.Status = models.TaskStatusPaused
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

	s.publishEventsWithTime(ctx, userRole, updated, prevStatus, nil, occurredAt)
	return updated, nil
}

func (s *taskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevStatus models.TaskStatus
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
		expectedStatus := task.Status
		expectedUpdatedAt := task.UpdatedAt
		if !canTransition(task.Status, models.TaskStatusCancelled) {
			return ErrTaskInvalidTransition
		}
		prevStatus = task.Status
		task.Status = models.TaskStatusCancelled
		applyTimestampsOnStatusChange(task, prevStatus, models.TaskStatusCancelled)
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

	s.publishEventsWithTime(ctx, userRole, updated, prevStatus, nil, occurredAt)
	return updated, nil
}

func (s *taskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	var (
		updated    *models.Task
		prevStatus models.TaskStatus
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
		expectedStatus := task.Status
		expectedUpdatedAt := task.UpdatedAt
		if task.Status != models.TaskStatusPaused && task.Status != models.TaskStatusFailed {
			return ErrTaskInvalidTransition
		}
		if !canTransition(task.Status, models.TaskStatusPending) {
			return ErrTaskInvalidTransition
		}
		prevStatus = task.Status
		task.Status = models.TaskStatusPending
		applyTimestampsOnStatusChange(task, prevStatus, models.TaskStatusPending)
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

	s.publishEventsWithTime(ctx, userRole, updated, prevStatus, nil, occurredAt)
	return updated, nil
}

func (s *taskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	sanitized, err := ValidateAndSanitizeUserCorrection(text)
	if err != nil {
		return nil, err
	}

	var (
		updated    *models.Task
		prevStatus models.TaskStatus
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
		if isTerminalTaskStatus(task.Status) || task.Status == models.TaskStatusPaused {
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

		prevStatus = task.Status
		expectedUpdatedAt := task.UpdatedAt
		nextStatus := prevStatus
		switch prevStatus {
		case models.TaskStatusReview, models.TaskStatusTesting:
			nextStatus = models.TaskStatusInProgress
		default:
			// planning, in_progress, changes_requested — только обновление контекста
		}

		task.Context = newContext
		if nextStatus != prevStatus {
			if !canTransition(prevStatus, nextStatus) {
				return ErrTaskInvalidTransition
			}
			task.Status = nextStatus
			applyTimestampsOnStatusChange(task, prevStatus, nextStatus)
		}

		if err := s.taskRepo.Update(txCtx, task, prevStatus, expectedUpdatedAt); err != nil {
			return mapTaskRepoErr(err)
		}
		updated = task
		occurredAt = task.UpdatedAt
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.publishEventsWithTime(ctx, userRole, updated, prevStatus, msg, occurredAt)
	return updated, nil
}

func (s *taskService) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error) {
	if !newStatus.IsValid() {
		return nil, ErrTaskInvalidStatus
	}

	var (
		updated    *models.Task
		from       models.TaskStatus
		occurredAt time.Time
	)

	err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		task, err := s.taskRepo.GetByID(txCtx, taskID)
		if err != nil {
			return mapTaskRepoErr(err)
		}
		from = task.Status
		expectedUpdatedAt := task.UpdatedAt
		if isTerminalTaskStatus(from) {
			return ErrTaskTerminalStatus
		}
		if !canTransition(from, newStatus) {
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
		task.Status = newStatus
		applyTimestampsOnStatusChange(task, from, newStatus)
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

	s.publishEvents(ctx, userRole, task, task.Status, createdMsg)
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
