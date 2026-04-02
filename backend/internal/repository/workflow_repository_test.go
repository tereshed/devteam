//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/datatypes"
)

func TestWorkflowRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewWorkflowRepository(db)
	ctx := context.Background()

	// 1. Create Workflow
	wf := &models.Workflow{
		Name:          "test_wf",
		Description:   "Test Workflow",
		Configuration: datatypes.JSON([]byte(`{"steps":{}}`)),
		IsActive:      true,
	}
	err := repo.CreateWorkflow(ctx, wf)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, wf.ID)

	// 2. Get Workflow
	fetchedWf, err := repo.GetWorkflowByID(ctx, wf.ID)
	require.NoError(t, err)
	assert.Equal(t, wf.Name, fetchedWf.Name)

	// 3. Create Execution
	exec := &models.Execution{
		WorkflowID:    wf.ID,
		Status:        models.ExecutionPending,
		CurrentStepID: "start",
		InputData:     "input",
	}
	err = repo.CreateExecution(ctx, exec)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, exec.ID)

	// 4. Update Execution
	exec.Status = models.ExecutionRunning
	err = repo.UpdateExecution(ctx, exec)
	require.NoError(t, err)

	fetchedExec, err := repo.GetExecutionByID(ctx, exec.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ExecutionRunning, fetchedExec.Status)
}

func TestWorkflowRepository_Schedule(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewWorkflowRepository(db)
	ctx := context.Background()

	// Create Schedule
	schedule := &models.ScheduledWorkflow{
		Name:           "daily_run",
		WorkflowName:   "test_wf",
		CronExpression: "0 0 * * *",
		IsActive:       true,
	}
	err := repo.CreateScheduledWorkflow(ctx, schedule)
	require.NoError(t, err)

	// List
	schedules, err := repo.ListActiveSchedules(ctx)
	require.NoError(t, err)
	assert.Len(t, schedules, 1)
	assert.Equal(t, schedule.Name, schedules[0].Name)
}

func TestWorkflowRepository_Listings(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewWorkflowRepository(db)
	ctx := context.Background()

	// Setup Data
	wf := &models.Workflow{Name: "wf1", Configuration: datatypes.JSON([]byte("{}"))}
	repo.CreateWorkflow(ctx, wf)

	exec1 := &models.Execution{WorkflowID: wf.ID, Status: models.ExecutionCompleted}
	repo.CreateExecution(ctx, exec1)
	exec2 := &models.Execution{WorkflowID: wf.ID, Status: models.ExecutionRunning}
	repo.CreateExecution(ctx, exec2)

	step := &models.ExecutionStep{ExecutionID: exec1.ID, StepID: "step1"}
	repo.AddExecutionStep(ctx, step)

	// Test ListWorkflows
	wfs, err := repo.ListWorkflows(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(wfs), 1)

	// Test ListExecutions
	execs, count, err := repo.ListExecutions(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, execs, 2)

	// Test GetExecutionSteps
	steps, err := repo.GetExecutionSteps(ctx, exec1.ID)
	require.NoError(t, err)
	assert.Len(t, steps, 1)
	assert.Equal(t, step.StepID, steps[0].StepID)
}

