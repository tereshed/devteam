package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tools_orchestration_v2_test.go — Sprint 17 / 6.3.
//
// Покрывает деструктивный worktree_release MCP-инструмент. Цель — гарантировать,
// что multi-layer guard (confirm + admin-роль) НЕ обходится при кривом вызове
// LLM. Для каждого слоя — отдельный кейс: убрал confirm → 1й guard, не админ →
// 2й guard, плохой UUID → 3й guard, happy → 4й слой (ReleaseManual).

// adminCtx/userCtx определены в tools_agent_settings_test.go (тот же пакет mcp_test) —
// возвращают (ctx, uuid). Нам uuid не нужен, поэтому оборачиваем в firstCtx.
func firstCtx(ctx context.Context, _ uuid.UUID) context.Context { return ctx }

// memWorktreeRepo — минимальная in-memory реализация repository.WorktreeRepository,
// достаточная для MCP-handler'а worktree_release. Параллельных тестов не делаем,
// поэтому mutex'ы не нужны; sqlite не используем потому что postgres-специфичный
// `gen_random_uuid()` default из миграций ломает CREATE TABLE на sqlite.
type memWorktreeRepo struct {
	data map[uuid.UUID]*models.Worktree
}

func newMemWorktreeRepo() *memWorktreeRepo {
	return &memWorktreeRepo{data: map[uuid.UUID]*models.Worktree{}}
}

func (r *memWorktreeRepo) Create(_ context.Context, w *models.Worktree) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	w.BranchName = models.MakeWorktreeBranchName(w.TaskID, w.ID)
	w.AllocatedAt = time.Now()
	if w.State == "" {
		w.State = models.WorktreeStateAllocated
	}
	r.data[w.ID] = w
	return nil
}

func (r *memWorktreeRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Worktree, error) {
	w, ok := r.data[id]
	if !ok {
		return nil, repository.ErrWorktreeNotFound
	}
	cp := *w
	return &cp, nil
}

func (r *memWorktreeRepo) ListByTaskID(_ context.Context, taskID uuid.UUID) ([]models.Worktree, error) {
	var out []models.Worktree
	for _, w := range r.data {
		if w.TaskID == taskID {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (r *memWorktreeRepo) List(_ context.Context, _ repository.WorktreeFilter) ([]models.Worktree, error) {
	return nil, nil
}

func (r *memWorktreeRepo) UpdateState(_ context.Context, id uuid.UUID, s models.WorktreeState) error {
	w, ok := r.data[id]
	if !ok {
		return repository.ErrWorktreeNotFound
	}
	w.State = s
	if s == models.WorktreeStateReleased {
		now := time.Now()
		w.ReleasedAt = &now
	}
	return nil
}

func (r *memWorktreeRepo) MarkInUse(_ context.Context, id uuid.UUID, jobID int64) error {
	w, ok := r.data[id]
	if !ok || w.State != models.WorktreeStateAllocated {
		return repository.ErrWorktreeNotFound
	}
	w.State = models.WorktreeStateInUse
	w.AgentJobID = &jobID
	return nil
}

func (r *memWorktreeRepo) ListForCleanup(_ context.Context, _ time.Time) ([]models.Worktree, error) {
	return nil, nil
}

func (r *memWorktreeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.data[id]; !ok {
		return repository.ErrWorktreeNotFound
	}
	delete(r.data, id)
	return nil
}

// newTestWorktreeMgrSqlite — реальный WorktreeManager с in-memory репой. Не пытаемся
// выполнять git (repoRoot = пустой tmpdir): removeAndMarkReleased гарантирует, что
// UpdateState всё равно отработает — этого достаточно для проверки маппинга ошибок.
func newTestWorktreeMgrSqlite(t *testing.T) (*service.WorktreeManager, repository.WorktreeRepository) {
	t.Helper()
	repo := newMemWorktreeRepo()
	mgr, err := service.NewWorktreeManager(
		service.WorktreeManagerConfig{
			RepoRoot:      filepath.Clean(t.TempDir()),
			WorktreesRoot: filepath.Clean(t.TempDir()),
		},
		repo,
		nil,
	)
	require.NoError(t, err)
	return mgr, repo
}

// callTool — синхронный вызов MCP-handler'а с разбором JSON-ответа в Response.
func callTool(t *testing.T, h func(context.Context, *gomcp.CallToolRequest, WorktreeReleaseParams) (*gomcp.CallToolResult, any, error),
	ctx context.Context, p WorktreeReleaseParams) *Response {
	t.Helper()
	res, _, err := h(ctx, nil, p)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Content, "expected MCP content")
	tc, ok := res.Content[0].(*gomcp.TextContent)
	require.True(t, ok, "expected text content, got %T", res.Content[0])

	var resp Response
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	return &resp
}

func TestWorktreeReleaseHandler_ConfirmFalse_Rejected(t *testing.T) {
	mgr, _ := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	resp := callTool(t, h, firstCtx(adminCtx()), WorktreeReleaseParams{
		WorktreeID: uuid.New().String(),
		Confirm:    false,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "confirm must be true")
}

func TestWorktreeReleaseHandler_NonAdmin_Rejected(t *testing.T) {
	mgr, _ := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	resp := callTool(t, h, firstCtx(userCtx()), WorktreeReleaseParams{
		WorktreeID: uuid.New().String(),
		Confirm:    true,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "admin role")
}

func TestWorktreeReleaseHandler_NoUserContext_Rejected(t *testing.T) {
	mgr, _ := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	// Без role в context'е — admin-guard должен отказать (а не fall-through на дефолт).
	resp := callTool(t, h, context.Background(), WorktreeReleaseParams{
		WorktreeID: uuid.New().String(),
		Confirm:    true,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "admin role")
}

func TestWorktreeReleaseHandler_BadUUID_Rejected(t *testing.T) {
	mgr, _ := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	resp := callTool(t, h, firstCtx(adminCtx()), WorktreeReleaseParams{
		WorktreeID: "not-a-uuid",
		Confirm:    true,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "must be UUID")
}

func TestWorktreeReleaseHandler_NotFound(t *testing.T) {
	mgr, _ := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	resp := callTool(t, h, firstCtx(adminCtx()), WorktreeReleaseParams{
		WorktreeID: uuid.New().String(),
		Confirm:    true,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "not found")
}

func TestWorktreeReleaseHandler_AlreadyReleased(t *testing.T) {
	mgr, repo := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	wt := &models.Worktree{TaskID: uuid.New(), BaseBranch: "main", State: models.WorktreeStateReleased}
	require.NoError(t, repo.Create(context.Background(), wt))

	resp := callTool(t, h, firstCtx(adminCtx()), WorktreeReleaseParams{
		WorktreeID: wt.ID.String(),
		Confirm:    true,
	})
	assert.Equal(t, StatusError, resp.Status)
	assert.Contains(t, resp.Details, "already released")
}

func TestWorktreeReleaseHandler_HappyPath(t *testing.T) {
	mgr, repo := newTestWorktreeMgrSqlite(t)
	h := makeWorktreeReleaseHandler(mgr)

	wt := &models.Worktree{TaskID: uuid.New(), BaseBranch: "main", State: models.WorktreeStateInUse}
	require.NoError(t, repo.Create(context.Background(), wt))

	resp := callTool(t, h, firstCtx(adminCtx()), WorktreeReleaseParams{
		WorktreeID: wt.ID.String(),
		Confirm:    true,
	})
	assert.Equal(t, StatusOK, resp.Status, "details=%s", resp.Details)

	// State в БД действительно released.
	got, err := repo.GetByID(context.Background(), wt.ID)
	require.NoError(t, err)
	assert.Equal(t, models.WorktreeStateReleased, got.State)
}
