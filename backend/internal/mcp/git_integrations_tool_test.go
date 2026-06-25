package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockGitIntegrationService struct {
	mock.Mock
}

func (m *mockGitIntegrationService) InitGitHub(ctx context.Context, userID uuid.UUID, redirectURI string) (*service.GitIntegrationInitResult, error) {
	args := m.Called(ctx, userID, redirectURI)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitIntegrationInitResult), args.Error(1)
}

func (m *mockGitIntegrationService) InitGitLabShared(ctx context.Context, userID uuid.UUID, redirectURI string) (*service.GitIntegrationInitResult, error) {
	args := m.Called(ctx, userID, redirectURI)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitIntegrationInitResult), args.Error(1)
}

func (m *mockGitIntegrationService) InitGitLabBYO(ctx context.Context, userID uuid.UUID, redirectURI string, byo service.BYOGitLabInit) (*service.GitIntegrationInitResult, error) {
	args := m.Called(ctx, userID, redirectURI, byo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitIntegrationInitResult), args.Error(1)
}

func (m *mockGitIntegrationService) HandleCallback(ctx context.Context, code, state, providerError string) (*service.GitIntegrationCallbackResult, error) {
	args := m.Called(ctx, code, state, providerError)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitIntegrationCallbackResult), args.Error(1)
}

func (m *mockGitIntegrationService) Revoke(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (bool, error) {
	args := m.Called(ctx, userID, provider)
	return args.Bool(0), args.Error(1)
}

func (m *mockGitIntegrationService) RevokeByID(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockGitIntegrationService) Status(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (*service.GitIntegrationStatus, error) {
	args := m.Called(ctx, userID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitIntegrationStatus), args.Error(1)
}

func (m *mockGitIntegrationService) ListStatuses(ctx context.Context, userID uuid.UUID) ([]service.GitIntegrationStatus, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]service.GitIntegrationStatus), args.Error(1)
}

func (m *mockGitIntegrationService) ListRepositories(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider, accountID uuid.UUID) ([]service.GitRepository, error) {
	args := m.Called(ctx, userID, provider, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]service.GitRepository), args.Error(1)
}

func (m *mockGitIntegrationService) CreateRepository(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider, accountID uuid.UUID, name string, private bool, description string) (*service.GitRepository, error) {
	args := m.Called(ctx, userID, provider, accountID, name, private, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*service.GitRepository), args.Error(1)
}

func TestListGitIntegrations_Success(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeListGitIntegrationsHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	now := time.Now().UTC()
	svc.On("ListStatuses", mock.Anything, uid).Return([]service.GitIntegrationStatus{
		{
			Provider:    models.GitIntegrationProviderGitHub,
			Connected:   true,
			ConnectedAt: &now,
		},
	}, nil)

	result, structured, err := h(ctx, nil, &ListGitIntegrationsParams{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(map[string]any)
	assert.Contains(t, data, "integrations")
	svc.AssertExpectations(t)
}

func TestListGitRepositories_Success(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeListGitRepositoriesHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	svc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitHub, uuid.Nil).Return([]service.GitRepository{
		{
			Name:        "repo1",
			FullName:    "owner/repo1",
			HTMLURL:     "https://github.com/owner/repo1",
			CloneURL:    "https://github.com/owner/repo1.git",
			Description: "desc1",
		},
	}, nil)

	result, structured, err := h(ctx, nil, &ListGitRepositoriesParams{Provider: "github"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(map[string]any)
	assert.Contains(t, data, "repositories")
	svc.AssertExpectations(t)
}

func TestListGitRepositories_PassesAccountID(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeListGitRepositoriesHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	acc := uuid.New()
	svc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitLab, acc).
		Return([]service.GitRepository{}, nil)

	result, _, err := h(ctx, nil, &ListGitRepositoriesParams{Provider: "gitlab", AccountID: acc.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestListGitRepositories_BadAccountID(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeListGitRepositoriesHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &ListGitRepositoriesParams{Provider: "gitlab", AccountID: "not-a-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "ListRepositories")
}

func TestListGitRepositories_InvalidProvider(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeListGitRepositoriesHandler(svc)
	ctx := testUserCtx(t)

	result, _, err := h(ctx, nil, &ListGitRepositoriesParams{Provider: "invalid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "ListRepositories")
}

func TestCreateGitRepository_Success(t *testing.T) {
	svc := new(mockGitIntegrationService)
	h := makeCreateGitRepositoryHandler(svc)
	ctx := testUserCtx(t)
	uid, _ := UserIDFromContext(ctx)

	repo := &service.GitRepository{
		Name:        "new-repo",
		FullName:    "owner/new-repo",
		HTMLURL:     "https://github.com/owner/new-repo",
		CloneURL:    "https://github.com/owner/new-repo.git",
		Description: "some desc",
	}

	svc.On("CreateRepository", mock.Anything, uid, models.GitIntegrationProviderGitHub, uuid.Nil, "new-repo", true, "some desc").Return(repo, nil)

	result, structured, err := h(ctx, nil, &CreateGitRepositoryParams{
		Provider:    "github",
		Name:        "new-repo",
		Private:     true,
		Description: "some desc",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.(map[string]any)
	assert.Contains(t, data, "repository")
	svc.AssertExpectations(t)
}
