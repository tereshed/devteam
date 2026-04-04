package mcp

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
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
	teamSvc.On("GetByProjectID", mock.Anything, pid).Return(team, nil)

	result, structured, err := h(ctx, nil, &TeamGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.TeamResponse)
	assert.Equal(t, team.Name, data.Name)
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
