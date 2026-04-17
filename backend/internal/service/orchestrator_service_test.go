package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
func (m *mockTaskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

func TestPollIndicatesStepRestart(t *testing.T) {
	require.True(t, pollIndicatesStepRestart(models.TaskStatusReview, models.TaskStatusInProgress))
	require.True(t, pollIndicatesStepRestart(models.TaskStatusTesting, models.TaskStatusInProgress))
	require.True(t, pollIndicatesStepRestart(models.TaskStatusChangesRequested, models.TaskStatusInProgress))
	require.False(t, pollIndicatesStepRestart(models.TaskStatusInProgress, models.TaskStatusInProgress))
	require.False(t, pollIndicatesStepRestart(models.TaskStatusReview, models.TaskStatusReview))
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
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

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
	tr.On("GetByID", mock.Anything, taskID).Return(task, nil).Once() // finishStep до Transition
	wr.On("GetAgentByID", mock.Anything, agentID).Return(agentModel, nil)
	
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)

	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
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
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))
	
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

// TestOrchestratorProcessTask_FullHappyPath проверяет полный путь: Pending -> Planning -> InProgress -> Review -> Testing -> Completed
func TestOrchestratorProcessTask_FullHappyPath(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	plannerAgentID := uuid.New()
	developerAgentID := uuid.New()
	reviewerAgentID := uuid.New()
	testerAgentID := uuid.New()

	// Step 1: Pending -> Planning (Planner LLM Agent)
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &plannerAgentID,
	}, nil).Once()
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &plannerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, plannerAgentID).Return(&models.Agent{
		ID:   plannerAgentID,
		Role: models.AgentRolePlanner,
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Task plan created",
		ArtifactsJSON: []byte(`{"plan": ["Step 1", "Step 2"]}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusPlanning, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusPlanning,
		AssignedAgentID: &plannerAgentID,
	}, nil).Once()

	// Step 2: Planning -> InProgress (Developer Sandbox Agent)
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPlanning,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPlanning,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, developerAgentID).Return(&models.Agent{
		ID:          developerAgentID,
		Role:        models.AgentRoleDeveloper,
			CodeBackend: &[]models.CodeBackend{models.CodeBackendClaudeCode}[0],
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Code implemented",
		ArtifactsJSON: []byte(`{"diff": "+code", "files": ["main.go"]}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()

	// Step 3: InProgress -> Review (Review happens after development)
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &reviewerAgentID,
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &reviewerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, reviewerAgentID).Return(&models.Agent{
		ID:   reviewerAgentID,
		Role: models.AgentRoleReviewer,
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Code reviewed and approved",
		ArtifactsJSON: []byte(`{"decision": "approved"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusReview, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &reviewerAgentID,
	}, nil).Once()

	// Step 4: Review -> Testing (Reviewer approved)
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, testerAgentID).Return(&models.Agent{
		ID:          testerAgentID,
		Role:        models.AgentRoleTester,
			CodeBackend: &[]models.CodeBackend{models.CodeBackendClaudeCode}[0],
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Tests passed",
		ArtifactsJSON: []byte(`{"decision": "passed", "test_count": 5}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusTesting, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusTesting,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()

	// Step 5: Testing -> Completed
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusTesting,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusTesting,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, testerAgentID).Return(&models.Agent{
		ID:          testerAgentID,
		Role:        models.AgentRoleTester,
			CodeBackend: &[]models.CodeBackend{models.CodeBackendClaudeCode}[0],
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "All tests passed successfully",
		ArtifactsJSON: []byte(`{"decision": "passed"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusCompleted, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusCompleted,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()

	// Final: Completed is terminal, loop exits
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusCompleted,
		AssignedAgentID: &testerAgentID,
	}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)

	tr.AssertExpectations(t)
	le.AssertExpectations(t)
	se.AssertExpectations(t)
	ts.AssertExpectations(t)
}

// TestOrchestratorProcessTask_ChangesRequestedFlow проверяет возврат к Develop при ChangesRequested
func TestOrchestratorProcessTask_ChangesRequestedFlow(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	developerAgentID := uuid.New()
	reviewerAgentID := uuid.New()

	// Start at InProgress
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, developerAgentID).Return(&models.Agent{
		ID:          developerAgentID,
		Role:        models.AgentRoleDeveloper,
			CodeBackend: &[]models.CodeBackend{models.CodeBackendClaudeCode}[0],
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Code implemented",
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusReview, mock.Anything).Return(&models.Task{
		ID:              taskID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &reviewerAgentID,
	}, nil).Once()

	// Review: Changes Requested
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &reviewerAgentID,
		Context:         []byte(`{}`),
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &reviewerAgentID,
		Context:         []byte(`{}`),
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, reviewerAgentID).Return(&models.Agent{
		ID:   reviewerAgentID,
		Role: models.AgentRoleReviewer,
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Please fix the error handling",
		ArtifactsJSON: []byte(`{"decision": "changes_requested"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.MatchedBy(func(opts TransitionOpts) bool {
		// Проверяем что result записан
		return opts.Result != nil
	})).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusChangesRequested,
		AssignedAgentID: &reviewerAgentID,
		Context:         []byte(`{"iteration_count": 1}`),
	}, nil).Once()

	// Back to InProgress for fixes
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusChangesRequested,
		AssignedAgentID: &developerAgentID,
		Context:         []byte(`{"iteration_count": 1}`),
	}, nil).Once()
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusChangesRequested,
		AssignedAgentID: &developerAgentID,
		Context:         []byte(`{"iteration_count": 1}`),
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, developerAgentID).Return(&models.Agent{
		ID:          developerAgentID,
		Role:        models.AgentRoleDeveloper,
		CodeBackend: &[]models.CodeBackend{models.CodeBackendClaudeCode}[0],
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Fixed error handling",
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusInProgress,
		AssignedAgentID: &developerAgentID,
	}, nil).Once()

	// Final: After returning to InProgress, the loop will re-read task
	// We return Completed to exit
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusCompleted, // Terminal status to exit
		AssignedAgentID: &developerAgentID,
	}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)

	tr.AssertExpectations(t)
	ts.AssertExpectations(t)
}

// TestOrchestratorProcessTask_IterationLimitReached проверяет достижение лимита итераций
func TestOrchestratorProcessTask_IterationLimitReached(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(3) // Лимит 3 итерации
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	reviewerAgentID := uuid.New()

	// Review with iteration_count = 3 (max reached)
	reviewTask := &models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusReview,
		AssignedAgentID: &reviewerAgentID,
		Context:         []byte(`{"iteration_count": 3}`),
	}
	tr.On("GetByID", mock.Anything, taskID).Return(reviewTask, nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	wr.On("GetAgentByID", mock.Anything, reviewerAgentID).Return(&models.Agent{
		ID:   reviewerAgentID,
		Role: models.AgentRoleReviewer,
	}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true,
		Output:  "Still needs changes",
		ArtifactsJSON: []byte(`{"decision": "changes_requested"}`),
	}, nil).Once()

	// Ожидаем переход в Failed из-за превышения лимита
	// Используем mock.Anything для контекста, так как сервис использует cleanupCtx с таймаутом
	ts.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(opts TransitionOpts) bool {
		return opts.ErrorMessage != nil && strings.Contains(*opts.ErrorMessage, "iteration limit")
	})).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusFailed,
		AssignedAgentID: &reviewerAgentID,
	}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrchestratorIterationLimitReached)

	tr.AssertExpectations(t)
	ts.AssertExpectations(t)
}

// TestOrchestratorProcessTask_ContextCancellation проверяет отмену контекста
// и корректный переход задачи в статус Cancelled
func TestOrchestratorProcessTask_ContextCancellation(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx, cancel := context.WithCancel(context.Background())
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()

	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &agentID,
	}, nil).Once()
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	wr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{
		ID:   agentID,
		Role: models.AgentRolePlanner,
	}, nil)

	// Ожидаем что задача будет переведена в Cancelled при отмене контекста
	ts.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{
		ID:        taskID,
		Status:    models.TaskStatusCancelled,
	}, nil).Once()

	// Отменяем контекст до выполнения
	cancel()

	// Execute не должен быть вызван, так как мы отменили контекст
	le.AssertNotCalled(t, "Execute")

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// Проверяем что задача была переведена в Cancelled
	ts.AssertExpectations(t)
}

// TestOrchestratorProcessTask_EmptyResult проверяет обработку nil результата от агента
func TestOrchestratorProcessTask_EmptyResult(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()

	pendingTask := &models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &agentID,
	}
	tr.On("GetByID", mock.Anything, taskID).Return(pendingTask, nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	wr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{
		ID:   agentID,
		Role: models.AgentRolePlanner,
	}, nil)

	// Агент возвращает nil результат (что-то пошло не так)
	le.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Once()

	// Ожидаем переход в Failed при nil результате
	ts.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(opts TransitionOpts) bool {
		return opts.ErrorMessage != nil && strings.Contains(*opts.ErrorMessage, "nil")
	})).Return(&models.Task{
		ID:        taskID,
		Status:    models.TaskStatusFailed,
	}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil")

	tr.AssertExpectations(t)
	le.AssertExpectations(t)
	ts.AssertExpectations(t)
}

// TestOrchestratorProcessTask_AgentFailure проверяет обработку неуспешного результата агента (Success=false)
func TestOrchestratorProcessTask_AgentFailure(t *testing.T) {
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockTaskProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockAgentExecutor)
	se := new(mockAgentExecutor)
	ts := new(mockTaskService)
	pipe := NewPipelineEngine(5)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil)

	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, nil, nil, WithStepPollInterval(0))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()

	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &agentID,
	}, nil).Once()
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusPending,
		AssignedAgentID: &agentID,
	}, nil).Once()
	wr.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{
		ID:   agentID,
		Role: models.AgentRolePlanner,
	}, nil)

	// Агент возвращает Success=false
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: false,
		Output:  "Failed to create plan: insufficient context",
	}, nil).Once()

	// В транзакции создаётся сообщение агента
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)

	// Ожидаем переход в Failed
	ts.On("Transition", ctx, taskID, models.TaskStatusFailed, mock.MatchedBy(func(opts TransitionOpts) bool {
		return opts.Result != nil && strings.Contains(*opts.Result, "Failed to create plan")
	})).Return(&models.Task{
		ID:        taskID,
		Status:    models.TaskStatusFailed,
	}, nil).Once()

	// После перехода сервис перечитывает задачу для следующей итерации
	tr.On("GetByID", ctx, taskID).Return(&models.Task{
		ID:              taskID,
		ProjectID:       projectID,
		Status:          models.TaskStatusFailed, // Терминальный статус - выходим из цикла
		AssignedAgentID: &agentID,
	}, nil).Once()

	// ProcessTask не возвращает ошибку, когда задача успешно переведена в Failed
	// Это штатное завершение работы пайплайна
	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)

	tr.AssertExpectations(t)
	le.AssertExpectations(t)
	ts.AssertExpectations(t)
}
