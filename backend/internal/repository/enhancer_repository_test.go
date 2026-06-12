//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupEnhancerFixture создаёт изолированные user+project и регистрирует
// точечную уборку (project удаляется каскадом вместе с enhancer_* строками).
func setupEnhancerFixture(t *testing.T, db *gorm.DB) (*models.User, *models.Project) {
	t.Helper()
	ctx := context.Background()
	user := createProjectTestUser(t, db, "enh-"+uuid.NewString()+"@example.com")
	p := &models.Project{
		Name:        "enh-proj-" + uuid.NewString()[:8],
		GitProvider: models.GitProviderLocal,
		UserID:      user.ID,
		Status:      models.ProjectStatusActive,
	}
	require.NoError(t, NewProjectRepository(db).Create(ctx, p))
	t.Cleanup(func() {
		_ = db.Exec(`DELETE FROM projects WHERE id = ?`, p.ID).Error
		_ = db.Exec(`DELETE FROM users WHERE id = ?`, user.ID).Error
	})
	return user, p
}

func TestEnhancerRepository_ConfigCRUDAndListDue(t *testing.T) {
	db := setupTestDB(t)
	user, project := setupEnhancerFixture(t, db)
	repo := NewEnhancerRepository(db)
	ctx := context.Background()

	_, err := repo.GetConfigByProjectID(ctx, project.ID)
	require.ErrorIs(t, err, ErrEnhancerConfigNotFound)

	cron := "0 9 * * *"
	past := time.Now().Add(-time.Minute)
	cfg := &models.EnhancerConfig{
		ProjectID:          project.ID,
		CreatedBy:          user.ID,
		IsActive:           true,
		Autonomy:           models.EnhancerAutonomyPropose,
		CronExpression:     &cron,
		AnalysisWindowDays: 7,
		MaxChangesPerRun:   5,
		NextRunAt:          &past,
	}
	require.NoError(t, repo.CreateConfig(ctx, cfg))

	got, err := repo.GetConfigByProjectID(ctx, project.ID)
	require.NoError(t, err)
	require.True(t, got.IsActive)
	require.Equal(t, models.EnhancerAutonomyPropose, got.Autonomy)

	due, err := repo.ListDueConfigs(ctx, time.Now(), 0)
	require.NoError(t, err)
	found := false
	for _, d := range due {
		if d.ProjectID == project.ID {
			found = true
		}
	}
	require.True(t, found, "созревший конфиг обязан попасть в ListDue")

	future := time.Now().Add(time.Hour)
	got.NextRunAt = &future
	require.NoError(t, repo.UpdateConfig(ctx, got))
	due, err = repo.ListDueConfigs(ctx, time.Now(), 0)
	require.NoError(t, err)
	for _, d := range due {
		require.NotEqual(t, project.ID, d.ProjectID, "конфиг с next_run_at в будущем не должен быть due")
	}
}

func TestEnhancerRepository_RunsAndChanges(t *testing.T) {
	db := setupTestDB(t)
	_, project := setupEnhancerFixture(t, db)
	repo := NewEnhancerRepository(db)
	ctx := context.Background()

	run := &models.EnhancerRun{
		ProjectID:   project.ID,
		TriggerKind: models.EnhancerRunTriggerManual,
		Status:      models.EnhancerRunStatusRunning,
		StartedAt:   time.Now(),
	}
	require.NoError(t, repo.CreateRun(ctx, run))

	// Свежий running блокирует новый прогон.
	busy, err := repo.HasRunningRun(ctx, project.ID, time.Hour)
	require.NoError(t, err)
	require.True(t, busy)

	// Предложение + лимит-счётчик.
	change := &models.EnhancerChange{
		RunID:          run.ID,
		ProjectID:      project.ID,
		TargetKind:     models.EnhancerChangeKindProjectDescription,
		Payload:        []byte(`{"old":"a","new":"b"}`),
		Reason:         "tasks lacked context",
		ExpectedEffect: "fewer needs_human",
		Status:         models.EnhancerChangeStatusProposed,
	}
	require.NoError(t, repo.CreateChange(ctx, change))
	count, err := repo.CountChangesByRunID(ctx, run.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	changes, err := repo.ListChangesByRunID(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, models.EnhancerChangeStatusProposed, changes[0].Status)

	// Завершение прогона.
	now := time.Now()
	run.Status = models.EnhancerRunStatusDone
	run.Report = "report body"
	run.FinishedAt = &now
	require.NoError(t, repo.UpdateRun(ctx, run))

	runs, err := repo.ListRunsByProjectID(ctx, project.ID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, models.EnhancerRunStatusDone, runs[0].Status)
	require.Equal(t, "report body", runs[0].Report)

	busy, err = repo.HasRunningRun(ctx, project.ID, time.Hour)
	require.NoError(t, err)
	require.False(t, busy)
}

func TestEnhancerRepository_StaleRunningRecovered(t *testing.T) {
	db := setupTestDB(t)
	_, project := setupEnhancerFixture(t, db)
	repo := NewEnhancerRepository(db)
	ctx := context.Background()

	stale := &models.EnhancerRun{
		ProjectID:   project.ID,
		TriggerKind: models.EnhancerRunTriggerCron,
		Status:      models.EnhancerRunStatusRunning,
		StartedAt:   time.Now().Add(-2 * time.Hour),
	}
	require.NoError(t, repo.CreateRun(ctx, stale))

	// Прогоны старше staleAfter гасятся в failed и не блокируют новые.
	busy, err := repo.HasRunningRun(ctx, project.ID, time.Hour)
	require.NoError(t, err)
	require.False(t, busy)

	got, err := repo.GetRunByID(ctx, stale.ID)
	require.NoError(t, err)
	require.Equal(t, models.EnhancerRunStatusFailed, got.Status)
	require.NotNil(t, got.FinishedAt)
}

func TestEnhancerRepository_OverridesUpsertAndRebuildSource(t *testing.T) {
	db := setupTestDB(t)
	user, project := setupEnhancerFixture(t, db)
	repo := NewEnhancerRepository(db)
	ctx := context.Background()

	// Агент в команде проекта (нужен валидный FK agents.id).
	team := &models.Team{ProjectID: project.ID, Name: "enh-team", Type: "development"}
	require.NoError(t, db.WithContext(ctx).Create(team).Error)
	agent := &models.Agent{
		Name:          "enh-dev-" + uuid.NewString()[:8],
		Role:          models.AgentRoleDeveloper,
		ExecutionKind: models.AgentExecutionKindLLM,
		TeamID:        &team.ID,
		IsActive:      true,
		Skills:        []byte(`[]`),
		Settings:      []byte(`{}`),
		ModelConfig:   []byte(`{}`),
	}
	require.NoError(t, db.WithContext(ctx).Create(agent).Error)
	t.Cleanup(func() {
		_ = db.Exec(`DELETE FROM agents WHERE id = ?`, agent.ID).Error
		_ = db.Exec(`DELETE FROM teams WHERE id = ?`, team.ID).Error
	})

	_, err := repo.GetActiveOverride(ctx, project.ID, agent.ID)
	require.ErrorIs(t, err, ErrProjectAgentOverrideNotFound)

	// Upsert: создание → обновление → удаление пустой свёрткой.
	require.NoError(t, repo.UpsertOverride(ctx, project.ID, agent.ID, "правило один", user.ID))
	o, err := repo.GetActiveOverride(ctx, project.ID, agent.ID)
	require.NoError(t, err)
	require.Equal(t, "правило один", o.PromptAddendum)

	require.NoError(t, repo.UpsertOverride(ctx, project.ID, agent.ID, "правило один\n\nправило два", user.ID))
	o, err = repo.GetActiveOverride(ctx, project.ID, agent.ID)
	require.NoError(t, err)
	require.Contains(t, o.PromptAddendum, "правило два")

	require.NoError(t, repo.UpsertOverride(ctx, project.ID, agent.ID, "", user.ID))
	_, err = repo.GetActiveOverride(ctx, project.ID, agent.ID)
	require.ErrorIs(t, err, ErrProjectAgentOverrideNotFound)

	// ListAppliedAgentChanges — источник пересборки: только applied и только agent_override.
	run := &models.EnhancerRun{ProjectID: project.ID, TriggerKind: models.EnhancerRunTriggerManual, Status: models.EnhancerRunStatusDone, StartedAt: time.Now()}
	require.NoError(t, repo.CreateRun(ctx, run))
	now := time.Now()
	mk := func(status models.EnhancerChangeStatus, appliedAt *time.Time) *models.EnhancerChange {
		ch := &models.EnhancerChange{
			RunID: run.ID, ProjectID: project.ID,
			TargetKind: models.EnhancerChangeKindAgentOverride, TargetAgentID: &agent.ID,
			Payload: []byte(`{"prompt_addendum":"a"}`), Status: status, AppliedAt: appliedAt,
		}
		require.NoError(t, repo.CreateChange(ctx, ch))
		return ch
	}
	mk(models.EnhancerChangeStatusApplied, &now)
	mk(models.EnhancerChangeStatusProposed, nil)
	mk(models.EnhancerChangeStatusRolledBack, &now)

	applied, err := repo.ListAppliedAgentChanges(ctx, project.ID, agent.ID)
	require.NoError(t, err)
	require.Len(t, applied, 1)
	require.Equal(t, models.EnhancerChangeStatusApplied, applied[0].Status)
}
