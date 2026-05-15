package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// worktree_manager.go — Sprint 17 / Orchestration v2 — изоляция параллельных
// sandbox-агентов через `git worktree`.
//
// Контракт безопасности (продублирован из docs/orchestration-v2-plan.md §2.9):
//   1. ВСЕ git-команды используют `--` separator перед user/LLM-controlled args.
//      Это project-wide convention (см. internal/agent/execution_types.go godoc).
//   2. Путь к worktree ВСЕГДА вычисляется в Go из типизированных uuid.UUID
//      (Worktree.ComputePath). НИКОГДА не читается из БД-строки — колонка `path`
//      намеренно отсутствует в таблице worktrees.
//   3. branch_name форсится backend'ом по шаблону "task-<task_uuid>-wt-<wt_uuid>"
//      (Worktree.BeforeCreate). LLM/пользователь его не задают.
//   4. base_branch валидируется через ValidateBaseBranch (regex + отказ при
//     ведущем '-' или '.' — защита от git flag injection).
//   5. CleanupExpired перед os.RemoveAll делает filepath.Clean + prefix-check,
//      чтобы даже при ошибке в коде путь не вышел за WorktreesRoot.
//
// Политика логирования (Sprint 17 / 6.3 review):
//   - В Kibana и observability НЕ светим внутренний нейминг: ни branch_name
//     (детерминированно вычислимый из task_id+worktree_id), ни абсолютный
//     путь к каталогу worktree. По task_id + worktree_id оператор всегда может
//     восстановить branch_name локально через MakeWorktreeBranchName.
//   - Это политика для ВСЕХ логов в этом файле, не только для manual-audit:
//     при инцидент-разборе достаточно ID-пары; экспортировать имя ветки в
//     centralized logging означает раскрыть internal scheme'у в Kibana, где
//     к ней получают доступ роли с broader-чем-БД scope'ом.

// WorktreeManagerConfig — конфигурация менеджера.
type WorktreeManagerConfig struct {
	// RepoRoot — корень основного git-репозитория, в котором выполняются
	// `git worktree add` / `git worktree remove`. Должен существовать
	// и быть рабочей копией git (содержать .git или быть worktree сам).
	RepoRoot string

	// WorktreesRoot — каталог под все worktree'ы. Создаётся если не существует.
	// Все вычисленные пути worktree'ев — потомки этого каталога; defence-in-depth
	// проверки убедятся в этом перед любым os.RemoveAll.
	WorktreesRoot string
}

// Validate проверяет конфигурацию.
func (c WorktreeManagerConfig) Validate() error {
	if c.RepoRoot == "" {
		return fmt.Errorf("worktree manager: RepoRoot is required")
	}
	if c.WorktreesRoot == "" {
		return fmt.Errorf("worktree manager: WorktreesRoot is required")
	}
	if !filepath.IsAbs(c.RepoRoot) {
		return fmt.Errorf("worktree manager: RepoRoot must be absolute path, got %q", c.RepoRoot)
	}
	if !filepath.IsAbs(c.WorktreesRoot) {
		return fmt.Errorf("worktree manager: WorktreesRoot must be absolute path, got %q", c.WorktreesRoot)
	}
	if _, err := os.Stat(c.RepoRoot); err != nil {
		return fmt.Errorf("worktree manager: RepoRoot inaccessible: %w", err)
	}
	return nil
}

// ErrWorktreeStateConflict — попытка release/markInUse worktree в неподходящем стейте.
var ErrWorktreeStateConflict = errors.New("worktree in unexpected state")

// ErrWorktreeAlreadyReleased — manual-release вызван для worktree который уже released.
// В отличие от Release (идемпотентен для cancel-flow), ReleaseManual возвращает эту
// ошибку, чтобы handler ответил 409 — оператору важно знать что кнопка не сработала
// потому что состояние уже целевое, а не из-за бага.
var ErrWorktreeAlreadyReleased = errors.New("worktree already released")

// ErrWorktreeInvalidPath — defence-in-depth: вычисленный путь к worktree вышел за
// WorktreesRoot. На практике невозможно (ComputePath строится от типизированных UUID),
// но если когда-нибудь будет — `git worktree remove --force --` под uid backend'а
// смог бы снести произвольные файлы. Лучше упасть раньше.
var ErrWorktreeInvalidPath = errors.New("worktree path outside root")

// WorktreeManager — основной API изоляции git worktree'ев.
type WorktreeManager struct {
	cfg    WorktreeManagerConfig
	repo   repository.WorktreeRepository
	logger *slog.Logger
}

// NewWorktreeManager — конструктор. Валидирует конфиг и создаёт WorktreesRoot если нужно.
// logger должен быть с redact-обёрткой (logging.NewHandler).
func NewWorktreeManager(cfg WorktreeManagerConfig, repo repository.WorktreeRepository, logger *slog.Logger) (*WorktreeManager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.WorktreesRoot, 0o755); err != nil {
		return nil, fmt.Errorf("worktree manager: create WorktreesRoot: %w", err)
	}
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &WorktreeManager{cfg: cfg, repo: repo, logger: logger}, nil
}

// Allocate создаёт новый git worktree для подзадачи и сохраняет запись в БД.
//
// Шаги:
//  1. Валидация baseBranch (защита от git flag injection).
//  2. Создание DB-записи (state=allocated). BeforeCreate генерирует ID и branch_name.
//  3. Вычисление пути через Worktree.ComputePath (типизированные UUID).
//  4. `git -C <repoRoot> worktree add <path> -b <branch_name> -- <baseBranch>` — обязательный `--`.
//  5. Если git упал — откат DB-записи через repo.Delete, чтобы не было «потерянных» строк.
//
// subtaskID — опциональный (uuid.Nil допустимо), используется только для трассировки.
func (m *WorktreeManager) Allocate(ctx context.Context, taskID, subtaskID uuid.UUID, baseBranch string) (*models.Worktree, error) {
	if taskID == uuid.Nil {
		return nil, fmt.Errorf("worktree manager: taskID is required")
	}
	if err := ValidateBaseBranch(baseBranch); err != nil {
		// Не логируем baseBranch — содержимое может быть adversarial.
		m.logger.WarnContext(ctx, "worktree allocate rejected: unsafe base branch",
			"task_id", taskID, "error", err.Error())
		return nil, fmt.Errorf("worktree allocate: %w", err)
	}

	wt := &models.Worktree{
		TaskID:     taskID,
		BaseBranch: baseBranch,
		State:      models.WorktreeStateAllocated,
	}
	if subtaskID != uuid.Nil {
		wt.SubtaskID = &subtaskID
	}

	if err := m.repo.Create(ctx, wt); err != nil {
		return nil, fmt.Errorf("worktree allocate: create db record: %w", err)
	}

	path, err := wt.ComputePath(m.cfg.WorktreesRoot)
	if err != nil {
		_ = m.repo.Delete(ctx, wt.ID) // best-effort rollback
		return nil, fmt.Errorf("worktree allocate: compute path: %w", err)
	}

	// `--` separator обязателен: baseBranch — пользовательский ввод, иначе git мог бы
	// интерпретировать "-h" или "--upload-pack=..." как флаг. Также path фиксированный
	// (мы только что его вычислили), но для единообразия `--` ставим именно перед base.
	cmd := exec.CommandContext(ctx, "git", "-C", m.cfg.RepoRoot,
		"worktree", "add", path, "-b", wt.BranchName, "--", baseBranch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Откат DB-записи — оставлять "висящую" allocated-запись без файлов не хочется.
		// Это всё равно best-effort: если Delete упадёт, retention cron приберёт.
		_ = m.repo.Delete(ctx, wt.ID)
		m.logger.ErrorContext(ctx, "git worktree add failed",
			"task_id", taskID, "worktree_id", wt.ID, "error", err.Error(),
			// stdout/stderr git'а — обычно безопасно для логов (имена веток, ошибки FS),
			// но усекаем на всякий случай.
			"git_output", truncate(string(out), 1024))
		return nil, fmt.Errorf("worktree allocate: git worktree add: %w (output: %s)", err, truncate(string(out), 256))
	}

	// branch_name НЕ логируем (см. "Политика логирования" в godoc файла).
	// base_branch — оставляем: это вход от пользователя/LLM, нужен для дебага
	// "почему агент пошёл от не той ветки".
	m.logger.InfoContext(ctx, "worktree allocated",
		"task_id", taskID, "worktree_id", wt.ID, "base", baseBranch)
	return wt, nil
}

// MarkInUse переводит worktree allocated → in_use и привязывает agent_job_id.
// Вызывается AgentWorker'ом при старте работы (после claim'а task_event).
func (m *WorktreeManager) MarkInUse(ctx context.Context, worktreeID uuid.UUID, agentJobID int64) error {
	if err := m.repo.MarkInUse(ctx, worktreeID, agentJobID); err != nil {
		return fmt.Errorf("worktree mark in_use: %w", err)
	}
	return nil
}

// Release выполняет `git worktree remove --force -- <path>` и помечает запись released.
//
// `--force` нужен потому что Developer-агент мог оставить uncommitted changes
// (нормальное поведение: его diff потом подберёт другой агент). git worktree remove
// без --force откажет в этом случае.
//
// Идемпотентность: если worktree уже released — возвращает nil. Это упрощает
// cancel-флоу (Step может позвать Release для всех worktree'ев задачи без проверки).
func (m *WorktreeManager) Release(ctx context.Context, worktreeID uuid.UUID) error {
	wt, err := m.repo.GetByID(ctx, worktreeID)
	if err != nil {
		if errors.Is(err, repository.ErrWorktreeNotFound) {
			return nil // уже исчез — ничего не делаем
		}
		return fmt.Errorf("worktree release: load: %w", err)
	}
	if wt.State == models.WorktreeStateReleased {
		return nil // идемпотентность
	}
	return m.removeAndMarkReleased(ctx, wt)
}

// ReleaseManual — вариант Release для ручного "unstick" из admin UI (Sprint 17 / 6.3).
//
// Отличия от Release:
//   - НЕ идемпотентен: повторный вызов на released worktree возвращает
//     ErrWorktreeAlreadyReleased (handler отвечает 409). Operator должен знать что
//     кнопка не сработала именно из-за уже-целевого состояния, а не из-за бага.
//   - Возвращает repository.ErrWorktreeNotFound если worktree не существует
//     (handler → 404), а не nil как Release.
//   - Audit-log включает userID/userRole триггера. path и branch_name НЕ логируются
//     (внутренний нейминг, в Kibana не нужно).
//
// Безопасность (см. §6.3 docs/orchestration-v2-plan.md): команда строится
// исключительно через массив exec.CommandContext + `--` separator; путь вычисляется
// из типизированных uuid.UUID в Go (НЕ читается из БД-строки) и проверяется на
// принадлежность WorktreesRoot до запуска git.
func (m *WorktreeManager) ReleaseManual(ctx context.Context, worktreeID uuid.UUID, userID uuid.UUID, userRole string) (*models.Worktree, error) {
	wt, err := m.repo.GetByID(ctx, worktreeID)
	if err != nil {
		if errors.Is(err, repository.ErrWorktreeNotFound) {
			return nil, err // 404 на handler-слое
		}
		return nil, fmt.Errorf("worktree release_manual: load: %w", err)
	}
	if wt.State == models.WorktreeStateReleased {
		return nil, ErrWorktreeAlreadyReleased
	}

	if err := m.removeAndMarkReleased(ctx, wt); err != nil {
		return nil, err
	}

	// Audit-log. Нужен в Kibana для разбора инцидентов "почему worktree исчез
	// посреди работы" — кто и когда нажал кнопку. Никаких PII/секретов.
	m.logger.InfoContext(ctx, "worktree manually released",
		"worktree_id", wt.ID,
		"task_id", wt.TaskID,
		"user_id", userID,
		"user_role", userRole,
	)

	// Re-fetch чтобы вернуть released_at, который выставила репа в UpdateState.
	// Если по какой-то причине запись пропала между UpdateState и Get — отдаём
	// in-memory копию с обновлённым state'ом (best-effort, не падаем).
	updated, err := m.repo.GetByID(ctx, worktreeID)
	if err != nil {
		wt.State = models.WorktreeStateReleased
		return wt, nil
	}
	return updated, nil
}

// removeAndMarkReleased — общая часть Release / ReleaseManual: defence-in-depth path
// validation → `git worktree remove --force --` → UpdateState(released).
//
// Все callers полагаются на то, что метод НЕ возвращает ошибку если git упал
// (worktree-каталог мог быть удалён вручную, ветка пропасть и т.п.) — state=released
// важнее, retention cron подберёт остатки. Ошибки возвращаются только для
// (a) UpdateState и (b) явного ErrWorktreeInvalidPath (defence-in-depth).
func (m *WorktreeManager) removeAndMarkReleased(ctx context.Context, wt *models.Worktree) error {
	path, err := wt.ComputePath(m.cfg.WorktreesRoot)
	if err != nil {
		return fmt.Errorf("worktree release: compute path: %w", err)
	}
	if err := assertPathInsideRoot(path, m.cfg.WorktreesRoot); err != nil {
		// CRITICAL: не запускаем git если путь вышел за корень. На практике невозможно
		// (ComputePath из uuid.UUID), но если когда-нибудь будет — git worktree remove
		// --force --  под uid backend'а снесёт реальные файлы.
		m.logger.ErrorContext(ctx, "worktree release: REFUSING git remove (path outside root)",
			"worktree_id", wt.ID, "error", err.Error())
		return err
	}

	// args строятся через buildGitWorktreeRemoveArgs (отдельный helper, чтобы
	// инвариант "флаги перед --, путь после --" был unit-тестируемым: см.
	// TestBuildGitWorktreeRemoveArgs_HasSeparatorAfterFlags).
	args := buildGitWorktreeRemoveArgs(m.cfg.RepoRoot, path)
	cmd := exec.CommandContext(ctx, "git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Не критичная ошибка — мог уже быть удалён вручную, или ветки нет. Логируем
		// и продолжаем: state=released важнее консистентности файловой системы
		// (cleanup-cron всё равно подберёт остатки).
		m.logger.WarnContext(ctx, "git worktree remove failed (continuing to mark released)",
			"worktree_id", wt.ID, "error", err.Error(),
			"git_output", truncate(string(out), 1024))
	}

	if err := m.repo.UpdateState(ctx, wt.ID, models.WorktreeStateReleased); err != nil {
		return fmt.Errorf("worktree release: update state: %w", err)
	}

	// branch_name НЕ логируем (см. "Политика логирования" в godoc файла).
	m.logger.InfoContext(ctx, "worktree released", "worktree_id", wt.ID, "task_id", wt.TaskID)
	return nil
}

// buildGitWorktreeRemoveArgs — строит argv для `git worktree remove`. Вынесено в
// helper исключительно ради unit-тестирования инварианта "argv-форма + `--`":
// регрессию вида "кто-то поменял на bash -c / fmt.Sprintf, потерял `--`,
// добавил --upload-pack" статический анализ не поймает, а тест на форму args —
// поймает. Helper не зависит от состояния менеджера.
//
// Контракт: возвращаемый срез ВСЕГДА содержит `--` ровно один раз, ПЕРЕД path.
// path всегда последний элемент. Все флаги (--force) — ДО `--`. Если изменишь —
// сломай тест намеренно, не молча.
func buildGitWorktreeRemoveArgs(repoRoot, path string) []string {
	return []string{
		"-C", repoRoot,
		"worktree", "remove",
		"--force",
		"--",
		path,
	}
}

// assertPathInsideRoot — defence-in-depth проверка: filepath.Clean(path) должен
// быть строго ВНУТРИ root (не равен корню, не выше). Возвращает ErrWorktreeInvalidPath
// если что-то не так. Используется в Release/ReleaseManual и CleanupExpired.
//
// Ровно та же логика, что в CleanupExpired (внутри файла) — выделена в helper, чтобы
// (a) не дублировать защиту в двух местах, (b) можно было unit-тестировать на
// adversarial-входах напрямую.
func assertPathInsideRoot(path, root string) error {
	clean := filepath.Clean(path)
	rootClean := filepath.Clean(root)
	rootPrefix := rootClean + string(filepath.Separator)
	if clean == rootClean {
		return fmt.Errorf("%w: path equals root %q", ErrWorktreeInvalidPath, rootClean)
	}
	if !strings.HasPrefix(clean+string(filepath.Separator), rootPrefix) {
		return fmt.Errorf("%w: %q not inside %q", ErrWorktreeInvalidPath, clean, rootClean)
	}
	return nil
}

// CleanupExpired физически удаляет released-worktree'ы старше cutoff:
// сначала os.RemoveAll каталога (с prefix-check), затем repo.Delete.
//
// Возвращает количество фактически удалённых записей.
//
// Вызывается cron'ом раз в час (см. cron-job в Sprint 4). При сбое os.RemoveAll
// записываем log и продолжаем со следующей — не блокируем cleanup всей пачки.
func (m *WorktreeManager) CleanupExpired(ctx context.Context, cutoff time.Time) (int, error) {
	expired, err := m.repo.ListForCleanup(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("worktree cleanup: list expired: %w", err)
	}

	cleaned := 0
	for i := range expired {
		wt := &expired[i]

		path, err := wt.ComputePath(m.cfg.WorktreesRoot)
		if err != nil {
			m.logger.ErrorContext(ctx, "worktree cleanup: compute path",
				"worktree_id", wt.ID, "error", err.Error())
			continue
		}
		clean := filepath.Clean(path)

		// Defence-in-depth: REFUSE если путь равен корню или вышел за него.
		// Без этой проверки повреждённые UUID, схлопывающие filepath.Join к корню,
		// привели бы к RemoveAll(WorktreesRoot) — catastrophic data loss всех
		// worktree'ев всех задач.
		if err := assertPathInsideRoot(clean, m.cfg.WorktreesRoot); err != nil {
			m.logger.ErrorContext(ctx, "worktree cleanup: REFUSING to remove unsafe path",
				"worktree_id", wt.ID, "path", clean, "error", err.Error())
			continue
		}

		if err := os.RemoveAll(clean); err != nil {
			m.logger.ErrorContext(ctx, "worktree cleanup: RemoveAll failed",
				"worktree_id", wt.ID, "path", clean, "error", err.Error())
			continue
		}
		if err := m.repo.Delete(ctx, wt.ID); err != nil {
			m.logger.ErrorContext(ctx, "worktree cleanup: db delete failed",
				"worktree_id", wt.ID, "error", err.Error())
			continue
		}
		cleaned++
	}
	if cleaned > 0 {
		m.logger.InfoContext(ctx, "worktree cleanup completed", "removed", cleaned, "considered", len(expired))
	}
	return cleaned, nil
}

// ListByTaskID — pass-through для observability/debug-эндпоинта.
func (m *WorktreeManager) ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]models.Worktree, error) {
	return m.repo.ListByTaskID(ctx, taskID)
}

// ResolvePath — единственная функция для получения пути к worktree снаружи манагера.
// Используется AgentWorker'ом для bind-mount в sandbox.
func (m *WorktreeManager) ResolvePath(wt *models.Worktree) (string, error) {
	return wt.ComputePath(m.cfg.WorktreesRoot)
}

// truncate возвращает первые n РУН строки + маркер усечения. Rune-safe для логов
// (cyrillic в git-output, эмодзи в .gitmessage'ах и т.д.) — байтовый срез мог бы
// поломать UTF-8 на середине символа и сломать downstream-парсеры логов.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "...(truncated)"
}
