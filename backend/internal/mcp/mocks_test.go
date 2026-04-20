package mcp

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
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

// --- ProjectService mock ---

type mockProjectService struct {
	mock.Mock
}

func (m *mockProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	args := m.Called(ctx, userID, userRole, req)
	var projects []models.Project
	if args.Get(0) != nil {
		projects = args.Get(0).([]models.Project)
	}
	total := int64(0)
	if args.Get(1) != nil {
		total = args.Get(1).(int64)
	}
	return projects, total, args.Error(2)
}

func (m *mockProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *mockProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}

func (m *mockProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}

// --- TeamService mock ---

type mockTeamService struct {
	mock.Mock
}

func (m *mockTeamService) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *mockTeamService) Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

// --- TaskService mock ---

type mockTaskService struct {
	mock.Mock
}

func (m *mockTaskService) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	var list []models.Task
	if v := args.Get(0); v != nil {
		list = v.([]models.Task)
	}
	var total int64
	if v := args.Get(1); v != nil {
		total = v.(int64)
	}
	return list, total, args.Error(2)
}

func (m *mockTaskService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, taskID).Error(0)
}

func (m *mockTaskService) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskStatus, opts service.TransitionOpts) (*models.Task, error) {
	args := m.Called(ctx, taskID, newStatus, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *mockTaskService) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}

func (m *mockTaskService) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	var list []models.TaskMessage
	if v := args.Get(0); v != nil {
		list = v.([]models.TaskMessage)
	}
	var total int64
	if v := args.Get(1); v != nil {
		total = v.(int64)
	}
	return list, total, args.Error(2)
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
