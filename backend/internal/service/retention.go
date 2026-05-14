package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

	// Interval — частота прогона полного цикла. Default 1 час.
	Interval time.Duration
}

// DefaultRetentionConfig — дефолты согласно плану.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		RouterDecisionsAge:   30 * 24 * time.Hour,
		WorktreesReleasedAge: 24 * time.Hour,
		Interval:             1 * time.Hour,
	}
}

// RetentionService — координатор фоновых retention-операций.
type RetentionService struct {
	decisionRepo repository.RouterDecisionRepository
	worktreeMgr  *WorktreeManager // опционально; если nil — worktree cleanup пропускается
	logger       *slog.Logger
	cfg          RetentionConfig
}

// NewRetentionService — конструктор. WorktreeManager может быть nil
// (например, для процесса который занимается ТОЛЬКО router_decisions retention).
func NewRetentionService(
	decisionRepo repository.RouterDecisionRepository,
	worktreeMgr *WorktreeManager,
	logger *slog.Logger,
	cfg RetentionConfig,
) *RetentionService {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.RouterDecisionsAge <= 0 {
		cfg.RouterDecisionsAge = 30 * 24 * time.Hour
	}
	if cfg.WorktreesReleasedAge <= 0 {
		cfg.WorktreesReleasedAge = 24 * time.Hour
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 1 * time.Hour
	}
	return &RetentionService{
		decisionRepo: decisionRepo, worktreeMgr: worktreeMgr,
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

// RunOnce — один полный цикл (decision + worktrees). Удобно для CLI и тестов.
// При ошибке одной из операций продолжает вторую и возвращает aggregate error.
func (s *RetentionService) RunOnce(ctx context.Context) error {
	var firstErr error
	if _, err := s.RunOnceRouterDecisions(ctx); err != nil {
		firstErr = err
		s.logger.ErrorContext(ctx, "router_decisions cleanup failed", "error", err.Error())
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
