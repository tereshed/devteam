package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	"github.com/devteam/backend/internal/indexer"
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
		mockGitSvc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitHub, uuid.Nil).Return([]service.GitRepository{
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
		mockGitSvc.On("ListRepositories", mock.Anything, uid, models.GitIntegrationProviderGitHub, uuid.Nil).Return(nil, repository.ErrGitIntegrationNotFound).Once()

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
		mockGitSvc.On("CreateRepository", mock.Anything, uid, models.GitIntegrationProviderGitHub, uuid.Nil, "new-repo", true, "hello").Return(&service.GitRepository{
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

func TestAuthorizedExecutor_TaskCreateAndUpdate(t *testing.T) {
	mockTaskSvc := new(mockTaskService)
	mockOrchSvc := new(mockTaskOrchestrator)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		TaskService:         mockTaskSvc,
		OrchestratorService: mockOrchSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	tid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}

	t.Run("Create Task Success", func(t *testing.T) {
		taskDesc := "Test task description"
		taskPriority := "medium"
		mockTask := &models.Task{
			ID:          tid,
			ProjectID:   pid,
			Title:       "Test Task",
			Description: taskDesc,
			State:       models.TaskStateActive,
		}

		mockTaskSvc.On("Create", mock.Anything, uid, models.RoleUser, pid, dto.CreateTaskRequest{
			Title:       "Test Task",
			Description: taskDesc,
			Priority:    taskPriority,
		}).Return(mockTask, nil).Once()

		mockOrchSvc.On("EnqueueInitialStep", mock.Anything, tid).Return(nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","title":"Test Task","description":"Test task description","priority":"medium"}`)
		res, err := executor.taskCreate(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   models.Task `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, tid, response.Data.ID)

		// Wait briefly for background goroutine to execute
		time.Sleep(50 * time.Millisecond)

		mockTaskSvc.AssertExpectations(t)
		mockOrchSvc.AssertExpectations(t)
	})

	t.Run("Update Task to Active triggers orchestration", func(t *testing.T) {
		mockTask := &models.Task{
			ID:        tid,
			ProjectID: pid,
			Title:     "Test Task",
			State:     models.TaskStateActive,
		}
		mockTaskSvc.On("GetByID", mock.Anything, uid, models.RoleUser, tid).Return(mockTask, nil).Once()

		activeState := "active"
		updatedTask := &models.Task{
			ID:        tid,
			ProjectID: pid,
			Title:     "Test Task",
			State:     models.TaskStateActive,
		}
		mockTaskSvc.On("Update", mock.Anything, uid, models.RoleUser, tid, dto.UpdateTaskRequest{
			Status: &activeState,
		}).Return(updatedTask, nil).Once()

		mockOrchSvc.On("EnqueueInitialStep", mock.Anything, tid).Return(nil).Once()

		args := json.RawMessage(`{"task_id":"` + tid.String() + `","status":"active"}`)
		res, err := executor.taskUpdate(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   models.Task `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, models.TaskStateActive, response.Data.State)

		// Wait briefly for background goroutine to execute
		time.Sleep(50 * time.Millisecond)

		mockTaskSvc.AssertExpectations(t)
		mockOrchSvc.AssertExpectations(t)
	})
}

func TestAuthorizedExecutor_CodeSearch(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}

	t.Run("Success", func(t *testing.T) {
		expectedChunks := []indexer.Chunk{
			{FilePath: "main.go", Content: "package main"},
		}
		mockProjectSvc.On("SearchCode", mock.Anything, uid, models.RoleUser, pid, "my query", 10).Return(expectedChunks, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","query":"my query"}`)
		res, err := executor.codeSearch(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string          `json:"status"`
			Data   []indexer.Chunk `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		require.Len(t, response.Data, 1)
		assert.Equal(t, "main.go", response.Data[0].FilePath)
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		args := json.RawMessage(`{"project_id":"` + pid.String() + `"}`) // missing query
		res, err := executor.codeSearch(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Contains(t, response.Message, "query is required")
	})
}

func TestAuthorizedExecutor_CodeFileRead(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}
	tmpDir := t.TempDir()

	// Write a test file
	filePath := filepath.Join(tmpDir, "test.txt")
	fileContent := "line1\nline2\nline3\nline4\nline5"
	err := os.WriteFile(filePath, []byte(fileContent), 0644)
	require.NoError(t, err)

	t.Run("Success full read", func(t *testing.T) {
		mockProjectSvc.On("GetProjectRepoPath", mock.Anything, uid, models.RoleUser, pid).Return(tmpDir, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","path":"test.txt"}`)
		res, err := executor.codeFileRead(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   struct {
				Content    string `json:"content"`
				TotalLines int    `json:"total_lines"`
				LineStart  int    `json:"line_start"`
				LineEnd    int    `json:"line_end"`
			} `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, fileContent, response.Data.Content)
		assert.Equal(t, 5, response.Data.TotalLines)
		assert.Equal(t, 1, response.Data.LineStart)
		assert.Equal(t, 5, response.Data.LineEnd)
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("Success partial read", func(t *testing.T) {
		mockProjectSvc.On("GetProjectRepoPath", mock.Anything, uid, models.RoleUser, pid).Return(tmpDir, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","path":"test.txt","line_start":2,"line_end":4}`)
		res, err := executor.codeFileRead(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string `json:"status"`
			Data   struct {
				Content    string `json:"content"`
				TotalLines int    `json:"total_lines"`
				LineStart  int    `json:"line_start"`
				LineEnd    int    `json:"line_end"`
			} `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		assert.Equal(t, "line2\nline3\nline4", response.Data.Content)
		assert.Equal(t, 5, response.Data.TotalLines)
		assert.Equal(t, 2, response.Data.LineStart)
		assert.Equal(t, 4, response.Data.LineEnd)
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("Path Traversal Blocked", func(t *testing.T) {
		mockProjectSvc.On("GetProjectRepoPath", mock.Anything, uid, models.RoleUser, pid).Return(tmpDir, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","path":"../etc/passwd"}`)
		res, err := executor.codeFileRead(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Contains(t, response.Message, "path traversal detected")
		mockProjectSvc.AssertExpectations(t)
	})
}

func TestAuthorizedExecutor_CodeDirList(t *testing.T) {
	mockProjectSvc := new(mockProjectService)
	executor := NewAuthorizedExecutor(AuthorizedExecutorDeps{
		ProjectService: mockProjectSvc,
	})

	uid := uuid.New()
	pid := uuid.New()
	auth := agentloop.AuthContext{UserID: uid.String(), ProjectID: pid.String()}
	tmpDir := t.TempDir()

	// Write some files/directories
	err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	t.Run("Success list root", func(t *testing.T) {
		mockProjectSvc.On("GetProjectRepoPath", mock.Anything, uid, models.RoleUser, pid).Return(tmpDir, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `"}`)
		res, err := executor.codeDirList(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status string     `json:"status"`
			Data   []dirEntry `json:"data"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response.Status)
		require.Len(t, response.Data, 2)

		var names []string
		for _, e := range response.Data {
			names = append(names, e.Name)
		}
		assert.Contains(t, names, "subdir")
		assert.Contains(t, names, "file.txt")
		mockProjectSvc.AssertExpectations(t)
	})

	t.Run("Path Traversal Blocked", func(t *testing.T) {
		mockProjectSvc.On("GetProjectRepoPath", mock.Anything, uid, models.RoleUser, pid).Return(tmpDir, nil).Once()

		args := json.RawMessage(`{"project_id":"` + pid.String() + `","path":"../"}`)
		res, err := executor.codeDirList(context.Background(), auth, args)
		require.NoError(t, err)

		var response struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(res, &response)
		require.NoError(t, err)
		assert.Equal(t, "validation", response.Status)
		assert.Contains(t, response.Message, "path traversal detected")
		mockProjectSvc.AssertExpectations(t)
	})
}

