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

func testUserCtx(t *testing.T) context.Context {
	t.Helper()
	uid := uuid.New()
	ctx := context.WithValue(context.Background(), CtxKeyUserID, uid)
	ctx = context.WithValue(ctx, CtxKeyUserRole, models.RoleUser)
	return ctx
}

func TestProjectList_Success(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	now := time.Now().UTC()
	svc.On("List", mock.Anything, uid, models.RoleUser, mock.MatchedBy(func(req dto.ListProjectsRequest) bool {
		return req.Limit == 20 && req.Offset == 0
	})).
		Return([]models.Project{
			{
				ID:               pid,
				Name:             "p1",
				GitProvider:      models.GitProviderLocal,
				GitDefaultBranch: "main",
				TechStack:        datatypes.JSON([]byte("{}")),
				Status:           models.ProjectStatusActive,
				Settings:         datatypes.JSON([]byte("{}")),
				UserID:           uid,
				CreatedAt:        now,
				UpdatedAt:        now,
			},
		}, int64(1), nil)

	result, structured, err := h(ctx, nil, &ProjectListParams{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, StatusOK, structured.(*Response).Status)
	data := structured.(*Response).Data.(dto.ProjectListResponse)
	assert.Len(t, data.Projects, 1)
	assert.Equal(t, int64(1), data.Total)
	svc.AssertExpectations(t)
}

func TestProjectList_NoAuth(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectListHandler(svc)

	result, _, err := h(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "List")
}

func TestProjectList_ServiceError(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectListHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("List", mock.Anything, uid, models.RoleUser, mock.MatchedBy(func(req dto.ListProjectsRequest) bool {
		return req.Limit == 20 && req.Offset == 0
	})).
		Return(nil, int64(0), assert.AnError)

	result, _, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestProjectGet_Success(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectGetHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	now := time.Now().UTC()
	proj := &models.Project{
		ID:               pid,
		Name:             "acme",
		GitProvider:      models.GitProviderLocal,
		GitDefaultBranch: "main",
		TechStack:        datatypes.JSON([]byte("{}")),
		Status:           models.ProjectStatusActive,
		Settings:         datatypes.JSON([]byte("{}")),
		UserID:           uid,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	svc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(proj, nil)

	result, structured, err := h(ctx, nil, &ProjectGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(dto.ProjectResponse)
	assert.Equal(t, pid.String(), data.ID)
	assert.Equal(t, "acme", data.Name)
	svc.AssertExpectations(t)
}

func TestProjectGet_MissingID(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectGetHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &ProjectGetParams{ProjectID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "GetByID")
}

func TestProjectGet_NotFound(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectGetHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)
	pid := uuid.New()
	svc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(nil, service.ErrProjectNotFound)

	result, _, err := h(ctx, nil, &ProjectGetParams{ProjectID: pid.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestProjectCreate_Success(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	pid := uuid.New()
	now := time.Now().UTC()
	proj := &models.Project{
		ID:               pid,
		Name:             "new-proj",
		GitProvider:      models.GitProviderLocal,
		GitDefaultBranch: "main",
		TechStack:        datatypes.JSON([]byte("{}")),
		Status:           models.ProjectStatusActive,
		Settings:         datatypes.JSON([]byte("{}")),
		UserID:           uid,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	svc.On("Create", mock.Anything, uid, mock.MatchedBy(func(r dto.CreateProjectRequest) bool {
		return r.Name == "new-proj"
	})).Return(proj, nil)

	result, structured, err := h(ctx, nil, &ProjectCreateParams{Name: "new-proj"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, StatusOK, structured.(*Response).Status)
	data := structured.(*Response).Data.(dto.ProjectResponse)
	assert.Equal(t, "new-proj", data.Name)
	assert.Equal(t, pid.String(), data.ID)
	svc.AssertExpectations(t)
}

func TestProjectCreate_MissingName(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &ProjectCreateParams{Name: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "Create")
}

func TestProjectCreate_NoAuth(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)

	result, structured, err := h(context.Background(), nil, &ProjectCreateParams{Name: "x"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, structured.(*Response).Details, "authentication")
	svc.AssertNotCalled(t, "Create")
}

func TestProjectCreate_DuplicateName(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("Create", mock.Anything, uid, mock.Anything).Return(nil, service.ErrProjectNameExists)

	result, _, err := h(ctx, nil, &ProjectCreateParams{Name: "dup"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestProjectCreate_WithOptionalFields(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	desc := "d"
	gp := "github"
	pid := uuid.New()
	now := time.Now().UTC()
	proj := &models.Project{
		ID:               pid,
		Name:             "p",
		Description:      desc,
		GitProvider:      models.GitProviderGitHub,
		GitDefaultBranch: "main",
		TechStack:        datatypes.JSON([]byte("{}")),
		Status:           models.ProjectStatusActive,
		Settings:         datatypes.JSON([]byte("{}")),
		UserID:           uid,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	svc.On("Create", mock.Anything, uid, mock.MatchedBy(func(r dto.CreateProjectRequest) bool {
		return r.Name == "p" && r.Description == desc && r.GitProvider == gp
	})).Return(proj, nil)

	result, _, err := h(ctx, nil, &ProjectCreateParams{Name: "p", Description: &desc, GitProvider: &gp})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestProjectCreate_ServiceError(t *testing.T) {
	svc := new(mockProjectService)
	h := makeProjectCreateHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("Create", mock.Anything, uid, mock.Anything).Return(nil, assert.AnError)

	result, _, err := h(ctx, nil, &ProjectCreateParams{Name: "n"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}
