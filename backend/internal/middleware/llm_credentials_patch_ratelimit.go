package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	go l.gcLoop()
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

// Handler возвращает gin.HandlerFunc (вызывать после AuthMiddleware).
func (l *LlmCredentialsPatchRateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, ok := GetUserID(c)
		if !ok {
			c.Next()
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
