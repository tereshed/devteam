package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/pkg/agentsloader"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"gorm.io/datatypes"
)

// --- Моки (имена из задачи 6.10: mockLLMAgentExecutor / mockSandboxAgentExecutor — один тип, два экземпляра) ---

type mockOrchestratorProjectService struct{ mock.Mock }

func (m *mockOrchestratorProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockOrchestratorProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockOrchestratorProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	return nil, 0, nil
}
func (m *mockOrchestratorProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	return nil, nil
}
func (m *mockOrchestratorProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return nil
}
func (m *mockOrchestratorProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}
func (m *mockOrchestratorProjectService) Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}

type mockLLMAgentExecutor struct{ mock.Mock }

func (m *mockLLMAgentExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*agent.ExecutionResult), args.Error(1)
}

type mockSandboxAgentExecutor struct{ mock.Mock }

func (m *mockSandboxAgentExecutor) Execute(ctx context.Context, in agent.ExecutionInput) (*agent.ExecutionResult, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*agent.ExecutionResult), args.Error(1)
}

type mockTaskSandboxStopper struct{ mock.Mock }

func (m *mockTaskSandboxStopper) StopTask(ctx context.Context, taskID string) error {
	return m.Called(ctx, taskID).Error(0)
}

type mockSandboxRunner struct{ mock.Mock }

func (m *mockSandboxRunner) RunTask(ctx context.Context, opts sandbox.SandboxOptions) (*sandbox.SandboxInstance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxInstance), args.Error(1)
}
func (m *mockSandboxRunner) Wait(ctx context.Context, sandboxID string) (*sandbox.SandboxStatus, error) {
	args := m.Called(ctx, sandboxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxStatus), args.Error(1)
}
func (m *mockSandboxRunner) GetStatus(ctx context.Context, sandboxID string) (*sandbox.SandboxStatus, error) {
	args := m.Called(ctx, sandboxID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sandbox.SandboxStatus), args.Error(1)
}
func (m *mockSandboxRunner) StreamLogs(ctx context.Context, sandboxID string) (<-chan sandbox.LogEntry, error) {
	args := m.Called(ctx, sandboxID)
	return args.Get(0).(<-chan sandbox.LogEntry), args.Error(1)
}
func (m *mockSandboxRunner) Stop(ctx context.Context, sandboxID string) error {
	return m.Called(ctx, sandboxID).Error(0)
}
func (m *mockSandboxRunner) StopTask(ctx context.Context, taskID string) error {
	return m.Called(ctx, taskID).Error(0)
}
func (m *mockSandboxRunner) Cleanup(ctx context.Context, sandboxID string) error {
	return m.Called(ctx, sandboxID).Error(0)
}

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
func (m *mockTaskService) Close() error {
	return nil
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

// mockAgents — пара LLM + Sandbox для table-driven сценариев.
type mockAgents struct {
	LLM      *mockLLMAgentExecutor
	Sandbox  *mockSandboxAgentExecutor
	Stopper  *mockTaskSandboxStopper
	TaskRepo *mockTaskRepository
	TMR      *mockOrchestratorTaskMessageRepository
	WR       *mockOrchestratorWorkflowRepository
	PS       *mockOrchestratorProjectService
	TS       *mockTaskService
}

// orchestratorHarnessConfig — единая точка инициализации моков (ревью 6.10: DRY, table-driven setup).
type orchestratorHarnessConfig struct {
	Bus             *UserTaskControlBus
	Stop            *mockTaskSandboxStopper
	PipelineMaxIter int
	Opts            []OrchestratorOption
}

// newTestOrchestratorHarness — фабрика сервиса и моков (задача 6.10).
func newTestOrchestratorHarness(t *testing.T, cfg orchestratorHarnessConfig) (OrchestratorService, *mockAgents) {
	t.Helper()
	tr := new(mockTaskRepository)
	tmr := new(mockOrchestratorTaskMessageRepository)
	wr := new(mockOrchestratorWorkflowRepository)
	ps := new(mockOrchestratorProjectService)
	tx := new(mockOrchestratorTransactionManager)
	le := new(mockLLMAgentExecutor)
	se := new(mockSandboxAgentExecutor)
	ts := new(mockTaskService)
	pipeMax := cfg.PipelineMaxIter
	if pipeMax <= 0 {
		pipeMax = 5
	}
	pipe := NewPipelineEngine(pipeMax)
	ctxB := NewContextBuilder(NoopEncryptor{}, nil, nil)

	all := []OrchestratorOption{WithStepPollInterval(0)}
	all = append(all, cfg.Opts...)
	svc := NewOrchestratorService(tr, tmr, wr, ps, tx, le, se, ts, pipe, ctxB, cfg.Stop, cfg.Bus, all...)
	return svc, &mockAgents{
		LLM: le, Sandbox: se, Stopper: cfg.Stop, TaskRepo: tr,
		TMR: tmr, WR: wr, PS: ps, TS: ts,
	}
}

func backendRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// .../internal/service -> backend
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func agentsDirs(t *testing.T) (agentsDir, promptsDir string) {
	root := backendRootDir(t)
	return filepath.Join(root, "agents"), filepath.Join(root, "prompts")
}

func copyDirFiles(t *testing.T, src, dst string, ext string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ext {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644))
	}
}

func copyAgentSchemaJSON(t *testing.T, agentsDir, dstDir string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(agentsDir, "agent_schema.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "agent_schema.json"), b, 0o644))
}

func baseTask(id, project uuid.UUID, status models.TaskStatus, agentID *uuid.UUID) *models.Task {
	return &models.Task{
		ID:              id,
		ProjectID:       project,
		Title:           "user task",
		Description:     "desc",
		Status:          status,
		AssignedAgentID: agentID,
		Context:         datatypes.JSON(`{}`),
	}
}

func codeBackendClaude() *models.CodeBackend {
	b := models.CodeBackendClaudeCode
	return &b
}

// --- Init (агенты загружаются через agentsloader при старте API; здесь те же контракты) ---

func TestOrchestratorInit_LoadsAgentConfigs(t *testing.T) {
	ad, pd := agentsDirs(t)
	cache, err := agentsloader.NewCache(ad, pd)
	require.NoError(t, err)
	require.NoError(t, cache.ValidateRequiredAgents())
	_, ok := cache.GetByPipelineRole("orchestrator")
	require.True(t, ok)
}

func TestOrchestratorInit_FailsIfAgentInactive(t *testing.T) {
	ad, pd := agentsDirs(t)
	tmp := t.TempDir()
	copyAgentSchemaJSON(t, ad, tmp)
	copyDirFiles(t, ad, tmp, ".yaml")
	copyDirFiles(t, ad, tmp, ".yml")
	orchPath := filepath.Join(tmp, "orchestrator.yaml")
	raw, err := os.ReadFile(orchPath)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "is_active: true", "is_active: false", 1))
	require.NoError(t, os.WriteFile(orchPath, raw, 0o644))
	cache, err := agentsloader.NewCache(tmp, pd)
	require.NoError(t, err)
	require.Error(t, cache.ValidateRequiredAgents())
}

func TestOrchestratorInit_FailsIfAgentMissing(t *testing.T) {
	ad, pd := agentsDirs(t)
	tmp := t.TempDir()
	copyAgentSchemaJSON(t, ad, tmp)
	copyDirFiles(t, ad, tmp, ".yaml")
	_ = os.Remove(filepath.Join(tmp, "tester.yaml"))
	cache, err := agentsloader.NewCache(tmp, pd)
	require.NoError(t, err)
	require.Error(t, cache.ValidateRequiredAgents())
}

func TestOrchestratorInit_FailsIfInvalidTemperature(t *testing.T) {
	ad, pd := agentsDirs(t)
	tmp := t.TempDir()
	copyAgentSchemaJSON(t, ad, tmp)
	copyDirFiles(t, ad, tmp, ".yaml")
	p := filepath.Join(tmp, "planner.yaml")
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "temperature: 0.2", "temperature: 9.0", 1))
	require.NoError(t, os.WriteFile(p, raw, 0o644))
	_, err = agentsloader.NewCache(tmp, pd)
	require.Error(t, err)
}

func TestOrchestratorInit_FailsIfPromptNameTraversal(t *testing.T) {
	ad, pd := agentsDirs(t)
	tmp := t.TempDir()
	copyDirFiles(t, ad, tmp, ".yaml")
	p := filepath.Join(tmp, "orchestrator.yaml")
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "prompt_name: orchestrator_prompt", "prompt_name: ../etc/passwd", 1))
	require.NoError(t, os.WriteFile(p, raw, 0o644))
	_, err = agentsloader.NewCache(tmp, pd)
	require.Error(t, err)
}

// --- Pipeline ---

func TestPipeline_FullSuccess(t *testing.T) {
	runFullFiveStepPipeline(t, false)
}

func TestPipeline_CallsOrchestrator(t *testing.T) {
	roles := runFullFiveStepPipelineCollectRoles(t)
	require.Equal(t, "orchestrator", roles[0])
}

func TestPipeline_CallsPlanner(t *testing.T) {
	roles := runFullFiveStepPipelineCollectRoles(t)
	require.Equal(t, "planner", roles[1])
}

func TestPipeline_CallsDeveloper(t *testing.T) {
	roles := runFullFiveStepPipelineCollectRoles(t)
	require.Equal(t, "developer", roles[2])
}

func TestPipeline_CallsReviewer(t *testing.T) {
	roles := runFullFiveStepPipelineCollectRoles(t)
	require.Equal(t, "reviewer", roles[3])
}

func TestPipeline_CallsTester(t *testing.T) {
	roles := runFullFiveStepPipelineCollectRoles(t)
	require.Equal(t, "tester", roles[4])
}

func TestPipeline_SetsFinalStatus(t *testing.T) {
	runFullFiveStepPipeline(t, true)
}

func runFullFiveStepPipelineCollectRoles(t *testing.T) []string {
	t.Helper()
	var roles []string
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	record := func(args mock.Arguments) {
		in := args.Get(1).(agent.ExecutionInput)
		roles = append(roles, in.Role)
	}
	h.LLM.On("Execute", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { record(a) }).Return(okLLMResult(), nil)
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { record(a) }).Return(okSandboxResult(), nil)

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	aidO := uuid.New()
	aidP := uuid.New()
	aidD := uuid.New()
	aidR := uuid.New()
	aidT := uuid.New()

	wireFullPipelineMocks(t, h.TaskRepo, h.TMR, h.WR, h.PS, h.TS, taskID, projectID, aidO, aidP, aidD, aidR, aidT)

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)
	return roles
}

func runFullFiveStepPipeline(t *testing.T, assertCompletedTransition bool) {
	t.Helper()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(okLLMResult(), nil)
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Return(okSandboxResult(), nil)

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	aidO := uuid.New()
	aidP := uuid.New()
	aidD := uuid.New()
	aidR := uuid.New()
	aidT := uuid.New()

	wireFullPipelineMocks(t, h.TaskRepo, h.TMR, h.WR, h.PS, h.TS, taskID, projectID, aidO, aidP, aidD, aidR, aidT)

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)
	if assertCompletedTransition {
		h.TS.AssertCalled(t, "Transition", mock.Anything, taskID, models.TaskStatusCompleted, mock.Anything)
	}
}

func okLLMResult() *agent.ExecutionResult {
	return &agent.ExecutionResult{
		Success:       true,
		Output:        "ok",
		ArtifactsJSON: []byte(`{"note":"ok"}`),
	}
}

func okSandboxResult() *agent.ExecutionResult {
	return &agent.ExecutionResult{
		Success:       true,
		Output:        "ok",
		ArtifactsJSON: []byte(`{"diff":"+x","decision":"passed"}`),
	}
}

func wireFullPipelineMocks(t *testing.T, tr *mockTaskRepository, tmr *mockOrchestratorTaskMessageRepository, wr *mockOrchestratorWorkflowRepository, ps *mockOrchestratorProjectService, ts *mockTaskService, taskID, projectID, orch, planner, dev, rev, tester uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	// ProcessTask: первый GetByID до цикла
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil).Once()

	// --- шаг 1: orchestrator (pending → planning) ---
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, orch).Return(&models.Agent{ID: orch, Role: models.AgentRoleOrchestrator}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusPlanning, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()

	// --- шаг 2: planner (planning → in_progress) ---
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	wr.On("GetAgentByID", mock.Anything, planner).Return(&models.Agent{ID: planner, Role: models.AgentRolePlanner}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()

	// --- шаг 3: developer (in_progress → review) ---
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusReview, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()

	// --- шаг 4: reviewer (review → testing) ---
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusTesting, mock.Anything).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()

	// --- шаг 5: tester (testing → completed) ---
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusCompleted, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
}

// wireFullPipelineMocksFromPlanning — первый GetByID уже planning + planner (сценарий после Resume); дальше planner→developer→reviewer→tester→completed.
func wireFullPipelineMocksFromPlanning(t *testing.T, tr *mockTaskRepository, tmr *mockOrchestratorTaskMessageRepository, wr *mockOrchestratorWorkflowRepository, ps *mockOrchestratorProjectService, ts *mockTaskService, taskID, projectID, planner, dev, rev, tester uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()

	// planner (planning → in_progress)
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	wr.On("GetAgentByID", mock.Anything, planner).Return(&models.Agent{ID: planner, Role: models.AgentRolePlanner}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()

	// developer → review → testing → completed (как wireFullPipelineMocks)
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusReview, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()

	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusTesting, mock.Anything).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()

	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusCompleted, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
}

func withBranch(tk *models.Task, b string) *models.Task {
	tk.BranchName = &b
	return tk
}

// branchingHarness — общий setup для группы Branching (table-driven сценарии ниже; pipelineMax: 0 = 5).
func branchingHarness(t *testing.T, pipelineMax int) (OrchestratorService, *mockAgents) {
	t.Helper()
	pipeMax := pipelineMax
	if pipeMax <= 0 {
		pipeMax = 5
	}
	return newTestOrchestratorHarness(t, orchestratorHarnessConfig{PipelineMaxIter: pipeMax})
}

// --- Branching: changes_requested (table-driven: общий harness + отдельные TestPipeline_* по чеклисту) ---

func TestPipeline_ReviewerRequestsChanges(t *testing.T) {
	svc, h := branchingHarness(t, 5)
	tr, tmr, wr, ps, le, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.LLM, h.TS

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	rev := uuid.New()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusReview, &rev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusReview, &rev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "fix", ArtifactsJSON: []byte(`{"decision":"changes_requested"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.Anything).Return(
		baseTask(taskID, projectID, models.TaskStatusChangesRequested, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &rev), nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.NoError(t, err)
	le.AssertExpectations(t)
}

func TestPipeline_DeveloperReRunsAfterChanges(t *testing.T) {
	var devCalls atomic.Int32
	svc, h := branchingHarness(t, 5)
	tr, tmr, wr, ps, le, se, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.LLM, h.Sandbox, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	rev := uuid.New()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Run(func(_ mock.Arguments) { devCalls.Add(1) }).Return(okSandboxResult(), nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusReview, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "cr", ArtifactsJSON: []byte(`{"decision":"changes_requested"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.Anything).Return(
		baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Run(func(_ mock.Arguments) { devCalls.Add(1) }).Return(okSandboxResult(), nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &dev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &dev), nil).Once()

	require.NoError(t, svc.ProcessTask(ctx, taskID))
	require.Equal(t, int32(2), devCalls.Load())
}

func TestPipeline_ContinuesToTesterAfterApproval(t *testing.T) {
	var calls []string
	svc, h := branchingHarness(t, 5)
	tr, tmr, wr, ps, le, se, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.LLM, h.Sandbox, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	rev := uuid.New()
	tester := uuid.New()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	le.On("Execute", mock.Anything, mock.Anything).Run(func(a mock.Arguments) {
		in := a.Get(1).(agent.ExecutionInput)
		calls = append(calls, in.Role)
	}).Return(&agent.ExecutionResult{Success: true, Output: "ok", ArtifactsJSON: []byte(`{"decision":"approved"}`)}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusTesting, mock.Anything).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Run(func(a mock.Arguments) {
		in := a.Get(1).(agent.ExecutionInput)
		calls = append(calls, in.Role)
	}).Return(okSandboxResult(), nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusCompleted, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()

	require.NoError(t, svc.ProcessTask(ctx, taskID))
	require.Equal(t, []string{"reviewer", "tester"}, calls)
}

func TestPipeline_MaxRetriesReached(t *testing.T) {
	svc, h := branchingHarness(t, 3)
	tr, tmr, wr, ps, le, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.LLM, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	rev := uuid.New()
	tk := baseTask(taskID, projectID, models.TaskStatusReview, &rev)
	tk.Context = datatypes.JSON(`{"iteration_count":3}`)
	tr.On("GetByID", mock.Anything, taskID).Return(tk, nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	wr.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	le.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "x", ArtifactsJSON: []byte(`{"decision":"changes_requested"}`),
	}, nil).Once()
	ts.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(o TransitionOpts) bool {
		return o.ErrorMessage != nil && strings.Contains(*o.ErrorMessage, "iteration")
	})).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrchestratorIterationLimitReached)
}

// --- Branching: tester failed ---

func TestPipeline_TesterFailed(t *testing.T) {
	svc, h := branchingHarness(t, 5)
	tr, tmr, wr, ps, se, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.Sandbox, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	tester := uuid.New()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "fail", ArtifactsJSON: []byte(`{"decision":"failed"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.Anything).Return(
		baseTask(taskID, projectID, models.TaskStatusChangesRequested, &tester), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
	require.NoError(t, svc.ProcessTask(ctx, taskID))
}

func TestPipeline_DeveloperReRunsAfterTestFailure(t *testing.T) {
	var devCalls atomic.Int32
	svc, h := branchingHarness(t, 5)
	tr, tmr, wr, ps, se, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.Sandbox, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	tester := uuid.New()
	branch := "main"
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)

	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "t", ArtifactsJSON: []byte(`{"decision":"failed"}`),
	}, nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.Anything).Return(
		withBranch(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &dev), branch), nil).Once()
	wr.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	se.On("Execute", mock.Anything, mock.Anything).Run(func(_ mock.Arguments) { devCalls.Add(1) }).Return(okSandboxResult(), nil).Once()
	ts.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &dev), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &dev), nil).Once()

	require.NoError(t, svc.ProcessTask(ctx, taskID))
	require.Equal(t, int32(1), devCalls.Load())
}

func TestPipeline_MaxTestRetriesReached(t *testing.T) {
	svc, h := branchingHarness(t, 2)
	tr, tmr, wr, ps, se, ts := h.TaskRepo, h.TMR, h.WR, h.PS, h.Sandbox, h.TS
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	tester := uuid.New()
	branch := "main"
	tk := withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch)
	tk.Context = datatypes.JSON(`{"iteration_count":2}`)
	tr.On("GetByID", mock.Anything, taskID).Return(tk, nil)
	ps.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	wr.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	tmr.On("Create", mock.Anything, mock.Anything).Return(nil)
	se.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "t", ArtifactsJSON: []byte(`{"decision":"failed"}`),
	}, nil).Once()
	ts.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrchestratorIterationLimitReached)
}

// --- Cancellation ---

func TestCancel_DuringDeveloper(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(0)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	tr := h.TaskRepo
	tr.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	userID := uuid.New()
	branch := "main"
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Once()
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		close(execStarted)
		c := args.Get(0).(context.Context)
		<-c.Done()
	}).Return(nil, context.Canceled)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, ProjectID: projectID, Status: models.TaskStatusCancelled, Title: "user task", Description: "d"}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil)
	h.PS.On("GetByID", mock.Anything, userID, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, ProjectID: projectID, Title: "user task", Description: "d", Status: models.TaskStatusInProgress, AssignedAgentID: &dev}, nil).Maybe()

	done := make(chan error)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted
	bus.PublishCommand(ctx, UserTaskControlCommand{Kind: UserTaskControlCancel, TaskID: taskID, UserID: userID, UserRole: models.RoleAdmin})
	time.Sleep(50 * time.Millisecond)

	select {
	case err := <-done:
		// handleProcessTaskError возвращает nil, если в БД уже cancelled (имитация успешной отмены).
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	cancelStart()
}

func TestCancel_DuringTester(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(0)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	tr := h.TaskRepo
	tr.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	tester := uuid.New()
	userID := uuid.New()
	branch := "main"
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	tr.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	h.WR.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		close(execStarted)
		c := args.Get(0).(context.Context)
		<-c.Done()
	}).Return(nil, context.Canceled)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled, Title: "user task", Description: "d", ProjectID: projectID}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil)
	h.PS.On("GetByID", mock.Anything, userID, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	tr.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, ProjectID: projectID, Title: "user task", Description: "d", Status: models.TaskStatusTesting, AssignedAgentID: &tester}, nil).Maybe()

	done := make(chan error)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted
	bus.PublishCommand(ctx, UserTaskControlCommand{Kind: UserTaskControlCancel, TaskID: taskID, UserID: userID, UserRole: models.RoleAdmin})
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	cancelStart()
}

func TestCancel_FinalStatusIsCancelled(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &agentID), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.ErrorIs(t, err, context.Canceled)
}

func TestCancel_NoFurtherAgentCalls(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithCancel(context.Background())
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &agentID), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	cancel()
	_ = svc.ProcessTask(ctx, taskID)
	h.LLM.AssertNotCalled(t, "Execute")
}

// --- Pause / Resume ---

func TestPause_DuringExecution(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(5 * time.Millisecond), WithGracefulPauseTimeout(15 * time.Millisecond)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	h.TaskRepo.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	branch := "main"
	ip := withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(ip, nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(ip, nil).Once()
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		close(execStarted)
		time.Sleep(80 * time.Millisecond)
	}).Return(okSandboxResult(), nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(ip, nil).Times(4)
	paused := &models.Task{ID: taskID, ProjectID: projectID, Status: models.TaskStatusPaused, Title: "user task", Description: "d", AssignedAgentID: &dev}
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(paused, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil)
	done := make(chan error)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	cancelStart()
}

func TestPause_StatusIsPaused(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.TaskRepo.On("GetByID", ctx, taskID).Return(&models.Task{
		ID: taskID, ProjectID: projectID, Status: models.TaskStatusPaused,
		Title: "user task", Description: "d", AssignedAgentID: &agentID,
	}, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	require.NoError(t, svc.ProcessTask(ctx, taskID))
}

func TestResume_ContinuesFromStep(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	planner := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.WR.On("GetAgentByID", mock.Anything, planner).Return(&models.Agent{ID: planner, Role: models.AgentRolePlanner}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(okLLMResult(), nil).Once()
	h.TS.On("Transition", ctx, taskID, models.TaskStatusInProgress, mock.Anything).Return(baseTask(taskID, projectID, models.TaskStatusInProgress, &planner), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &planner), nil).Once()
	require.NoError(t, svc.ProcessTask(ctx, taskID))
}

func TestResume_FullPipelineCompletes(t *testing.T) {
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(okLLMResult(), nil)
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Return(okSandboxResult(), nil)
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	planner := uuid.New()
	aidD := uuid.New()
	aidR := uuid.New()
	aidT := uuid.New()
	wireFullPipelineMocksFromPlanning(t, h.TaskRepo, h.TMR, h.WR, h.PS, h.TS, taskID, projectID, planner, aidD, aidR, aidT)
	require.NoError(t, svc.ProcessTask(ctx, taskID))
	h.TS.AssertCalled(t, "Transition", mock.Anything, taskID, models.TaskStatusCompleted, mock.Anything)
}

// --- Error handling (table-driven: runErrorScenario + отдельные TestError_* по чеклисту) ---

func runErrorScenario(t *testing.T, key string) {
	t.Helper()
	ctx := context.Background()
	switch key {
	case "executor_timeout":
		tests := []struct {
			name string
			err  error
		}{
			{"deadline", context.DeadlineExceeded},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Cleanup(func() { goleak.VerifyNone(t) })
				taskID := uuid.New()
				projectID := uuid.New()
				agentID := uuid.New()
				svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
				h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &agentID), nil)
				h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
				h.WR.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID, Role: models.AgentRoleOrchestrator}, nil)
				h.LLM.On("Execute", mock.Anything, mock.Anything).Return(nil, tc.err).Once()
				h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
				require.Error(t, svc.ProcessTask(ctx, taskID))
			})
		}
	case "executor_error":
		taskID := uuid.New()
		projectID := uuid.New()
		orch := uuid.New()
		sentinel := errors.New("模型API错误")
		svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
		h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil)
		h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
		h.WR.On("GetAgentByID", mock.Anything, orch).Return(&models.Agent{ID: orch, Role: models.AgentRoleOrchestrator}, nil)
		h.LLM.On("Execute", mock.Anything, mock.Anything).Return(nil, sentinel).Once()
		h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
		require.Error(t, svc.ProcessTask(ctx, taskID))
	case "error_message_propagated":
		taskID := uuid.New()
		projectID := uuid.New()
		orch := uuid.New()
		root := errors.New("root cause")
		svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
		h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil)
		h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
		h.WR.On("GetAgentByID", mock.Anything, orch).Return(&models.Agent{ID: orch, Role: models.AgentRoleOrchestrator}, nil)
		h.LLM.On("Execute", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("orchestrator failed: %w", root)).Once()
		h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(o TransitionOpts) bool {
			return o.ErrorMessage != nil && strings.Contains(*o.ErrorMessage, "orchestrator failed")
		})).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
		_ = svc.ProcessTask(ctx, taskID)
	case "orchestrator_fails":
		taskID := uuid.New()
		projectID := uuid.New()
		orch := uuid.New()
		svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
		h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil)
		h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
		h.WR.On("GetAgentByID", mock.Anything, orch).Return(&models.Agent{ID: orch, Role: models.AgentRoleOrchestrator}, nil)
		h.LLM.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("orchestrator LLM down")).Once()
		h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
		require.Error(t, svc.ProcessTask(ctx, taskID))
	case "planner_fails":
		taskID := uuid.New()
		projectID := uuid.New()
		planner := uuid.New()
		svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
		h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil)
		h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
		h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
		h.WR.On("GetAgentByID", mock.Anything, planner).Return(&models.Agent{ID: planner, Role: models.AgentRolePlanner}, nil)
		h.LLM.On("Execute", mock.Anything, mock.MatchedBy(func(in agent.ExecutionInput) bool {
			return in.Role == string(models.AgentRolePlanner)
		})).Return(nil, errors.New("planner decomposition failed")).Once()
		h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
		require.Error(t, svc.ProcessTask(ctx, taskID))
	case "developer_fails":
		taskID := uuid.New()
		projectID := uuid.New()
		dev := uuid.New()
		branch := "main"
		svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
		h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil)
		h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
		h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
		h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
		h.Sandbox.On("Execute", mock.Anything, mock.Anything).Return(nil, errors.New("sandbox boom")).Once()
		h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
		require.Error(t, svc.ProcessTask(ctx, taskID))
	default:
		t.Fatalf("unknown error scenario %q", key)
	}
}

func TestError_ExecutorTimeout(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	runErrorScenario(t, "executor_timeout")
}

func TestError_ExecutorError(t *testing.T) {
	runErrorScenario(t, "executor_error")
}

func TestError_ErrorMessagePropagated(t *testing.T) {
	runErrorScenario(t, "error_message_propagated")
}

func TestError_ErrorUnwrapping(t *testing.T) {
	root := errors.New("root")
	wrapped := fmt.Errorf("planner failed: %w", root)
	require.True(t, errors.Is(wrapped, root))
}

func TestError_OrchestratorFails(t *testing.T) {
	runErrorScenario(t, "orchestrator_fails")
}

func TestError_PlannerFails(t *testing.T) {
	runErrorScenario(t, "planner_fails")
}

func TestError_DeveloperFails(t *testing.T) {
	runErrorScenario(t, "developer_fails")
}

// --- Retry ---

func TestRetry_IncrementsOnChangesRequested(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	rev := uuid.New()
	svc, h := branchingHarness(t, 5)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusReview, &rev), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.WR.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{Success: true, Output: "x", ArtifactsJSON: []byte(`{"decision":"changes_requested"}`)}, nil).Once()
	h.TS.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.MatchedBy(func(o TransitionOpts) bool {
		if o.Context == nil {
			return false
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(*o.Context), &m); err != nil {
			return false
		}
		ic, ok := m["iteration_count"].(float64)
		return ok && ic == 1
	})).Return(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &rev), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &rev), nil).Once()
	require.NoError(t, svc.ProcessTask(ctx, taskID))
}

func TestRetry_IncrementsOnTestFailure(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	tester := uuid.New()
	branch := "main"
	svc, h := branchingHarness(t, 5)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusTesting, &tester), branch), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.WR.On("GetAgentByID", mock.Anything, tester).Return(&models.Agent{ID: tester, Role: models.AgentRoleTester, CodeBackend: codeBackendClaude()}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "t", ArtifactsJSON: []byte(`{"decision":"failed"}`),
	}, nil).Once()
	h.TS.On("Transition", ctx, taskID, models.TaskStatusChangesRequested, mock.MatchedBy(func(o TransitionOpts) bool {
		if o.Context == nil {
			return false
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(*o.Context), &m); err != nil {
			return false
		}
		ic, ok := m["iteration_count"].(float64)
		return ok && ic == 1
	})).Return(baseTask(taskID, projectID, models.TaskStatusChangesRequested, &tester), nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusCompleted, &tester), nil).Once()
	require.NoError(t, svc.ProcessTask(ctx, taskID))
}

func TestRetry_ResetsOnSuccess(t *testing.T) {
	pipe := NewPipelineEngine(5)
	tk := &models.Task{Context: datatypes.JSON(`{"iteration_count":2}`)}
	require.Equal(t, 2, pipe.GetIterationCount(tk))
}

func TestRetry_StopsAtLimit(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	rev := uuid.New()
	svc, h := branchingHarness(t, 3)
	tk := baseTask(taskID, projectID, models.TaskStatusReview, &rev)
	tk.Context = datatypes.JSON(`{"iteration_count":3}`)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(tk, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.WR.On("GetAgentByID", mock.Anything, rev).Return(&models.Agent{ID: rev, Role: models.AgentRoleReviewer}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "x", ArtifactsJSON: []byte(`{"decision":"changes_requested"}`),
	}, nil).Once()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.MatchedBy(func(o TransitionOpts) bool {
		return o.ErrorMessage != nil && strings.Contains(*o.ErrorMessage, "iteration")
	})).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil).Once()

	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrchestratorIterationLimitReached)
}

// --- Edge cases ---

func TestEdgeCase_EmptyUserMessage(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	emptyTask := &models.Task{
		ID: taskID, ProjectID: projectID, Status: models.TaskStatusPending,
		Title: "", Description: "", AssignedAgentID: &agentID,
	}
	h.TaskRepo.On("GetByID", ctx, taskID).Return(emptyTask, nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(emptyTask, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.ErrorIs(t, err, ErrOrchestratorInvalidUserMessage)
}

func TestEdgeCase_NilUserMessage(t *testing.T) {
	require.False(t, taskHasVisibleUserContent(nil))
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	emptyTask := &models.Task{
		ID: taskID, ProjectID: projectID, Status: models.TaskStatusPending,
		Title: "", Description: "", AssignedAgentID: &agentID,
	}
	h.TaskRepo.On("GetByID", ctx, taskID).Return(emptyTask, nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(emptyTask, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.ErrorIs(t, err, ErrOrchestratorInvalidUserMessage)
}

func TestEdgeCase_WhitespaceOnlyMessage(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	wsTask := &models.Task{
		ID: taskID, ProjectID: projectID, Status: models.TaskStatusPending,
		Title: "\t\n ", Description: "  ", AssignedAgentID: &agentID,
	}
	h.TaskRepo.On("GetByID", ctx, taskID).Return(wsTask, nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(wsTask, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.ErrorIs(t, err, ErrOrchestratorInvalidUserMessage)
}

func TestEdgeCase_EmptyPlannerResult(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	bt := baseTask(taskID, projectID, models.TaskStatusPending, &agentID)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(bt, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.WR.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID, Role: models.AgentRoleOrchestrator}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{Success: true, Output: "", ArtifactsJSON: []byte(`{}`)}, nil).Once()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPipelineEmptyResult) || strings.Contains(err.Error(), "empty result"))
}

func TestEdgeCase_EmptyDeveloperDiff(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	branch := "main"
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	ip := withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(ip, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Return(&agent.ExecutionResult{
		Success: true, Output: "x", ArtifactsJSON: []byte(`{"diff":""}`),
	}, nil).Once()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPipelineEmptyDiff) || strings.Contains(err.Error(), "diff"))
}

func TestEdgeCase_NilExecutorResult(t *testing.T) {
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	bt := baseTask(taskID, projectID, models.TaskStatusPending, &agentID)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(bt, nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.WR.On("GetAgentByID", mock.Anything, agentID).Return(&models.Agent{ID: agentID, Role: models.AgentRoleOrchestrator}, nil)
	h.LLM.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Once()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusFailed, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusFailed}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.Error(t, err)
}

func TestEdgeCase_InvalidStatusTransition(t *testing.T) {
	pipe := NewPipelineEngine(5)
	tk := &models.Task{Status: models.TaskStatus("bogus_status")}
	_, err := pipe.DetermineNextStatus(tk, okLLMResult())
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPipelineInvalidTransition))
}

func TestEdgeCase_ContextCancelledBeforeStart(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	taskID := uuid.New()
	projectID := uuid.New()
	agentID := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.TaskRepo.On("GetByID", ctx, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &agentID), nil).Once()
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	err := svc.ProcessTask(ctx, taskID)
	require.ErrorIs(t, err, context.Canceled)
	h.LLM.AssertNotCalled(t, "Execute")
}

// --- Security ---

func TestSecurity_SanitizesSandboxInputs(t *testing.T) {
	runner := new(mockSandboxRunner)
	ex := agent.NewSandboxAgentExecutor(runner, "img")
	ctx := context.Background()
	_, err := ex.Execute(ctx, agent.ExecutionInput{
		TaskID: "1", ProjectID: "2", GitURL: "https://github.com/o/r.git",
		BranchName: "--upload-pack=foo; rm -rf /", CodeBackend: "claude-code",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, agent.ErrInvalidExecutionInput)
}

func TestSecurity_UserMessagePassedToLLMAsIs(t *testing.T) {
	b := NewContextBuilder(NoopEncryptor{}, nil, nil)
	ctx := context.Background()
	raw := "a && b | c > d"
	in, err := b.Build(ctx, &models.Task{Title: raw, Description: raw, Context: []byte(`{}`)}, &models.Agent{ID: uuid.New(), Role: models.AgentRolePlanner}, &models.Project{})
	require.NoError(t, err)
	require.Equal(t, raw, in.Title)
	require.Equal(t, raw, in.Description)
}

func TestSecurity_PromptNameNoTraversal(t *testing.T) {
	ad, pd := agentsDirs(t)
	tmp := t.TempDir()
	copyDirFiles(t, ad, tmp, ".yaml")
	p := filepath.Join(tmp, "orchestrator.yaml")
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), "prompt_name: orchestrator_prompt", "prompt_name: ../etc/passwd", 1))
	require.NoError(t, os.WriteFile(p, raw, 0o644))
	_, err = agentsloader.NewCache(tmp, pd)
	require.Error(t, err)
}

// --- Concurrency ---

func TestConcurrency_CancelDuringStateTransition(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithCancel(context.Background())
	taskID := uuid.New()
	projectID := uuid.New()
	orch := uuid.New()
	planner := uuid.New()
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{})
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil).Once()
	// finishStepExecution (orchestrator_service.go:529): до Transition в БД задача ещё pending.
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPending, &orch), nil).Once()
	// ProcessTask цикл (стр. 324): после Transition — planning + planner.
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.WR.On("GetAgentByID", mock.Anything, orch).Return(&models.Agent{ID: orch, Role: models.AgentRoleOrchestrator}, nil).Once()
	h.LLM.On("Execute", mock.Anything, mock.MatchedBy(func(in agent.ExecutionInput) bool {
		return in.Role == string(models.AgentRoleOrchestrator)
	})).Return(okLLMResult(), nil).Once()
	h.TS.On("Transition", ctx, taskID, models.TaskStatusPlanning, mock.Anything).
		Run(func(mock.Arguments) { cancel() }).
		Return(baseTask(taskID, projectID, models.TaskStatusPlanning, &planner), nil).Once()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)

	require.ErrorIs(t, svc.ProcessTask(ctx, taskID), context.Canceled)
	h.LLM.AssertNumberOfCalls(t, "Execute", 1)
}

func TestConcurrency_PauseDuringDeveloper(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(5 * time.Millisecond), WithGracefulPauseTimeout(15 * time.Millisecond)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	h.TaskRepo.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	branch := "main"
	ip := withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(ip, nil).Once()
	h.TaskRepo.On("GetByID", ctx, taskID).Return(ip, nil).Once()
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.MatchedBy(func(in agent.ExecutionInput) bool {
		return in.Role == string(models.AgentRoleDeveloper)
	})).Run(func(mock.Arguments) {
		close(execStarted)
		time.Sleep(80 * time.Millisecond)
	}).Return(okSandboxResult(), nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(ip, nil).Times(4)
	paused := &models.Task{ID: taskID, ProjectID: projectID, Status: models.TaskStatusPaused, Title: "user task", Description: "d", AssignedAgentID: &dev}
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(paused, nil)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil)
	done := make(chan error)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	cancelStart()
}

func TestConcurrency_ResumePauseRace(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(3 * time.Millisecond), WithGracefulPauseTimeout(40 * time.Millisecond)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	h.TaskRepo.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))
	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	userID := uuid.New()
	branch := "main"
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.PS.On("GetByID", mock.Anything, userID, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	ip := withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(ip, nil).Times(2)
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		close(execStarted)
		time.Sleep(100 * time.Millisecond)
	}).Return(okSandboxResult(), nil).Once()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(ip, nil).Times(10)
	paused := &models.Task{ID: taskID, ProjectID: projectID, Status: models.TaskStatusPaused, Title: "user task", Description: "d", AssignedAgentID: &dev}
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(paused, nil).Maybe()
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil)

	done := make(chan error, 1)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.PublishCommand(ctx, UserTaskControlCommand{Kind: UserTaskControlPause, TaskID: taskID, UserID: userID, UserRole: models.RoleAdmin})
			bus.PublishCommand(ctx, UserTaskControlCommand{Kind: UserTaskControlResume, TaskID: taskID, UserID: userID, UserRole: models.RoleAdmin})
		}()
	}
	wg.Wait()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock")
	}
}

func TestConcurrency_MultipleCancelSignals(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	bus := NewUserTaskControlBus()
	stop := new(mockTaskSandboxStopper)
	svc, h := newTestOrchestratorHarness(t, orchestratorHarnessConfig{
		Bus:  bus,
		Stop: stop,
		Opts: []OrchestratorOption{WithStepPollInterval(0)},
	})
	startCtx, cancelStart := context.WithCancel(context.Background())
	defer cancelStart()
	h.TaskRepo.On("List", mock.Anything, mock.Anything).Return([]models.Task{}, int64(0), nil)
	require.NoError(t, svc.Start(startCtx))

	ctx := context.Background()
	taskID := uuid.New()
	projectID := uuid.New()
	dev := uuid.New()
	userID := uuid.New()
	branch := "main"
	h.TMR.On("Create", mock.Anything, mock.Anything).Return(nil)
	h.PS.On("GetByID", ctx, uuid.Nil, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID, GitURL: "https://github.com/o/r"}, nil)
	h.PS.On("GetByID", mock.Anything, userID, models.RoleAdmin, projectID).Return(&models.Project{ID: projectID}, nil)
	h.TaskRepo.On("GetByID", ctx, taskID).Return(withBranch(baseTask(taskID, projectID, models.TaskStatusInProgress, &dev), branch), nil).Times(2)
	h.WR.On("GetAgentByID", mock.Anything, dev).Return(&models.Agent{ID: dev, Role: models.AgentRoleDeveloper, CodeBackend: codeBackendClaude()}, nil)
	execStarted := make(chan struct{})
	h.Sandbox.On("Execute", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		close(execStarted)
		c := args.Get(0).(context.Context)
		<-c.Done()
	}).Return(nil, context.Canceled)
	h.TS.On("Transition", mock.Anything, taskID, models.TaskStatusCancelled, mock.Anything).Return(&models.Task{ID: taskID, Status: models.TaskStatusCancelled}, nil)
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, ProjectID: projectID, Status: models.TaskStatusCancelled, Title: "user task", Description: "d"}, nil)
	stop.On("StopTask", mock.Anything, taskID.String()).Return(nil).Maybe()
	h.TaskRepo.On("GetByID", mock.Anything, taskID).Return(&models.Task{ID: taskID, ProjectID: projectID, Title: "user task", Description: "d", Status: models.TaskStatusInProgress, AssignedAgentID: &dev}, nil).Maybe()

	done := make(chan error, 1)
	go func() { done <- svc.ProcessTask(ctx, taskID) }()
	<-execStarted
	for i := 0; i < 5; i++ {
		bus.PublishCommand(ctx, UserTaskControlCommand{Kind: UserTaskControlCancel, TaskID: taskID, UserID: userID, UserRole: models.RoleAdmin})
	}
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
	h.Sandbox.AssertNumberOfCalls(t, "Execute", 1)
}

