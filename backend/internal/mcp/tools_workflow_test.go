package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
)

// --- workflow_list ---

func TestWorkflowList_Success(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowListHandler(engine)

	engine.On("ListWorkflows", mock.Anything).Return([]models.Workflow{
		{ID: uuid.New(), Name: "wf1", Description: "d1", IsActive: true},
		{ID: uuid.New(), Name: "wf2", Description: "d2", IsActive: false}, // inactive
		{ID: uuid.New(), Name: "wf3", Description: "d3", IsActive: true},
	}, nil)

	result, structured, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowListData)
	assert.Equal(t, 2, data.Count) // только активные
	assert.Len(t, data.Workflows, 2)
	engine.AssertExpectations(t)
}

func TestWorkflowList_ServiceError(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowListHandler(engine)

	engine.On("ListWorkflows", mock.Anything).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	engine.AssertExpectations(t)
}

func TestWorkflowList_Pagination(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowListHandler(engine)

	workflows := make([]models.Workflow, 10)
	for i := range workflows {
		workflows[i] = models.Workflow{ID: uuid.New(), Name: "wf", IsActive: true}
	}
	engine.On("ListWorkflows", mock.Anything).Return(workflows, nil)

	limit := 3
	offset := 2
	result, structured, err := handler(context.Background(), nil, &WorkflowListParams{Limit: &limit, Offset: &offset})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowListData)
	assert.Equal(t, 10, data.Count)
	assert.Len(t, data.Workflows, 3)
	engine.AssertExpectations(t)
}

// --- workflow_start ---

func TestWorkflowStart_NilParams(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	handler := makeWorkflowStartHandler(engine, cfg)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStart_EmptyName(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	handler := makeWorkflowStartHandler(engine, cfg)

	result, _, err := handler(context.Background(), nil, &WorkflowStartParams{
		WorkflowName: "  ",
		Input:        "data",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStart_EmptyInput(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	handler := makeWorkflowStartHandler(engine, cfg)

	result, _, err := handler(context.Background(), nil, &WorkflowStartParams{
		WorkflowName: "test-wf",
		Input:        "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStart_InputTooLong(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	cfg.MaxInputRunes = 10
	handler := makeWorkflowStartHandler(engine, cfg)

	result, _, err := handler(context.Background(), nil, &WorkflowStartParams{
		WorkflowName: "test-wf",
		Input:        strings.Repeat("x", 11),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStart_Success(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	handler := makeWorkflowStartHandler(engine, cfg)

	execID := uuid.New()
	wfID := uuid.New()
	engine.On("StartWorkflow", mock.Anything, "my-wf", "input data").Return(&models.Execution{
		ID:         execID,
		WorkflowID: wfID,
		Status:     "pending",
	}, nil)

	result, structured, err := handler(context.Background(), nil, &WorkflowStartParams{
		WorkflowName: "my-wf",
		Input:        "input data",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowStartData)
	assert.Equal(t, execID.String(), data.ExecutionID)
	assert.Equal(t, "my-wf", data.WorkflowName)
	assert.Equal(t, wfID.String(), data.WorkflowID)
	engine.AssertExpectations(t)
}

func TestWorkflowStart_ServiceError(t *testing.T) {
	engine := new(mockWorkflowEngine)
	cfg := defaultMCPConfig()
	handler := makeWorkflowStartHandler(engine, cfg)

	engine.On("StartWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, &WorkflowStartParams{
		WorkflowName: "test",
		Input:        "data",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	engine.AssertExpectations(t)
}

// --- workflow_status ---

func TestWorkflowStatus_NilParams(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStatusHandler(engine)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStatus_InvalidUUID(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStatusHandler(engine)

	result, _, err := handler(context.Background(), nil, &ExecutionIDParams{ExecutionID: "not-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowStatus_Success(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStatusHandler(engine)

	execID := uuid.New()
	wfID := uuid.New()
	now := time.Now()

	engine.On("GetExecution", mock.Anything, execID).Return(&models.Execution{
		ID:         execID,
		WorkflowID: wfID,
		Status:     "running",
		StepCount:  3,
		MaxSteps:   10,
		InputData:  "some input",
		CreatedAt:  now,
	}, nil)

	result, structured, err := handler(context.Background(), nil, &ExecutionIDParams{ExecutionID: execID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowStatusData)
	assert.Equal(t, execID.String(), data.ExecutionID)
	assert.Equal(t, "running", data.Status)
	assert.Equal(t, 3, data.StepCount)
	assert.Equal(t, 10, data.MaxSteps)
	assert.Nil(t, data.FinishedAt) // running — FinishedAt не задан
	engine.AssertExpectations(t)
}

func TestWorkflowStatus_SuccessWithFinishedAt(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStatusHandler(engine)

	execID := uuid.New()
	wfID := uuid.New()
	now := time.Now()
	finishedAt := now.Add(5 * time.Minute)

	engine.On("GetExecution", mock.Anything, execID).Return(&models.Execution{
		ID:         execID,
		WorkflowID: wfID,
		Status:     "completed",
		StepCount:  10,
		MaxSteps:   10,
		CreatedAt:  now,
		FinishedAt: &finishedAt,
	}, nil)

	result, structured, err := handler(context.Background(), nil, &ExecutionIDParams{ExecutionID: execID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowStatusData)
	assert.Equal(t, "completed", data.Status)
	require.NotNil(t, data.FinishedAt)
	assert.Equal(t, finishedAt.Format(time.RFC3339), *data.FinishedAt)
	engine.AssertExpectations(t)
}

func TestWorkflowStatus_NotFound(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStatusHandler(engine)

	execID := uuid.New()
	engine.On("GetExecution", mock.Anything, execID).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, &ExecutionIDParams{ExecutionID: execID.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	engine.AssertExpectations(t)
}

// --- workflow_steps ---

func TestWorkflowSteps_NilParams(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStepsHandler(engine)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowSteps_InvalidUUID(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStepsHandler(engine)

	result, _, err := handler(context.Background(), nil, &WorkflowStepsParams{ExecutionID: "bad"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestWorkflowSteps_Success(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStepsHandler(engine)

	execID := uuid.New()
	agentID := uuid.New()
	now := time.Now()

	engine.On("GetExecutionSteps", mock.Anything, execID).Return([]models.ExecutionStep{
		{
			ID:            uuid.New(),
			StepID:        "step_1",
			AgentID:       &agentID,
			InputContext:  "ctx",
			OutputContent: "output",
			TokensUsed:    100,
			DurationMs:    500,
			CreatedAt:     now,
		},
	}, nil)

	result, structured, err := handler(context.Background(), nil, &WorkflowStepsParams{ExecutionID: execID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowStepsData)
	assert.Equal(t, execID.String(), data.ExecutionID)
	assert.Equal(t, 1, data.Count)
	assert.Len(t, data.Steps, 1)
	assert.Equal(t, "step_1", data.Steps[0].StepID)
	assert.NotNil(t, data.Steps[0].AgentID)
	assert.Equal(t, 100, data.Steps[0].TokensUsed)
	engine.AssertExpectations(t)
}

func TestWorkflowSteps_Pagination(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStepsHandler(engine)

	execID := uuid.New()
	steps := make([]models.ExecutionStep, 5)
	for i := range steps {
		steps[i] = models.ExecutionStep{ID: uuid.New(), StepID: "s"}
	}
	engine.On("GetExecutionSteps", mock.Anything, execID).Return(steps, nil)

	limit := 2
	offset := 1
	result, structured, err := handler(context.Background(), nil, &WorkflowStepsParams{
		ExecutionID: execID.String(),
		Limit:       &limit,
		Offset:      &offset,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*WorkflowStepsData)
	assert.Equal(t, 5, data.Count)
	assert.Len(t, data.Steps, 2)
	engine.AssertExpectations(t)
}

func TestWorkflowSteps_ServiceError(t *testing.T) {
	engine := new(mockWorkflowEngine)
	handler := makeWorkflowStepsHandler(engine)

	execID := uuid.New()
	engine.On("GetExecutionSteps", mock.Anything, execID).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, &WorkflowStepsParams{ExecutionID: execID.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	engine.AssertExpectations(t)
}
