package service

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"log/slog"
	"os"
	"gorm.io/datatypes"
	"errors"
)

type mockTaskIndexer struct {
	mock.Mock
}

func (m *mockTaskIndexer) IndexTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}

func (m *mockTaskIndexer) IndexTaskFromModel(ctx context.Context, task *models.Task) error {
	return m.Called(ctx, task).Error(0)
}

func (m *mockTaskIndexer) IndexTaskWithData(ctx context.Context, task *models.Task, messages []models.TaskMessage) error {
	return m.Called(ctx, task, messages).Error(0)
}

func (m *mockTaskIndexer) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	return m.Called(ctx, taskID).Error(0)
}

func (m *mockTaskIndexer) DeleteProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}

func (m *mockTaskIndexer) IndexProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	return m.Called(ctx, projectID).Error(0)
}

type mockEventBus struct {
	mock.Mock
}

func (m *mockEventBus) Publish(ctx context.Context, ev events.DomainEvent) {
	m.Called(ctx, ev)
}

func (m *mockEventBus) Subscribe(name string, buffer int) (<-chan events.DomainEvent, func()) {
	args := m.Called(name, buffer)
	return args.Get(0).(<-chan events.DomainEvent), args.Get(1).(func())
}

func (m *mockEventBus) Close() {
	m.Called()
}

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
func (m *mockTaskRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Task, error) {
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
func (m *mockTaskRepository) Update(ctx context.Context, task *models.Task, expectedStatus models.TaskState, expectedUpdatedAt time.Time) error {
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
func (m *mockTaskProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	return m.Called(ctx, userID, userRole, projectID).Error(0)
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
func (m *mockTaskProjectService) Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
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

func (m *mockTaskTeamService) PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, agentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *mockTaskTeamService) GetAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, actor, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *mockTaskTeamService) UpdateAgentSettings(ctx context.Context, actor AgentSettingsActor, agentID uuid.UUID, req dto.UpdateAgentSettingsRequest) (*models.Agent, error) {
	args := m.Called(ctx, actor, agentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

type mockTransactionManager struct{ mock.Mock }

func (m *mockTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, fn)
	if fnErr := fn(ctx); fnErr != nil {
		return fnErr
	}
	return args.Error(0)
}

func newTaskServiceHarness() (*mockTaskRepository, *mockTaskMessageRepository, *mockTaskProjectService, *mockTaskTeamService, *mockTransactionManager, TaskService) {
	tr, tmr, ps, tms, txm, _, svc := newTaskServiceHarnessWithBus()
	return tr, tmr, ps, tms, txm, svc
}

func newTaskServiceHarnessWithBus() (*mockTaskRepository, *mockTaskMessageRepository, *mockTaskProjectService, *mockTaskTeamService, *mockTransactionManager, *mockEventBus, TaskService) {
	tr, tmr, ps, tms, txm, bus, _, svc := newTaskServiceHarnessFull()
	return tr, tmr, ps, tms, txm, bus, svc
}

func newTaskServiceHarnessFull() (*mockTaskRepository, *mockTaskMessageRepository, *mockTaskProjectService, *mockTaskTeamService, *mockTransactionManager, *mockEventBus, *mockTaskIndexer, TaskService) {
	tr := new(mockTaskRepository)
	tmr := new(mockTaskMessageRepository)
	ps := new(mockTaskProjectService)
	tms := new(mockTaskTeamService)
	txm := new(mockTransactionManager)
	bus := new(mockEventBus)
	idx := new(mockTaskIndexer)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	txm.On("WithTransaction", mock.Anything, mock.Anything).Return(nil).Maybe()
	bus.On("Publish", mock.Anything, mock.Anything).Return().Maybe()
	svc := NewTaskService(tr, tmr, ps, tms, txm, bus, idx, logger)
	return tr, tmr, ps, tms, txm, bus, idx, svc
}

func ownedProject() *models.Project {
	return &models.Project{ID: tsProjectID, UserID: tsUserID}
}

func TestTaskCreate_Success(t *testing.T) {
	tr, _, ps, _, _, _, idx, svc := newTaskServiceHarnessFull()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		task := args.Get(1).(*models.Task)
		task.ID = tsTaskID
	}).Return(nil)

	idx.On("IndexTaskFromModel", mock.Anything, mock.Anything).Return(nil)

	got, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "Hello"})
	require.NoError(t, err)
	assert.Equal(t, models.TaskStateActive, got.State)
	assert.Equal(t, models.CreatedByUser, got.CreatedByType)
	assert.Equal(t, tsUserID, got.CreatedByID)

	svc.Close()
	tr.AssertExpectations(t)
	idx.AssertExpectations(t)
}

func TestTaskEvents_Create(t *testing.T) {
	tr, _, ps, _, _, bus, idx, svc := newTaskServiceHarnessFull()
	ctx := context.Background()

	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		task := args.Get(1).(*models.Task)
		task.ID = tsTaskID
	}).Return(nil)

	bus.On("Publish", mock.Anything, mock.MatchedBy(func(ev events.DomainEvent) bool {
		e, ok := ev.(events.TaskStatusChanged)
		return ok && e.TaskID == tsTaskID && e.Current == string(models.TaskStateActive) && e.Previous == ""
	})).Return()

	idx.On("IndexTaskFromModel", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "Task 1"})
	require.NoError(t, err)

	svc.Close()
	bus.AssertExpectations(t)
	idx.AssertExpectations(t)
}

func TestTaskEvents_Cancel(t *testing.T) {
	tr, _, ps, _, _, bus, idx, svc := newTaskServiceHarnessFull()
	ctx := context.Background()

	task := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive, UpdatedAt: time.Now()}
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(task, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	tr.On("Update", ctx, mock.Anything, models.TaskStateActive, mock.Anything).Return(nil)

	bus.On("Publish", mock.Anything, mock.MatchedBy(func(ev events.DomainEvent) bool {
		e, ok := ev.(events.TaskStatusChanged)
		return ok && e.TaskID == tsTaskID && e.Current == string(models.TaskStateCancelled) && e.Previous == string(models.TaskStateActive)
	})).Return()

	idx.On("IndexTaskFromModel", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)

	svc.Close()
	bus.AssertExpectations(t)
	idx.AssertExpectations(t)
}

func TestTaskEvents_AddMessage(t *testing.T) {
	tr, tmr, ps, _, _, bus, idx, svc := newTaskServiceHarnessFull()
	ctx := context.Background()

	task := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(task, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	msgID := uuid.New()
	tmr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.TaskMessage).ID = msgID
	}).Return(nil)

	msg := &models.TaskMessage{ID: msgID, TaskID: tsTaskID, Content: "Hello", MessageType: models.MessageTypeComment}
	tmr.On("GetByID", ctx, msgID).Return(msg, nil)

	bus.On("Publish", mock.Anything, mock.MatchedBy(func(ev events.DomainEvent) bool {
		e, ok := ev.(events.TaskMessageCreated)
		return ok && e.MessageID == msgID && e.Content == "Hello"
	})).Return()

	_, err := svc.AddMessage(ctx, tsUserID, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{
		Content: "Hello", MessageType: string(models.MessageTypeComment),
	})
	require.NoError(t, err)

	svc.Close()
	bus.AssertExpectations(t)
	idx.AssertNotCalled(t, "IndexTask", mock.Anything, mock.Anything)
}

func TestTaskEvents_NoPublishOnRepositoryError(t *testing.T) {
	tr, _, ps, _, _, bus, _, svc := newTaskServiceHarnessFull()
	ctx := context.Background()

	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Create", ctx, mock.Anything).Return(errors.New("internal error"))

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "Task 1"})
	assert.Error(t, err)

	bus.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestTaskDelete_Indexer(t *testing.T) {
	tr, _, ps, _, _, _, idx, svc := newTaskServiceHarnessFull()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Delete", ctx, tsTaskID).Return(nil)
	idx.On("DeleteTask", mock.Anything, tsTaskID).Return(nil)

	err := svc.Delete(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)

	svc.Close()
	idx.AssertExpectations(t)
}

func TestTaskCreate_ProjectForbidden(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.Create(ctx, tsOtherUser, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x"})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskCreate_ProjectNotFound(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(nil, ErrProjectNotFound)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x"})
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestTaskCreate_EmptyTitle(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "   "})
	assert.ErrorIs(t, err, ErrTaskInvalidTitle)
}

func TestTaskCreate_InvalidPriority(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "ok", Priority: "nope"})
	assert.ErrorIs(t, err, ErrTaskInvalidPriority)
}

func TestTaskCreate_WithParentTask(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("GetByID", ctx, tsParentID).Return(&models.Task{ID: tsParentID, ProjectID: tsProjectID}, nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Task).ID = tsTaskID
	}).Return(nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	require.NoError(t, err)
}

func TestTaskCreate_ParentNotFound(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("GetByID", ctx, tsParentID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	assert.ErrorIs(t, err, ErrTaskParentNotFound)
}

func TestTaskCreate_WithAssignedAgent(t *testing.T) {
	tr, _, ps, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	tr.On("Create", ctx, mock.Anything).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Task).ID = tsTaskID
	}).Return(nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x", AssignedAgentID: &tsAgentID})
	require.NoError(t, err)
}

func TestTaskCreate_AgentNotInTeam(t *testing.T) {
	_, _, ps, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "x", AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskGetByID_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	task := &models.Task{ID: tsTaskID, ProjectID: tsProjectID}
	tr.On("GetByID", ctx, tsTaskID).Return(task, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	got, err := svc.GetByID(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
	assert.Equal(t, tsTaskID, got.ID)
}

func TestTaskGetByID_NotFound(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.GetByID(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskGetByID_ProjectForbidden(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.GetByID(ctx, tsOtherUser, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskList_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
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
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool { return f.Limit == 50 })).Return([]models.Task{}, int64(0), nil)

	_, _, err := svc.List(ctx, tsUserID, models.RoleUser, tsProjectID, dto.ListTasksRequest{Limit: 0})
	require.NoError(t, err)
}

func TestTaskList_MaxLimit(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("List", ctx, mock.MatchedBy(func(f repository.TaskFilter) bool { return f.Limit == 200 })).Return([]models.Task{}, int64(0), nil)

	_, _, err := svc.List(ctx, tsUserID, models.RoleUser, tsProjectID, dto.ListTasksRequest{Limit: 500})
	require.NoError(t, err)
}

func TestTaskUpdate_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Title: "old", State: models.TaskStateActive, Priority: models.TaskPriorityLow}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	newTitle := "new"
	desc := "d"
	pr := "high"
	got, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Title: &newTitle, Description: &desc, Priority: &pr})
	require.NoError(t, err)
	assert.Equal(t, "new", got.Title)
}

func TestTaskUpdate_Forbidden(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.Update(ctx, tsOtherUser, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}



func TestTaskUpdate_ReassignAgent(t *testing.T) {
	tr, _, ps, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{AssignedAgentID: &tsAgentID})
	require.NoError(t, err)
}

func TestTaskUpdate_ReassignAgentNotInTeam(t *testing.T) {
	tr, _, ps, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskUpdate_Concurrent(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, Title: "t", State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.Anything, models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(repository.ErrTaskConcurrentUpdate)

	newTitle := "x"
	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID, dto.UpdateTaskRequest{Title: &newTitle})
	assert.ErrorIs(t, err, ErrTaskConcurrentUpdate)
}




func TestTaskTransition_TestingToCompleted(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateDone && tk.CompletedAt != nil
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateDone, TransitionOpts{})
	require.NoError(t, err)
}

func TestTaskTransition_ToFailed(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	em := "boom"
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateFailed && tk.CompletedAt != nil && tk.ErrorMessage != nil && *tk.ErrorMessage == "boom"
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateFailed, TransitionOpts{ErrorMessage: &em})
	require.NoError(t, err)
}


func TestTaskTransition_FromTerminal(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateDone}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateActive, TransitionOpts{})
	assert.ErrorIs(t, err, ErrTaskTerminalStatus)
}

func TestTaskTransition_WithOpts(t *testing.T) {
	tr, _, _, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{{ID: tsAgentID}}}, nil)
	res := "done"
	art := datatypes.JSON([]byte(`{"pr":"http://x"}`))
	tr.On("Update", mock.Anything, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateActive && tk.AssignedAgentID != nil && *tk.AssignedAgentID == tsAgentID &&
			tk.Result != nil && *tk.Result == "done" && len(tk.Artifacts) > 0
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateActive, TransitionOpts{
		AssignedAgentID: &tsAgentID,
		Result:          &res,
		Artifacts:       &art,
	})
	require.NoError(t, err)
}

func TestTaskTransition_WithOptsAgentNotInTeam(t *testing.T) {
	tr, _, _, ts, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ts.On("GetByProjectID", ctx, tsProjectID).Return(&models.Team{Agents: []models.Agent{}}, nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateActive, TransitionOpts{AssignedAgentID: &tsAgentID})
	assert.ErrorIs(t, err, ErrAgentNotInTeam)
}

func TestTaskTransition_EmptyArtifactsBecomesEmptyJSONObject(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	emptySlice := datatypes.JSON([]byte{})
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	tr.On("Update", mock.Anything, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateActive && string(tk.Artifacts) == "{}"
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Transition(ctx, tsTaskID, models.TaskStateActive, TransitionOpts{Artifacts: &emptySlice})
	require.NoError(t, err)
}

// Sprint 17 / 6.10: Pause → state='paused' (а не needs_human). Использует
// GetByIDForUpdate для NOWAIT-защиты от гонок с финализацией воркером.
func TestTaskPause_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool { return tk.State == models.TaskStatePaused }), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}


func TestTaskPause_AlreadyPaused(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStatePaused}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

// Sprint 17 / 6.10: Pause при уже-терминальной задаче → ErrTaskAlreadyTerminal (HTTP 409),
// чтобы фронт показал info-toast, а не красный snack.
func TestTaskPause_FromTerminal(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateDone}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskAlreadyTerminal)
}

// Sprint 17 / 6.10: row-lock race — воркер прямо сейчас финализирует задачу.
// Pause не должен блокировать UI; отдаём 409.
func TestTaskPause_RowLocked_ReturnsAlreadyTerminal(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(nil, repository.ErrTaskLocked)

	_, err := svc.Pause(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskAlreadyTerminal)
}

func TestTaskCancel_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateCancelled && tk.CompletedAt != nil
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskCancel_FromTerminal(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateDone}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskAlreadyTerminal)
}

// Race-условие: между чтением state'а на фронте и POST /cancel воркер успел залочить
// строку для финализации (SELECT FOR UPDATE NOWAIT внутри Orchestrator). Cancel должен
// вернуть ErrTaskAlreadyTerminal (→ HTTP 409), не 500.
func TestTaskCancel_RowLocked_ReturnsAlreadyTerminal(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByIDForUpdate", ctx, tsTaskID).Return(nil, repository.ErrTaskLocked)

	_, err := svc.Cancel(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskAlreadyTerminal)
}

// Sprint 17 / 6.10: Resume из настоящего paused-состояния (новый v2-сентинель).
func TestTaskResume_FromPausedV2(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStatePaused}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateActive && tk.CompletedAt == nil
	}), models.TaskStatePaused, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskResume_FromNeedsHuman(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateNeedsHuman, CompletedAt: ptrTime(time.Now())}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateActive && tk.CompletedAt == nil
	}), models.TaskStateNeedsHuman, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskResume_FromFailed(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateFailed, CompletedAt: ptrTime(time.Now())}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.State == models.TaskStateActive && tk.CompletedAt == nil
	}), models.TaskStateFailed, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskResume_NotPausedOrFailed(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Resume(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskInvalidTransition)
}

func TestTaskAddMessage_Success(t *testing.T) {
	tr, tmr, ps, _, _, svc := newTaskServiceHarness()
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
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	_, err := svc.AddMessage(ctx, tsUserID, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{Content: "x", MessageType: string(models.MessageTypeInstruction)})
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

func TestTaskAddMessage_ProjectForbidden(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	_, err := svc.AddMessage(ctx, tsOtherUser, models.RoleUser, tsTaskID, dto.CreateTaskMessageRequest{Content: "x", MessageType: string(models.MessageTypeInstruction)})
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskListMessages_Success(t *testing.T) {
	tr, tmr, ps, _, _, svc := newTaskServiceHarness()
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
	tr, tmr, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tmr.On("ListByTaskID", ctx, tsTaskID, mock.MatchedBy(func(f repository.TaskMessageFilter) bool { return f.Limit == 50 })).
		Return([]models.TaskMessage{}, int64(0), nil)

	_, _, err := svc.ListMessages(ctx, tsUserID, models.RoleUser, tsTaskID, dto.ListTaskMessagesRequest{Limit: 0})
	require.NoError(t, err)
}

func TestTaskDelete_Success(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Delete", ctx, tsTaskID).Return(nil)

	err := svc.Delete(ctx, tsUserID, models.RoleUser, tsTaskID)
	require.NoError(t, err)
}

func TestTaskDelete_Forbidden(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(&models.Task{ID: tsTaskID, ProjectID: tsProjectID}, nil)
	ps.On("GetByID", ctx, tsOtherUser, models.RoleUser, tsProjectID).Return(ownedProject(), ErrProjectForbidden)

	err := svc.Delete(ctx, tsOtherUser, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrProjectForbidden)
}

func TestTaskDelete_NotFound(t *testing.T) {
	tr, _, _, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	tr.On("GetByID", ctx, tsTaskID).Return(nil, repository.ErrTaskNotFound)

	err := svc.Delete(ctx, tsUserID, models.RoleUser, tsTaskID)
	assert.ErrorIs(t, err, ErrTaskNotFound)
}

// --- custom_timeout server-side bounds (orchestration-v2-plan.md §6.5) ---

func strPtr(s string) *string { return &s }

func TestTaskCreate_RejectsTimeoutBelowMin(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	for _, raw := range []string{"0s", "30s", "59s"} {
		_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID,
			dto.CreateTaskRequest{Title: "x", CustomTimeout: strPtr(raw)})
		assert.ErrorIs(t, err, ErrTaskInvalidTimeout, "raw=%q", raw)
	}
}

func TestTaskCreate_RejectsTimeoutAboveMax(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	for _, raw := range []string{"73h", "168h", "9223372036s"} {
		_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID,
			dto.CreateTaskRequest{Title: "x", CustomTimeout: strPtr(raw)})
		assert.ErrorIs(t, err, ErrTaskInvalidTimeout, "raw=%q", raw)
	}
}

func TestTaskCreate_RejectsTimeoutMalformed(t *testing.T) {
	_, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	for _, raw := range []string{"abc", "4 hours", "-1h"} {
		_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID,
			dto.CreateTaskRequest{Title: "x", CustomTimeout: strPtr(raw)})
		assert.ErrorIs(t, err, ErrTaskInvalidTimeout, "raw=%q", raw)
	}
}

func TestTaskCreate_AcceptsBoundaryValues(t *testing.T) {
	for _, raw := range []string{"1m", "4h", "72h"} {
		tr, _, ps, _, _, svc := newTaskServiceHarness()
		ctx := context.Background()
		ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
		tr.On("Create", ctx, mock.MatchedBy(func(tk *models.Task) bool {
			return tk.CustomTimeout != nil && tk.CustomTimeout.Duration() > 0
		})).Run(func(args mock.Arguments) {
			args.Get(1).(*models.Task).ID = tsTaskID
		}).Return(nil)

		_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID,
			dto.CreateTaskRequest{Title: "x", CustomTimeout: strPtr(raw)})
		require.NoError(t, err, "raw=%q", raw)
	}
}

func TestTaskCreate_EmptyTimeoutIsNoOverride(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Create", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.CustomTimeout == nil
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*models.Task).ID = tsTaskID
	}).Return(nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID,
		dto.CreateTaskRequest{Title: "x", CustomTimeout: strPtr("")})
	require.NoError(t, err)
}

func TestTaskUpdate_RejectsTimeoutBelowMin(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID,
		dto.UpdateTaskRequest{CustomTimeout: strPtr("30s")})
	assert.ErrorIs(t, err, ErrTaskInvalidTimeout)
}

func TestTaskUpdate_RejectsTimeoutAboveMax(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil)
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID,
		dto.UpdateTaskRequest{CustomTimeout: strPtr("100h")})
	assert.ErrorIs(t, err, ErrTaskInvalidTimeout)
}

func TestTaskUpdate_AcceptsBoundaryValues(t *testing.T) {
	for _, raw := range []string{"1m", "4h", "72h"} {
		tr, _, ps, _, _, svc := newTaskServiceHarness()
		ctx := context.Background()
		base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive}
		tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
		ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
		tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
			return tk.CustomTimeout != nil && tk.CustomTimeout.Duration() > 0
		}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

		_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID,
			dto.UpdateTaskRequest{CustomTimeout: strPtr(raw)})
		require.NoError(t, err, "raw=%q", raw)
	}
}

func TestTaskUpdate_EmptyTimeoutClearsOverride(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	prev := models.IntervalDuration(2 * time.Hour)
	base := &models.Task{ID: tsTaskID, ProjectID: tsProjectID, State: models.TaskStateActive, CustomTimeout: &prev}
	tr.On("GetByID", ctx, tsTaskID).Return(base, nil).Once()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	tr.On("Update", ctx, mock.MatchedBy(func(tk *models.Task) bool {
		return tk.CustomTimeout == nil
	}), models.TaskStateActive, mock.AnythingOfType("time.Time")).Return(nil)

	_, err := svc.Update(ctx, tsUserID, models.RoleUser, tsTaskID,
		dto.UpdateTaskRequest{CustomTimeout: strPtr("")})
	require.NoError(t, err)
}

func TestTaskCreate_ParentWrongProject(t *testing.T) {
	tr, _, ps, _, _, svc := newTaskServiceHarness()
	ctx := context.Background()
	ps.On("GetByID", ctx, tsUserID, models.RoleUser, tsProjectID).Return(ownedProject(), nil)
	otherProject := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tr.On("GetByID", ctx, tsParentID).Return(&models.Task{ID: tsParentID, ProjectID: otherProject}, nil)

	_, err := svc.Create(ctx, tsUserID, models.RoleUser, tsProjectID, dto.CreateTaskRequest{Title: "sub", ParentTaskID: &tsParentID})
	assert.ErrorIs(t, err, ErrTaskParentNotFound)
}

func ptrTime(t time.Time) *time.Time { return &t }
