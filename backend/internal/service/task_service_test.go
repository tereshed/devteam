package service

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
	"github.com/devteam/backend/internal/repository"
	"gorm.io/datatypes"
)

var (
	tsUserID    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	tsProjectID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tsTaskID    = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	tsParentID  = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	tsAgentID   = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	tsOtherUser = uuid.MustParse("66666666-6666-6666-6666-666666666666")
)

type mockTaskRepository struct{ mock.Mock }

func (m *mockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	return m.Called(ctx, task).Error(0)
}
func (m *mockTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}
func (m *mockTaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]models.Task, int64, error) {
	args := m.Called(ctx, filter)
	var list []models.Task
	if v := args.Get(0); v != nil {
		list = v.([]models.Task)
	}
	return list, args.Get(1).(int64), args.Error(2)
}
func (m *mockTaskRepository) Update(ctx context.Context, task *models.Task, expectedStatus models.TaskStatus, expectedUpdatedAt time.Time) error {
	return m.Called(ctx, task, expectedStatus, expectedUpdatedAt).Error(0)
}
func (m *mockTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockTaskRepository) CountByProjectID(ctx context.Context, projectID uuid.UUID) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockTaskRepository) ListByParentID(ctx context.Context, parentTaskID uuid.UUID) ([]models.Task, error) {
	args := m.Called(ctx, parentTaskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

type mockTaskMessageRepository struct{ mock.Mock }

func (m *mockTaskMessageRepository) Create(ctx context.Context, msg *models.TaskMessage) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *mockTaskMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskMessage, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}
func (m *mockTaskMessageRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, taskID, filter)
	var list []models.TaskMessage
	if v := args.Get(0); v != nil {
		list = v.([]models.TaskMessage)
	}
	return list, args.Get(1).(int64), args.Error(2)
}
func (m *mockTaskMessageRepository) ListBySender(ctx context.Context, senderType models.SenderType, senderID uuid.UUID, filter repository.TaskMessageFilter) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, senderType, senderID, filter)
	var list []models.TaskMessage
	if v := args.Get(0); v != nil {
		list = v.([]models.TaskMessage)
	}
	return list, args.Get(1).(int64), args.Error(2)
}
func (m *mockTaskMessageRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(int64), args.Error(1)
}

type mockTaskProjectService struct{ mock.Mock }

func (m *mockTaskProjectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockTaskProjectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockTaskProjectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	args := m.Called(ctx, userID, userRole, req)
	var list []models.Project
	if v := args.Get(0); v != nil {
		list = v.([]models.Project)
	}
	return list, args.Get(1).(int64), args.Error(2)
}
func (m *mockTaskProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}
func (m *mockTaskProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
}

type mockTaskTeamService struct{ mock.Mock }

func (m *mockTaskTeamService) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}
func (m *mockTaskTeamService) Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func newTaskServiceHarness() (*mockTaskRepository, *mockTaskMessageRepository, *mockTaskProjectService, *mockTaskTeamService, TaskService) {
	tr := new(mockTaskRepository)
	tmr := new(mockTaskMessageRepository)
	ps := new(mockTaskProjectService)
	tms := new(mockTaskTeamService)
	return tr, tmr, ps, tms, NewTaskService(tr, tmr, ps, tms)
}

func ownedProject() *models.Project {
	return &models.Project{ID: tsProjectID, UserID: tsUserID}
}

func TestTaskCreate_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		task := args.Get(1).(*models.Task)
		task.ID = tsTaskID
	}).Return(nil)
	out := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Title: "Hello", Status: models.TaskStatusPending,
		Priority: models.TaskPriorityMedium, CreatedByType: models.CreatedByUser, CreatedByID: tsUserID}
	tr.On("GetByID", ctx, tsTaskID).Return(out, nil)

	got, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "Hello"})
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusPending, got.Status)
	assert.Equal(t, models.CreatedByUser, got.CreatedByType)
	assert.Equal(t, tsUserID, got.CreatedByID)
	tr.AssertExpectations(t)
}

func TestTaskCreate_ProjectForbidden(t *testing.T) {
	_, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.Create(ctx, tsOtherUser, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x"})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskCreate_ProjectNotFound(t *testing.T) {
	_, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(nil, ErrProjectNotFound)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x"})
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestTaskCreate_EmptyTitle(t *testing.T) {
	_, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "   "})
	assert.ErrorIs(t, err, ErrTaskInvalidTitle)
}

func TestTaskCreate_InvalidPriority(t *testing.T) {
	_, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "ok", Priority: "nope"})
	assert.ErrorIs(t, err, ErrTaskInvalidPriority)
}

func TestTaskCreate_WithParentTask(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("GetByID", ctx, tsParentID).Return(&models.Task{ID: tsParentID, ProjectID: tsProjectID}, nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Task).ID = tsTaskID
	}).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, ParentTaskID: &tsParentID}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	require.NoError(t, err)
}

func TestTaskCreate_ParentNotFound(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("GetByID", ctx, tsParentID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	assert.ErrorIs(t, err, ErrTaskParentNotFound)
}

func TestTaskCreate_WithAssignedAgent(t *testing.T) {
	tr, _, ps, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Task).ID = tsTaskID
	}).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, AssignedAgentID: &tsAgentID}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x", AssignedAgentID: &tsAgentID})
	require.NoError(t, err)
}

func TestTaskCreate_AgentNotInTeam(t *testing.T) {
	_, _, ps, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x", AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskGetByID_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	task := &models.Task{ID: tsTaskID, ProjectID: tsProjectID}
	tr.On("GetByID", ctx, tsTaskID).Return(task, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	got, err := svc.GetByID(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
	assert.Equal(t, tsTaskID, got.ID)
}

func TestTaskGetByID_NotFound(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.GetByID(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskGetByID_ProjectForbidden(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.GetByID(ctx, tsOtherUser, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskList_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	want := []models.Task{{ID: tsTaskID}}
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool {
		return f.ProjectID != nil && *f.ProjectID == tsProjectID && f.Limit == 50
	})).Return(want, int64(1), nil)

	list, total, err := svc.List(ctx, tsUserID, models.RoleUser, tsProjectID, dto.ListTasksRequest{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(1), total)
}

func TestTaskList_DefaultPagination(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool { return f.Limit == 50 })).Return([]models.Task{}, int64(0), nil)

	_, _, err := svc.List(ctx, tsUserID, models.RoleUser, tsProjectID, dto.ListTasksRequest{Limit: 0})
	require.NoError(t, err)
}

func TestTaskList_MaxLimit(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool { return f.Limit == 200 })).Return([]models.Task{}, int64(0), nil)

	_, _, err := svc.List(ctx, tsUserID, models.RoleUser, tsProjectID, dto.ListTasksRequest{Limit: 500})
	require.NoError(t, err)
}

func TestTaskUpdate_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Title: "old", Status: models.TaskStatusPending, Priority: models.TaskPriorityLow}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Title: "new", Description: "d", Priority: models.TaskPriorityHigh, Status: models.TaskStatusPending}, nil).Once()

	newTitle := "new"
	desc := "d"
	pr := "high"
	got, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Title: &newTitle, Description: &desc, Priority: &pr})
	require.NoError(t, err)
	assert.Equal(t, "new", got.Title)
}

func TestTaskUpdate_Forbidden(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.Update(ctx, tsOtherUser, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskUpdate_ChangeStatus(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPlanning}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusInProgress && tk.StartedAt != nil
	}), models.TaskStatusPlanning, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusInProgress}, nil).Once()

	st := "in_progress"
	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Status: &st})
	require.NoError(t, err)
}

func TestTaskUpdate_InvalidTransition(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	st := "completed"
	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Status: &st})
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskUpdate_ReassignAgent(t *testing.T) {
	tr, _, ps, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, AssignedAgentID: &tsAgentID}, nil).Once()

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{AssignedAgentID: &tsAgentID})
	require.NoError(t, err)
}

func TestTaskUpdate_ReassignAgentNotInTeam(t *testing.T) {
	tr, _, ps, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskUpdate_Concurrent(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Title: "t", Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(repository.ErrTaskConcurrentUpdate)

	newTitle := "x"
	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Title: &newTitle})
	assert.ErrorIs(t, err, ErrTaskConcurrentUpdate)
}

func TestTaskTransition_PendingToPlanning(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool { return tk.Status == models.TaskStatusPlanning }), models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPlanning}, nil).Once()

	got, err := svc.Transition(ctx, tsTaskID, models.TaskStatusPlanning, TransitionOpts{})
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusPlanning, got.Status)
}

func TestTaskTransition_InProgressToReview(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	started := time.Now().Add(-time.Hour)
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusInProgress, StartedAt: &started}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusReview && tk.StartedAt != nil
	}), models.TaskStatusInProgress, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusReview}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusReview, TransitionOpts{})
	require.NoError(t, err)
}

func TestTaskTransition_ReviewToChangesRequested(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusReview}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool { return tk.Status == models.TaskStatusChangesRequested }), models.TaskStatusReview, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusChangesRequested}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusChangesRequested, TransitionOpts{})
	require.NoError(t, err)
}

func TestTaskTransition_TestingToCompleted(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusTesting}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusCompleted && tk.CompletedAt != nil
	}), models.TaskStatusTesting, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusCompleted}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusCompleted, TransitionOpts{})
	require.NoError(t, err)
}

func TestTaskTransition_ToFailed(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPlanning}
	em := "boom"
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusFailed && tk.CompletedAt != nil && tk.ErrorMessage != nil && *tk.ErrorMessage == "boom"
	}), models.TaskStatusPlanning, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusFailed}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusFailed, TransitionOpts{ErrorMessage: &em})
	require.NoError(t, err)
}

func TestTaskTransition_InvalidTransition(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusCompleted, TransitionOpts{})
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskTransition_FromTerminal(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusCompleted}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusPlanning, TransitionOpts{})
	assert.ErrorIs(t, err, ErrTaskTerminalStatus)
}

func TestTaskTransition_WithOpts(t *testing.T) {
	tr, _, _, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	res := "done"
	art := datatypes.JSON([]byte(`{"pr":"http://x"}`))
	tr.On("Update", mock.Anything, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusPlanning && tk.AssignedAgentID != nil && *tk.AssignedAgentID == tsAgentID &&
			tk.Result != nil && *tk.Result == "done" && len(tk.Artifacts) > 0
	}), models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPlanning}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusPlanning, TransitionOpts{
		AssignedAgentID: &tsAgentID,
		Result:          &res,
		Artifacts:       &art,
	})
	require.NoError(t, err)
}

func TestTaskTransition_WithOptsAgentNotInTeam(t *testing.T) {
	tr, _, _, ts, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusPlanning, TransitionOpts{AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskTransition_EmptyArtifactsBecomesEmptyJSONObject(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}
	emptySlice := datatypes.JSON([]byte{})
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", mock.Anything, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusPlanning && string(tk.Artifacts) == "{}"
	}), models.TaskStatusPending, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPlanning, Artifacts: datatypes.JSON([]byte("{}"))}, nil).Once()

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStatusPlanning, TransitionOpts{Artifacts: &emptySlice})
	require.NoError(t, err)
}

func TestTaskPause_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusInProgress}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool { return tk.Status == models.TaskStatusPaused }), models.TaskStatusInProgress, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPaused}, nil).Once()

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskPause_FromPending(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPending}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskPause_AlreadyPaused(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPaused}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskCancel_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusInProgress}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusCancelled && tk.CompletedAt != nil
	}), models.TaskStatusInProgress, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusCancelled}, nil).Once()

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskCancel_FromTerminal(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusCompleted}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskResume_FromPaused(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusPaused, CompletedAt: ptrTime(time.Now())}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusPending && tk.CompletedAt == nil
	}), models.TaskStatusPaused, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPending}, nil).Once()

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskResume_FromFailed(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusFailed, CompletedAt: ptrTime(time.Now())}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.Status == models.TaskStatusPending && tk.CompletedAt == nil
	}), models.TaskStatusFailed, mock.AnythingOfType("time.Time")).Return(nil)
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, Status: models.TaskStatusPending}, nil).Once()

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskResume_NotPausedOrFailed(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, Status: models.TaskStatusInProgress}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskAddMessage_Success(t *testing.T) {
	tr, tmr, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	msgID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	tmr.On("Create", mock.Anything, mock.MatchedBy(func(m *models.TaskMessage) bool {
		return m.SenderType == models.SenderTypeUser && m.SenderID == tsUserID && m.MessageType == models.MessageTypeInstruction
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.TaskMessage).ID = msgID
	}).Return(nil)
	tmr.On("GetByID", mock.Anything, msgID).Return(&models.TaskMessage{ID: msgID, SenderType: models.SenderTypeUser}, nil)

	got, err := svc.AddMessage(ctx, tsUserID, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{
		Content: "hi", MessageType: string(models.MessageTypeInstruction),
	})
	require.NoError(t, err)
	assert.Equal(t, models.SenderTypeUser, got.SenderType)
}

func TestTaskAddMessage_TaskNotFound(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.AddMessage(ctx, tsUserID, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{Content: "x", MessageType: string(models.MessageTypeInstruction)})
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskAddMessage_ProjectForbidden(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.AddMessage(ctx, tsOtherUser, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{Content: "x", MessageType: string(models.MessageTypeInstruction)})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskListMessages_Success(t *testing.T) {
	tr, tmr, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tmr.On("ListByTaskID", ctx, tsTaskID, mock.MatchedBy(func(f repository.TaskMessageFilter) bool { return f.Limit == 50 })).
		Return([]models.TaskMessage{{ID: uuid.New()}}, int64(1), nil)

	list, n, err := svc.ListMessages(ctx, tsUserID, models.RoleUser, tsTaskID, dto.ListTaskMessagesRequest{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(1), n)
}

func TestTaskListMessages_DefaultPagination(t *testing.T) {
	tr, tmr, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tmr.On("ListByTaskID", ctx, tsTaskID, mock.MatchedBy(func(f repository.TaskMessageFilter) bool { return f.Limit == 50 })).
		Return([]models.TaskMessage{}, int64(0), nil)

	_, _, err := svc.ListMessages(ctx, tsUserID, models.RoleUser, tsTaskID, dto.ListTaskMessagesRequest{Limit: 0})
	require.NoError(t, err)
}

func TestTaskDelete_Success(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Delete", ctx, tsTaskID).Return(nil)

	err := svc.Delete(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskDelete_Forbidden(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	err := svc.Delete(ctx, tsOtherUser, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskDelete_NotFound(t *testing.T) {
	tr, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	err := svc.Delete(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskCreate_ParentWrongProject(t *testing.T) {
	tr, _, ps, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	otherProject := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tr.On("GetByID", ctx, tsParentID).Return(&models.Task{ID: tsParentID, ProjectID: otherProject}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	assert.ErrorIs(t, err, ErrTaskParentNotFound)
}

func ptrTime(t time.Time) *time.Time { return &t }
