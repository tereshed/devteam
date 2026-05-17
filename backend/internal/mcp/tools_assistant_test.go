package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test doubles
// ─────────────────────────────────────────────────────────────────────────────

type recordingNotifier struct {
	UserID  string
	MsgType string
	Payload []byte
	Calls   int
	ErrRet  error // если != nil — SendToUser возвращает эту ошибку
}

func (r *recordingNotifier) SendToUser(userID, msgType string, payload []byte) error {
	r.Calls++
	r.UserID = userID
	r.MsgType = msgType
	r.Payload = payload
	return r.ErrRet
}

type mockUserRepoForAssistant struct {
	mock.Mock
}

func (m *mockUserRepoForAssistant) Create(ctx context.Context, user *models.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockUserRepoForAssistant) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}
func (m *mockUserRepoForAssistant) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}
func (m *mockUserRepoForAssistant) Update(ctx context.Context, user *models.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockUserRepoForAssistant) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// app_navigate
// ─────────────────────────────────────────────────────────────────────────────

func TestAppNavigate_Success(t *testing.T) {
	n := &recordingNotifier{}
	h := makeAppNavigateHandler(n)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	result, structured, err := h(ctx, nil, &AppNavigateParams{Route: "/projects/abc"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	resp := structured.(*Response)
	assert.Equal(t, StatusOK, resp.Status)
	data := resp.Data.(AppNavigateData)
	assert.Equal(t, "sent", data.Status)
	assert.Equal(t, "/projects/abc", data.Route)

	// WS-сторонний эффект.
	assert.Equal(t, 1, n.Calls)
	assert.Equal(t, uid.String(), n.UserID)
	assert.Equal(t, wsTypeAssistantNavigate, n.MsgType)
	var p map[string]string
	require.NoError(t, json.Unmarshal(n.Payload, &p))
	assert.Equal(t, wsTypeAssistantNavigate, p["type"])
	assert.Equal(t, "/projects/abc", p["route"])
}

func TestAppNavigate_NoAuth(t *testing.T) {
	n := &recordingNotifier{}
	h := makeAppNavigateHandler(n)

	result, _, err := h(context.Background(), nil, &AppNavigateParams{Route: "/x"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 0, n.Calls)
}

func TestAppNavigate_EmptyRoute(t *testing.T) {
	n := &recordingNotifier{}
	h := makeAppNavigateHandler(n)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &AppNavigateParams{Route: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 0, n.Calls)
}

func TestAppNavigate_RouteMissingSlash(t *testing.T) {
	n := &recordingNotifier{}
	h := makeAppNavigateHandler(n)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &AppNavigateParams{Route: "projects"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 0, n.Calls)
}

func TestAppNavigate_NilParams(t *testing.T) {
	n := &recordingNotifier{}
	h := makeAppNavigateHandler(n)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 0, n.Calls)
}

func TestAppNavigate_NotifierError(t *testing.T) {
	n := &recordingNotifier{ErrRet: errors.New("hub down")}
	h := makeAppNavigateHandler(n)
	ctx := testUserCtx(t)

	result, structured, err := h(ctx, nil, &AppNavigateParams{Route: "/dashboard"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, StatusError, structured.(*Response).Status)
	assert.Equal(t, 1, n.Calls)
}

// ─────────────────────────────────────────────────────────────────────────────
// assistant_active_tasks_count
// ─────────────────────────────────────────────────────────────────────────────

func TestAssistantActiveTasksCount_Success(t *testing.T) {
	svc := new(mockTaskService)
	h := makeAssistantActiveTasksCountHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	rows := []repository.ActiveTaskRow{
		{TaskID: uuid.New(), ProjectID: uuid.New(), ProjectName: "p1", Title: "t1", State: models.TaskStateActive, UpdatedAt: time.Now().UTC()},
		{TaskID: uuid.New(), ProjectID: uuid.New(), ProjectName: "p2", Title: "t2", State: models.TaskStateActive, UpdatedAt: time.Now().UTC()},
		{TaskID: uuid.New(), ProjectID: uuid.New(), ProjectName: "p3", Title: "t3", State: models.TaskStateActive, UpdatedAt: time.Now().UTC()},
	}
	svc.On("ListActiveByUser", mock.Anything, uid, []models.TaskState{models.TaskStateActive}, activeTasksCountLimit).
		Return(rows, nil)

	result, structured, err := h(ctx, nil, &AssistantActiveTasksCountParams{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	resp := structured.(*Response)
	assert.Equal(t, StatusOK, resp.Status)
	assert.Equal(t, 3, resp.Data.(AssistantActiveTasksCountData).Count)
	svc.AssertExpectations(t)
}

func TestAssistantActiveTasksCount_Empty(t *testing.T) {
	svc := new(mockTaskService)
	h := makeAssistantActiveTasksCountHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("ListActiveByUser", mock.Anything, uid, []models.TaskState{models.TaskStateActive}, activeTasksCountLimit).
		Return([]repository.ActiveTaskRow{}, nil)

	result, structured, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, 0, structured.(*Response).Data.(AssistantActiveTasksCountData).Count)
}

func TestAssistantActiveTasksCount_NoAuth(t *testing.T) {
	svc := new(mockTaskService)
	h := makeAssistantActiveTasksCountHandler(svc)

	result, _, err := h(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "ListActiveByUser")
}

func TestAssistantActiveTasksCount_ServiceError(t *testing.T) {
	svc := new(mockTaskService)
	h := makeAssistantActiveTasksCountHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("ListActiveByUser", mock.Anything, uid, []models.TaskState{models.TaskStateActive}, activeTasksCountLimit).
		Return(nil, assert.AnError)

	result, _, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ─────────────────────────────────────────────────────────────────────────────
// whoami
// ─────────────────────────────────────────────────────────────────────────────

func TestWhoAmI_Success(t *testing.T) {
	repo := new(mockUserRepoForAssistant)
	h := makeWhoAmIHandler(repo)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	repo.On("GetByID", mock.Anything, uid).
		Return(&models.User{
			ID:            uid,
			Email:         "alice@example.com",
			Role:          models.RoleUser,
			EmailVerified: true,
		}, nil)

	result, structured, err := h(ctx, nil, &WhoAmIParams{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	resp := structured.(*Response)
	assert.Equal(t, StatusOK, resp.Status)
	data := resp.Data.(WhoAmIData)
	assert.Equal(t, uid.String(), data.UserID)
	assert.Equal(t, "alice@example.com", data.Email)
	assert.Equal(t, string(models.RoleUser), data.Role)
	assert.True(t, data.EmailVerified)
}

func TestWhoAmI_NoAuth(t *testing.T) {
	repo := new(mockUserRepoForAssistant)
	h := makeWhoAmIHandler(repo)

	result, _, err := h(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	repo.AssertNotCalled(t, "GetByID")
}

func TestWhoAmI_UserNotFound(t *testing.T) {
	repo := new(mockUserRepoForAssistant)
	h := makeWhoAmIHandler(repo)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	repo.On("GetByID", mock.Anything, uid).Return(nil, repository.ErrUserNotFound)

	result, structured, err := h(ctx, nil, nil)
	require.NoError(t, err)
	// Не-фатально: возвращаем минимум из ctx.
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(WhoAmIData)
	assert.Equal(t, uid.String(), data.UserID)
	assert.Equal(t, string(models.RoleUser), data.Role)
	assert.Empty(t, data.Email)
}

func TestWhoAmI_RepoError(t *testing.T) {
	repo := new(mockUserRepoForAssistant)
	h := makeWhoAmIHandler(repo)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	repo.On("GetByID", mock.Anything, uid).Return(nil, errors.New("db down"))

	result, _, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration smoke: nil deps → ничего не падает.
// ─────────────────────────────────────────────────────────────────────────────

func TestRegisterAssistantTools_AllDepsRegister(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test", Version: "0"}, nil)
	require.NotPanics(t, func() {
		RegisterAssistantTools(server, AssistantToolsDeps{
			Notifier:    &recordingNotifier{},
			TaskService: new(mockTaskService),
			UserRepo:    new(mockUserRepoForAssistant),
		})
	})
}

func TestRegisterAssistantTools_NilDepsAreNoOp(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test", Version: "0"}, nil)
	require.NotPanics(t, func() {
		RegisterAssistantTools(server, AssistantToolsDeps{}) // все nil → no-op
	})
}
