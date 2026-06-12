package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/repository"
)

// retention.go — Sprint 17 / Orchestration v2 — фоновая очистка устаревших записей.
//
// Два независимых цикла (план §2.5, §2.9):
//   1. router_decisions старше 30 дней (compliance + disk usage).
//   2. worktrees: физически удалить с диска released worktree'ы старше 1 суток.
//
// Дизайн:
//   - Функции RunOnce* — синхронные шаги, возвращают результат + error.
//     Полезны для CLI-команд и интеграционных тестов.
//   - Метод Run — длинный цикл с тикером, удобен для подключения как goroutine
//     в cmd/api/main.go.

// RetentionConfig — настройки кронов.
type RetentionConfig struct {
	// RouterDecisionsAge — записи router_decisions старше этого возраста удаляются.
	// Default 30 дней (план §2.5).
	RouterDecisionsAge time.Duration

	// WorktreesReleasedAge — released-worktree'ы старше этого возраста физически
	// удаляются с диска. Default 24 часа (план §2.9).
	WorktreesReleasedAge time.Duration

	// TaskEventsStuckAge — зависшие блокировки событий задач старше этого возраста освобождаются.
	// Default 15 минут.
	TaskEventsStuckAge time.Duration

	// IndexingStuckAge — осиротевшие status='indexing' (процесс умер посреди индексации)
	// старше этого возраста сбрасываются в 'indexing_failed'. Default 30 минут —
	// двойной indexingTimeout: живой пайплайн сам завершает CAS не позже таймаута,
	// так что более старый indexing гарантированно ничейный.
	IndexingStuckAge time.Duration

	// Interval — частота прогона полного цикла. Default 1 час.
	Interval time.Duration
}

// DefaultRetentionConfig — дефолты согласно плану.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RouterDecisionsAge:   30 * 24 * time.Hour,
		WorktreesReleasedAge: 24 * time.Hour,
		TaskEventsStuckAge:   15 * time.Minute,
		IndexingStuckAge:     2 * indexingTimeout,
		Interval:             1 * time.Hour,
	}
}

// StuckIndexingReleaser — сброс осиротевших status='indexing' → 'indexing_failed'.
// Реализуют repository.ProjectRepository и repository.ProjectRepoRepository.
type StuckIndexingReleaser interface {
	ReleaseStuckIndexing(ctx context.Context, cutoff time.Time) (int64, error)
}

// RetentionService — координатор фоновых retention-операций.
type RetentionService struct {
	decisionRepo repository.RouterDecisionRepository
	eventRepo    repository.TaskEventRepository
	worktreeMgr  *WorktreeManager // опционально; если nil — worktree cleanup пропускается
	// projectStuck/repoStuck — recovery осиротевшего status='indexing' на уровне
	// проектов и их репозиториев; опциональны (nil — шаг пропускается).
	projectStuck StuckIndexingReleaser
	repoStuck    StuckIndexingReleaser
	logger       *slog.Logger
	cfg          RetentionConfig
}

// NewRetentionService — конструктор. WorktreeManager, projectStuck и repoStuck могут
// быть nil (например, для процесса который занимается ТОЛЬКО router_decisions retention).
func NewRetentionService(
	decisionRepo repository.RouterDecisionRepository,
	eventRepo repository.TaskEventRepository,
	worktreeMgr *WorktreeManager,
	projectStuck StuckIndexingReleaser,
	repoStuck StuckIndexingReleaser,
	logger *slog.Logger,
	cfg RetentionConfig,
) *RetentionService {
	if logger == nil {
		logger = logging.NopLogger()
	}
	if cfg.RouterDecisionsAge <= 0 {
		cfg.RouterDecisionsAge = 30 * 24 * time.Hour
	}
	if cfg.WorktreesReleasedAge <= 0 {
		cfg.WorktreesReleasedAge = 24 * time.Hour
	}
	if cfg.TaskEventsStuckAge <= 0 {
		cfg.TaskEventsStuckAge = 15 * time.Minute
	}
	if cfg.IndexingStuckAge <= 0 {
		cfg.IndexingStuckAge = 2 * indexingTimeout
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 1 * time.Hour
	}
	return &RetentionService{
		decisionRepo: decisionRepo, eventRepo: eventRepo, worktreeMgr: worktreeMgr,
		projectStuck: projectStuck, repoStuck: repoStuck,
		logger: logger, cfg: cfg,
	}
}

// RunOnceRouterDecisions — синхронный прогон очистки router_decisions.
// Возвращает количество удалённых записей.
func (s *RetentionService) RunOnceRouterDecisions(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-s.cfg.RouterDecisionsAge)
	n, err := s.decisionRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("router_decisions retention: %w", err)
	}
	if n > 0 {
		s.logger.InfoContext(ctx, "router_decisions retention cleanup completed",
			"removed", n, "older_than", cutoff)
	}
	return n, nil
}

// RunOnceStuckLocks — синхронный прогон освобождения зависших task_events.
// Возвращает количество освобождённых locks.
func (s *RetentionService) RunOnceStuckLocks(ctx context.Context) (int64, error) {
	if s.eventRepo == nil {
		return 0, nil
	}
	cutoff := time.Now().Add(-s.cfg.TaskEventsStuckAge)
	n, err := s.eventRepo.ReleaseStuckLocks(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("task_events stuck locks retention: %w", err)
	}
	if n > 0 {
		s.logger.InfoContext(ctx, "task_events stuck locks released",
			"released_count", n, "older_than", cutoff)
	}
	return n, nil
}

// RunOnceStuckIndexing — синхронный сброс осиротевших status='indexing' →
// 'indexing_failed' (процесс умер посреди индексации, финальный CAS не выполнился).
// Без этого зависший проект невидим: RunBackgroundReindexing его молча скипает,
// а ручной Reindex отвечает 409. Возвращает суммарное число освобождённых строк
// (projects + project_repositories).
func (s *RetentionService) RunOnceStuckIndexing(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-s.cfg.IndexingStuckAge)
	var total int64
	var firstErr error
	for _, t := range []struct {
		name     string
		releaser StuckIndexingReleaser
	}{
		{"projects", s.projectStuck},
		{"project_repositories", s.repoStuck},
	} {
		if t.releaser == nil {
			continue
		}
		n, err := t.releaser.ReleaseStuckIndexing(ctx, cutoff)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s stuck indexing retention: %w", t.name, err)
			}
			continue
		}
		if n > 0 {
			s.logger.WarnContext(ctx, "stuck indexing released",
				"table", t.name, "released_count", n, "older_than", cutoff)
		}
		total += n
	}
	return total, firstErr
}

// RunOnceWorktrees — синхронный прогон очистки worktrees. No-op если worktreeMgr=nil.
// Возвращает количество физически удалённых worktree'ев.
func (s *RetentionService) RunOnceWorktrees(ctx context.Context) (int, error) {
	if s.worktreeMgr == nil {
		return 0, nil
	}
	cutoff := time.Now().Add(-s.cfg.WorktreesReleasedAge)
	n, err := s.worktreeMgr.CleanupExpired(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("worktrees retention: %w", err)
	}
	return n, nil
}

// RunOnce — один полный цикл (decision + stuck locks + worktrees). Удобно для CLI и тестов.
// При ошибке одной из операций продолжает вторую и возвращает aggregate error.
func (s *RetentionService) RunOnce(ctx context.Context) error {
	var firstErr error
	if _, err := s.RunOnceRouterDecisions(ctx); err != nil {
		firstErr = err
		s.logger.ErrorContext(ctx, "router_decisions cleanup failed", "error", err.Error())
	}
	if _, err := s.RunOnceStuckLocks(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		s.logger.ErrorContext(ctx, "stuck locks cleanup failed", "error", err.Error())
	}
	if _, err := s.RunOnceStuckIndexing(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		s.logger.ErrorContext(ctx, "stuck indexing cleanup failed", "error", err.Error())
	}
	if _, err := s.RunOnceWorktrees(ctx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
		s.logger.ErrorContext(ctx, "worktrees cleanup failed", "error", err.Error())
	}
	return firstErr
}

// Run — длительный цикл с тикером. Блокирует до ctx.Done(). Подключается как
// goroutine в cmd/api/main.go. На рестарте процесса — просто стартует заново.
func (s *RetentionService) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "retention service started",
		"interval", s.cfg.Interval,
		"router_decisions_age", s.cfg.RouterDecisionsAge,
		"task_events_stuck_age", s.cfg.TaskEventsStuckAge,
		"indexing_stuck_age", s.cfg.IndexingStuckAge,
		"worktrees_released_age", s.cfg.WorktreesReleasedAge,
	)

	// Первый цикл — сразу, чтобы освободить старое после рестарта.
	if err := s.RunOnce(ctx); err != nil {
		s.logger.ErrorContext(ctx, "initial retention cycle failed", "error", err.Error())
	}

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "retention service stopping")
			return nil
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				s.logger.ErrorContext(ctx, "retention cycle failed", "error", err.Error())
			}
		}
	}
}
