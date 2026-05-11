package mcp

import (
	"encoding/json"
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

func TestTeamGet_Success(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamGetHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	now := time.Now().UTC()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid, UserID: uid}, nil)

	team := &models.Team{
		ID:        uuid.New(),
		Name:      "T",
		ProjectID: pid,
		Type:      models.TeamTypeDevelopment,
		CreatedAt: now,
		UpdatedAt: now,
	}
	tid := uuid.MustParse("99999999-9999-4999-8999-999999999999")
	aid := uuid.New()
	agent := models.Agent{
		ID:       aid,
		Name:     "Agent1",
		Role:     models.AgentRoleDeveloper,
		Skills:   datatypes.JSON([]byte("[]")),
		Settings: datatypes.JSON([]byte("{}")),
		IsActive: true,
		ToolBindings: []models.AgentToolBinding{
			{
				AgentID:          aid,
				ToolDefinitionID: tid,
				ToolDefinition: &models.ToolDefinition{
					ID:       tid,
					Name:     "vector_search",
					Category: "search",
				},
			},
		},
	}
	team.Agents = []models.Agent{agent}
	teamSvc.On("GetByProjectID", mock.Anything, pid).Return(team, nil)

	result, structured, err := h(ctx, nil, &TeamGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TeamResponse)
	assert.Equal(t, team.Name, data.Name)
	require.Len(t, data.Agents, 1)
	require.Len(t, data.Agents[0].ToolBindings, 1)
	assert.Equal(t, tid.String(), data.Agents[0].ToolBindings[0].ToolDefinitionID)
	assert.Equal(t, "vector_search", data.Agents[0].ToolBindings[0].Name)
	assert.Equal(t, "search", data.Agents[0].ToolBindings[0].Category)
	projectSvc.AssertExpectations(t)
	teamSvc.AssertExpectations(t)
}

func TestTeamGet_ProjectForbidden(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamGetHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).
		Return(nil, service.ErrProjectForbidden)

	result, _, err := h(ctx, nil, &TeamGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	teamSvc.AssertNotCalled(t, "GetByProjectID")
}

func TestTeamGet_TeamNotFound(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamGetHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid, UserID: uid}, nil)
	teamSvc.On("GetByProjectID", mock.Anything, pid).Return(nil, service.ErrTeamNotFound)

	result, _, err := h(ctx, nil, &TeamGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestTeamUpdate_Success(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamUpdateHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	now := time.Now().UTC()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid, UserID: uid}, nil)

	n := "New"
	updated := &models.Team{
		ID:        uuid.New(),
		Name:      n,
		ProjectID: pid,
		Type:      models.TeamTypeDevelopment,
		CreatedAt: now,
		UpdatedAt: now,
	}
	teamSvc.On("Update", mock.Anything, pid, mock.MatchedBy(func(r dto.UpdateTeamRequest) bool {
		return r.Name != nil && *r.Name == n
	})).Return(updated, nil)

	result, structured, err := h(ctx, nil, &TeamUpdateParams{ProjectID: pid.String(), Name: &n})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TeamResponse)
	assert.Equal(t, n, data.Name)
}

func TestTeamUpdate_InvalidName(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamUpdateHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid, UserID: uid}, nil)
	empty := "   "
	teamSvc.On("Update", mock.Anything, pid, mock.Anything).Return(nil, service.ErrTeamInvalidName)

	result, _, err := h(ctx, nil, &TeamUpdateParams{ProjectID: pid.String(), Name: &empty})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestTeamAgentPatch_Success(t *testing.T) {
	projectSvc := new(mockProjectService)
	teamSvc := new(mockTeamService)
	h := makeTeamAgentPatchHandler(projectSvc, teamSvc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	aid := uuid.New()
	now := time.Now().UTC()
	projectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid, UserID: uid}, nil)

	updated := &models.Team{
		ID:        uuid.New(),
		Name:      "T",
		ProjectID: pid,
		Type:      models.TeamTypeDevelopment,
		CreatedAt: now,
		UpdatedAt: now,
	}
	teamSvc.On("PatchAgent", mock.Anything, pid, aid, mock.AnythingOfType("dto.PatchAgentRequest")).Return(updated, nil)

	active := true
	result, structured, err := h(ctx, nil, &TeamAgentPatchParams{
		ProjectID: pid.String(),
		AgentID:   aid.String(),
		IsActive:  &active,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TeamResponse)
	assert.Equal(t, updated.Name, data.Name)
}

func TestTeamAgentPatchWireJSON_RejectEmptyPromptID(t *testing.T) {
	empty := ""
	_, err := teamAgentPatchWireJSON(&TeamAgentPatchParams{
		ProjectID: "p",
		AgentID:   "a",
		PromptID:  &empty,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt_id")
}

func TestTeamAgentPatchWireJSON_RejectWhitespaceOnlyModel(t *testing.T) {
	spaces := "   "
	_, err := teamAgentPatchWireJSON(&TeamAgentPatchParams{
		ProjectID: "p",
		AgentID:   "a",
		Model:     &spaces,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestTeamAgentPatchWireJSON_ToolBindings(t *testing.T) {
	ids := []string{
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
	}
	raw, err := teamAgentPatchWireJSON(&TeamAgentPatchParams{
		ProjectID:         "p",
		AgentID:           "a",
		ToolDefinitionIDs: &ids,
	})
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	tb, ok := m["tool_bindings"].([]any)
	require.True(t, ok)
	require.Len(t, tb, 2)
}

func TestTeamAgentPatchWireJSON_InvalidToolDefinitionIDUUID(t *testing.T) {
	bad := "not-a-uuid"
	_, err := teamAgentPatchWireJSON(&TeamAgentPatchParams{
		ProjectID:         "p",
		AgentID:           "a",
		ToolDefinitionIDs: &[]string{bad},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool_definition_id")
}
