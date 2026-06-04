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

func (m *MockTeamService) PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, agentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) CreateAgent(ctx context.Context, projectID uuid.UUID, teamID uuid.UUID, req dto.CreateTeamAgentRequest) (*models.Agent, error) {
	args := m.Called(ctx, projectID, teamID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockTeamService) DeleteAgent(ctx context.Context, projectID, agentID uuid.UUID) error {
	args := m.Called(ctx, projectID, agentID)
	return args.Error(0)
}

func (m *MockTeamService) GetAgentSettings(ctx context.Context, actor service.AgentSettingsActor, agentID uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, actor, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockTeamService) UpdateAgentSettings(ctx context.Context, actor service.AgentSettingsActor, agentID uuid.UUID, req dto.UpdateAgentSettingsRequest) (*models.Agent, error) {
	args := m.Called(ctx, actor, agentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockTeamService) ListByProjectID(ctx context.Context, projectID uuid.UUID) ([]models.Team, error) {
	args := m.Called(ctx, projectID)
	var teams []models.Team
	if v := args.Get(0); v != nil {
		teams = v.([]models.Team)
	}
	return teams, args.Error(1)
}

func (m *MockTeamService) Create(ctx context.Context, projectID uuid.UUID, req dto.CreateTeamRequest) (*models.Team, error) {
	args := m.Called(ctx, projectID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Team), args.Error(1)
}

func (m *MockTeamService) Delete(ctx context.Context, projectID, teamID uuid.UUID) error {
	args := m.Called(ctx, projectID, teamID)
	return args.Error(0)
}

func (m *MockTeamService) ListTeamTypes(ctx context.Context) ([]models.TeamTypeModel, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.TeamTypeModel), args.Error(1)
}

func (m *MockTeamService) CreateTeamType(ctx context.Context, req dto.CreateTeamTypeRequest) (*models.TeamTypeModel, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TeamTypeModel), args.Error(1)
}

func (m *MockTeamService) DeleteTeamType(ctx context.Context, code string) error {
	args := m.Called(ctx, code)
	return args.Error(0)
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
	g.PATCH("/:id/team/agents/:agentId", h.PatchAgent)
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
		tid := team.ID
		team.Agents = append(team.Agents, models.Agent{
			ID:          agentIDs[i],
			Name:        name,
			Role:        models.AgentRoleDeveloper,
			TeamID:      &tid,
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

func TestTeam_PatchAgent_InvalidAgentUUID(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/projects/"+pid.String()+"/team/agents/bad", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	teamMock.AssertNotCalled(t, "PatchAgent")
}

func TestTeam_PatchAgent_Success(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	aid := uuid.MustParse("44444444-4444-4444-4444-444444444441")
	team := sampleTeamWithAgents(pid, 1)

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("PatchAgent", mock.Anything, pid, aid, mock.AnythingOfType("dto.PatchAgentRequest")).Return(team, nil)

	w := httptest.NewRecorder()
	body := `{"is_active":false}`
	req := httptest.NewRequest(http.MethodPatch, "/projects/"+pid.String()+"/team/agents/"+aid.String(), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got dto.TeamResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, team.Name, got.Name)
	teamMock.AssertExpectations(t)
}

func TestTeam_PatchAgent_Conflict(t *testing.T) {
	teamMock := new(MockTeamService)
	projectMock := new(MockProjectService)
	pid := sampleProject().ID
	aid := uuid.MustParse("44444444-4444-4444-4444-444444444441")

	projectMock.On("GetByID", mock.Anything, testProjectUserID, models.RoleUser, pid).Return(sampleProject(), nil)
	teamMock.On("PatchAgent", mock.Anything, pid, aid, mock.Anything).Return(nil, service.ErrTeamAgentConflict)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/projects/"+pid.String()+"/team/agents/"+aid.String(), bytes.NewBufferString(`{"model":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	setupTeamRouter(teamMock, projectMock, true).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTeam_ListTeamTypes(t *testing.T) {
	teamMock := new(MockTeamService)
	teamTypes := []models.TeamTypeModel{
		{Code: "dev", Name: "Development", IsSystem: true},
		{Code: "research", Name: "Research", IsSystem: false},
	}
	teamMock.On("ListTeamTypes", mock.Anything).Return(teamTypes, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", testProjectUserID)
		c.Set("userRole", string(models.RoleUser))
		c.Next()
	})
	h := NewTeamHandler(teamMock, nil)
	r.GET("/team-types", h.ListTeamTypes)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/team-types", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got []dto.TeamTypeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 2)
	assert.Equal(t, "dev", got[0].Code)
	assert.Equal(t, "Development", got[0].Name)
	assert.True(t, got[0].IsSystem)
	teamMock.AssertExpectations(t)
}

func TestTeam_CreateTeamType(t *testing.T) {
	teamMock := new(MockTeamService)
	reqDto := dto.CreateTeamTypeRequest{Code: "marketing", Name: "Marketing"}
	resModel := models.TeamTypeModel{Code: "marketing", Name: "Marketing", IsSystem: false}
	teamMock.On("CreateTeamType", mock.Anything, reqDto).Return(&resModel, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", testProjectUserID)
		c.Set("userRole", string(models.RoleAdmin))
		c.Next()
	})
	h := NewTeamHandler(teamMock, nil)
	r.POST("/admin/team-types", h.CreateTeamType)

	w := httptest.NewRecorder()
	body, _ := json.Marshal(reqDto)
	req := httptest.NewRequest(http.MethodPost, "/admin/team-types", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var got dto.TeamTypeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "marketing", got.Code)
	assert.Equal(t, "Marketing", got.Name)
	assert.False(t, got.IsSystem)
	teamMock.AssertExpectations(t)
}

func TestTeam_DeleteTeamType(t *testing.T) {
	teamMock := new(MockTeamService)
	teamMock.On("DeleteTeamType", mock.Anything, "marketing").Return(nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", testProjectUserID)
		c.Set("userRole", string(models.RoleAdmin))
		c.Next()
	})
	h := NewTeamHandler(teamMock, nil)
	r.DELETE("/admin/team-types/:code", h.DeleteTeamType)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/team-types/marketing", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	teamMock.AssertExpectations(t)
}
