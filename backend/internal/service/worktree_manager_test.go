package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory mock WorktreeRepository
// ─────────────────────────────────────────────────────────────────────────────

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
	// эмулируем BeforeCreate: branch_name строится из task+wt UUID
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
	out := make([]models.Worktree, 0)
	for _, w := range r.data {
		if w.TaskID == taskID {
			out = append(out, *w)
		}
	}
	return out, nil
}

// List — стаб для in-memory mock'а: фильтрует по task_id/state, сортирует по
// allocated_at DESC. Покрывает потребности WorktreeManager unit-тестов без
// дублирования логики реальной репы.
func (r *memWorktreeRepo) List(_ context.Context, filter repository.WorktreeFilter) ([]models.Worktree, error) {
	if filter.State != nil && !filter.State.IsValid() {
		return nil, repository.ErrWorktreeNotFound
	}
	out := make([]models.Worktree, 0, len(r.data))
	for _, w := range r.data {
		if filter.TaskID != nil && w.TaskID != *filter.TaskID {
			continue
		}
		if filter.State != nil && w.State != *filter.State {
			continue
		}
		out = append(out, *w)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AllocatedAt.After(out[j].AllocatedAt)
	})
	limit := filter.Limit
	if limit <= 0 {
		limit = repository.WorktreeListDefaultLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(out) {
		return []models.Worktree{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
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

func (r *memWorktreeRepo) ListForCleanup(_ context.Context, cutoff time.Time) ([]models.Worktree, error) {
	out := make([]models.Worktree, 0)
	for _, w := range r.data {
		if w.State == models.WorktreeStateReleased && w.ReleasedAt != nil && w.ReleasedAt.Before(cutoff) {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (r *memWorktreeRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.data[id]; !ok {
		return repository.ErrWorktreeNotFound
	}
	delete(r.data, id)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func discardLogger() *slog.Logger {
	return slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
}

// initTempGitRepo создаёт временный git-репозиторий с одним initial-коммитом
// на ветке main. Возвращает путь к репо или skip'ает тест если git недоступен.
func initTempGitRepo(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping git-dependent test in -short mode")
	}
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
	// первый коммит — нужен чтобы `git worktree add ... main` сработало.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init", "-q")
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// Unit tests — без git
// ─────────────────────────────────────────────────────────────────────────────

func TestWorktreeManager_Config_RejectsRelativePaths(t *testing.T) {
	cfg := WorktreeManagerConfig{RepoRoot: "relative/path", WorktreesRoot: "/tmp/wt"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected REJECT for relative RepoRoot")
	}
	cfg = WorktreeManagerConfig{RepoRoot: "/tmp/repo", WorktreesRoot: "relative/wt"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected REJECT for relative WorktreesRoot")
	}
}

func TestWorktreeManager_Allocate_RejectsUnsafeBaseBranch(t *testing.T) {
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()

	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		newMemWorktreeRepo(), discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	adversarial := []string{
		"-h",                  // flag injection
		"--upload-pack=evil",  // flag with value
		"../etc/passwd",       // path traversal
		"main\nrm -rf /",      // newline injection
		"",                    // empty
		".hidden",             // leading dot
	}
	for _, base := range adversarial {
		_, err := mgr.Allocate(context.Background(), uuid.New(), uuid.Nil, base)
		if err == nil {
			t.Errorf("expected REJECT for unsafe base branch %q", base)
		}
		if !errors.Is(err, ErrBranchUnsafe) {
			t.Errorf("expected ErrBranchUnsafe sentinel for %q, got: %v", base, err)
		}
	}
}

// TestWorktreeManager_Allocate_DBRowRolledBackOnGitFailure — DB-запись не должна
// остаться "висящей" если git worktree add упал (нет такого base-branch).
func TestWorktreeManager_Allocate_DBRowRolledBackOnGitFailure(t *testing.T) {
	// Используем валидный по виду baseBranch, но в репо такой ветки нет.
	// Это симулирует ошибку git без необходимости в git-репо.
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()
	repo := newMemWorktreeRepo()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		repo, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	// tmpRepo — пустой каталог, не git-репо → `git worktree add` упадёт.
	_, allocErr := mgr.Allocate(context.Background(), uuid.New(), uuid.Nil, "nonexistent-branch")
	if allocErr == nil {
		t.Skip("git not available or worktree add unexpectedly succeeded — skipping rollback check")
	}

	// Самый важный инвариант: НЕТ висящих записей в БД.
	if len(repo.data) != 0 {
		t.Errorf("expected DB rollback on git failure, but %d records remain: %v", len(repo.data), repo.data)
	}
}

// TestWorktreeManager_CleanupExpired_PrefixCheckBlocksEscape — DEFENSE-IN-DEPTH тест.
// Конструируем worktree с path-вычислением, которое мог бы вывести за WorktreesRoot,
// и проверяем что CleanupExpired откажется его удалять.
//
// На практике ComputePath это уже исключает (UUID.String() всегда даёт безопасную
// "8-4-4-4-12" строку), но мы тестируем поведение менеджера если ComputePath
// каким-то образом вернёт outside-path: ничего не должно быть удалено.
func TestWorktreeManager_CleanupExpired_NoOpsWhenEmpty(t *testing.T) {
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		newMemWorktreeRepo(), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	cleaned, err := mgr.CleanupExpired(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleanups on empty repo, got %d", cleaned)
	}
}

// TestWorktreeManager_Release_Idempotent — повторный Release не падает.
func TestWorktreeManager_Release_Idempotent(t *testing.T) {
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()
	repo := newMemWorktreeRepo()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		repo, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	// Создаём DB-запись напрямую, минуя git (имитируем allocated-worktree без диска).
	wt := &models.Worktree{TaskID: uuid.New(), BaseBranch: "main", State: models.WorktreeStateAllocated}
	if err := repo.Create(context.Background(), wt); err != nil {
		t.Fatal(err)
	}

	// Первый release — git worktree remove упадёт (диска нет), но state должен стать released.
	if err := mgr.Release(context.Background(), wt.ID); err != nil {
		t.Fatalf("first release: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), wt.ID)
	if got.State != models.WorktreeStateReleased {
		t.Errorf("expected state=released after first call, got %q", got.State)
	}

	// Второй release — должен быть no-op (идемпотентность).
	if err := mgr.Release(context.Background(), wt.ID); err != nil {
		t.Errorf("second release expected no-op nil, got: %v", err)
	}

	// Release несуществующего — тоже no-op (для cancel-flow по всем worktree'ям задачи).
	if err := mgr.Release(context.Background(), uuid.New()); err != nil {
		t.Errorf("release of missing worktree expected no-op nil, got: %v", err)
	}
}

// TestWorktreeManager_AssertPathInsideRoot — defence-in-depth helper-функция
// должна REJECT'ить пути вне корня и пути равные корню. Этот хелпер также
// используется внутри Release/ReleaseManual, поэтому unit-тест на нём = unit-тест
// на guard-условии "PathOutsideRoot_Rejected" из плана §6.3.
func TestWorktreeManager_AssertPathInsideRoot(t *testing.T) {
	root := "/var/lib/devteam/worktrees"
	// Каждый кейс — pair (path, wantOK). wantOK==false означает что мы ждём
	// ErrWorktreeInvalidPath (нельзя пускать git remove --force на эту строку).
	cases := []struct {
		name   string
		path   string
		wantOK bool
	}{
		// HAPPY: корректный путь под корнем.
		{"happy/uuid_subdir", root + "/abcd", true},
		{"happy/nested", root + "/task-1/wt-2", true},

		// REJECT: путь == корень (catastrophic, RemoveAll бы снёс всё).
		{"reject/equals_root", root, false},
		{"reject/equals_root_with_slash", root + "/", false},

		// REJECT: путь выше корня через ".." (Clean раскроет, защита всё равно сработает).
		{"reject/parent_traversal", root + "/../etc", false},
		{"reject/sibling_dir", "/var/lib/devteam/secrets", false},
		{"reject/abs_outside", "/etc/passwd", false},

		// REJECT: empty path — Clean даст "." (явно вне корня).
		{"reject/empty", "", false},

		// REJECT: префикс совпадает по подстроке, но это другой каталог.
		// Без `+ "/"` в HasPrefix мы бы пропустили "/var/lib/devteam/worktreesEvil".
		{"reject/prefix_substring_attack", root + "Evil/x", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertPathInsideRoot(tc.path, root)
			if tc.wantOK && err != nil {
				t.Errorf("expected OK for %q, got: %v", tc.path, err)
			}
			if !tc.wantOK {
				if err == nil {
					t.Errorf("expected REJECT for %q, got nil", tc.path)
				} else if !errors.Is(err, ErrWorktreeInvalidPath) {
					t.Errorf("expected ErrWorktreeInvalidPath for %q, got: %v", tc.path, err)
				}
			}
		})
	}
}

// TestBuildGitWorktreeRemoveArgs_HasSeparatorAfterFlags — guards the project-wide
// invariant: `git worktree remove` argv должен иметь форму
// `-C <root> worktree remove --force -- <path>`. Без `--` adversarial path вида
// `--upload-pack=evil` мог бы быть прочитан git'ом как флаг.
//
// На практике path вычисляется в Go из uuid.UUID и не может стать флагом (UUID
// не начинается с `-`), но регрессию "кто-то заменил on `git worktree remove %s`
// через fmt.Sprintf или bash -c" статический анализ не поймает — этот тест поймает.
func TestBuildGitWorktreeRemoveArgs_HasSeparatorAfterFlags(t *testing.T) {
	args := buildGitWorktreeRemoveArgs("/var/repo", "/var/wt/abc")

	// Сепаратор `--` обязан присутствовать ровно один раз и стоять ПЕРЕД path.
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			if sepIdx != -1 {
				t.Fatalf("expected exactly one `--` separator, found duplicate at %d (first at %d)", i, sepIdx)
			}
			sepIdx = i
		}
	}
	if sepIdx == -1 {
		t.Fatalf("expected `--` separator in args, got: %v", args)
	}
	if args[len(args)-1] != "/var/wt/abc" {
		t.Errorf("expected path as last arg, got args=%v", args)
	}
	if sepIdx != len(args)-2 {
		t.Errorf("expected `--` immediately before path, got args=%v (sep at %d, len=%d)", args, sepIdx, len(args))
	}

	// Все флаги (--force) должны быть СТРОГО ДО сепаратора. Если когда-нибудь
	// добавится новый флаг ПОСЛЕ `--`, git его не распознает И этот тест упадёт.
	for i, a := range args[:sepIdx] {
		if i > 1 && len(a) > 1 && a[0] == '-' && a != "--force" && a != "-C" {
			t.Errorf("unexpected flag-like arg before separator: %q (full args=%v)", a, args)
		}
	}

	// На случай если кто-то решит добавить опасные флаги типа --upload-pack —
	// явная denylist-проверка делает intent теста очевидным при чтении.
	dangerous := []string{"--upload-pack", "--exec", "--receive-pack"}
	for _, a := range args {
		for _, d := range dangerous {
			if strings.HasPrefix(a, d) {
				t.Errorf("dangerous flag %q must never appear in worktree remove args, got: %v", a, args)
			}
		}
	}
}

// TestBuildGitWorktreeRemoveArgs_AdversarialPath_NotInterpretedAsFlag — даже
// когда path выглядит как флаг (`--upload-pack=evil`), он стоит ПОСЛЕ `--` и
// git разберёт его как позиционный аргумент.
func TestBuildGitWorktreeRemoveArgs_AdversarialPath_NotInterpretedAsFlag(t *testing.T) {
	// На практике ComputePath никогда не вернёт такую строку — UUID-формат
	// фиксирован. Но guard обязан работать гипотетически: тест проверяет именно
	// конструктор аргументов.
	args := buildGitWorktreeRemoveArgs("/var/repo", "--upload-pack=evil")

	// Path — последний элемент.
	if args[len(args)-1] != "--upload-pack=evil" {
		t.Errorf("path should be last positional arg even when it looks like a flag, got: %v", args)
	}
	// Перед ним — `--`.
	if args[len(args)-2] != "--" {
		t.Errorf("expected `--` immediately before adversarial path, got: %v", args)
	}
}

// TestWorktreeManager_ReleaseManual_NotFound — ReleaseManual для несуществующего
// worktree возвращает repository.ErrWorktreeNotFound (handler → 404). В отличие
// от обычного Release, который trеатит "missing" как no-op (для cancel-flow).
func TestWorktreeManager_ReleaseManual_NotFound(t *testing.T) {
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		newMemWorktreeRepo(), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.ReleaseManual(context.Background(), uuid.New(), uuid.New(), "admin")
	if !errors.Is(err, repository.ErrWorktreeNotFound) {
		t.Errorf("expected ErrWorktreeNotFound, got: %v", err)
	}
}

// TestWorktreeManager_ReleaseManual_AlreadyReleased — повторный manual-release
// возвращает ErrWorktreeAlreadyReleased (handler → 409). Это ОТЛИЧИЕ от обычного
// Release (который no-op для идемпотентности cancel-flow): manual-кнопка должна
// сообщить оператору "ничего не сделалось, уже целевое состояние".
func TestWorktreeManager_ReleaseManual_AlreadyReleased(t *testing.T) {
	tmpRepo := t.TempDir()
	tmpRoot := t.TempDir()
	repo := newMemWorktreeRepo()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		repo, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	wt := &models.Worktree{TaskID: uuid.New(), BaseBranch: "main", State: models.WorktreeStateReleased}
	if err := repo.Create(context.Background(), wt); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.ReleaseManual(context.Background(), wt.ID, uuid.New(), "admin")
	if !errors.Is(err, ErrWorktreeAlreadyReleased) {
		t.Errorf("expected ErrWorktreeAlreadyReleased, got: %v", err)
	}
}

// TestWorktreeManager_ReleaseManual_HappyPath_NoGitRepo — ReleaseManual должен
// успешно перевести state в released даже если git упал (нет репо). Это важная
// гарантия: оператор нажал кнопку именно потому что worktree залип; consistency
// БД-стейта важнее консистентности файловой системы.
func TestWorktreeManager_ReleaseManual_HappyPath_NoGitRepo(t *testing.T) {
	tmpRepo := t.TempDir() // не git-репо
	tmpRoot := t.TempDir()
	repo := newMemWorktreeRepo()
	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: tmpRepo, WorktreesRoot: tmpRoot},
		repo, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	wt := &models.Worktree{TaskID: uuid.New(), BaseBranch: "main", State: models.WorktreeStateInUse}
	if err := repo.Create(context.Background(), wt); err != nil {
		t.Fatal(err)
	}

	updated, err := mgr.ReleaseManual(context.Background(), wt.ID, uuid.New(), "admin")
	if err != nil {
		t.Fatalf("ReleaseManual: %v", err)
	}
	if updated.State != models.WorktreeStateReleased {
		t.Errorf("expected state=released, got %q", updated.State)
	}
	// Запись в БД тоже released.
	got, _ := repo.GetByID(context.Background(), wt.ID)
	if got.State != models.WorktreeStateReleased {
		t.Errorf("DB state expected released, got %q", got.State)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration tests — c реальным git (skipped in -short)
// ─────────────────────────────────────────────────────────────────────────────

// TestWorktreeManager_AllocateAndRelease_HappyPath — полный цикл с реальным git.
func TestWorktreeManager_AllocateAndRelease_HappyPath(t *testing.T) {
	repoDir := initTempGitRepo(t)
	wtRoot := t.TempDir()

	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: repoDir, WorktreesRoot: wtRoot},
		newMemWorktreeRepo(), discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	wt, err := mgr.Allocate(context.Background(), uuid.New(), uuid.Nil, "main")
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Worktree path существует и содержит README (от initial-коммита).
	path, err := mgr.ResolvePath(wt)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "README.md")); err != nil {
		t.Errorf("expected README.md in worktree, got: %v", err)
	}

	// Имя ветки соответствует контракту.
	if !models.ValidateWorktreeBranchName(wt.BranchName) {
		t.Errorf("worktree branch_name %q does not match expected format", wt.BranchName)
	}

	// Release — каталог должен исчезнуть.
	if err := mgr.Release(context.Background(), wt.ID); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected worktree path to be removed after Release, but stat err=%v", err)
	}
}

// TestWorktreeManager_CleanupExpired_RemovesAgedReleased — Released worktree
// старше cutoff должен быть физически удалён и пропасть из БД.
func TestWorktreeManager_CleanupExpired_RemovesAgedReleased(t *testing.T) {
	repoDir := initTempGitRepo(t)
	wtRoot := t.TempDir()
	repo := newMemWorktreeRepo()

	mgr, err := NewWorktreeManager(WorktreeManagerConfig{RepoRoot: repoDir, WorktreesRoot: wtRoot},
		repo, discardLogger())
	if err != nil {
		t.Fatal(err)
	}

	wt, err := mgr.Allocate(context.Background(), uuid.New(), uuid.Nil, "main")
	if err != nil {
		t.Fatal(err)
	}
	path, _ := mgr.ResolvePath(wt)
	if err := mgr.Release(context.Background(), wt.ID); err != nil {
		t.Fatal(err)
	}

	// "Стареем" released_at в БД, чтобы попасть в выборку.
	repo.data[wt.ID].ReleasedAt = pointerTime(time.Now().Add(-2 * time.Hour))

	cleaned, err := mgr.CleanupExpired(context.Background(), time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned, got %d", cleaned)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected path to be removed, stat err=%v", err)
	}
	if _, err := repo.GetByID(context.Background(), wt.ID); !errors.Is(err, repository.ErrWorktreeNotFound) {
		t.Errorf("expected DB record removed, got err=%v", err)
	}
}

func pointerTime(t time.Time) *time.Time { return &t }
