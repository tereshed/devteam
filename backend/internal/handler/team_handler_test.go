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
)

// MockTeamService мок TeamService для unit-тестов.
type MockTeamService struct {
	mock.Mock
}

func (m *MockTeamService) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func setupTeamRouter(teamMock *MockTeamService, projectMock *MockProjectService, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTeamHandler(teamMock, projectMock)
	g := r.Group("/projects")
	if withAuth {
		g.Use(func(c *gin.Context) {
			c.Set("userID", testProjectUserID)
			c.Set("userRole", string(models.RoleUser))
			c.Next()
		})
	}
	g.GET("/:id/team", h.GetByProjectID)
	g.PUT("/:id/team", h.Update)
	return r
}

func sampleTeamWithAgents(projectID uuid.UUID, nAgents int) *models.Team {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	team := &models.Team{
		ID:        tid,
		Name:      "Dev Team",
		ProjectID: projectID,
		Type:      models.TeamTypeDevelopment,
		CreatedAt: now,
		UpdatedAt: now,
	}
	agentIDs := []uuid.UUID{
		uuid.MustParse("44444444-4444-4444-4444-444444444441"),
		uuid.MustParse("44444444-4444-4444-4444-444444444442"),
		uuid.MustParse("44444444-4444-4444-4444-444444444443"),
	}
	for i := 0; i < nAgents && i < len(agentIDs); i++ {
		name := "agent"
		cb := models.CodeBackendClaudeCode
		team.Agents = append(team.Agents, models.Agent{
			ID:          agentIDs[i],
			Name:        name,
			Role:        models.AgentRoleDeveloper,
			Model:       &name,
			CodeBackend: &cb,
			IsActive:    true,
			Prompt:      &models.Prompt{Name: "sys-prompt"},
		})
	}
	return team
}

func TestTeam_GetByProjectID_Success(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	team := sampleTeamWithAgents(pid, 0)

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("GetByProjectID", mock.Anything, pid).Return(team, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TeamResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, team.ID.String(), got.ID)
	assert.Equal(t, team.Name, got.Name)
	assert.Equal(t, pid.String(), got.ProjectID)
	projectMock.AssertExpectations(t)
	teamMock.AssertExpectations(t)
}

func TestTeam_GetByProjectID_InvalidUUID(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/not-a-uuid/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	projectMock.AssertNotCalled(t, "GetByID")
	teamMock.AssertNotCalled(t, "GetByProjectID")
}

func TestTeam_GetByProjectID_ProjectNotFound(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).
		Return(nil, service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	teamMock.AssertNotCalled(t, "GetByProjectID")
}

func TestTeam_GetByProjectID_ProjectForbidden(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).
		Return(nil, service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	teamMock.AssertNotCalled(t, "GetByProjectID")
}

func TestTeam_GetByProjectID_TeamNotFound(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("GetByProjectID", mock.Anything, pid).Return(nil, service.ErrTeamNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTeam_GetByProjectID_WithAgents(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	team := sampleTeamWithAgents(pid, 3)

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("GetByProjectID", mock.Anything, pid).Return(team, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TeamResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got.Agents, 3)
}

func TestTeam_GetByProjectID_NoAuth(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/"+pid.String()+"/team", nil)
	setupTeamRouter(teamMock, projectMock, false).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	projectMock.AssertNotCalled(t, "GetByID")
}

func TestTeam_Update_Success(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	updated := sampleTeamWithAgents(pid, 0)
	updated.Name = "Renamed"

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("Update", mock.Anything, pid, mock.AnythingOfType("dto.UpdateTeamRequest")).Return(updated, nil)

	w := httptest.NewRecorder()
	body := `{"name":"Renamed"}`
	req := httptest.NewRequest(http.MethodPut, "/projects/"+pid.String()+"/team", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TeamResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Renamed", got.Name)
}

func TestTeam_Update_InvalidJSON(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/projects/"+pid.String()+"/team", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	teamMock.AssertNotCalled(t, "Update")
}

func TestTeam_Update_ProjectNotFound(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).
		Return(nil, service.ErrProjectNotFound)

	w := httptest.NewRecorder()
	body := `{"name":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/projects/"+pid.String()+"/team", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	teamMock.AssertNotCalled(t, "Update")
}

func TestTeam_Update_ProjectForbidden(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).
		Return(nil, service.ErrProjectForbidden)

	w := httptest.NewRecorder()
	body := `{"name":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/projects/"+pid.String()+"/team", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	teamMock.AssertNotCalled(t, "Update")
}

func TestTeam_Update_TeamNotFound(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("Update", mock.Anything, pid, mock.AnythingOfType("dto.UpdateTeamRequest")).
		Return(nil, service.ErrTeamNotFound)

	w := httptest.NewRecorder()
	body := `{"name":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/projects/"+pid.String()+"/team", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
