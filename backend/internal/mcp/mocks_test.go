package mcp

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
)

// --- Shared test helpers ---

func defaultMCPConfig() config.MCPConfig {
	return config.MCPConfig{
		MaxPromptRunes: 1000,
		MaxTokensLimit: 4096,
		MaxInputRunes:  500,
	}
}

// --- LLMService mock ---

type mockLLMService struct {
	mock.Mock
}

func (m *mockLLMService) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.Response), args.Error(1)
}

func (m *mockLLMService) ListLogs(ctx context.Context, limit, offset int) ([]models.LLMLog, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.LLMLog), args.Get(1).(int64), args.Error(2)
}

// --- PromptService mock ---

type mockPromptService struct {
	mock.Mock
}

func (m *mockPromptService) Create(ctx context.Context, req dto.CreatePromptRequest) (*models.Prompt, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *mockPromptService) GetByID(ctx context.Context, id uuid.UUID) (*models.Prompt, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *mockPromptService) GetByName(ctx context.Context, name string) (*models.Prompt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *mockPromptService) List(ctx context.Context) ([]models.Prompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Prompt), args.Error(1)
}

func (m *mockPromptService) Update(ctx context.Context, id uuid.UUID, req dto.UpdatePromptRequest) (*models.Prompt, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Prompt), args.Error(1)
}

func (m *mockPromptService) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// --- WorkflowEngine mock ---

type mockWorkflowEngine struct {
	mock.Mock
}

func (m *mockWorkflowEngine) StartWorkflow(ctx context.Context, workflowName string, input string) (*models.Execution, error) {
	args := m.Called(ctx, workflowName, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *mockWorkflowEngine) GetExecution(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *mockWorkflowEngine) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Workflow), args.Error(1)
}

func (m *mockWorkflowEngine) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.Execution), args.Get(1).(int64), args.Error(2)
}

func (m *mockWorkflowEngine) GetExecutionSteps(ctx context.Context, id uuid.UUID) ([]models.ExecutionStep, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ExecutionStep), args.Error(1)
}

func (m *mockWorkflowEngine) RunWorker(ctx context.Context) {
	m.Called(ctx)
}
