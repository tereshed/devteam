package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
)

func TestAuthorizedExecutor_Catalog(t *testing.T) {
	mockGitSvc := new(mockGitIntegrationService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		GitIntegrationService: mockGitSvc,
	})

	catalog := executor.Catalog()
	
	// Verify git integration tools are present
	var listIntegrationsTool *agentloop.Tool
	var listReposTool *agentloop.Tool
	var createRepoTool *agentloop.Tool

	for i := range catalog {
		switch catalog[i].Name {
		case "list_git_integrations":
			listIntegrationsTool = &catalog[i]
		case "list_git_repositories":
			listReposTool = &catalog[i]
		case "create_git_repository":
			createRepoTool = &catalog[i]
		}
	}

	require.NotNil(t, listIntegrationsTool, "list_git_integrations tool should be present")
	assert.False(t, listIntegrationsTool.RequiresConfirmation)

	require.NotNil(t, listReposTool, "list_git_repositories tool should be present")
	assert.False(t, listReposTool.RequiresConfirmation)

	require.NotNil(t, createRepoTool, "create_git_repository tool should be present")
	assert.True(t, createRepoTool.RequiresConfirmation)
}

func TestAuthorizedExecutor_ListGitIntegrations(t *testing.T) {
	mockGitSvc := new(mockGitIntegrationService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		GitIntegrationService: mockGitSvc,
	})

	uid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String()}
	now := time.Now().UTC()

	t.Run("Success", func(t *testing.T) {
		mockGitSvc.On("ListStatuses", mock.Anything, uid).Return([]service.GitIntegrationStatus{
			{
				Provider:    models.GitIntegrationProviderGitHub,
				Connected:   true,
				ConnectedAt: &now,
			},
		}, nil).Once()

		res, err := executor.listGitIntegrations(context.Background(), auth, nil)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   struct {
				Integrations []service.GitIntegrationStatus `json:"integrations"`
			} `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		require.Len(t, response.Data.Integrations, 1)
		assert.Equal(t, models.GitIntegrationProviderGitHub, response.Data.Integrations[0].Provider)
		assert.True(t, response.Data.Integrations[0].Connected)
		mockGitSvc.AssertExpectations(t)
	})

	t.Run("Service Error", func(t *testing.T) {
		mockGitSvc.On("ListStatuses", mock.Anything, uid).Return(nil, errors.New("some DB error")).Once()

		res, err := executor.listGitIntegrations(context.Background(), auth, nil)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "error", response.Status)
		assert.Equal(t, "внутренняя ошибка при выполнении инструмента", response.Message)
		mockGitSvc.AssertExpectations(t)
	})
}

func TestAuthorizedExecutor_ListGitRepositories(t *testing.T) {
	mockGitSvc := new(mockGitIntegrationService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		GitIntegrationService: mockGitSvc,
	})

	uid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String()}

	t.Run("Success", func(t *testing.T) {
		mockGitSvc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitHub).Return([]service.GitRepository{
			{
				Name:        "test-repo",
				FullName:    "user/test-repo",
				HTMLURL:     "https://github.com/user/test-repo",
				CloneURL:    "https://github.com/user/test-repo.git",
				Description: "My test repo",
			},
		}, nil).Once()

		args := json.RawMessage(`{"provider":"github"}`)
		res, err := executor.listGitRepositories(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   struct {
				Repositories []service.GitRepository `json:"repositories"`
			} `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		require.Len(t, response.Data.Repositories, 1)
		assert.Equal(t, "test-repo", response.Data.Repositories[0].Name)
		mockGitSvc.AssertExpectations(t)
	})

	t.Run("Missing Provider", func(t *testing.T) {
		args := json.RawMessage(`{}`)
		res, err := executor.listGitRepositories(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Equal(t, "provider is required", response.Message)
	})

	t.Run("Invalid Provider", func(t *testing.T) {
		args := json.RawMessage(`{"provider":"invalid"}`)
		res, err := executor.listGitRepositories(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Equal(t, "invalid provider, must be 'github' or 'gitlab'", response.Message)
	})

	t.Run("Not Found / Not Connected", func(t *testing.T) {
		mockGitSvc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitHub).Return(nil, repository.ErrGitIntegrationNotFound).Once()

		args := json.RawMessage(`{"provider":"github"}`)
		res, err := executor.listGitRepositories(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Equal(t, "git-интеграция не найдена (сначала подключите её в настройках)", response.Message)
		mockGitSvc.AssertExpectations(t)
	})
}

func TestAuthorizedExecutor_CreateGitRepository(t *testing.T) {
	mockGitSvc := new(mockGitIntegrationService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		GitIntegrationService: mockGitSvc,
	})

	uid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String()}

	t.Run("Success", func(t *testing.T) {
		mockGitSvc.On("CreateRepository", mock.Anything, uid, models.GitIntegrationProviderGitHub, "new-repo", true, "hello").Return(&service.GitRepository{
			Name:     "new-repo",
			FullName: "user/new-repo",
			HTMLURL:  "https://github.com/user/new-repo",
		}, nil).Once()

		args := json.RawMessage(`{"provider":"github","name":"new-repo","private":true,"description":"hello"}`)
		res, err := executor.createGitRepository(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   struct {
				Repository service.GitRepository `json:"repository"`
			} `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "new-repo", response.Data.Repository.Name)
		mockGitSvc.AssertExpectations(t)
	})

	t.Run("Validation Error (Missing fields)", func(t *testing.T) {
		args := json.RawMessage(`{"provider":"github"}`)
		res, err := executor.createGitRepository(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Equal(t, "provider and name are required", response.Message)
	})
}

func TestAuthorizedExecutor_TeamAgentCatalog(t *testing.T) {
	mockTeamSvc := new(mockTeamService)
	dummyAgentSvc := &service.AgentService{}
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		TeamService:  mockTeamSvc,
		AgentService: dummyAgentSvc,
	})

	catalog := executor.Catalog()
	var createAgentTool *agentloop.Tool
	var deleteAgentTool *agentloop.Tool

	for i := range catalog {
		switch catalog[i].Name {
		case "team_agent_create":
			createAgentTool = &catalog[i]
		case "team_agent_delete":
			deleteAgentTool = &catalog[i]
		}
	}

	require.NotNil(t, createAgentTool, "team_agent_create tool should be present")
	assert.True(t, createAgentTool.RequiresConfirmation)

	require.NotNil(t, deleteAgentTool, "team_agent_delete tool should be present")
	assert.True(t, deleteAgentTool.RequiresConfirmation)
}

func TestAuthorizedExecutor_TeamAgentCreate(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	mockTeamSvc := new(mockTeamService)
	mockAgentRepo := new(mockAgentRepository)
	mockSecretRepo := new(mockAgentSecretRepository)

	agentSvc := service.NewAgentService(
		mockAgentRepo,
		mockSecretRepo,
		nil,
		&mockTransactionManager{},
	)

	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
		TeamService:    mockTeamSvc,
		AgentService:   agentSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	tid := uuid.New()
	promptID := uuid.New()
	auth := agentloop.AuthContext{
		UserID:    uid.String(),
		ProjectID: pid.String(),
	}

	// 1. Mock ProjectService.GetByID to check access
	mockProjectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{
		ID: pid,
	}, nil).Once()

	// 2. Mock TeamService.ListByProjectID to get team ID
	mockTeamSvc.On("ListByProjectID", mock.Anything, pid).Return([]models.Team{
		{
			ID:        tid,
			ProjectID: pid,
		},
	}, nil).Twice()

	// 3. Mock AgentRepository.Create
	mockAgentRepo.On("Create", mock.Anything, mock.MatchedBy(func(agent *models.Agent) bool {
		return agent.Name == "new-agent" &&
			agent.Role == models.AgentRoleDeveloper &&
			agent.ExecutionKind == models.AgentExecutionKindSandbox &&
			agent.TeamID != nil && *agent.TeamID == tid &&
			agent.UserID == nil &&
			agent.PromptID != nil && *agent.PromptID == promptID
	})).Return(nil).Once()

	args := json.RawMessage(`{
		"project_id": "` + pid.String() + `",
		"name": "new-agent",
		"role": "developer",
		"execution_kind": "sandbox",
		"code_backend": "claude-code",
		"prompt_id": "` + promptID.String() + `"
	}`)

	res, err := executor.teamAgentCreate(context.Background(), auth, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)

	mockProjectSvc.AssertExpectations(t)
	mockTeamSvc.AssertExpectations(t)
	mockAgentRepo.AssertExpectations(t)
}

func TestAuthorizedExecutor_TeamAgentDelete(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	mockTeamSvc := new(mockTeamService)
	mockAgentRepo := new(mockAgentRepository)
	mockSecretRepo := new(mockAgentSecretRepository)

	agentSvc := service.NewAgentService(
		mockAgentRepo,
		mockSecretRepo,
		nil,
		&mockTransactionManager{},
	)

	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
		TeamService:    mockTeamSvc,
		AgentService:   agentSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	tid := uuid.New()
	aid := uuid.New()
	auth := agentloop.AuthContext{
		UserID:    uid.String(),
		ProjectID: pid.String(),
	}

	// 1. Mock ProjectService.GetByID to check access
	mockProjectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{
		ID: pid,
	}, nil).Once()

	// 2. Mock AgentRepository.GetByID to verify agent ownership
	mockAgentRepo.On("GetByID", mock.Anything, aid).Return(&models.Agent{
		ID:     aid,
		TeamID: &tid,
	}, nil).Once()

	// 3. Mock TeamService.ListByProjectID to get team ID
	mockTeamSvc.On("ListByProjectID", mock.Anything, pid).Return([]models.Team{
		{
			ID:        tid,
			ProjectID: pid,
		},
	}, nil).Twice()

	// 4. Mock AgentSecretRepository.DeleteByAgentID
	mockSecretRepo.On("DeleteByAgentID", mock.Anything, aid).Return(nil).Once()

	// 5. Mock AgentRepository.Delete
	mockAgentRepo.On("Delete", mock.Anything, aid).Return(nil).Once()

	args := json.RawMessage(`{
		"project_id": "` + pid.String() + `",
		"agent_id": "` + aid.String() + `"
	}`)

	res, err := executor.teamAgentDelete(context.Background(), auth, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)

	mockProjectSvc.AssertExpectations(t)
	mockTeamSvc.AssertExpectations(t)
	mockAgentRepo.AssertExpectations(t)
	mockSecretRepo.AssertExpectations(t)
}

func TestAuthorizedExecutor_TeamList(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
		TeamService:    mockTeamSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}

	mockProjectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid}, nil).Once()
	mockTeamSvc.On("ListByProjectID", mock.Anything, pid).Return([]models.Team{{ID: uuid.New(), Name: "Team A"}}, nil).Once()

	res, err := executor.teamList(context.Background(), auth, nil)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestAuthorizedExecutor_TeamCreate(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
		TeamService:    mockTeamSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}

	mockProjectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid}, nil).Once()
	mockTeamSvc.On("Create", mock.Anything, pid, dto.CreateTeamRequest{Name: "New Team", Type: "research"}).Return(&models.Team{ID: uuid.New(), Name: "New Team", Type: "research"}, nil).Once()

	args := json.RawMessage(`{"project_id":"` + pid.String() + `","name":"New Team","type":"research"}`)
	res, err := executor.teamCreate(context.Background(), auth, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestAuthorizedExecutor_TeamDelete(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
		TeamService:    mockTeamSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	tid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}

	mockProjectSvc.On("GetByID", mock.Anything, uid, models.RoleUser, pid).Return(&models.Project{ID: pid}, nil).Once()
	mockTeamSvc.On("Delete", mock.Anything, pid, tid).Return(nil).Once()

	args := json.RawMessage(`{"project_id":"` + pid.String() + `","team_id":"` + tid.String() + `"}`)
	res, err := executor.teamDelete(context.Background(), auth, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestAuthorizedExecutor_TeamTypeList(t *testing.T) {
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		TeamService: mockTeamSvc,
	})

	mockTeamSvc.On("ListTeamTypes", mock.Anything).Return([]models.TeamTypeModel{{Code: "research", Name: "Research"}}, nil).Once()

	res, err := executor.teamTypeList(context.Background(), agentloop.AuthContext{}, nil)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestAuthorizedExecutor_TeamTypeCreate(t *testing.T) {
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		TeamService: mockTeamSvc,
	})

	mockTeamSvc.On("CreateTeamType", mock.Anything, dto.CreateTeamTypeRequest{Code: "custom", Name: "Custom"}).Return(&models.TeamTypeModel{Code: "custom", Name: "Custom"}, nil).Once()

	args := json.RawMessage(`{"code":"custom","name":"Custom"}`)
	res, err := executor.teamTypeCreate(context.Background(), agentloop.AuthContext{}, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

func TestAuthorizedExecutor_TeamTypeDelete(t *testing.T) {
	mockTeamSvc := new(mockTeamService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		TeamService: mockTeamSvc,
	})

	mockTeamSvc.On("DeleteTeamType", mock.Anything, "custom").Return(nil).Once()

	args := json.RawMessage(`{"code":"custom"}`)
	res, err := executor.teamTypeDelete(context.Background(), agentloop.AuthContext{}, args)
	require.NoError(t, err)

	var response struct {
		Status string `json:"status"`
	}
	err = json.Unmarshal(res, &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response.Status)
}

