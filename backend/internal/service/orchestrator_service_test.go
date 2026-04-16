package service

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
)

type mockOrchestratorWorkflowRepository struct{ mock.Mock }

func (m *mockOrchestratorWorkflowRepository) CreateWorkflow(ctx context.Context, wf *models.Workflow) error {
	return m.Called(ctx, wf).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) GetWorkflowByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Workflow), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) CreateAgent(ctx context.Context, a *models.Agent) error {
	return m.Called(ctx, a).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) GetAgentByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) CreateExecution(ctx context.Context, exec *models.Execution) error {
	return m.Called(ctx, exec).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) GetExecutionByID(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) UpdateExecution(ctx context.Context, exec *models.Execution) error {
	return m.Called(ctx, exec).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.Execution), args.Get(1).(int64), args.Error(2)
}
func (m *mockOrchestratorWorkflowRepository) AddExecutionStep(ctx context.Context, step *models.ExecutionStep) error {
	return m.Called(ctx, step).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) GetExecutionSteps(ctx context.Context, executionID uuid.UUID) ([]models.ExecutionStep, error) {
	args := m.Called(ctx, executionID)
	return args.Get(0).([]models.ExecutionStep), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) GetNextPendingExecution(ctx context.Context) (*models.Execution, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) CreateScheduledWorkflow(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	return m.Called(ctx, schedule).Error(0)
}
func (m *mockOrchestratorWorkflowRepository) ListActiveSchedules(ctx context.Context) ([]models.ScheduledWorkflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.ScheduledWorkflow), args.Error(1)
}
func (m *mockOrchestratorWorkflowRepository) UpdateSchedule(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	return m.Called(ctx, schedule).Error(0)
}

type mockAgentExecutor struct{ mock.Mock }

func (m *mockAgentExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*agent.ExecutionResult), args.Error(1)
}

type mockOrchestratorTransactionManager struct{}

func (m *mockOrchestratorTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockTaskService struct{ mock.Mock }

func (m *mockTaskService) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	return args.Get(0).([]models.Task), args.Get(1).(int64), args.Error(2)
}
func (m *mockTaskService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, taskID).Error(0)
}
func (m *mockTaskService) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts TransitionOpts) (*models.Task, error) {
	args := m.Called(ctx, taskID, newStatus, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskService) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}
func (m *mockTaskService) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	return args.Get(0).([]models.TaskMessage), args.Get(1).(int64), args.Error(2)
}

type mockOrchestratorTaskMessageRepository struct{ mock.Mock }

func (m *mockOrchestratorTaskMessageRepository) Create(ctx context.Context, msg *models.TaskMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *mockOrchestratorTaskMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskMessage, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}
func (m *mockOrchestratorTaskMessageRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, taskID, filter)
	return args.Get(0).([]models.TaskMessage), args.Get(1).(int64), args.Error(2)
}
func (m *mockOrchestratorTaskMessageRepository) ListBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, senderType, senderID, filter)
	return args.Get(0).([]models.TaskMessage), args.Get(1).(int64), args.Error(2)
}
func (m *mockOrchestratorTaskMessageRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(int64), args.Error(1)
}

func TestOrchestratorProcessTask_Success(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{})

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB)

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()

	task := &models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &agentID,
	}

	project := &models.Project{ID: projectID}
	agentModel := &models.Agent{
		ID:   agentID,
		Role: models.AgentRolePlanner,
	}

	// Первая итерация: Pending -> Planning
	tr.On("GetByID", ctx, taskID).Return(task, nil).Once()
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(project, nil)
	wr.On("GetAgentByID", ctx, agentID).Return(agentModel, nil)
	
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)

	le.On("Execute", ctx, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "plan",
	}, nil).Once()

	ts.On("Transition", ctx, taskID, models.TaskStatusPlanning, mock.Anything).Return(&models.Task{
		ID:     taskID,
		Status: models.TaskStatusPlanning,
	}, nil)

	// После перехода перечитываем задачу
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusCompleted, // Терминальный статус для выхода
		AssignedAgentID: &agentID,
	}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)
	
	tr.AssertExpectations(t)
	le.AssertExpectations(t)
	ts.AssertExpectations(t)
}

func TestOrchestratorProcessTask_ZombieRecovery(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{})

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	zombieTaskID := uuid.New()
	
	// Список "зомби" задач
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool {
		return f.UpdatedAtBefore != nil && len(f.Statuses) > 0
	})).Return([]models.Task{
		{ID: zombieTaskID, Status: models.TaskStatusInProgress, UpdatedAt: time.Now().Add(-2 * time.Hour)},
	}, int64(1), nil)
	
	ts.On("Transition", ctx, zombieTaskID, models.TaskStatusFailed, mock.MatchedBy(func(opts TransitionOpts) bool {
		return opts.ErrorMessage != nil && *opts.ErrorMessage == "Task timed out (zombie detection)"
	})).Return(&models.Task{}, nil)
	
	err := svc.Start(ctx)
	require.NoError(t, err)
	
	// Даем тикеру немного времени (хотя в тесте мы проверяем только начальный вызов Start)
	time.Sleep(10 * time.Millisecond)
	
	tr.AssertExpectations(t)
	ts.AssertExpectations(t)
}
