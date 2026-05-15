package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock WorktreeRepository (минимальный, только для тестов ListWorktrees)
// ─────────────────────────────────────────────────────────────────────────────

type mockWorktreeRepo struct {
	mock.Mock
}

func (m *mockWorktreeRepo) Create(ctx context.Context, w *models.Worktree) error {
	return m.Called(ctx, w).Error(0)
}

func (m *mockWorktreeRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Worktree, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Worktree), args.Error(1)
}

func (m *mockWorktreeRepo) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.Worktree, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Worktree), args.Error(1)
}

func (m *mockWorktreeRepo) List(ctx context.Context, filter repository.WorktreeFilter) ([]models.Worktree, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Worktree), args.Error(1)
}

func (m *mockWorktreeRepo) UpdateState(ctx context.Context, id uuid.UUID, newState models.WorktreeState) error {
	return m.Called(ctx, id, newState).Error(0)
}

func (m *mockWorktreeRepo) MarkInUse(ctx context.Context, id uuid.UUID, agentJobID int64) error {
	return m.Called(ctx, id, agentJobID).Error(0)
}

func (m *mockWorktreeRepo) ListForCleanup(ctx context.Context, cutoff time.Time) ([]models.Worktree, error) {
	args := m.Called(ctx, cutoff)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Worktree), args.Error(1)
}

func (m *mockWorktreeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

func setupOrchestrationV2Router(
	wtRepo repository.WorktreeRepository,
	taskSvc service.TaskService,
	userID uuid.UUID,
	userRole models.UserRole,
) *gin.Engine {
	return setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, nil, userID, userRole)
}

// setupOrchestrationV2RouterWithMgr — версия для ReleaseWorktree-тестов: позволяет
// передать реальный WorktreeManager (с тем же mockWorktreeRepo, что и handler).
// Когда mgr=nil — endpoint /release должен ответить 503 (см. handler).
func setupOrchestrationV2RouterWithMgr(
	wtRepo repository.WorktreeRepository,
	taskSvc service.TaskService,
	mgr *service.WorktreeManager,
	userID uuid.UUID,
	userRole models.UserRole,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOrchestrationV2Handler(nil, nil, wtRepo, taskSvc, mgr)

	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", string(userRole))
		c.Next()
	})
	r.GET("/worktrees", h.ListWorktrees)
	r.POST("/worktrees/:id/release", h.ReleaseWorktree)
	return r
}

// newTestWorktreeMgr — реальный WorktreeManager поверх wtRepo. RepoRoot/WorktreesRoot —
// tmpdir'ы, реального git нет: поэтому `git worktree remove` упадёт, но это нормально
// — handler-тесту важен только маппинг ошибок ReleaseManual в HTTP-коды, а не успешное
// удаление каталога. UpdateState всё равно вызывается (так задумано в removeAndMarkReleased).
func newTestWorktreeMgr(t *testing.T, wtRepo repository.WorktreeRepository) *service.WorktreeManager {
	t.Helper()
	mgr, err := service.NewWorktreeManager(
		service.WorktreeManagerConfig{
			RepoRoot:      t.TempDir(),
			WorktreesRoot: t.TempDir(),
		},
		wtRepo,
		nil, // logger=nil → discard
	)
	require.NoError(t, err)
	return mgr
}

func sampleWorktree(id, taskID uuid.UUID, state models.WorktreeState, allocatedAt time.Time) models.Worktree {
	return models.Worktree{
		ID:          id,
		TaskID:      taskID,
		BaseBranch:  "main",
		BranchName:  models.MakeWorktreeBranchName(taskID, id),
		State:       state,
		AllocatedAt: allocatedAt,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ListWorktrees — security split
// ─────────────────────────────────────────────────────────────────────────────

func TestListWorktrees_NoTaskID_NonAdmin_Returns403(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/worktrees", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	// Repository НЕ должен быть дёрнут — security-проверка делает short-circuit.
	wtRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
}

func TestListWorktrees_OwnTaskID_NonAdmin_Returns200(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	// Owner-проверка — задача найдена (TaskService.GetByID == nil error).
	taskSvc.On("GetByID", mock.Anything, userID, models.UserRole(models.RoleUser), taskID).
		Return(&models.Task{ID: taskID}, nil)

	expected := []models.Worktree{
		sampleWorktree(uuid.New(), taskID, models.WorktreeStateInUse, now),
	}
	wtRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.WorktreeFilter) bool {
		return f.TaskID != nil && *f.TaskID == taskID && f.State == nil
	})).Return(expected, nil)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, userID, models.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?task_id="+taskID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
	wtRepo.AssertExpectations(t)
	taskSvc.AssertExpectations(t)
}

func TestListWorktrees_TaskID_NoAccess_Returns403(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()

	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	taskSvc.On("GetByID", mock.Anything, userID, models.UserRole(models.RoleUser), taskID).
		Return(nil, service.ErrProjectForbidden)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, userID, models.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?task_id="+taskID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	wtRepo.AssertNotCalled(t, "List", mock.Anything, mock.Anything)
}

func TestListWorktrees_TaskID_NotFound_Returns404(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()

	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	taskSvc.On("GetByID", mock.Anything, userID, models.UserRole(models.RoleUser), taskID).
		Return(nil, service.ErrTaskNotFound)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, userID, models.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?task_id="+taskID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// ListWorktrees — admin path / filters / ordering
// ─────────────────────────────────────────────────────────────────────────────

func TestListWorktrees_GlobalList_ReturnsRecentFirst(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	taskID := uuid.New()
	now := time.Now().UTC()
	older := sampleWorktree(uuid.New(), taskID, models.WorktreeStateReleased, now.Add(-2*time.Hour))
	newer := sampleWorktree(uuid.New(), taskID, models.WorktreeStateAllocated, now)

	// Repository обязан вернуть в порядке allocated_at DESC — handler не пересортировывает.
	wtRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.WorktreeFilter) bool {
		return f.TaskID == nil && f.State == nil
	})).Return([]models.Worktree{newer, older}, nil)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []struct {
			ID          string    `json:"id"`
			AllocatedAt time.Time `json:"allocated_at"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 2)
	assert.Equal(t, newer.ID.String(), resp.Items[0].ID, "newer worktree must come first")
	assert.Equal(t, older.ID.String(), resp.Items[1].ID)
	assert.Equal(t, 2, resp.Total)
}

func TestListWorktrees_FilterByState(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	taskID := uuid.New()
	now := time.Now().UTC()
	inUse := sampleWorktree(uuid.New(), taskID, models.WorktreeStateInUse, now)

	wtRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.WorktreeFilter) bool {
		return f.State != nil && *f.State == models.WorktreeStateInUse && f.TaskID == nil
	})).Return([]models.Worktree{inUse}, nil)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?state=in_use", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "in_use", resp.Items[0]["state"])
	wtRepo.AssertExpectations(t)
}

func TestListWorktrees_InvalidState_Returns400(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?state=bogus", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListWorktrees_InvalidTaskID_Returns400(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?task_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListWorktrees_LimitCappedAt200(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	// Любой ?limit > 200 должен быть подрезан до 200 ДО передачи в репу,
	// иначе клиент сможет затребовать миллион строк → OOM.
	wtRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.WorktreeFilter) bool {
		return f.Limit == repository.WorktreeListDefaultLimit
	})).Return([]models.Worktree{}, nil)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?limit=999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	wtRepo.AssertExpectations(t)
}

func TestListWorktrees_LimitBelowCap_PassedThrough(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	// При limit ≤ 200 значение пробрасывается как есть.
	wtRepo.On("List", mock.Anything, mock.MatchedBy(func(f repository.WorktreeFilter) bool {
		return f.Limit == 50
	})).Return([]models.Worktree{}, nil)

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees?limit=50", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	wtRepo.AssertExpectations(t)
}

func TestListWorktrees_RepoError_Returns500(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	wtRepo.On("List", mock.Anything, mock.Anything).Return(nil, errors.New("boom"))

	r := setupOrchestrationV2Router(wtRepo, taskSvc, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/worktrees", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// ReleaseWorktree — Sprint 17 / 6.3 (manual unstick)
// ─────────────────────────────────────────────────────────────────────────────

func TestReleaseWorktree_NonAdmin_Returns403(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)
	mgr := newTestWorktreeMgr(t, wtRepo)

	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, mgr, uuid.New(), models.RoleUser)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/"+uuid.New().String()+"/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	// Mgr НЕ должен быть дёрнут — admin-guard работает до ReleaseManual.
	wtRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestReleaseWorktree_MgrNil_Returns503(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)

	// mgr=nil имитирует unset WORKTREES_ROOT/REPO_ROOT в проде. Admin должен
	// получить явный 503, не молчаливый 500.
	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, nil, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/"+uuid.New().String()+"/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	wtRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestReleaseWorktree_BadUUID_Returns400(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)
	mgr := newTestWorktreeMgr(t, wtRepo)

	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, mgr, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/not-a-uuid/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	wtRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
}

func TestReleaseWorktree_NotFound_Returns404(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)
	mgr := newTestWorktreeMgr(t, wtRepo)

	wtID := uuid.New()
	wtRepo.On("GetByID", mock.Anything, wtID).Return(nil, repository.ErrWorktreeNotFound)

	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, mgr, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/"+wtID.String()+"/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	wtRepo.AssertExpectations(t)
}

func TestReleaseWorktree_AlreadyReleased_Returns409(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)
	mgr := newTestWorktreeMgr(t, wtRepo)

	wtID := uuid.New()
	taskID := uuid.New()
	wt := sampleWorktree(wtID, taskID, models.WorktreeStateReleased, time.Now())
	wtRepo.On("GetByID", mock.Anything, wtID).Return(&wt, nil)

	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, mgr, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/"+wtID.String()+"/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	// UpdateState НЕ должен вызываться при already-released.
	wtRepo.AssertNotCalled(t, "UpdateState", mock.Anything, mock.Anything, mock.Anything)
}

func TestReleaseWorktree_HappyPath_Returns200WithReleasedState(t *testing.T) {
	wtRepo := new(mockWorktreeRepo)
	taskSvc := new(MockTaskService)
	mgr := newTestWorktreeMgr(t, wtRepo)

	wtID := uuid.New()
	taskID := uuid.New()
	wtBefore := sampleWorktree(wtID, taskID, models.WorktreeStateInUse, time.Now())
	wtAfter := sampleWorktree(wtID, taskID, models.WorktreeStateReleased, wtBefore.AllocatedAt)
	releasedAt := time.Now().UTC()
	wtAfter.ReleasedAt = &releasedAt

	// 1) первичный GetByID — state='in_use'
	// 2) UpdateState → released (git remove упадёт в tmpdir не-репо, но это нормально:
	//    removeAndMarkReleased продолжит к UpdateState даже при ошибке git)
	// 3) повторный GetByID для возврата актуального wt с released_at
	wtRepo.On("GetByID", mock.Anything, wtID).Return(&wtBefore, nil).Once()
	wtRepo.On("UpdateState", mock.Anything, wtID, models.WorktreeStateReleased).Return(nil).Once()
	wtRepo.On("GetByID", mock.Anything, wtID).Return(&wtAfter, nil).Once()

	r := setupOrchestrationV2RouterWithMgr(wtRepo, taskSvc, mgr, uuid.New(), models.RoleAdmin)
	req := httptest.NewRequest(http.MethodPost, "/worktrees/"+wtID.String()+"/release", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, wtID.String(), resp["id"])
	assert.Equal(t, "released", resp["state"], "response должен отражать новое состояние, чтобы UI обновил tile без refetch")
	wtRepo.AssertExpectations(t)
}
