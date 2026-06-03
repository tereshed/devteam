package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RateLimiterOption — опция для LlmCredentialsPatchRateLimiter.
type RateLimiterOption func(*LlmCredentialsPatchRateLimiter)

// WithPatchRateLimitClock подменяет время (тесты).
func WithPatchRateLimitClock(now func() time.Time) RateLimiterOption {
	return func(l *LlmCredentialsPatchRateLimiter) {
		if now != nil {
			l.now = now
		}
	}
}

// WithPatchRateLimitGCInterval задаёт период фоновой очистки устаревших uid (по умолчанию max(window/4, 1s)).
func WithPatchRateLimitGCInterval(d time.Duration) RateLimiterOption {
	return func(l *LlmCredentialsPatchRateLimiter) {
		if d > 0 {
			l.gcInterval = d
		}
	}
}

// WithPatchRateLimitRedis включает shared-лимит поверх Redis (для multi-instance деплоя).
// Без него лимит считается per-instance, и пользователь может обойти его, разложив запросы
// по репликам (30×N вместо 30). При nil-клиенте опция игнорируется (остаётся in-memory).
func WithPatchRateLimitRedis(client *redis.Client) RateLimiterOption {
	return func(l *LlmCredentialsPatchRateLimiter) {
		if client != nil {
			l.redis = client
		}
	}
}

// LlmCredentialsPatchRateLimiter — лимит PATCH /me/llm-credentials (30 запросов / минуту / user_id из JWT).
type LlmCredentialsPatchRateLimiter struct {
	mu         sync.Mutex
	hits       map[uuid.UUID][]time.Time
	max        int
	window     time.Duration
	now        func() time.Time
	gcInterval time.Duration
	stop       chan struct{}
	closeOnce  sync.Once
	// redis — если задан, лимит считается shared через INCR+EXPIRE (in-memory путь не используется).
	redis *redis.Client
}

// NewLlmCredentialsPatchRateLimiter создаёт лимитер (max событий за window на пользователя).
func NewLlmCredentialsPatchRateLimiter(max int, window time.Duration, opts ...RateLimiterOption) *LlmCredentialsPatchRateLimiter {
	if max <= 0 {
		max = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	l := &LlmCredentialsPatchRateLimiter{
		hits:       make(map[uuid.UUID][]time.Time),
		max:        max,
		window:     window,
		now:        time.Now,
		gcInterval: maxDuration(window/4, time.Second),
		stop:       make(chan struct{}),
	}
	for _, o := range opts {
		o(l)
	}
	// In-memory режим чистит карту фоном; в Redis-режиме истечение делает сам TTL ключа.
	if l.redis == nil {
		go l.gcLoop()
	}
	return l
}

// Close останавливает фоновый GC (для тестов и корректного завершения).
func (l *LlmCredentialsPatchRateLimiter) Close() {
	l.closeOnce.Do(func() {
		close(l.stop)
	})
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func (l *LlmCredentialsPatchRateLimiter) gcLoop() {
	tick := l.gcInterval
	if tick <= 0 {
		tick = time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			l.pruneAllStaleEntries()
		case <-l.stop:
			return
		}
	}
}

// pruneAllStaleEntries удаляет uid, у которых не осталось попаданий в окне (защита от роста карты).
func (l *LlmCredentialsPatchRateLimiter) pruneAllStaleEntries() {
	cutoff := l.now().Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	for uid, slice := range l.hits {
		pruned := filterHitsAfter(slice, cutoff)
		if len(pruned) == 0 {
			delete(l.hits, uid)
		} else {
			l.hits[uid] = pruned
		}
	}
}

func filterHitsAfter(slice []time.Time, cutoff time.Time) []time.Time {
	out := make([]time.Time, 0, len(slice))
	for _, t := range slice {
		if t.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}

// allowRedis — shared fixed-window счётчик: INCR ключа пользователя, на первом инкременте
// вешаем EXPIRE на окно (ключ сам исчезнет → следующее окно). true, если в пределах лимита.
// При ошибке Redis возвращаем true (fail-open).
func (l *LlmCredentialsPatchRateLimiter) allowRedis(c *gin.Context, uid uuid.UUID) bool {
	ctx := c.Request.Context()
	key := fmt.Sprintf("devteam:ratelimit:llmcred:%s", uid.String())
	n, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if n == 1 {
		// Истечение привязано к первому запросу окна; ошибку EXPIRE игнорируем (TTL критичен,
		// но даже без него следующее окно лишь сдвинется — не безопасностная регрессия).
		_ = l.redis.Expire(ctx, key, l.window).Err()
	}
	return int(n) <= l.max
}

// Handler возвращает gin.HandlerFunc (вызывать после AuthMiddleware).
func (l *LlmCredentialsPatchRateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := GetUserID(c)
		if !ok {
			c.Next()
			return
		}

		// Shared-лимит через Redis (multi-instance). Fail-open: при сбое Redis не роняем
		// легитимные запросы — лимитер здесь это hardening, а не критический путь.
		if l.redis != nil {
			if l.allowRedis(c, uid) {
				c.Next()
			} else {
				c.Header("Retry-After", "60")
				apierror.AbortJSON(c, http.StatusTooManyRequests, apierror.ErrTooManyRequests, "Too many requests, try again later")
			}
			return
		}

		now := l.now()
		cutoff := now.Add(-l.window)

		l.mu.Lock()
		slice := l.hits[uid]
		pruned := filterHitsAfter(slice, cutoff)
		if len(pruned) == 0 {
			delete(l.hits, uid)
		}
		if len(pruned) >= l.max {
			l.hits[uid] = pruned
			l.mu.Unlock()
			c.Header("Retry-After", "60")
			apierror.AbortJSON(c, http.StatusTooManyRequests, apierror.ErrTooManyRequests, "Too many requests, try again later")
			return
		}
		pruned = append(pruned, now)
		l.hits[uid] = pruned
		l.mu.Unlock()

		c.Next()
	}
}
