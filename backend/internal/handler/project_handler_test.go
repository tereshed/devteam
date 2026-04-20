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
	"gorm.io/datatypes"
)

var testProjectUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// MockProjectService мок ProjectService для unit-тестов хендлера.
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
	var projects []models.Project
	if args.Get(0) != nil {
		projects = args.Get(0).([]models.Project)
	}
	total := int64(0)
	if args.Get(1) != nil {
		total = args.Get(1).(int64)
	}
	return projects, total, args.Error(2)
}

func (m *MockProjectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	args := m.Called(ctx, userID, userRole, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Project), args.Error(1)
}

func (m *MockProjectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}

func (m *MockProjectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	args := m.Called(ctx, userID, userRole, projectID)
	return args.Error(0)
}

func setupProjectRouter(mockSvc *MockProjectService, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProjectHandler(mockSvc)

	projects := r.Group("/projects")
	if withAuth {
		projects.Use(func(c *gin.Context) {
			c.Set("userID", testProjectUserID)
			c.Set("userRole", string(models.RoleUser))
			c.Next()
		})
	}
	projects.POST("", h.Create)
	projects.GET("", h.List)
	projects.GET("/:id", h.GetByID)
	projects.PUT("/:id", h.Update)
	projects.DELETE("/:id", h.Delete)
	return r
}

func sampleProject() *models.Project {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return &models.Project{
		ID:               id,
		Name:             "acme",
		Description:      "desc",
		GitProvider:      models.GitProviderLocal,
		GitURL:           "",
		GitDefaultBranch: "main",
		VectorCollection: "",
		TechStack:        datatypes.JSON([]byte("{}")),
		Status:           models.ProjectStatusActive,
		Settings:         datatypes.JSON([]byte("{}")),
		UserID:           testProjectUserID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestProject_Create_Success(t *testing.T) {
	mockSvc := new(MockProjectService)
	p := sampleProject()
	mockSvc.On("Create", mock.Anything, testProjectUserID, mock.AnythingOfType("dto.CreateProjectRequest")).
		Return(p, nil)

	w := httptest.NewRecorder()
	body := `{"name":"acme","description":"desc"}`
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.ProjectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, p.ID.String(), got.ID)
	assert.Equal(t, "acme", got.Name)
	mockSvc.AssertExpectations(t)
}

func TestProject_Create_InvalidJSON(t *testing.T) {
	mockSvc := new(MockProjectService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestProject_Create_MissingName(t *testing.T) {
	mockSvc := new(MockProjectService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestProject_Create_DuplicateName(t *testing.T) {
	mockSvc := new(MockProjectService)
	mockSvc.On("Create", mock.Anything, testProjectUserID, mock.AnythingOfType("dto.CreateProjectRequest")).
		Return(nil, service.ErrProjectNameExists)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"dup"}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Create_CredentialForbidden(t *testing.T) {
	mockSvc := new(MockProjectService)
	mockSvc.On("Create", mock.Anything, testProjectUserID, mock.AnythingOfType("dto.CreateProjectRequest")).
		Return(nil, service.ErrGitCredentialForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Create_NoAuth(t *testing.T) {
	mockSvc := new(MockProjectService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, false).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockSvc.AssertNotCalled(t, "Create")
}

func TestProject_List_Success(t *testing.T) {
	mockSvc := new(MockProjectService)
	p := sampleProject()
	mockSvc.On("List", mock.Anything, testProjectUserID, models.RoleUser, mock.AnythingOfType("dto.ListProjectsRequest")).
		Return([]models.Project{*p}, int64(1), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.ProjectListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Projects, 1)
	assert.Equal(t, int64(1), got.Total)
	assert.Equal(t, 20, got.Limit)
	assert.Equal(t, 0, got.Offset)
	mockSvc.AssertExpectations(t)
}

func TestProject_List_WithQueryParams(t *testing.T) {
	mockSvc := new(MockProjectService)
	status := "active"
	search := "acme"
	mockSvc.On("List", mock.Anything, testProjectUserID, models.RoleUser, mock.MatchedBy(func(r dto.ListProjectsRequest) bool {
		return r.Status != nil && *r.Status == status &&
			r.Search != nil && *r.Search == search &&
			r.Limit == 10
	})).Return([]models.Project{}, int64(0), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects?status=active&search=acme&limit=10", nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.ProjectListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, 10, got.Limit)
	mockSvc.AssertExpectations(t)
}

func TestProject_GetByID_Success(t *testing.T) {
	mockSvc := new(MockProjectService)
	p := sampleProject()
	mockSvc.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, p.ID).
		Return(p, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+p.ID.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.ProjectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, p.ID.String(), got.ID)
	mockSvc.AssertExpectations(t)
}

func TestProject_GetByID_InvalidUUID(t *testing.T) {
	mockSvc := new(MockProjectService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/not-uuid", nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "GetByID")
}

func TestProject_GetByID_NotFound(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, id).
		Return(nil, service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+id.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_GetByID_Forbidden(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, id).
		Return(nil, service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+id.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Update_Success(t *testing.T) {
	mockSvc := new(MockProjectService)
	p := sampleProject()
	p.Name = "newname"
	newName := "newname"
	mockSvc.On("Update", mock.Anything, testProjectUserID, models.RoleUser, p.ID, mock.MatchedBy(func(r dto.UpdateProjectRequest) bool {
		return r.Name != nil && *r.Name == newName
	})).Return(p, nil)

	body := `{"name":"newname"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/projects/"+p.ID.String(), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.ProjectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "newname", got.Name)
	mockSvc.AssertExpectations(t)
}

func TestProject_Update_InvalidJSON(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/projects/"+id.String(), bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockSvc.AssertNotCalled(t, "Update")
}

func TestProject_Update_NotFound(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("Update", mock.Anything, testProjectUserID, models.RoleUser, id, mock.AnythingOfType("dto.UpdateProjectRequest")).
		Return(nil, service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/projects/"+id.String(), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Update_Forbidden(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("Update", mock.Anything, testProjectUserID, models.RoleUser, id, mock.AnythingOfType("dto.UpdateProjectRequest")).
		Return(nil, service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/projects/"+id.String(), bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Delete_Success(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("Delete", mock.Anything, testProjectUserID, models.RoleUser, id).
		Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/projects/"+id.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "project deleted successfully", got["message"])
	mockSvc.AssertExpectations(t)
}

func TestProject_Delete_NotFound(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("Delete", mock.Anything, testProjectUserID, models.RoleUser, id).
		Return(service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/projects/"+id.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockSvc.AssertExpectations(t)
}

func TestProject_Delete_Forbidden(t *testing.T) {
	mockSvc := new(MockProjectService)
	id := uuid.New()
	mockSvc.On("Delete", mock.Anything, testProjectUserID, models.RoleUser, id).
		Return(service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/projects/"+id.String(), nil)
	setupProjectRouter(mockSvc, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockSvc.AssertExpectations(t)
}
