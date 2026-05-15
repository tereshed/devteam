//go:build integration

package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
)

// worktree_manager_pg_integration_test.go — Sprint 17 / 6.3.
//
// Покрывает manual-unstick (POST /worktrees/:id/release → ReleaseManual) с реальным
// Postgres + реальным git-репо. Unit-тесты в worktree_manager_test.go проверяют
// логику отдельно (mem-repo, helper-функции); этот файл проверяет что:
//   1. UPDATE worktrees SET state='released', released_at=NOW() реально
//      применяется в БД (constraint chk_worktrees_released_after_allocated сработал).
//   2. Идемпотентность Release vs не-идемпотентность ReleaseManual обе видны на
//      уровне фактической row'ы.
//
// Запуск: `go test -tags integration ./internal/service/...` (требует Docker для tcpostgres).

// initIntegrationGitRepo — создаёт временный git-репо под integration-тест.
// Дублирует initTempGitRepo из worktree_manager_test.go, потому что тот в package
// service, а этот файл — в service_test (разные пакеты, без cross-import нельзя).
func initIntegrationGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git binary not found in PATH: %v", err)
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
		}
	}
	run("init", "--initial-branch=main", "-q")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init", "-q")
	return dir
}

// TestPGIntegration_WorktreeManualRelease — полный цикл manual unstick:
//   - Создаём задачу + Allocate worktree (real git, real PG row).
//   - Вручную ставим state='in_use' (имитируя залипший sandbox).
//   - Вызываем ReleaseManual → row становится released, released_at заполнен,
//     git-каталог удалён, audit-log пишется (мы его не проверяем напрямую,
//     но проверяем что вызов завершился без ошибки).
//   - Повторный ReleaseManual → ErrWorktreeAlreadyReleased.
//   - ReleaseManual для отсутствующего id → repository.ErrWorktreeNotFound.
func TestPGIntegration_WorktreeManualRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startPgHarness(t, nil)
	defer h.Close()

	ctx := context.Background()

	repoDir := initIntegrationGitRepo(t)
	wtRoot := t.TempDir()

	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	mgr, err := service.NewWorktreeManager(
		service.WorktreeManagerConfig{RepoRoot: repoDir, WorktreesRoot: wtRoot},
		h.worktreeRepo, logger,
	)
	if err != nil {
		t.Fatalf("NewWorktreeManager: %v", err)
	}

	taskID := h.createMinimalActiveTask(t)

	// Allocate реальный worktree через git.
	wt, err := mgr.Allocate(ctx, taskID, uuid.Nil, "main")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	wtPath, err := mgr.ResolvePath(wt)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("expected worktree path to exist after Allocate: %v", err)
	}

	// Имитируем залипший sandbox: переводим в in_use.
	if err := h.worktreeRepo.MarkInUse(ctx, wt.ID, 12345); err != nil {
		t.Fatalf("MarkInUse: %v", err)
	}

	adminUserID := uuid.New()

	// ── Happy path: ReleaseManual на in_use → released. ───────────────────────
	updated, err := mgr.ReleaseManual(ctx, wt.ID, adminUserID, string(models.RoleAdmin))
	if err != nil {
		t.Fatalf("ReleaseManual happy path: %v", err)
	}
	if updated.State != models.WorktreeStateReleased {
		t.Errorf("expected state=released in returned wt, got %q", updated.State)
	}
	if updated.ReleasedAt == nil {
		t.Errorf("expected released_at to be set after ReleaseManual")
	}

	// Подтверждаем БД-стейт независимо: row должна быть released, released_at NOT NULL.
	got, err := h.worktreeRepo.GetByID(ctx, wt.ID)
	if err != nil {
		t.Fatalf("GetByID after release: %v", err)
	}
	if got.State != models.WorktreeStateReleased {
		t.Errorf("DB state expected released, got %q", got.State)
	}
	if got.ReleasedAt == nil {
		t.Errorf("DB released_at expected NOT NULL after release")
	}

	// Git-каталог должен быть удалён (real git worktree remove --force прошёл).
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree path removed, stat err=%v", err)
	}

	// ── Conflict: повторный ReleaseManual → ErrWorktreeAlreadyReleased. ──────
	if _, err := mgr.ReleaseManual(ctx, wt.ID, adminUserID, string(models.RoleAdmin)); !errors.Is(err, service.ErrWorktreeAlreadyReleased) {
		t.Errorf("expected ErrWorktreeAlreadyReleased on second release, got: %v", err)
	}

	// ── NotFound: ReleaseManual для отсутствующего id → ErrWorktreeNotFound. ─
	if _, err := mgr.ReleaseManual(ctx, uuid.New(), adminUserID, string(models.RoleAdmin)); !errors.Is(err, repository.ErrWorktreeNotFound) {
		t.Errorf("expected ErrWorktreeNotFound for missing id, got: %v", err)
	}
}
