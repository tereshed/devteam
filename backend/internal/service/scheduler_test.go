package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
)

// MockProjectService for scheduler tests
type MockProjectService struct {
	mock.Mock
}

func (m *MockProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *MockProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *MockProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	args := m.Called(ctx, userID, userRole, req)
	return args.Get(0).([]models.Project), args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *MockProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

func (m *MockProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

func (m *MockProjectService) Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

func (m *MockProjectService) GetOwnerID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (m *MockProjectService) RunBackgroundReindexing(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// MockWorkflowEngine for scheduler tests
type MockWorkflowEngine struct {
	mock.Mock
}

func (m *MockWorkflowEngine) StartWorkflow(ctx context.Context, workflowName string, input string) (*models.Execution, error) {
	args := m.Called(ctx, workflowName, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *MockWorkflowEngine) GetExecution(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Execution), args.Error(1)
}

func (m *MockWorkflowEngine) RunWorker(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockWorkflowEngine) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Workflow), args.Error(1)
}

func (m *MockWorkflowEngine) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.Execution), args.Get(1).(int64), args.Error(2)
}

func (m *MockWorkflowEngine) GetExecutionSteps(ctx context.Context, id uuid.UUID) ([]models.ExecutionStep, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]models.ExecutionStep), args.Error(1)
}

func TestScheduler_Start(t *testing.T) {
	repo := new(MockWorkflowRepository)
	engine := new(MockWorkflowEngine)
	scheduler := NewScheduler(repo, engine, nil, nil, "")

	// Mock Schedules
	schedules := []models.ScheduledWorkflow{
		{
			Name:           "test_schedule",
			WorkflowName:   "test_wf",
			CronExpression: "@every 1s", // Run every second for test
			InputTemplate:  "test input",
			IsActive:       true,
		},
	}

	// Repo Expectations
	repo.On("ListActiveSchedules", mock.Anything).Return(schedules, nil)

	// Engine Expectations (Async, might happen)
	// Note: Since cron runs in background, asserting calls is tricky without waiting.
	// For unit test of Start(), we just check it loads schedules.

	// Start
	err := scheduler.Start(context.Background())
	assert.NoError(t, err)

	// Stop immediately
	scheduler.Stop()

	repo.AssertExpectations(t)
}
