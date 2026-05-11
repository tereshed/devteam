//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func teamTestProject(t *testing.T, db *gorm.DB) (*models.User, *models.Project) {
	t.Helper()
	ctx := context.Background()
	user := createProjectTestUser(t, db, "team-"+uuid.NewString()+"@example.com")
	repo := NewProjectRepository(db)
	p := &models.Project{
		Name:        "team-proj",
		GitProvider: models.GitProviderLocal,
		UserID:      user.ID,
		Status:      models.ProjectStatusActive,
	}
	require.NoError(t, repo.Create(ctx, p))
	return user, p
}

func teamRepoCreate(t *testing.T, db *gorm.DB, projectID uuid.UUID, name string) *models.Team {
	t.Helper()
	ctx := context.Background()
	team := &models.Team{
		Name:      name,
		ProjectID: projectID,
		Type:      models.TeamTypeDevelopment,
	}
	require.NoError(t, NewTeamRepository(db).Create(ctx, team))
	return team
}

func TestTeamRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	repo := NewTeamRepository(db)
	ctx := context.Background()

	team := &models.Team{
		Name:      "Core squad",
		ProjectID: p.ID,
		Type:      models.TeamTypeDevelopment,
	}
	require.NoError(t, repo.Create(ctx, team))
	assert.NotEqual(t, uuid.Nil, team.ID)

	got, err := repo.GetByProjectID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, team.ID, got.ID)
	assert.Equal(t, "Core squad", got.Name)
	assert.Equal(t, p.ID, got.ProjectID)
	assert.Equal(t, models.TeamTypeDevelopment, got.Type)
}

func TestTeamRepository_Create_DuplicateProject(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	repo := NewTeamRepository(db)
	ctx := context.Background()

	t1 := &models.Team{Name: "First", ProjectID: p.ID, Type: models.TeamTypeDevelopment}
	require.NoError(t, repo.Create(ctx, t1))

	t2 := &models.Team{Name: "Second", ProjectID: p.ID, Type: models.TeamTypeDevelopment}
	err := repo.Create(ctx, t2)
	assert.ErrorIs(t, err, ErrTeamAlreadyExists)
}

func TestTeamRepository_Create_InvalidProjectID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	repo := NewTeamRepository(db)
	ctx := context.Background()

	team := &models.Team{
		Name:      "orphan",
		ProjectID: uuid.New(),
		Type:      models.TeamTypeDevelopment,
	}
	err := repo.Create(ctx, team)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrTeamAlreadyExists))
}

func TestTeamRepository_GetByID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "gbyid")
	ctx := context.Background()

	// Создаём агентов в порядке, отличном от role ASC (worker, developer, planner → developer, planner, worker)
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	for _, tc := range []struct {
		name string
		role models.AgentRole
	}{
		{"w", models.AgentRoleWorker},
		{"d", models.AgentRoleDeveloper},
		{"p", models.AgentRolePlanner},
	} {
		a := &models.Agent{
			Name:     tc.name,
			Role:     tc.role,
			TeamID:   &team.ID,
			Skills:   skills,
			Settings: settings,
		}
		require.NoError(t, db.WithContext(ctx).Create(a).Error)
	}

	repo := NewTeamRepository(db)
	got, err := repo.GetByID(ctx, team.ID)
	require.NoError(t, err)
	require.Len(t, got.Agents, 3)
	assert.Equal(t, models.AgentRoleDeveloper, got.Agents[0].Role)
	assert.Equal(t, models.AgentRolePlanner, got.Agents[1].Role)
	assert.Equal(t, models.AgentRoleWorker, got.Agents[2].Role)
}

func TestTeamRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, err := NewTeamRepository(db).GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestTeamRepository_GetByProjectID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "gbyp")
	ctx := context.Background()

	pr := &models.Prompt{
		Name:        "tp-" + uuid.NewString(),
		Description: "d",
		Template:    "system",
	}
	require.NoError(t, db.WithContext(ctx).Create(pr).Error)

	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))
	pid := pr.ID
	agent := &models.Agent{
		Name:     "solo",
		Role:     models.AgentRoleReviewer,
		TeamID:   &team.ID,
		PromptID: &pid,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)

	got, err := NewTeamRepository(db).GetByProjectID(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, got.Agents, 1)
	require.NotNil(t, got.Agents[0].Prompt)
	assert.Equal(t, pr.Name, got.Agents[0].Prompt.Name)
	assert.Equal(t, "system", got.Agents[0].Prompt.Template)
}

func TestTeamRepository_GetByProjectID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, err := NewTeamRepository(db).GetByProjectID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestTeamRepository_GetByProjectID_WithAgents(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "three")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	for i, role := range []models.AgentRole{
		models.AgentRoleDeveloper,
		models.AgentRoleTester,
		models.AgentRoleDevOps,
	} {
		a := &models.Agent{
			Name:     string(rune('a' + i)),
			Role:     role,
			TeamID:   &team.ID,
			Skills:   skills,
			Settings: settings,
		}
		require.NoError(t, db.WithContext(ctx).Create(a).Error)
	}

	got, err := NewTeamRepository(db).GetByProjectID(ctx, p.ID)
	require.NoError(t, err)
	assert.Len(t, got.Agents, 3)
}

func TestTeamRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "old name")
	ctx := context.Background()
	repo := NewTeamRepository(db)

	before := team.UpdatedAt
	time.Sleep(5 * time.Millisecond)

	team.Name = "new name"
	require.NoError(t, repo.Update(ctx, team))

	got, err := repo.GetByProjectID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "new name", got.Name)
	assert.True(t, got.UpdatedAt.After(before) || !got.UpdatedAt.Equal(before))
}

func TestTeamRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "del")
	ctx := context.Background()
	repo := NewTeamRepository(db)

	require.NoError(t, repo.Delete(ctx, team.ID))
	_, err := repo.GetByID(ctx, team.ID)
	assert.ErrorIs(t, err, ErrTeamNotFound)

	// проект остался
	projRepo := NewProjectRepository(db)
	got, err := projRepo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)
}

func TestTeamRepository_Delete_AgentsSetNull(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "nullteam")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	a := &models.Agent{
		Name:     "stay",
		Role:     models.AgentRoleWorker,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(a).Error)
	agentID := a.ID

	require.NoError(t, NewTeamRepository(db).Delete(ctx, team.ID))

	var got models.Agent
	require.NoError(t, db.WithContext(ctx).First(&got, "id = ?", agentID).Error)
	assert.Nil(t, got.TeamID)
}

func TestTeamRepository_Cascade_ProjectDelete(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "cascade")
	ctx := context.Background()

	require.NoError(t, NewProjectRepository(db).Delete(ctx, p.ID))

	_, err := NewTeamRepository(db).GetByID(ctx, team.ID)
	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestTeamRepository_SaveAgentWithToolBindings_Replace(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "bind")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	td1 := models.ToolDefinition{
		Name:             "tool-" + uuid.NewString(),
		Description:      "d",
		Category:         "cat",
		ParametersSchema: datatypes.JSON([]byte("{}")),
		IsActive:         true,
	}
	td2 := models.ToolDefinition{
		Name:             "tool2-" + uuid.NewString(),
		Description:      "d2",
		Category:         "cat2",
		ParametersSchema: datatypes.JSON([]byte("{}")),
		IsActive:         true,
	}
	require.NoError(t, db.WithContext(ctx).Create(&td1).Error)
	require.NoError(t, db.WithContext(ctx).Create(&td2).Error)

	agent := &models.Agent{
		Name:     "solo",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)

	repo := NewTeamRepository(db)
	require.NoError(t, repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{td1.ID}))

	var n int64
	require.NoError(t, db.Model(&models.AgentToolBinding{}).Where("agent_id = ?", agent.ID).Count(&n).Error)
	assert.Equal(t, int64(1), n)

	require.NoError(t, repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{td2.ID}))
	require.NoError(t, db.Model(&models.AgentToolBinding{}).Where("agent_id = ?", agent.ID).Count(&n).Error)
	assert.Equal(t, int64(1), n)
	var got models.AgentToolBinding
	require.NoError(t, db.Where("agent_id = ?", agent.ID).First(&got).Error)
	assert.Equal(t, td2.ID, got.ToolDefinitionID)
}

func TestTeamRepository_SaveAgentWithToolBindings_RepeatedSameSetTouchesUpdatedAt(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "touch-upd")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	td1 := models.ToolDefinition{
		Name:             "touch-" + uuid.NewString(),
		Description:      "d",
		Category:         "c",
		ParametersSchema: datatypes.JSON([]byte("{}")),
		IsActive:         true,
	}
	require.NoError(t, db.WithContext(ctx).Create(&td1).Error)

	agent := &models.Agent{
		Name:     "solo",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)

	repo := NewTeamRepository(db)
	require.NoError(t, repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{td1.ID}))

	var a1 models.Agent
	require.NoError(t, db.WithContext(ctx).First(&a1, "id = ?", agent.ID).Error)
	t1 := a1.UpdatedAt

	time.Sleep(15 * time.Millisecond)
	require.NoError(t, db.WithContext(ctx).First(agent, agent.ID).Error)
	require.NoError(t, repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{td1.ID}))

	var a2 models.Agent
	require.NoError(t, db.WithContext(ctx).First(&a2, "id = ?", agent.ID).Error)
	assert.True(t, a2.UpdatedAt.After(t1), "updated_at must advance on identical tool_bindings replace (13.3.1 A.5)")
}

func TestTeamRepository_SaveAgentWithToolBindings_RollbackOnFK(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "rb")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	td1 := models.ToolDefinition{
		Name:             "tkeep-" + uuid.NewString(),
		Description:      "d",
		Category:         "c",
		ParametersSchema: datatypes.JSON([]byte("{}")),
		IsActive:         true,
	}
	require.NoError(t, db.WithContext(ctx).Create(&td1).Error)

	agent := &models.Agent{
		Name:     "solo",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)

	repo := NewTeamRepository(db)
	require.NoError(t, repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{td1.ID}))

	bad := uuid.New()
	err := repo.SaveAgentWithToolBindings(ctx, agent, true, []uuid.UUID{bad})
	require.Error(t, err)

	var n int64
	require.NoError(t, db.Model(&models.AgentToolBinding{}).Where("agent_id = ?", agent.ID).Count(&n).Error)
	assert.Equal(t, int64(1), n)
	var got models.AgentToolBinding
	require.NoError(t, db.Where("agent_id = ?", agent.ID).First(&got).Error)
	assert.Equal(t, td1.ID, got.ToolDefinitionID)
}

func TestTeamRepository_GetByProjectID_PreloadsToolBindings(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	_, p := teamTestProject(t, db)
	team := teamRepoCreate(t, db, p.ID, "preload-tb")
	ctx := context.Background()
	skills := datatypes.JSON([]byte("[]"))
	settings := datatypes.JSON([]byte("{}"))

	td := models.ToolDefinition{
		Name:             "pre-" + uuid.NewString(),
		Description:      "d",
		Category:         "search",
		ParametersSchema: datatypes.JSON([]byte("{}")),
		IsActive:         true,
	}
	require.NoError(t, db.WithContext(ctx).Create(&td).Error)

	agent := &models.Agent{
		Name:     "a1",
		Role:     models.AgentRoleDeveloper,
		TeamID:   &team.ID,
		Skills:   skills,
		Settings: settings,
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)
	require.NoError(t, db.WithContext(ctx).Create(&models.AgentToolBinding{
		AgentID:          agent.ID,
		ToolDefinitionID: td.ID,
		Config:           datatypes.JSON([]byte("{}")),
	}).Error)

	got, err := NewTeamRepository(db).GetByProjectID(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, got.Agents, 1)
	require.Len(t, got.Agents[0].ToolBindings, 1)
	require.NotNil(t, got.Agents[0].ToolBindings[0].ToolDefinition)
	assert.Equal(t, td.Name, got.Agents[0].ToolBindings[0].ToolDefinition.Name)
}
