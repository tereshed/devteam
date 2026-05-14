package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"gorm.io/datatypes"
)

func sampleTask(t *testing.T, uid, projectID uuid.UUID) *models.Task {
	t.Helper()
	now := time.Now().UTC()
	tid := uuid.New()
	return &models.Task{
		ID:            tid,
		ProjectID:     projectID,
		Title:         "T1",
		Description:   "",
		State:         models.TaskStateActive,
		Priority:      models.TaskPriorityMedium,
		CreatedByType: models.CreatedByUser,
		CreatedByID:   uid,
		Context:       datatypes.JSON([]byte("{}")),
		Artifacts:     datatypes.JSON([]byte("{}")),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestTaskList_Success(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)

	svc.On("List", mock.Anything, uid, models.RoleUser, pid, mock.MatchedBy(func(req dto.ListTasksRequest) bool {
		return req.Limit == 50 && req.Offset == 0
	})).Return([]models.Task{*task}, int64(1), nil)

	result, structured, err := h(ctx, nil, &TaskListParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, StatusOK, structured.(*Response).Status)
	data := structured.(*Response).Data.(dto.TaskListResponse)
	assert.Len(t, data.Tasks, 1)
	assert.Equal(t, int64(1), data.Total)
	assert.Equal(t, 50, data.Limit)
	assert.Equal(t, 0, data.Offset)
	svc.AssertExpectations(t)
}

func TestTaskList_MissingProjectID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskListParams{ProjectID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "List")
}

func TestTaskList_InvalidProjectID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskListParams{ProjectID: "not-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "List")
}

func TestTaskList_NoAuth(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	pid := uuid.New()

	result, structured, err := h(context.Background(), nil, &TaskListParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, structured.(*Response).Details, "authentication")
	svc.AssertNotCalled(t, "List")
}

func TestTaskList_ProjectForbidden(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()

	svc.On("List", mock.Anything, uid, models.RoleUser, pid, mock.Anything).
		Return(nil, int64(0), service.ErrProjectForbidden)

	result, _, err := h(ctx, nil, &TaskListParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskList_DefaultPagination(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()

	svc.On("List", mock.Anything, uid, models.RoleUser, pid, mock.MatchedBy(func(req dto.ListTasksRequest) bool {
		return req.Limit == 50 && req.Offset == 0
	})).Return([]models.Task{}, int64(0), nil)

	_, _, err := h(ctx, nil, &TaskListParams{ProjectID: pid.String()})
	require.NoError(t, err)
	svc.AssertExpectations(t)
}

func TestTaskList_WithFilters(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	agentID := uuid.New()
	st := "pending"
	pr := "high"

	svc.On("List", mock.Anything, uid, models.RoleUser, pid, mock.MatchedBy(func(req dto.ListTasksRequest) bool {
		return req.Status != nil && *req.Status == st &&
			req.Priority != nil && *req.Priority == pr &&
			req.AssignedAgentID != nil && *req.AssignedAgentID == agentID &&
			req.Limit == 10 && req.Offset == 5
	})).Return([]models.Task{}, int64(0), nil)

	limit := 10
	offset := 5
	_, _, err := h(ctx, nil, &TaskListParams{
		ProjectID:       pid.String(),
		Status:          &st,
		Priority:        &pr,
		AssignedAgentID: ptrStr(agentID.String()),
		Limit:           &limit,
		Offset:          &offset,
	})
	require.NoError(t, err)
	svc.AssertExpectations(t)
}

func ptrStr(s string) *string { return &s }

func TestTaskGet_Success(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskGetHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)

	svc.On("GetByID", mock.Anything, uid, models.RoleUser, task.ID).Return(task, nil)

	result, structured, err := h(ctx, nil, &TaskGetParams{TaskID: task.ID.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TaskResponse)
	assert.Equal(t, task.ID.String(), data.ID)
	assert.Equal(t, "T1", data.Title)
	assert.Equal(t, string(models.TaskStateActive), data.Status)
	svc.AssertExpectations(t)
}

func TestTaskGet_MissingTaskID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskGetHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskGetParams{TaskID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "GetByID")
}

func TestTaskGet_InvalidTaskID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskGetHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskGetParams{TaskID: "bad"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "GetByID")
}

func TestTaskGet_NotFound(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskGetHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	tid := uuid.New()

	svc.On("GetByID", mock.Anything, uid, models.RoleUser, tid).Return(nil, service.ErrTaskNotFound)

	result, _, err := h(ctx, nil, &TaskGetParams{TaskID: tid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskGet_Forbidden(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskGetHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	tid := uuid.New()

	svc.On("GetByID", mock.Anything, uid, models.RoleUser, tid).Return(nil, service.ErrProjectForbidden)

	result, _, err := h(ctx, nil, &TaskGetParams{TaskID: tid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskCreate_Success(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)
	task.Title = "New"

	svc.On("Create", mock.Anything, uid, models.RoleUser, pid, mock.MatchedBy(func(r dto.CreateTaskRequest) bool {
		return r.Title == "New"
	})).Return(task, nil)

	result, structured, err := h(ctx, nil, &TaskCreateParams{ProjectID: pid.String(), Title: "New"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TaskResponse)
	assert.Equal(t, "New", data.Title)
	assert.Equal(t, task.ID.String(), data.ID)
	svc.AssertExpectations(t)
}

func TestTaskCreate_MissingTitle(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)
	pid := uuid.New()

	result, structured, err := h(ctx, nil, &TaskCreateParams{ProjectID: pid.String(), Title: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, structured.(*Response).Details, "title is required")
	svc.AssertNotCalled(t, "Create")
}

func TestTaskCreate_MissingProjectID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)

	result, structured, err := h(ctx, nil, &TaskCreateParams{Title: "x"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, structured.(*Response).Details, "project_id is required")
	svc.AssertNotCalled(t, "Create")
}

func TestTaskCreate_InvalidProjectID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskCreateParams{ProjectID: "x", Title: "t"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "Create")
}

func TestTaskCreate_NoAuth(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	pid := uuid.New()

	result, structured, err := h(context.Background(), nil, &TaskCreateParams{ProjectID: pid.String(), Title: "t"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, structured.(*Response).Details, "authentication")
	svc.AssertNotCalled(t, "Create")
}

func TestTaskCreate_AgentNotInTeam(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	aid := uuid.New()

	svc.On("Create", mock.Anything, uid, models.RoleUser, pid, mock.Anything).
		Return(nil, service.ErrAgentNotInTeam)

	result, _, err := h(ctx, nil, &TaskCreateParams{
		ProjectID:       pid.String(),
		Title:           "t",
		AssignedAgentID: ptrStr(aid.String()),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskCreate_WithOptionalFields(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	aid := uuid.New()
	task := sampleTask(t, uid, pid)

	svc.On("Create", mock.Anything, uid, models.RoleUser, pid, mock.MatchedBy(func(r dto.CreateTaskRequest) bool {
		return r.Title == "t" && r.Description == "d" && r.Priority == "high" &&
			r.AssignedAgentID != nil && *r.AssignedAgentID == aid
	})).Return(task, nil)

	desc := "d"
	pr := "high"
	_, _, err := h(ctx, nil, &TaskCreateParams{
		ProjectID:       pid.String(),
		Title:           "t",
		Description:     &desc,
		Priority:        &pr,
		AssignedAgentID: ptrStr(aid.String()),
	})
	require.NoError(t, err)
	svc.AssertExpectations(t)
}

func TestTaskCreate_InvalidAgentID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskCreateHandler(svc, nil)
	ctx := testUserCtx(t)
	pid := uuid.New()

	result, _, err := h(ctx, nil, &TaskCreateParams{
		ProjectID:       pid.String(),
		Title:           "t",
		AssignedAgentID: ptrStr("nope"),
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "Create")
}

func TestTaskUpdate_Success(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)
	task.Title = "Updated"

	svc.On("Update", mock.Anything, uid, models.RoleUser, task.ID, mock.MatchedBy(func(r dto.UpdateTaskRequest) bool {
		return r.Title != nil && *r.Title == "Updated"
	})).Return(task, nil)

	newTitle := "Updated"
	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: task.ID.String(), Title: &newTitle})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskUpdate_MissingTaskID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "Update")
}

func TestTaskUpdate_InvalidTaskID(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: "bad"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "Update")
}

func TestTaskUpdate_NotFound(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	tid := uuid.New()

	svc.On("Update", mock.Anything, uid, models.RoleUser, tid, mock.Anything).
		Return(nil, service.ErrTaskNotFound)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: tid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskUpdate_InvalidTransition(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	tid := uuid.New()

	svc.On("Update", mock.Anything, uid, models.RoleUser, tid, mock.Anything).
		Return(nil, service.ErrTaskInvalidTransition)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: tid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskUpdate_ConcurrentUpdate(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	tid := uuid.New()

	svc.On("Update", mock.Anything, uid, models.RoleUser, tid, mock.Anything).
		Return(nil, service.ErrTaskConcurrentUpdate)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: tid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskUpdate_ChangeStatus(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)
	task.State = models.TaskStateActive

	st := "planning"
	svc.On("Update", mock.Anything, uid, models.RoleUser, task.ID, mock.MatchedBy(func(r dto.UpdateTaskRequest) bool {
		return r.Status != nil && *r.Status == st
	})).Return(task, nil)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: task.ID.String(), Status: &st})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestTaskUpdate_ClearAgent(t *testing.T) {
	svc := new(mockTaskService)
	h := makeTaskUpdateHandler(svc, nil)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	task := sampleTask(t, uid, pid)

	svc.On("Update", mock.Anything, uid, models.RoleUser, task.ID, mock.MatchedBy(func(r dto.UpdateTaskRequest) bool {
		return r.ClearAssignedAgent == true
	})).Return(task, nil)

	result, _, err := h(ctx, nil, &TaskUpdateParams{TaskID: task.ID.String(), ClearAssignedAgent: true})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}
