package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/tidwall/sjson"
	"gorm.io/datatypes"
)

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
}

// NewTaskService создаёт сервис задач.
func NewTaskService(
	taskRepo repository.TaskRepository,
	taskMsgRepo repository.TaskMessageRepository,
	projectSvc ProjectService,
	teamSvc TeamService,
) TaskService {
	return &taskService{
		taskRepo:    taskRepo,
		taskMsgRepo: taskMsgRepo,
		projectSvc:  projectSvc,
		teamSvc:     teamSvc,
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
	if req.ParentTaskID != nil {
		parent, err := s.taskRepo.GetByID(ctx, *req.ParentTaskID)
		if err != nil {
			if errors.Is(err, repository.ErrTaskNotFound) {
				return nil, ErrTaskParentNotFound
			}
			return nil, mapTaskRepoErr(err)
		}
		if parent.ProjectID != projectID {
			return nil, ErrTaskParentNotFound
		}
		task.ParentTaskID = req.ParentTaskID
	}
	if req.AssignedAgentID != nil {
		if err := s.checkAgentInTeam(ctx, projectID, *req.AssignedAgentID); err != nil {
			return nil, err
		}
		task.AssignedAgentID = req.AssignedAgentID
	}
	if err := s.taskRepo.Create(ctx, task); err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, fmt.Errorf("assigned agent not found: %w", err)
		}
		return nil, err
	}
	return s.taskRepo.GetByID(ctx, task.ID)
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
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	expectedStatus := task.Status
	expectedUpdatedAt := task.UpdatedAt
	prevStatus := task.Status

	if req.Title != nil {
		if err := validateTaskTitle(*req.Title); err != nil {
			return nil, err
		}
		task.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Priority != nil {
		p, err := parseTaskPriority(*req.Priority)
		if err != nil {
			return nil, err
		}
		task.Priority = p
	}
	if req.ClearAssignedAgent {
		task.AssignedAgentID = nil
	} else if req.AssignedAgentID != nil {
		if err := s.checkAgentInTeam(ctx, task.ProjectID, *req.AssignedAgentID); err != nil {
			return nil, err
		}
		task.AssignedAgentID = req.AssignedAgentID
	}
	if req.BranchName != nil {
		task.BranchName = req.BranchName
	}
	if req.Status != nil {
		newStatus, err := parseTaskStatus(*req.Status)
		if err != nil {
			return nil, err
		}
		if newStatus != task.Status {
			if isTerminalTaskStatus(task.Status) {
				return nil, ErrTaskTerminalStatus
			}
			// Переход review/testing → in_progress только через API correct (задача 6.7).
			if newStatus == models.TaskStatusInProgress {
				if task.Status == models.TaskStatusReview || task.Status == models.TaskStatusTesting {
					return nil, ErrTaskInvalidTransition
				}
			}
			if !canTransition(task.Status, newStatus) {
				return nil, ErrTaskInvalidTransition
			}
			task.Status = newStatus
			applyTimestampsOnStatusChange(task, prevStatus, newStatus)
		}
	}
	if err := s.taskRepo.Update(ctx, task, expectedStatus, expectedUpdatedAt); err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, fmt.Errorf("assigned agent not found: %w", err)
		}
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
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
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	expectedStatus := task.Status
	expectedUpdatedAt := task.UpdatedAt
	if !canTransition(task.Status, models.TaskStatusPaused) {
		return nil, ErrTaskInvalidTransition
	}
	task.Status = models.TaskStatusPaused
	if err := s.taskRepo.Update(ctx, task, expectedStatus, expectedUpdatedAt); err != nil {
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	expectedStatus := task.Status
	expectedUpdatedAt := task.UpdatedAt
	if !canTransition(task.Status, models.TaskStatusCancelled) {
		return nil, ErrTaskInvalidTransition
	}
	prev := task.Status
	task.Status = models.TaskStatusCancelled
	applyTimestampsOnStatusChange(task, prev, models.TaskStatusCancelled)
	if err := s.taskRepo.Update(ctx, task, expectedStatus, expectedUpdatedAt); err != nil {
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	expectedStatus := task.Status
	expectedUpdatedAt := task.UpdatedAt
	if task.Status != models.TaskStatusPaused && task.Status != models.TaskStatusFailed {
		return nil, ErrTaskInvalidTransition
	}
	if !canTransition(task.Status, models.TaskStatusPending) {
		return nil, ErrTaskInvalidTransition
	}
	prev := task.Status
	task.Status = models.TaskStatusPending
	applyTimestampsOnStatusChange(task, prev, models.TaskStatusPending)
	if err := s.taskRepo.Update(ctx, task, expectedStatus, expectedUpdatedAt); err != nil {
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	sanitized, err := ValidateAndSanitizeUserCorrection(text)
	if err != nil {
		return nil, err
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	if isTerminalTaskStatus(task.Status) || task.Status == models.TaskStatusPaused {
		return nil, ErrTaskInvalidTransition
	}

	newContextBytes, err := sjson.SetBytes(task.Context, "user_correction", sanitized)
	if err != nil {
		return nil, fmt.Errorf("failed to patch task context: %w", err)
	}
	newContextBytes, err = sjson.SetBytes(newContextBytes, "user_correction_at", time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("failed to patch task context: %w", err)
	}
	newContext := datatypes.JSON(newContextBytes)

	msg := &models.TaskMessage{
		TaskID:      task.ID,
		SenderType:  models.SenderTypeUser,
		SenderID:    userID,
		Content:     FormatCorrectionForPrompt(sanitized),
		MessageType: models.MessageTypeFeedback,
		Metadata:    datatypes.JSON([]byte("{}")),
	}
	if err := s.taskMsgRepo.Create(ctx, msg); err != nil {
		return nil, mapTaskRepoErr(err)
	}

	from := task.Status
	expectedUpdatedAt := task.UpdatedAt
	nextStatus := from
	switch from {
	case models.TaskStatusReview, models.TaskStatusTesting:
		nextStatus = models.TaskStatusInProgress
	default:
		// planning, in_progress, changes_requested — только обновление контекста
	}

	task.Context = newContext
	if nextStatus != from {
		if !canTransition(from, nextStatus) {
			return nil, ErrTaskInvalidTransition
		}
		task.Status = nextStatus
		applyTimestampsOnStatusChange(task, from, nextStatus)
	}

	if err := s.taskRepo.Update(ctx, task, from, expectedUpdatedAt); err != nil {
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskService) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error) {
	if !newStatus.IsValid() {
		return nil, ErrTaskInvalidStatus
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	from := task.Status
	expectedUpdatedAt := task.UpdatedAt
	if isTerminalTaskStatus(from) {
		return nil, ErrTaskTerminalStatus
	}
	if !canTransition(from, newStatus) {
		return nil, ErrTaskInvalidTransition
	}
	if opts.AssignedAgentID != nil {
		if err := s.checkAgentInTeam(ctx, task.ProjectID, *opts.AssignedAgentID); err != nil {
			return nil, err
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
	if err := s.taskRepo.Update(ctx, task, from, expectedUpdatedAt); err != nil {
		if errors.Is(err, repository.ErrAgentNotFound) {
			return nil, fmt.Errorf("assigned agent not found: %w", err)
		}
		return nil, mapTaskRepoErr(err)
	}
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *taskService) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, mapTaskRepoErr(err)
	}
	if err := s.checkTaskAccess(ctx, userID, userRole, task); err != nil {
		return nil, err
	}
	mt := models.MessageType(req.MessageType)
	if !mt.IsValid() {
		return nil, ErrTaskMessageInvalidType
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
	if err := s.taskMsgRepo.Create(ctx, msg); err != nil {
		return nil, mapTaskRepoErr(err)
	}
	return s.taskMsgRepo.GetByID(ctx, msg.ID)
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
