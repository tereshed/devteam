package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/models"
)

// MockWorkflowEngine
type MockWorkflowEngine struct {
	mock.Mock
}

func (m *MockWorkflowEngine) StartWorkflow(ctx context.Context, name string, input string) (*models.Execution, error) {
	args := m.Called(ctx, name, input)
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
func (m *MockWorkflowEngine) RunWorker(ctx context.Context) { m.Called(ctx) }
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

func TestWorkflowHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockEngine := new(MockWorkflowEngine)
	h := NewWorkflowHandler(mockEngine)

	wfs := []models.Workflow{{Name: "test"}}
	mockEngine.On("ListWorkflows", mock.Anything).Return(wfs, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/workflows", nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockEngine.AssertExpectations(t)
}

func TestWorkflowHandler_ListExecutions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockEngine := new(MockWorkflowEngine)
	h := NewWorkflowHandler(mockEngine)

	execs := []models.Execution{{ID: uuid.New()}}
	mockEngine.On("ListExecutions", mock.Anything, 20, 0).Return(execs, int64(1), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/executions", nil)

	h.ListExecutions(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockEngine.AssertExpectations(t)
}

