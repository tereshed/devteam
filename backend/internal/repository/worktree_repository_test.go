//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// cleanupWorktrees — точечная очистка worktrees между тестами.
// (Project-cleanup'у не доверяемся, чтобы тесты были self-contained и не зависели
// от порядка DELETE в cleanupProjectIntegrationDB.)
func cleanupWorktrees(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec("DELETE FROM worktrees").Error)
}

func createWorktreeTestTask(t *testing.T, db *gorm.DB) *models.Task {
	t.Helper()
	user, project := taskRepoTestUserProject(t, db)
	task := newTestTask(project.ID, user.ID, "wt-test-"+uuid.NewString()[:8])
	require.NoError(t, NewTaskRepository(db).Create(context.Background(), task))
	return task
}

func createTestWorktree(
	t *testing.T,
	db *gorm.DB,
	taskID uuid.UUID,
	state models.WorktreeState,
	allocatedAt time.Time,
) *models.Worktree {
	t.Helper()
	wt := &models.Worktree{
		TaskID:      taskID,
		BaseBranch:  "main",
		State:       state,
		AllocatedAt: allocatedAt,
	}
	// BranchName заполнит BeforeCreate. allocated_at в БД имеет default=NOW(),
	// поэтому для проверки сортировки используем Update — GORM не пишет zero-value
	// для time.Time в Create при default-стратегии. См. ниже после Create.
	require.NoError(t, NewWorktreeRepository(db).Create(context.Background(), wt))
	// Принудительно фиксируем allocated_at, чтобы получить детерминированный порядок.
	require.NoError(t, db.Model(&models.Worktree{}).
		Where("id = ?", wt.ID).
		Update("allocated_at", allocatedAt).Error)
	wt.AllocatedAt = allocatedAt
	return wt
}

func TestWorktreeRepository_List_GlobalReturnsRecentFirst(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)
	cleanupWorktrees(t, db)

	task1 := createWorktreeTestTask(t, db)
	task2 := createWorktreeTestTask(t, db)

	now := time.Now().UTC().Truncate(time.Microsecond)
	older := createTestWorktree(t, db, task1.ID, models.WorktreeStateReleased, now.Add(-2*time.Hour))
	mid := createTestWorktree(t, db, task1.ID, models.WorktreeStateInUse, now.Add(-1*time.Hour))
	newest := createTestWorktree(t, db, task2.ID, models.WorktreeStateAllocated, now)

	repo := NewWorktreeRepository(db)
	got, err := repo.List(context.Background(), WorktreeFilter{})
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Порядок строго allocated_at DESC: newest → mid → older.
	assert.Equal(t, newest.ID, got[0].ID)
	assert.Equal(t, mid.ID, got[1].ID)
	assert.Equal(t, older.ID, got[2].ID)
}

func TestWorktreeRepository_List_FilterByState(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)
	cleanupWorktrees(t, db)

	task := createWorktreeTestTask(t, db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	allocated := createTestWorktree(t, db, task.ID, models.WorktreeStateAllocated, now)
	_ = createTestWorktree(t, db, task.ID, models.WorktreeStateInUse, now.Add(-1*time.Hour))
	_ = createTestWorktree(t, db, task.ID, models.WorktreeStateReleased, now.Add(-2*time.Hour))

	repo := NewWorktreeRepository(db)
	state := models.WorktreeStateAllocated
	got, err := repo.List(context.Background(), WorktreeFilter{State: &state})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, allocated.ID, got[0].ID)
	assert.Equal(t, models.WorktreeStateAllocated, got[0].State)
}

func TestWorktreeRepository_List_FilterByTaskID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)
	cleanupWorktrees(t, db)

	task1 := createWorktreeTestTask(t, db)
	task2 := createWorktreeTestTask(t, db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	mine := createTestWorktree(t, db, task1.ID, models.WorktreeStateInUse, now)
	_ = createTestWorktree(t, db, task2.ID, models.WorktreeStateInUse, now.Add(-1*time.Hour))

	repo := NewWorktreeRepository(db)
	got, err := repo.List(context.Background(), WorktreeFilter{TaskID: &task1.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, mine.ID, got[0].ID)
	assert.Equal(t, task1.ID, got[0].TaskID)
}

func TestWorktreeRepository_List_InvalidStateRejected(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)
	cleanupWorktrees(t, db)

	repo := NewWorktreeRepository(db)
	bogus := models.WorktreeState("nope")
	_, err := repo.List(context.Background(), WorktreeFilter{State: &bogus})
	require.Error(t, err)
}

func TestWorktreeRepository_List_LimitAndOffset(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupProjectIntegrationDB(t, db)
	cleanupWorktrees(t, db)

	task := createWorktreeTestTask(t, db)
	base := time.Now().UTC().Truncate(time.Microsecond)
	// Создаём 5 worktree'ев со ступенчатым allocated_at; ожидаем порядок DESC.
	wts := make([]*models.Worktree, 0, 5)
	for i := 0; i < 5; i++ {
		wts = append(wts, createTestWorktree(
			t, db, task.ID, models.WorktreeStateReleased,
			base.Add(-time.Duration(i)*time.Minute),
		))
	}

	repo := NewWorktreeRepository(db)
	got, err := repo.List(context.Background(), WorktreeFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, wts[0].ID, got[0].ID)
	assert.Equal(t, wts[1].ID, got[1].ID)

	got2, err := repo.List(context.Background(), WorktreeFilter{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Len(t, got2, 2)
	assert.Equal(t, wts[2].ID, got2[0].ID)
	assert.Equal(t, wts[3].ID, got2[1].ID)
}
