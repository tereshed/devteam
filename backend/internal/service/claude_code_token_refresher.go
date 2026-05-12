package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/repository"
)

// ClaudeCodeTokenRefresher — фоновый воркер, обновляющий OAuth-токены Claude Code до их истечения (Sprint 15.13).
//
// Алгоритм:
//   - Периодически (раз в Interval) спрашивает у репо подписки с expires_at <= now + RefreshAhead.
//   - Для каждой вызывает ClaudeCodeAuthService.RefreshOne; ошибки логируются и не прерывают цикл.
//
// Запускается в main: refresher.Run(ctx).
type ClaudeCodeTokenRefresher struct {
	repo         repository.ClaudeCodeSubscriptionRepository
	auth         ClaudeCodeAuthService
	logger       *slog.Logger
	interval     time.Duration
	refreshAhead time.Duration
	clock        func() time.Time
}

// NewClaudeCodeTokenRefresher собирает воркер с дефолтами: tick=1m, refreshAhead=10m.
func NewClaudeCodeTokenRefresher(
	repo repository.ClaudeCodeSubscriptionRepository,
	auth ClaudeCodeAuthService,
	logger *slog.Logger,
) *ClaudeCodeTokenRefresher {
	if logger == nil {
		logger = slog.Default()
	}
	return &ClaudeCodeTokenRefresher{
		repo:         repo,
		auth:         auth,
		logger:       logger.With("component", "claude_code_token_refresher"),
		interval:     time.Minute,
		refreshAhead: 10 * time.Minute,
		clock:        time.Now,
	}
}

// WithInterval позволяет поменять период тика (для тестов).
func (r *ClaudeCodeTokenRefresher) WithInterval(d time.Duration) *ClaudeCodeTokenRefresher {
	r.interval = d
	return r
}

// WithRefreshAhead — порог "пора обновлять" (по умолчанию 10m).
func (r *ClaudeCodeTokenRefresher) WithRefreshAhead(d time.Duration) *ClaudeCodeTokenRefresher {
	r.refreshAhead = d
	return r
}

// Run запускает воркер; возвращается, когда ctx отменён.
func (r *ClaudeCodeTokenRefresher) Run(ctx context.Context) {
	r.logger.Info("starting", "interval", r.interval, "refresh_ahead", r.refreshAhead)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		// Первый прогон сразу при старте, чтобы быстро поднять просроченные сразу после рестарта.
		r.tick(ctx)
		select {
		case <-ctx.Done():
			r.logger.Info("stopping")
			return
		case <-ticker.C:
		}
	}
}

// Tick — один шаг (экспонируется для тестов).
func (r *ClaudeCodeTokenRefresher) Tick(ctx context.Context) { r.tick(ctx) }

func (r *ClaudeCodeTokenRefresher) tick(ctx context.Context) {
	subs, err := r.repo.ListExpiring(ctx, r.clock(), r.refreshAhead)
	if err != nil {
		r.logger.Error("list expiring", "err", err)
		return
	}
	for i := range subs {
		sub := &subs[i]
		if err := r.auth.RefreshOne(ctx, sub); err != nil {
			r.logger.Warn("refresh subscription failed",
				"user_id", sub.UserID.String(),
				"err", err)
			continue
		}
		r.logger.Debug("refreshed", "user_id", sub.UserID.String())
	}
}
