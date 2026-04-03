package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
	"gorm.io/datatypes"
)

// --- Mocks ---

type MockWorkflowRepository struct {
	mock.Mock
}

func (m *MockWorkflowRepository) CreateWorkflow(ctx context.Context, wf *models.Workflow) error {
	args := m.Called(ctx, wf)
	return args.Error(0)
}

func (m *MockWorkflowRepository) GetWorkflowByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockWorkflowRepository) GetAgentByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockWorkflowRepository) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockWorkflowRepository) CreateExecution(ctx context.Context, exec *models.Execution) error {
	args := m.Called(ctx, exec)
	// Simulate ID generation if needed, or just return success
	if exec.ID == uuid.Nil {
		exec.ID = uuid.New()
	}
	return args.Error(0)
}

func (m *MockWorkflowRepository) GetExecutionByID(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *MockWorkflowRepository) UpdateExecution(ctx context.Context, exec *models.Execution) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}

func (m *MockWorkflowRepository) AddExecutionStep(ctx context.Context, step *models.ExecutionStep) error {
	args := m.Called(ctx, step)
	return args.Error(0)
}

func (m *MockWorkflowRepository) GetNextPendingExecution(ctx context.Context) (*models.Execution, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *MockWorkflowRepository) CreateScheduledWorkflow(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockWorkflowRepository) ListActiveSchedules(ctx context.Context) ([]models.ScheduledWorkflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.ScheduledWorkflow), args.Error(1)
}

func (m *MockWorkflowRepository) UpdateSchedule(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockWorkflowRepository) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Workflow), args.Error(1)
}

func (m *MockWorkflowRepository) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.Execution), args.Get(1).(int64), args.Error(2)
}

func (m *MockWorkflowRepository) GetExecutionSteps(ctx context.Context, executionID uuid.UUID) ([]models.ExecutionStep, error) {
	args := m.Called(ctx, executionID)
	return args.Get(0).([]models.ExecutionStep), args.Error(1)
}

type MockLLMService struct {
	mock.Mock
}

func (m *MockLLMService) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.Response), args.Error(1)
}

func (m *MockLLMService) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.LLMLog), args.Get(1).(int64), args.Error(2)
}

// --- Tests ---

func TestStartWorkflow(t *testing.T) {
	repo := new(MockWorkflowRepository)
	llmService := new(MockLLMService)
	engine := NewWorkflowEngine(repo, llmService)

	wfName := "test_workflow"
	input := "test input"
	
	// Workflow Config
	config := models.WorkflowConfig{
		StartStep: "step1",
		MaxSteps:  5,
		Steps: map[string]models.StepConfig{
			"step1": {
				AgentID: uuid.New().String(),
				Next:    nil,
			},
		},
	}
	configBytes, _ := json.Marshal(config)

	wf := &models.Workflow{
		ID:            uuid.New(),
		Name:          wfName,
		Configuration: datatypes.JSON(configBytes),
	}

	// Expectations
	repo.On("GetWorkflowByName", mock.Anything, wfName).Return(wf, nil)
	repo.On("CreateExecution", mock.Anything, mock.MatchedBy(func(e *models.Execution) bool {
		return e.WorkflowID == wf.ID && e.InputData == input && e.CurrentStepID == "step1"
	})).Return(nil)

	// Execute
	exec, err := engine.StartWorkflow(context.Background(), wfName, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, exec)
	assert.Equal(t, wf.ID, exec.WorkflowID)
	assert.Equal(t, "step1", exec.CurrentStepID)
	
	repo.AssertExpectations(t)
}

func TestStartWorkflow_NotFound(t *testing.T) {
	repo := new(MockWorkflowRepository)
	llmService := new(MockLLMService)
	engine := NewWorkflowEngine(repo, llmService)

	wfName := "unknown"
	repo.On("GetWorkflowByName", mock.Anything, wfName).Return(nil, assert.AnError)

	exec, err := engine.StartWorkflow(context.Background(), wfName, "input")

	assert.Error(t, err)
	assert.Nil(t, exec)
}

