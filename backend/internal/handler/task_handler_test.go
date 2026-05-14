package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"gorm.io/datatypes"
)

var (
	testTaskProjectID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	testTaskID        = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
)

// MockTaskService мок TaskService для unit-тестов TaskHandler.
type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) Create(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.CreateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.ListTasksRequest) ([]models.Task, int64, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	var tasks []models.Task
	if args.Get(0) != nil {
		tasks = args.Get(0).([]models.Task)
	}
	total := int64(0)
	if args.Get(1) != nil {
		total = args.Get(1).(int64)
	}
	return tasks, total, args.Error(2)
}

func (m *MockTaskService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.UpdateTaskRequest) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, taskID)
	return args.Error(0)
}

func (m *MockTaskService) Pause(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) Cancel(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) Resume(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) Correct(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, text string) (*models.Task, error) {
	args := m.Called(ctx, userID, userRole, taskID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) Transition(ctx context.Context, taskID uuid.UUID, newStatus models.TaskState, opts service.TransitionOpts) (*models.Task, error) {
	args := m.Called(ctx, taskID, newStatus, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) AddMessage(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.CreateTaskMessageRequest) (*models.TaskMessage, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskMessage), args.Error(1)
}

func (m *MockTaskService) ListMessages(ctx context.Context, userID uuid.UUID, userRole models.UserRole, taskID uuid.UUID, req dto.ListTaskMessagesRequest) ([]models.TaskMessage, int64, error) {
	args := m.Called(ctx, userID, userRole, taskID, req)
	var msgs []models.TaskMessage
	if args.Get(0) != nil {
		msgs = args.Get(0).([]models.TaskMessage)
	}
	total := int64(0)
	if args.Get(1) != nil {
		total = args.Get(1).(int64)
	}
	return msgs, total, args.Error(2)
}

func (m *MockTaskService) Close() error {
	args := m.Called()
	return args.Error(0)
}

func setupTaskRouter(mockSvc *MockTaskService, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTaskHandler(mockSvc, nil, nil)

	authFn := func(c *gin.Context) {
		c.Set("userID", testProjectUserID)
		c.Set("userRole", string(models.RoleUser))
		c.Next()
	}

	projects := r.Group("/projects")
	if withAuth {
		projects.Use(authFn)
	}
	{
		projects.POST("/:id/tasks", h.Create)
		projects.GET("/:id/tasks", h.List)
	}

	tasks := r.Group("/tasks")
	if withAuth {
		tasks.Use(authFn)
	}
	{
		tasks.GET("/:id", h.GetByID)
		tasks.PUT("/:id", h.Update)
		tasks.DELETE("/:id", h.Delete)
		tasks.POST("/:id/pause", h.Pause)
		tasks.POST("/:id/cancel", h.Cancel)
		tasks.POST("/:id/resume", h.Resume)
		tasks.GET("/:id/messages", h.ListMessages)
		tasks.POST("/:id/messages", h.AddMessage)
	}
	return r
}

func sampleTaskModel() *models.Task {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	return &models.Task{
		ID:              testTaskID,
		ProjectID:       testTaskProjectID,
		Title:           "Task title",
		Description:     "",
		State:           models.TaskStateActive,
		Priority:        models.TaskPriorityMedium,
		CreatedByType:   models.CreatedByUser,
		CreatedByID:     testProjectUserID,
		Context:         datatypes.JSON([]byte("{}")),
		Artifacts:       datatypes.JSON([]byte("{}")),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func sampleTaskMessageModel() *models.TaskMessage {
	now := time.Date(2026, 3, 1, 13, 0, 0, 0, time.UTC)
	id := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	return &models.TaskMessage{
		ID:          id,
		TaskID:      testTaskID,
		SenderType:  models.SenderTypeUser,
		SenderID:    testProjectUserID,
		Content:     "hello",
		MessageType: models.MessageTypeInstruction,
		Metadata:    datatypes.JSON([]byte("{}")),
		CreatedAt:   now,
	}
}

func TestTask_Create_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	mockSvc.On("Create", mock.Anything, testProjectUserID, models.RoleUser, testTaskProjectID, mock.AnythingOfType("dto.CreateTaskRequest")).
		Return(task, nil)

	w := httptest.NewRecorder()
	body := `{"title":"Task title"}`
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, testTaskID.String(), got.ID)
	assert.Equal(t, "Task title", got.Title)
	mockSvc.AssertExpectations(t)
}

func TestTask_Create_InvalidJSON(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestTask_Create_MissingTitle(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestTask_Create_InvalidProjectID(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/not-a-uuid/tasks", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestTask_Create_ProjectNotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Create", mock.Anything, testProjectUserID, models.RoleUser, testTaskProjectID, mock.Anything).
		Return((*models.Task)(nil), service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrNotFound, er.Error)
}

func TestTask_Create_ProjectForbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*models.Task)(nil), service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTask_Create_AgentNotInTeam(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*models.Task)(nil), service.ErrAgentNotInTeam)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrUnprocessable, er.Error)
}

func TestTask_Create_NoAuth(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+testTaskProjectID.String()+"/tasks", bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, false).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestTask_List_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	mockSvc.On("List", mock.Anything, testProjectUserID, models.RoleUser, testTaskProjectID, mock.MatchedBy(func(r dto.ListTasksRequest) bool {
		return r.Limit == 50 && r.Offset == 0
	})).Return([]models.Task{*task}, int64(1), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+testTaskProjectID.String()+"/tasks", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Tasks, 1)
	assert.Equal(t, int64(1), got.Total)
	assert.Equal(t, 50, got.Limit)
	assert.Equal(t, 0, got.Offset)
}

func TestTask_List_WithQueryParams(t *testing.T) {
	mockSvc := new(MockTaskService)
	st := "in_progress"
	pr := "high"
	search := "auth"
	mockSvc.On("List", mock.Anything, testProjectUserID, models.RoleUser, testTaskProjectID, mock.MatchedBy(func(r dto.ListTasksRequest) bool {
		return r.Limit == 10 && r.Offset == 3 &&
			r.Status != nil && *r.Status == st &&
			r.Priority != nil && *r.Priority == pr &&
			r.Search != nil && *r.Search == search &&
			r.RootOnly
	})).Return([]models.Task{}, int64(0), nil)

	q := "/projects/" + testTaskProjectID.String() + "/tasks?status=in_progress&priority=high&search=auth&root_only=true&limit=10&offset=3"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, q, nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestTask_List_ProjectForbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("List", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]models.Task(nil), int64(0), service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+testTaskProjectID.String()+"/tasks", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTask_GetByID_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	mockSvc.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, testTaskID).Return(task, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, testTaskID.String(), got.ID)
}

func TestTask_GetByID_InvalidUUID(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/bad", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "GetByID")
}

func TestTask_GetByID_NotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("GetByID", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_GetByID_Forbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("GetByID", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTask_Update_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	task.Title = "Updated"
	mockSvc.On("Update", mock.Anything, testProjectUserID, models.RoleUser, testTaskID, mock.AnythingOfType("dto.UpdateTaskRequest")).
		Return(task, nil)

	w := httptest.NewRecorder()
	body := `{"title":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Updated", got.Title)
}

func TestTask_Update_InvalidJSON(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Update")
}

func TestTask_Update_NotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Update", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.Task)(nil), service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_Update_Forbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Update", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.Task)(nil), service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTask_Update_InvalidTransition(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Update", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.Task)(nil), service.ErrTaskInvalidTransition)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(`{"status":"completed"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrConflict, er.Error)
}

func TestTask_Update_Concurrent(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Update", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.Task)(nil), service.ErrTaskConcurrentUpdate)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrConflict, er.Error)
}

func TestTask_Update_AgentNotInTeam(t *testing.T) {
	aid := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	mockSvc := new(MockTaskService)
	mockSvc.On("Update", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.Task)(nil), service.ErrAgentNotInTeam)

	w := httptest.NewRecorder()
	body := `{"assigned_agent_id":"` + aid.String() + `"}`
	req := httptest.NewRequest(http.MethodPut, "/tasks/"+testTaskID.String(), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTask_Delete_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Delete", mock.Anything, testProjectUserID, models.RoleUser, testTaskID).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestTask_Delete_NotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Delete", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return(service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_Delete_Forbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Delete", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return(service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/tasks/"+testTaskID.String(), nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTask_Pause_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	task.State = models.TaskStateNeedsHuman
	mockSvc.On("Pause", mock.Anything, testProjectUserID, models.RoleUser, testTaskID).Return(task, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/pause", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, string(models.TaskStateNeedsHuman), got.Status)
}

func TestTask_Pause_InvalidTransition(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Pause", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrTaskInvalidTransition)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/pause", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTask_Pause_NotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Pause", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/pause", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_Cancel_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	task.State = models.TaskStateCancelled
	mockSvc.On("Cancel", mock.Anything, testProjectUserID, models.RoleUser, testTaskID).Return(task, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/cancel", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, string(models.TaskStateCancelled), got.Status)
}

func TestTask_Cancel_TerminalStatus(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Cancel", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrTaskTerminalStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/cancel", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTask_Resume_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	task := sampleTaskModel()
	task.State = models.TaskStateActive
	mockSvc.On("Resume", mock.Anything, testProjectUserID, models.RoleUser, testTaskID).Return(task, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/resume", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, string(models.TaskStateActive), got.Status)
}

func TestTask_Resume_InvalidTransition(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("Resume", mock.Anything, mock.Anything, mock.Anything, testTaskID).
		Return((*models.Task)(nil), service.ErrTaskInvalidTransition)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/resume", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTask_ListMessages_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	msg := sampleTaskMessageModel()
	mockSvc.On("ListMessages", mock.Anything, testProjectUserID, models.RoleUser, testTaskID, mock.MatchedBy(func(r dto.ListTaskMessagesRequest) bool {
		return r.Limit == 50 && r.Offset == 0
	})).Return([]models.TaskMessage{*msg}, int64(1), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID.String()+"/messages", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TaskMessageListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Messages, 1)
	assert.Equal(t, int64(1), got.Total)
}

func TestTask_ListMessages_WithQueryParams(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("ListMessages", mock.Anything, testProjectUserID, models.RoleUser, testTaskID, mock.MatchedBy(func(r dto.ListTaskMessagesRequest) bool {
		return r.Limit == 25 && r.Offset == 2 &&
			r.MessageType != nil && *r.MessageType == "result" &&
			r.SenderType != nil && *r.SenderType == "user"
	})).Return([]models.TaskMessage{}, int64(0), nil)

	q := "/tasks/" + testTaskID.String() + "/messages?message_type=result&sender_type=user&limit=25&offset=2"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, q, nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestTask_ListMessages_TaskNotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("ListMessages", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return([]models.TaskMessage(nil), int64(0), service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks/"+testTaskID.String()+"/messages", nil)
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_AddMessage_Success(t *testing.T) {
	mockSvc := new(MockTaskService)
	msg := sampleTaskMessageModel()
	mockSvc.On("AddMessage", mock.Anything, testProjectUserID, models.RoleUser, testTaskID, mock.AnythingOfType("dto.CreateTaskMessageRequest")).
		Return(msg, nil)

	w := httptest.NewRecorder()
	body := `{"content":"hello","message_type":"instruction"}`
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.TaskMessageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "hello", got.Content)
}

func TestTask_AddMessage_InvalidJSON(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/messages", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "AddMessage")
}

func TestTask_AddMessage_MissingContent(t *testing.T) {
	mockSvc := new(MockTaskService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/messages", bytes.NewBufferString(`{"message_type":"instruction"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "AddMessage")
}

func TestTask_AddMessage_TaskNotFound(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("AddMessage", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.TaskMessage)(nil), service.ErrTaskNotFound)

	w := httptest.NewRecorder()
	body := `{"content":"x","message_type":"instruction"}`
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTask_AddMessage_Forbidden(t *testing.T) {
	mockSvc := new(MockTaskService)
	mockSvc.On("AddMessage", mock.Anything, mock.Anything, mock.Anything, testTaskID, mock.Anything).
		Return((*models.TaskMessage)(nil), service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	body := `{"content":"x","message_type":"instruction"}`
	req := httptest.NewRequest(http.MethodPost, "/tasks/"+testTaskID.String()+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTaskRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
