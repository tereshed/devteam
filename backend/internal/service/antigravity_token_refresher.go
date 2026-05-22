package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/repository"
)

// AntigravityTokenRefresher — фоновый воркер, обновляющий OAuth-токены Antigravity до их истечения.
type AntigravityTokenRefresher struct {
	repo         repository.AntigravitySubscriptionRepository
	auth         AntigravityAuthService
	logger       *slog.Logger
	interval     time.Duration
	refreshAhead time.Duration
	clock        func() time.Time
}

// NewAntigravityTokenRefresher собирает воркер с дефолтами: tick=1m, refreshAhead=10m.
func NewAntigravityTokenRefresher(
	repo repository.AntigravitySubscriptionRepository,
	auth AntigravityAuthService,
	logger *slog.Logger,
) *AntigravityTokenRefresher {
	if logger == nil {
		logger = slog.Default()
	}
	return &AntigravityTokenRefresher{
		repo:         repo,
		auth:         auth,
		logger:       logger.With("component", "antigravity_token_refresher"),
		interval:     time.Minute,
		refreshAhead: 10 * time.Minute,
		clock:        time.Now,
	}
}

// WithInterval позволяет поменять период тика (для тестов).
func (r *AntigravityTokenRefresher) WithInterval(d time.Duration) *AntigravityTokenRefresher {
	r.interval = d
	return r
}

// WithRefreshAhead — порог "пора обновлять" (по умолчанию 10m).
func (r *AntigravityTokenRefresher) WithRefreshAhead(d time.Duration) *AntigravityTokenRefresher {
	r.refreshAhead = d
	return r
}

// Run запускает воркер; возвращается, когда ctx отменён.
func (r *AntigravityTokenRefresher) Run(ctx context.Context) {
	r.logger.Info("starting", "interval", r.interval, "refresh_ahead", r.refreshAhead)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		r.tick(ctx)
		select {
		case <-ctx.Done():
			r.logger.Info("stopping")
			return
		case <-ticker.C:
		}
	}
}

// Tick — один шаг.
func (r *AntigravityTokenRefresher) Tick(ctx context.Context) { r.tick(ctx) }

func (r *AntigravityTokenRefresher) tick(ctx context.Context) {
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
