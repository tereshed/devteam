package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

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
	scheduler := NewScheduler(repo, engine, nil)

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
