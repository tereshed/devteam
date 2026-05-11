//go:build integration
// +build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func cleanupProjectIntegrationDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Exec(`
		DELETE FROM conversation_messages;
		DELETE FROM conversations;
		DELETE FROM task_messages;
		DELETE FROM tasks;
		DELETE FROM agent_mcp_bindings;
		DELETE FROM mcp_server_configs;
		DELETE FROM teams;
		DELETE FROM projects;
		DELETE FROM git_credentials;
		DELETE FROM user_llm_credential_audit;
		DELETE FROM user_llm_credentials;
		DELETE FROM llm_logs;
		DELETE FROM scheduled_workflows;
		DELETE FROM execution_steps;
		DELETE FROM executions;
		DELETE FROM workflows;
		DELETE FROM agent_tool_bindings;
		DELETE FROM agents;
		DELETE FROM tool_definitions;
		DELETE FROM users;
		DELETE FROM prompts;
		DELETE FROM refresh_tokens;
		DELETE FROM api_keys;
	`).Error
	require.NoError(t, err)
}

func createProjectTestUser(t *testing.T, db *gorm.DB, email string) *models.User {
	t.Helper()
	repo := NewUserRepository(db)
	u := &models.User{
		Email:        email,
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func TestProjectRepository_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pcreate1@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	tech := datatypes.JSON([]byte(`{"lang":"go"}`))
	settings := datatypes.JSON([]byte(`{"k":1}`))
	p := &models.Project{
		Name:             "My Project",
		Description:      "Full description",
		GitProvider:      models.GitProviderGitHub,
		GitURL:           "https://github.com/org/repo",
		GitDefaultBranch: "develop",
		VectorCollection: "col1",
		TechStack:        tech,
		Status:           models.ProjectStatusPaused,
		Settings:         settings,
		UserID:           user.ID,
	}
	require.NoError(t, repo.Create(ctx, p))
	assert.NotEqual(t, uuid.Nil, p.ID)

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "My Project", got.Name)
	assert.Equal(t, "Full description", got.Description)
	assert.Equal(t, models.GitProviderGitHub, got.GitProvider)
	assert.Equal(t, "https://github.com/org/repo", got.GitURL)
	assert.Equal(t, "develop", got.GitDefaultBranch)
	assert.Equal(t, "col1", got.VectorCollection)
	assert.Equal(t, models.ProjectStatusPaused, got.Status)
	assert.Equal(t, user.ID, got.UserID)
}

func TestProjectRepository_Create_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pdup@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p1 := &models.Project{Name: "Same", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p1))

	p2 := &models.Project{Name: "Same", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	err := repo.Create(ctx, p2)
	assert.ErrorIs(t, err, ErrProjectNameExists)
}

func TestProjectRepository_GetByID_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pget@example.com")
	ctx := context.Background()

	gc := &models.GitCredential{
		UserID:         user.ID,
		Provider:       models.GitCredentialProviderGitHub,
		AuthType:       models.GitCredentialAuthToken,
		EncryptedValue: []byte("secret"),
		Label:          "work-github",
	}
	require.NoError(t, db.WithContext(ctx).Create(gc).Error)

	repo := NewProjectRepository(db)
	p := &models.Project{
		Name:             "With cred",
		GitProvider:      models.GitProviderGitHub,
		UserID:           user.ID,
		Status:           models.ProjectStatusActive,
		GitCredentialsID: &gc.ID,
	}
	require.NoError(t, repo.Create(ctx, p))

	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got.GitCredential)
	assert.Equal(t, "work-github", got.GitCredential.Label)
	assert.Equal(t, models.GitCredentialProviderGitHub, got.GitCredential.Provider)
}

func TestProjectRepository_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	repo := NewProjectRepository(db)
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestProjectRepository_List_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "plist@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		name := string(rune('A'+i)) + "-pag"
		p := &models.Project{Name: name, GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
		require.NoError(t, repo.Create(ctx, p))
	}

	list, total, err := repo.List(ctx, ProjectFilter{Limit: 2, Offset: 0, OrderBy: "name", OrderDir: "ASC"})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	require.Len(t, list, 2)
}

func TestProjectRepository_List_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pfst@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "active-p", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "arch-p", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusArchived}))

	st := models.ProjectStatusArchived
	list, total, err := repo.List(ctx, ProjectFilter{Status: &st, Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, models.ProjectStatusArchived, list[0].Status)
}

func TestProjectRepository_List_FilterByGitProvider(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pfgp@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "gh", GitProvider: models.GitProviderGitHub, UserID: user.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "loc", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))

	gp := models.GitProviderGitHub
	list, total, err := repo.List(ctx, ProjectFilter{GitProvider: &gp, Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, models.GitProviderGitHub, list[0].GitProvider)
}

func TestProjectRepository_List_Search(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "psearch@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "Alpha", Description: "no match here", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "Beta", Description: "contains QUIRKY text", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))

	q := "quirky"
	list, total, err := repo.List(ctx, ProjectFilter{Search: &q, Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "Beta", list[0].Name)
}

func TestProjectRepository_List_Search_EscapesPercentWildcard(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pwct@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "plain-alpha", Description: "no special", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "has-pct", Description: "discount 50% today", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))

	onlyPct := "%"
	list, total, err := repo.List(ctx, ProjectFilter{Search: &onlyPct, Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "has-pct", list[0].Name)
}

func TestProjectRepository_List_Search_EscapesUnderscoreWildcard(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "puscr@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "fooxbar", Description: "x", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "foo_bar", Description: "underscore", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))

	onlyUS := "_"
	list, total, err := repo.List(ctx, ProjectFilter{Search: &onlyUS, Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	assert.Equal(t, "foo_bar", list[0].Name)
}

func TestProjectRepository_List_LimitZeroUsesDefault(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "plim0@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	for i := range 25 {
		p := &models.Project{
			Name:        fmt.Sprintf("lim-proj-%02d", i),
			GitProvider: models.GitProviderLocal,
			UserID:      user.ID,
			Status:      models.ProjectStatusActive,
		}
		require.NoError(t, repo.Create(ctx, p))
	}

	list, total, err := repo.List(ctx, ProjectFilter{Limit: 0, Offset: 0, OrderBy: "name", OrderDir: "ASC"})
	require.NoError(t, err)
	assert.Equal(t, int64(25), total)
	require.Len(t, list, 20)
}

func TestProjectRepository_List_LimitCappedAtMax(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "plimmax@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	for i := range 120 {
		p := &models.Project{
			Name:        fmt.Sprintf("cap-proj-%03d", i),
			GitProvider: models.GitProviderLocal,
			UserID:      user.ID,
			Status:      models.ProjectStatusActive,
		}
		require.NoError(t, repo.Create(ctx, p))
	}

	list, total, err := repo.List(ctx, ProjectFilter{Limit: 500, Offset: 0, OrderBy: "name", OrderDir: "ASC"})
	require.NoError(t, err)
	assert.Equal(t, int64(120), total)
	require.Len(t, list, 100)
}

func TestProjectRepository_ListByUserID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	u1 := createProjectTestUser(t, db, "pu1@example.com")
	u2 := createProjectTestUser(t, db, "pu2@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "u1a", GitProvider: models.GitProviderLocal, UserID: u1.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "u1b", GitProvider: models.GitProviderLocal, UserID: u1.ID, Status: models.ProjectStatusActive}))
	require.NoError(t, repo.Create(ctx, &models.Project{Name: "u2only", GitProvider: models.GitProviderLocal, UserID: u2.ID, Status: models.ProjectStatusActive}))

	list, total, err := repo.ListByUserID(ctx, u1.ID, ProjectFilter{Limit: 20, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	for _, p := range list {
		assert.Equal(t, u1.ID, p.UserID)
	}
}

func TestProjectRepository_Update_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pupd@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := &models.Project{Name: "old", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p))

	loaded, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	prevUpdated := loaded.UpdatedAt

	time.Sleep(15 * time.Millisecond)

	loaded.Name = "new-name"
	loaded.Status = models.ProjectStatusArchived
	require.NoError(t, repo.Update(ctx, loaded))

	again, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "new-name", again.Name)
	assert.Equal(t, models.ProjectStatusArchived, again.Status)
	assert.True(t, again.UpdatedAt.After(prevUpdated) || !again.UpdatedAt.Equal(prevUpdated))
}

func TestProjectRepository_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pdel@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := &models.Project{Name: "to-delete", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p))

	require.NoError(t, repo.Delete(ctx, p.ID))
	_, err := repo.GetByID(ctx, p.ID)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

func TestProjectRepository_Delete_Cascade(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pcasc@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p := &models.Project{Name: "cascade-root", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p))

	team := &models.Team{
		Name:      "dev",
		ProjectID: p.ID,
		Type:      models.TeamTypeDevelopment,
	}
	require.NoError(t, db.WithContext(ctx).Create(team).Error)

	mcp := &models.MCPServerConfig{
		ProjectID: p.ID,
		Name:      "srv1",
		URL:       "http://localhost:9999/mcp",
		AuthType:  models.MCPAuthNone,
		Settings:  datatypes.JSON([]byte("{}")),
	}
	require.NoError(t, db.WithContext(ctx).Create(mcp).Error)

	require.NoError(t, repo.Delete(ctx, p.ID))

	var teamCount int64
	require.NoError(t, db.WithContext(ctx).Model(&models.Team{}).Where("project_id = ?", p.ID).Count(&teamCount).Error)
	assert.Zero(t, teamCount)

	var mcpCount int64
	require.NoError(t, db.WithContext(ctx).Model(&models.MCPServerConfig{}).Where("project_id = ?", p.ID).Count(&mcpCount).Error)
	assert.Zero(t, mcpCount)
}

func TestProjectRepository_List_OrderBy_Whitelist(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pord@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &models.Project{Name: "z-last", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}))

	filter := ProjectFilter{
		OrderBy:  "id; DROP TABLE projects; --",
		OrderDir: "ASC",
		Limit:    10,
		Offset:   0,
	}
	list, total, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, list, 1)
}

func TestProjectRepository_List_OrderDir_Sanitize(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)

	user := createProjectTestUser(t, db, "pordd@example.com")
	repo := NewProjectRepository(db)
	ctx := context.Background()

	p1 := &models.Project{Name: "first", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p1))
	time.Sleep(20 * time.Millisecond)
	p2 := &models.Project{Name: "second", GitProvider: models.GitProviderLocal, UserID: user.ID, Status: models.ProjectStatusActive}
	require.NoError(t, repo.Create(ctx, p2))

	descFallback, _, err := repo.ListByUserID(ctx, user.ID, ProjectFilter{
		OrderBy: "created_at", OrderDir: "not-a-dir", Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, descFallback, 2)
	assert.Equal(t, p2.ID, descFallback[0].ID)

	ascList, _, err := repo.ListByUserID(ctx, user.ID, ProjectFilter{
		OrderBy: "created_at", OrderDir: "ASC", Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, ascList, 2)
	assert.Equal(t, p1.ID, ascList[0].ID)
}
